package trademigration

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

const (
	tradeStatusPendingPayment    = 1
	tradeStatusPaid              = 2
	tradeStatusClosed            = 3
	tradeStatusPartiallyRefunded = 4
	tradeStatusRefunded          = 5

	orderStatusPendingPayment = 1
	orderStatusCancelled      = 5

	paymentStatusPending  = 1
	refundStatusSucceeded = 2
	backfillLockName      = "go_mall_m8_trade_backfill"
)

type Service struct {
	db          *sql.DB
	lockTimeout time.Duration
}

type BackfillResult struct {
	Orders      int64
	Trades      int64
	Payments    int64
	Allocations int64
	Refunds     int64
}

func (r *BackfillResult) add(other BackfillResult) {
	r.Orders += other.Orders
	r.Trades += other.Trades
	r.Payments += other.Payments
	r.Allocations += other.Allocations
	r.Refunds += other.Refunds
}

func NewService(db *sql.DB) (*Service, error) {
	if db == nil {
		return nil, fmt.Errorf("数据库连接不能为空")
	}
	return &Service{db: db, lockTimeout: 30 * time.Second}, nil
}

func (s *Service) ValidateSource(ctx context.Context) (ValidationReport, error) {
	return runCountQueries(ctx, s.db, sourceMetrics, sourceChecks)
}

func (s *Service) ValidateConsistency(ctx context.Context) (ValidationReport, error) {
	return runCountQueries(ctx, s.db, consistencyMetrics, consistencyChecks)
}

func (s *Service) Backfill(ctx context.Context, batchSize int) (BackfillResult, error) {
	if batchSize <= 0 || batchSize > 1000 {
		return BackfillResult{}, fmt.Errorf("batch size 必须在 1 到 1000 之间")
	}
	sourceReport, err := s.ValidateSource(ctx)
	if err != nil {
		return BackfillResult{}, err
	}
	if sourceReport.HasIssues() {
		return BackfillResult{}, fmt.Errorf("历史数据预检失败: %s", sourceReport.IssueSummary())
	}

	conn, err := s.db.Conn(ctx)
	if err != nil {
		return BackfillResult{}, fmt.Errorf("获取回填专用连接: %w", err)
	}
	defer conn.Close()
	if err := acquireLock(ctx, conn, backfillLockName, s.lockTimeout); err != nil {
		return BackfillResult{}, err
	}
	defer releaseLock(conn, backfillLockName)

	var total BackfillResult
	for {
		batch, err := backfillBatch(ctx, conn, batchSize)
		if err != nil {
			return total, err
		}
		total.add(batch)
		if batch.Orders == 0 {
			break
		}
	}
	return total, nil
}

type legacyOrder struct {
	ID             int64
	OrderNo        string
	UserID         int64
	MerchantID     int64
	MerchantName   string
	Status         int
	GoodsAmount    int64
	FreightAmount  int64
	DiscountAmount int64
	PayableAmount  int64
	PaidAt         sql.NullTime
	CancelledAt    sql.NullTime
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type legacyPayment struct {
	ID            int64
	Amount        int64
	Status        int
	ActiveOrderID sql.NullInt64
}

func backfillBatch(ctx context.Context, conn *sql.Conn, batchSize int) (result BackfillResult, returnErr error) {
	tx, err := conn.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return result, fmt.Errorf("开始回填事务: %w", err)
	}
	defer func() {
		if returnErr != nil {
			_ = tx.Rollback()
		}
	}()

	orders, err := selectLegacyOrders(ctx, tx, batchSize)
	if err != nil {
		return result, err
	}
	if len(orders) == 0 {
		if err := tx.Commit(); err != nil {
			return result, fmt.Errorf("提交空回填事务: %w", err)
		}
		return result, nil
	}

	for _, order := range orders {
		orderResult, err := backfillOrder(ctx, tx, order)
		if err != nil {
			return result, fmt.Errorf("回填订单 %d(%s): %w", order.ID, order.OrderNo, err)
		}
		result.add(orderResult)
	}
	if err := tx.Commit(); err != nil {
		return result, fmt.Errorf("提交回填事务: %w", err)
	}
	return result, nil
}

