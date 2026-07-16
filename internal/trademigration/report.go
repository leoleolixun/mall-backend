package trademigration

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type Metric struct {
	Name  string
	Count int64
}

type CheckResult struct {
	Name        string
	Description string
	Count       int64
}

type ValidationReport struct {
	GeneratedAt time.Time
	Metrics     []Metric
	Checks      []CheckResult
}

func (r ValidationReport) HasIssues() bool {
	for _, check := range r.Checks {
		if check.Count > 0 {
			return true
		}
	}
	return false
}

func (r ValidationReport) IssueSummary() string {
	parts := make([]string, 0)
	for _, check := range r.Checks {
		if check.Count > 0 {
			parts = append(parts, fmt.Sprintf("%s=%d", check.Name, check.Count))
		}
	}
	return strings.Join(parts, ", ")
}

type countQuery struct {
	name        string
	description string
	query       string
}

var sourceMetrics = []countQuery{
	{name: "orders", query: `SELECT COUNT(*) FROM orders`},
	{name: "payments", query: `SELECT COUNT(*) FROM payments`},
	{name: "refunds", query: `SELECT COUNT(*) FROM refunds`},
	{name: "orders_pending_backfill", query: `SELECT COUNT(*) FROM orders WHERE trade_id IS NULL`},
}

var sourceChecks = []countQuery{
	{
		name:        "orders_missing_merchant",
		description: "订单引用的商户不存在",
		query: `SELECT COUNT(*)
			FROM orders o
			LEFT JOIN merchants m ON m.id = o.merchant_id
			WHERE m.id IS NULL`,
	},
	{
		name:        "payments_missing_order",
		description: "支付单引用的订单不存在",
		query: `SELECT COUNT(*)
			FROM payments p
			LEFT JOIN orders o ON o.id = p.order_id
			WHERE o.id IS NULL`,
	},
	{
		name:        "payments_order_mismatch",
		description: "支付单的订单号、用户、商户或金额与订单快照不一致",
		query: `SELECT COUNT(*)
			FROM payments p
			JOIN orders o ON o.id = p.order_id
			WHERE p.order_no <> o.order_no
			   OR p.user_id <> o.user_id
			   OR p.merchant_id <> o.merchant_id
			   OR p.amount <> o.payable_amount`,
	},
	{
		name:        "payments_invalid_active_order",
		description: "有效待支付键不是当前待支付订单",
		query: `SELECT COUNT(*)
			FROM payments p
			WHERE p.active_order_id IS NOT NULL
			  AND (p.active_order_id <> p.order_id OR p.status <> 1)`,
	},
	{
		name:        "refunds_missing_reference",
		description: "退款引用的支付单或订单不存在",
		query: `SELECT COUNT(*)
			FROM refunds r
			LEFT JOIN payments p ON p.id = r.payment_id
			LEFT JOIN orders o ON o.id = r.order_id
			WHERE p.id IS NULL OR o.id IS NULL`,
	},
	{
		name:        "refunds_reference_mismatch",
		description: "退款的订单、用户或商户与原支付不一致",
		query: `SELECT COUNT(*)
			FROM refunds r
			JOIN payments p ON p.id = r.payment_id
			JOIN orders o ON o.id = r.order_id
			WHERE p.order_id <> r.order_id
			   OR p.user_id <> r.user_id
			   OR p.merchant_id <> r.merchant_id
			   OR o.user_id <> r.user_id
			   OR o.merchant_id <> r.merchant_id`,
	},
	{
		name:        "successful_refunds_exceed_payment",
		description: "支付单成功退款累计金额超过支付金额",
		query: `SELECT COUNT(*) FROM (
			SELECT p.id
			FROM payments p
			JOIN refunds r ON r.payment_id = p.id AND r.status = 2
			GROUP BY p.id, p.amount
			HAVING SUM(r.amount) > p.amount
		) invalid_refunds`,
	},
	{
		name:        "unbackfilled_orders_with_new_links",
		description: "尚未回填的订单已存在交易、支付或退款兼容链接",
		query: `SELECT COUNT(DISTINCT o.id)
			FROM orders o
			LEFT JOIN payments p ON p.order_id = o.id
			LEFT JOIN refunds r ON r.order_id = o.id
			WHERE o.trade_id IS NULL
			  AND (p.trade_id IS NOT NULL OR p.active_trade_id IS NOT NULL OR r.trade_id IS NOT NULL OR r.payment_allocation_id IS NOT NULL)`,
	},
}

var consistencyMetrics = []countQuery{
	{name: "trades", query: `SELECT COUNT(*) FROM trades`},
	{name: "orders", query: `SELECT COUNT(*) FROM orders`},
	{name: "payments", query: `SELECT COUNT(*) FROM payments`},
	{name: "payment_allocations", query: `SELECT COUNT(*) FROM payment_allocations`},
	{name: "refunds", query: `SELECT COUNT(*) FROM refunds`},
}

var consistencyChecks = []countQuery{
	{
		name:        "orders_missing_trade",
		description: "订单尚未关联交易单",
		query:       `SELECT COUNT(*) FROM orders WHERE trade_id IS NULL`,
	},
	{
		name:        "orders_missing_trade_record",
		description: "订单关联的交易单不存在",
		query: `SELECT COUNT(*)
			FROM orders o
			LEFT JOIN trades t ON t.id = o.trade_id
			WHERE o.trade_id IS NOT NULL AND t.id IS NULL`,
	},
	{
		name:        "orders_missing_merchant_snapshot",
		description: "订单缺少商户、佣金或结算快照",
		query: `SELECT COUNT(*)
			FROM orders
			WHERE trade_id IS NOT NULL
			  AND (merchant_name IS NULL OR commission_rate_bps IS NULL OR commission_amount IS NULL OR settlement_amount IS NULL)`,
	},
	{
		name:        "trade_order_user_mismatch",
		description: "交易单与子订单用户不一致",
		query: `SELECT COUNT(*)
			FROM orders o
			JOIN trades t ON t.id = o.trade_id
			WHERE o.user_id <> t.user_id`,
	},
	{
		name:        "duplicate_trade_merchant_orders",
		description: "同一交易内同一商户存在多张子订单",
		query: `SELECT COUNT(*) FROM (
			SELECT trade_id, merchant_id
			FROM orders
			WHERE trade_id IS NOT NULL
			GROUP BY trade_id, merchant_id
			HAVING COUNT(*) > 1
		) duplicates`,
	},
	{
		name:        "trade_amount_mismatch",
		description: "交易金额不等于全部子订单金额之和，或交易没有子订单",
		query: `SELECT COUNT(*) FROM (
			SELECT t.id
			FROM trades t
			LEFT JOIN orders o ON o.trade_id = t.id
			GROUP BY t.id, t.goods_amount, t.freight_amount, t.discount_amount, t.payable_amount
			HAVING COUNT(o.id) = 0
			   OR t.goods_amount <> COALESCE(SUM(o.goods_amount), 0)
			   OR t.freight_amount <> COALESCE(SUM(o.freight_amount), 0)
			   OR t.discount_amount <> COALESCE(SUM(o.discount_amount), 0)
			   OR t.payable_amount <> COALESCE(SUM(o.payable_amount), 0)
		) invalid_trades`,
	},
	{
		name:        "payments_missing_trade",
		description: "支付单尚未关联交易单",
		query:       `SELECT COUNT(*) FROM payments WHERE trade_id IS NULL`,
	},
	{
		name:        "payments_trade_reference_mismatch",
		description: "支付单、旧订单兼容字段和交易单不一致",
		query: `SELECT COUNT(*)
			FROM payments p
			LEFT JOIN trades t ON t.id = p.trade_id
			LEFT JOIN orders o ON o.id = p.order_id
			WHERE p.trade_id IS NOT NULL
			  AND (t.id IS NULL OR p.user_id <> t.user_id
			    OR (p.order_id IS NOT NULL AND (o.id IS NULL OR o.trade_id <> p.trade_id)))`,
	},
	{
		name:        "payments_invalid_active_trade",
		description: "有效待支付交易键与支付状态或交易引用不一致",
		query: `SELECT COUNT(*)
			FROM payments
			WHERE (active_trade_id IS NOT NULL AND (status <> 1 OR active_trade_id <> trade_id))
			   OR (status = 1 AND trade_id IS NOT NULL AND active_trade_id IS NULL)
			   OR (status <> 1 AND (active_trade_id IS NOT NULL OR active_order_id IS NOT NULL))`,
	},
	{
		name:        "payments_missing_allocation",
		description: "支付单没有金额分配记录",
		query: `SELECT COUNT(*)
			FROM payments p
			LEFT JOIN payment_allocations pa ON pa.payment_id = p.id
			WHERE pa.id IS NULL`,
	},
	{
		name:        "payment_allocation_sum_mismatch",
		description: "支付分配总额不等于支付金额",
		query: `SELECT COUNT(*) FROM (
			SELECT p.id
			FROM payments p
			LEFT JOIN payment_allocations pa ON pa.payment_id = p.id
			GROUP BY p.id, p.amount
			HAVING COALESCE(SUM(pa.amount), 0) <> p.amount
		) invalid_payments`,
	},
	{
		name:        "payment_allocation_reference_mismatch",
		description: "支付分配与支付单、交易单、订单或商户引用不一致",
		query: `SELECT COUNT(*)
			FROM payment_allocations pa
			LEFT JOIN payments p ON p.id = pa.payment_id
			LEFT JOIN trades t ON t.id = pa.trade_id
			LEFT JOIN orders o ON o.id = pa.order_id
			WHERE p.id IS NULL OR t.id IS NULL OR o.id IS NULL
			   OR p.trade_id <> pa.trade_id
			   OR o.trade_id <> pa.trade_id
			   OR o.merchant_id <> pa.merchant_id
			   OR pa.amount <> o.payable_amount`,
	},
	{
		name:        "refunds_missing_trade_or_allocation",
		description: "退款没有关联交易单或支付分配",
		query: `SELECT COUNT(*)
			FROM refunds
			WHERE trade_id IS NULL OR payment_allocation_id IS NULL`,
	},
	{
		name:        "refunds_allocation_reference_mismatch",
		description: "退款与支付分配、支付单、订单或商户引用不一致",
		query: `SELECT COUNT(*)
			FROM refunds r
			LEFT JOIN payment_allocations pa ON pa.id = r.payment_allocation_id
			WHERE pa.id IS NULL
			   OR pa.payment_id <> r.payment_id
			   OR pa.trade_id <> r.trade_id
			   OR pa.order_id <> r.order_id
			   OR pa.merchant_id <> r.merchant_id`,
	},
	{
		name:        "allocation_refunded_amount_mismatch",
		description: "支付分配累计退款金额与成功退款流水不一致或超过分配金额",
		query: `SELECT COUNT(*) FROM (
			SELECT pa.id
			FROM payment_allocations pa
			LEFT JOIN refunds r ON r.payment_allocation_id = pa.id AND r.status = 2
			GROUP BY pa.id, pa.amount, pa.refunded_amount
			HAVING pa.refunded_amount <> COALESCE(SUM(r.amount), 0)
			   OR pa.refunded_amount > pa.amount
		) invalid_allocations`,
	},
}

func runCountQueries(ctx context.Context, db queryer, metrics []countQuery, checks []countQuery) (ValidationReport, error) {
	report := ValidationReport{GeneratedAt: time.Now().UTC()}
	for _, item := range metrics {
		count, err := readCount(ctx, db, item.query)
		if err != nil {
			return ValidationReport{}, fmt.Errorf("读取指标 %s: %w", item.name, err)
		}
		report.Metrics = append(report.Metrics, Metric{Name: item.name, Count: count})
	}
	for _, item := range checks {
		count, err := readCount(ctx, db, item.query)
		if err != nil {
			return ValidationReport{}, fmt.Errorf("执行一致性检查 %s: %w", item.name, err)
		}
		report.Checks = append(report.Checks, CheckResult{Name: item.name, Description: item.description, Count: count})
	}
	return report, nil
}

type queryer interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func readCount(ctx context.Context, db queryer, query string) (int64, error) {
	var count int64
	if err := db.QueryRowContext(ctx, query).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}