func selectLegacyOrders(ctx context.Context, tx *sql.Tx, batchSize int) ([]legacyOrder, error) {
	rows, err := tx.QueryContext(ctx, `SELECT
		o.id, o.order_no, o.user_id, o.merchant_id, m.name, o.status,
		o.goods_amount, o.freight_amount, o.discount_amount, o.payable_amount,
		o.paid_at, o.cancelled_at, o.created_at, o.updated_at
		FROM orders o
		JOIN merchants m ON m.id = o.merchant_id
		WHERE o.trade_id IS NULL
		ORDER BY o.id ASC
		LIMIT ?
		FOR UPDATE`, batchSize)
	if err != nil {
		return nil, fmt.Errorf("锁定待回填订单: %w", err)
	}
	defer rows.Close()

	orders := make([]legacyOrder, 0, batchSize)
	for rows.Next() {
		var order legacyOrder
		if err := rows.Scan(
			&order.ID,
			&order.OrderNo,
			&order.UserID,
			&order.MerchantID,
			&order.MerchantName,
			&order.Status,
			&order.GoodsAmount,
			&order.FreightAmount,
			&order.DiscountAmount,
			&order.PayableAmount,
			&order.PaidAt,
			&order.CancelledAt,
			&order.CreatedAt,
			&order.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("解析待回填订单: %w", err)
		}
		orders = append(orders, order)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历待回填订单: %w", err)
	}
	return orders, nil
}

func backfillOrder(ctx context.Context, tx *sql.Tx, order legacyOrder) (BackfillResult, error) {
	refundedAmount, err := successfulRefundAmountForOrder(ctx, tx, order.ID)
	if err != nil {
		return BackfillResult{}, err
	}
	tradeStatus := historicalTradeStatus(order.Status, order.PayableAmount, refundedAmount)
	tradeNo := fmt.Sprintf("LEGACY%020d", order.ID)
	idempotencyKey := fmt.Sprintf("legacy-order:%d", order.ID)
	closedAt := any(nil)
	if order.CancelledAt.Valid {
		closedAt = order.CancelledAt.Time
	}
	paidAt := any(nil)
	if order.PaidAt.Valid {
		paidAt = order.PaidAt.Time
	}
	insertResult, err := tx.ExecContext(ctx, `INSERT INTO trades (
		trade_no, user_id, status, goods_amount, freight_amount, discount_amount,
		payable_amount, idempotency_key, paid_at, closed_at, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON DUPLICATE KEY UPDATE id = LAST_INSERT_ID(id)`,
		tradeNo,
		order.UserID,
		tradeStatus,
		order.GoodsAmount,
		order.FreightAmount,
		order.DiscountAmount,
		order.PayableAmount,
		idempotencyKey,
		paidAt,
		closedAt,
		order.CreatedAt,
		order.UpdatedAt,
	)
	if err != nil {
		return BackfillResult{}, fmt.Errorf("创建历史交易单: %w", err)
	}
	tradeID, err := insertResult.LastInsertId()
	if err != nil || tradeID <= 0 {
		return BackfillResult{}, fmt.Errorf("读取历史交易单 ID: %w", err)
	}

	updateResult, err := tx.ExecContext(ctx, `UPDATE orders
		SET trade_id = ?, merchant_name = ?, commission_rate_bps = 0,
			commission_amount = 0, settlement_amount = payable_amount
		WHERE id = ? AND trade_id IS NULL`, tradeID, order.MerchantName, order.ID)
	if err != nil {
		return BackfillResult{}, fmt.Errorf("关联订单交易单: %w", err)
	}
	if err := requireOneAffected(updateResult, "关联订单交易单"); err != nil {
		return BackfillResult{}, err
	}

	result := BackfillResult{Orders: 1, Trades: 1}
	payments, err := selectOrderPayments(ctx, tx, order.ID)
	if err != nil {
		return BackfillResult{}, err
	}
	for _, payment := range payments {
		refundedAmount, err := successfulRefundAmountForPayment(ctx, tx, payment.ID, order.ID)
		if err != nil {
			return BackfillResult{}, err
		}
		allocationResult, err := tx.ExecContext(ctx, `INSERT INTO payment_allocations (
			payment_id, trade_id, order_id, merchant_id, amount, refunded_amount, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, UTC_TIMESTAMP(3), UTC_TIMESTAMP(3))
		ON DUPLICATE KEY UPDATE id = LAST_INSERT_ID(id)`,
			payment.ID,
			tradeID,
			order.ID,
			order.MerchantID,
			payment.Amount,
			refundedAmount,
		)
		if err != nil {
			return BackfillResult{}, fmt.Errorf("创建支付分配 %d: %w", payment.ID, err)
		}
		allocationID, err := allocationResult.LastInsertId()
		if err != nil || allocationID <= 0 {
			return BackfillResult{}, fmt.Errorf("读取支付分配 ID: %w", err)
		}

		var activeTradeID any
		if payment.Status == paymentStatusPending && payment.ActiveOrderID.Valid {
			activeTradeID = tradeID
		}
		paymentUpdate, err := tx.ExecContext(ctx, `UPDATE payments
			SET trade_id = ?, active_trade_id = ?
			WHERE id = ? AND (trade_id IS NULL OR trade_id = ?)`, tradeID, activeTradeID, payment.ID, tradeID)
		if err != nil {
			return BackfillResult{}, fmt.Errorf("关联支付单 %d: %w", payment.ID, err)
		}
		if err := requireOneAffected(paymentUpdate, "关联支付单"); err != nil {
			return BackfillResult{}, err
		}

		refundUpdate, err := tx.ExecContext(ctx, `UPDATE refunds
			SET trade_id = ?, payment_allocation_id = ?
			WHERE payment_id = ? AND order_id = ?
			  AND (trade_id IS NULL OR trade_id = ?)
			  AND (payment_allocation_id IS NULL OR payment_allocation_id = ?)`,
			tradeID,
			allocationID,
			payment.ID,
			order.ID,
			tradeID,
			allocationID,
		)
		if err != nil {
			return BackfillResult{}, fmt.Errorf("关联退款支付分配 %d: %w", payment.ID, err)
		}
		refundCount, err := refundUpdate.RowsAffected()
		if err != nil {
			return BackfillResult{}, fmt.Errorf("读取退款回填数量: %w", err)
		}
		result.Payments++
		result.Allocations++
		result.Refunds += refundCount
	}
	return result, nil
}

func selectOrderPayments(ctx context.Context, tx *sql.Tx, orderID int64) ([]legacyPayment, error) {
	rows, err := tx.QueryContext(ctx, `SELECT id, amount, status, active_order_id
		FROM payments
		WHERE order_id = ?
		ORDER BY id ASC
		FOR UPDATE`, orderID)
	if err != nil {
		return nil, fmt.Errorf("锁定订单支付单: %w", err)
	}
	defer rows.Close()
	payments := make([]legacyPayment, 0)
	for rows.Next() {
		var payment legacyPayment
		if err := rows.Scan(&payment.ID, &payment.Amount, &payment.Status, &payment.ActiveOrderID); err != nil {
			return nil, fmt.Errorf("解析订单支付单: %w", err)
		}
		payments = append(payments, payment)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历订单支付单: %w", err)
	}
	return payments, nil
}

func successfulRefundAmountForOrder(ctx context.Context, tx *sql.Tx, orderID int64) (int64, error) {
	var amount sql.NullInt64
	err := tx.QueryRowContext(ctx, `SELECT SUM(amount) FROM refunds WHERE order_id = ? AND status = ?`, orderID, refundStatusSucceeded).Scan(&amount)
	if err != nil {
		return 0, fmt.Errorf("统计订单成功退款金额: %w", err)
	}
	return amount.Int64, nil
}

func successfulRefundAmountForPayment(ctx context.Context, tx *sql.Tx, paymentID, orderID int64) (int64, error) {
	var amount sql.NullInt64
	err := tx.QueryRowContext(ctx, `SELECT SUM(amount) FROM refunds WHERE payment_id = ? AND order_id = ? AND status = ?`, paymentID, orderID, refundStatusSucceeded).Scan(&amount)
	if err != nil {
		return 0, fmt.Errorf("统计支付单成功退款金额: %w", err)
	}
	return amount.Int64, nil
}

func historicalTradeStatus(orderStatus int, payableAmount, refundedAmount int64) int {
	if refundedAmount > 0 && refundedAmount >= payableAmount {
		return tradeStatusRefunded
	}
	if refundedAmount > 0 {
		return tradeStatusPartiallyRefunded
	}
	if orderStatus == orderStatusCancelled {
		return tradeStatusClosed
	}
	if orderStatus == orderStatusPendingPayment {
		return tradeStatusPendingPayment
	}
	return tradeStatusPaid
}

func requireOneAffected(result sql.Result, operation string) error {
	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("读取%s影响行数: %w", operation, err)
	}
	if count != 1 {
		return fmt.Errorf("%s影响 %d 行，预期 1 行", operation, count)
	}
	return nil
}

func acquireLock(ctx context.Context, conn *sql.Conn, name string, timeout time.Duration) error {
	var acquired sql.NullInt64
	if err := conn.QueryRowContext(ctx, `SELECT GET_LOCK(?, ?)`, name, int(timeout.Seconds())).Scan(&acquired); err != nil {
		return fmt.Errorf("获取历史回填锁: %w", err)
	}
	if !acquired.Valid || acquired.Int64 != 1 {
		return fmt.Errorf("%s 内未获得历史回填锁", timeout)
	}
	return nil
}

func releaseLock(conn *sql.Conn, name string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var released sql.NullInt64
	_ = conn.QueryRowContext(ctx, `SELECT RELEASE_LOCK(?)`, name).Scan(&released)
}
