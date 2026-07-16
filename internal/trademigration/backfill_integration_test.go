package trademigration

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"go-mall/internal/migration"
	migrationfiles "go-mall/migrations"
)

const backfillIntegrationDatabase = "go_mall_migration_test"

func TestHistoricalTradeBackfillOnMySQL(t *testing.T) {
	if os.Getenv("MALL_ALLOW_MIGRATION_INTEGRATION") != "1" {
		t.Skip("set MALL_ALLOW_MIGRATION_INTEGRATION=1 to run destructive backfill integration test")
	}
	dsn := os.Getenv("MALL_MIGRATION_TEST_DSN")
	if dsn == "" {
		t.Fatal("MALL_MIGRATION_TEST_DSN is required")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("open MySQL: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("ping MySQL: %v", err)
	}
	var databaseName string
	if err := db.QueryRowContext(ctx, `SELECT DATABASE()`).Scan(&databaseName); err != nil {
		t.Fatalf("read database name: %v", err)
	}
	if databaseName != backfillIntegrationDatabase {
		t.Fatalf("refusing destructive integration test on database %q; expected %q", databaseName, backfillIntegrationDatabase)
	}

	resetBackfillSchema(t, ctx, db)
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		resetBackfillSchema(t, cleanupCtx, db)
	})
	createBackfillLegacySchema(t, ctx, db)
	seedBackfillHistory(t, ctx, db)

	items, err := migration.LoadFiles(migrationfiles.Files)
	if err != nil {
		t.Fatalf("load migrations: %v", err)
	}
	runner, err := migration.NewRunner(db, items)
	if err != nil {
		t.Fatalf("new migration runner: %v", err)
	}
	if _, err := runner.Up(ctx); err != nil {
		t.Fatalf("apply M8 schema: %v", err)
	}

	service, err := NewService(db)
	if err != nil {
		t.Fatalf("new backfill service: %v", err)
	}
	sourceReport, err := service.ValidateSource(ctx)
	if err != nil {
		t.Fatalf("validate source: %v", err)
	}
	if sourceReport.HasIssues() {
		t.Fatalf("unexpected source issues: %s", sourceReport.IssueSummary())
	}

	result, err := service.Backfill(ctx, 2)
	if err != nil {
		t.Fatalf("backfill history: %v", err)
	}
	if result.Orders != 3 || result.Trades != 3 || result.Payments != 2 || result.Allocations != 2 || result.Refunds != 1 {
		t.Fatalf("unexpected backfill result: %+v", result)
	}
	assertBackfilledState(t, ctx, db)

	report, err := service.ValidateConsistency(ctx)
	if err != nil {
		t.Fatalf("validate backfill: %v", err)
	}
	if report.HasIssues() {
		t.Fatalf("unexpected consistency issues: %s", report.IssueSummary())
	}

	secondRun, err := service.Backfill(ctx, 2)
	if err != nil {
		t.Fatalf("repeat backfill: %v", err)
	}
	if secondRun != (BackfillResult{}) {
		t.Fatalf("repeat backfill was not idempotent: %+v", secondRun)
	}

	if _, err := db.ExecContext(ctx, `UPDATE payment_allocations SET amount = amount + 1 WHERE payment_id = 201`); err != nil {
		t.Fatalf("inject allocation mismatch: %v", err)
	}
	brokenReport, err := service.ValidateConsistency(ctx)
	if err != nil {
		t.Fatalf("validate injected mismatch: %v", err)
	}
	if !checkFailed(brokenReport, "payment_allocation_sum_mismatch") || !checkFailed(brokenReport, "payment_allocation_reference_mismatch") {
		t.Fatalf("validator did not detect injected amount mismatch: %+v", brokenReport.Checks)
	}
	if _, err := db.ExecContext(ctx, `UPDATE payment_allocations SET amount = amount - 1 WHERE payment_id = 201`); err != nil {
		t.Fatalf("restore allocation amount: %v", err)
	}

	if _, err := runner.Down(ctx, len(items)); err != nil {
		t.Fatalf("rollback M8 schema with historical data: %v", err)
	}
	var legacyOrders int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM orders`).Scan(&legacyOrders); err != nil {
		t.Fatalf("count legacy orders after rollback: %v", err)
	}
	if legacyOrders != 3 {
		t.Fatalf("legacy order rows changed during rollback: %d", legacyOrders)
	}
}

func resetBackfillSchema(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	tables := []string{
		"settlement_entries",
		"merchant_settlements",
		"payment_allocations",
		"trades",
		"schema_migrations",
		"refunds",
		"payments",
		"orders",
		"product_skus",
		"products",
		"categories",
		"merchants",
	}
	for _, table := range tables {
		if _, err := db.ExecContext(ctx, "DROP TABLE IF EXISTS `"+table+"`"); err != nil {
			t.Fatalf("drop table %s: %v", table, err)
		}
	}
}

func createBackfillLegacySchema(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	statements := []string{
		`CREATE TABLE merchants (
			id BIGINT NOT NULL AUTO_INCREMENT,
			name VARCHAR(100) NOT NULL,
			PRIMARY KEY (id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE orders (
			id BIGINT NOT NULL AUTO_INCREMENT,
			order_no VARCHAR(64) NOT NULL,
			user_id BIGINT NOT NULL,
			merchant_id BIGINT NOT NULL,
			status TINYINT NOT NULL DEFAULT 1,
			receiver_name VARCHAR(100) NOT NULL DEFAULT '',
			receiver_phone VARCHAR(20) NOT NULL DEFAULT '',
			receiver_address VARCHAR(500) NOT NULL DEFAULT '',
			goods_amount BIGINT NOT NULL DEFAULT 0,
			freight_amount BIGINT NOT NULL DEFAULT 0,
			discount_amount BIGINT NOT NULL DEFAULT 0,
			payable_amount BIGINT NOT NULL DEFAULT 0,
			user_coupon_id BIGINT NOT NULL DEFAULT 0,
			remark VARCHAR(255) NOT NULL DEFAULT '',
			paid_at DATETIME(3) NULL,
			cancelled_at DATETIME(3) NULL,
			completed_at DATETIME(3) NULL,
			created_at DATETIME(3) NOT NULL,
			updated_at DATETIME(3) NOT NULL,
			deleted_at DATETIME(3) NULL,
			PRIMARY KEY (id),
			UNIQUE KEY uk_orders_order_no (order_no)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE payments (
			id BIGINT NOT NULL AUTO_INCREMENT,
			payment_no VARCHAR(64) NOT NULL,
			order_id BIGINT NOT NULL,
			active_order_id BIGINT NULL,
			order_no VARCHAR(64) NOT NULL,
			user_id BIGINT NOT NULL,
			merchant_id BIGINT NOT NULL,
			pay_channel VARCHAR(32) NOT NULL,
			pay_scene VARCHAR(32) NOT NULL DEFAULT '',
			status TINYINT NOT NULL DEFAULT 1,
			amount BIGINT NOT NULL DEFAULT 0,
			transaction_id VARCHAR(128) NOT NULL DEFAULT '',
			failure_reason VARCHAR(255) NOT NULL DEFAULT '',
			paid_at DATETIME(3) NULL,
			closed_at DATETIME(3) NULL,
			created_at DATETIME(3) NOT NULL,
			updated_at DATETIME(3) NOT NULL,
			deleted_at DATETIME(3) NULL,
			PRIMARY KEY (id),
			UNIQUE KEY uk_payments_payment_no (payment_no),
			UNIQUE KEY uk_payments_active_order (active_order_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE refunds (
			id BIGINT NOT NULL AUTO_INCREMENT,
			refund_no VARCHAR(64) NOT NULL,
			after_sale_id BIGINT NOT NULL,
			payment_id BIGINT NOT NULL,
			order_id BIGINT NOT NULL,
			user_id BIGINT NOT NULL,
			merchant_id BIGINT NOT NULL,
			pay_channel VARCHAR(32) NOT NULL,
			amount BIGINT NOT NULL,
			status TINYINT NOT NULL DEFAULT 1,
			transaction_id VARCHAR(128) NOT NULL DEFAULT '',
			failure_reason VARCHAR(255) NOT NULL DEFAULT '',
			last_error VARCHAR(255) NOT NULL DEFAULT '',
			retry_count INT NOT NULL DEFAULT 0,
			last_attempt_at DATETIME(3) NULL,
			next_retry_at DATETIME(3) NULL,
			refunded_at DATETIME(3) NULL,
			created_at DATETIME(3) NOT NULL,
			updated_at DATETIME(3) NOT NULL,
			PRIMARY KEY (id),
			UNIQUE KEY uk_refunds_refund_no (refund_no)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE products (
			id BIGINT NOT NULL AUTO_INCREMENT,
			merchant_id BIGINT NOT NULL,
			status TINYINT NOT NULL DEFAULT 1,
			deleted_at DATETIME(3) NULL,
			PRIMARY KEY (id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE product_skus (
			id BIGINT NOT NULL AUTO_INCREMENT,
			merchant_id BIGINT NOT NULL,
			product_id BIGINT NOT NULL,
			status TINYINT NOT NULL DEFAULT 1,
			deleted_at DATETIME(3) NULL,
			PRIMARY KEY (id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE categories (
			id BIGINT NOT NULL AUTO_INCREMENT,
			merchant_id BIGINT NOT NULL,
			status TINYINT NOT NULL DEFAULT 1,
			deleted_at DATETIME(3) NULL,
			sort INT NOT NULL DEFAULT 0,
			PRIMARY KEY (id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	}
	for index, statement := range statements {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			t.Fatalf("create legacy schema statement %d: %v", index+1, err)
		}
	}
}

func seedBackfillHistory(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	statements := []string{
		`INSERT INTO merchants (id, name) VALUES (1, '历史商户一'), (2, '历史商户二')`,
		`INSERT INTO orders (
			id, order_no, user_id, merchant_id, status,
			goods_amount, freight_amount, discount_amount, payable_amount,
			paid_at, cancelled_at, created_at, updated_at
		) VALUES
			(101, 'ORDER-101', 1001, 1, 2, 10000, 500, 500, 10000, '2026-07-01 10:05:00.000', NULL, '2026-07-01 10:00:00.000', '2026-07-01 10:05:00.000'),
			(102, 'ORDER-102', 1001, 2, 1, 5000, 0, 0, 5000, NULL, NULL, '2026-07-02 10:00:00.000', '2026-07-02 10:00:00.000'),
			(103, 'ORDER-103', 1002, 1, 5, 2000, 0, 0, 2000, NULL, '2026-07-03 10:10:00.000', '2026-07-03 10:00:00.000', '2026-07-03 10:10:00.000')`,
		`INSERT INTO payments (
			id, payment_no, order_id, active_order_id, order_no, user_id, merchant_id,
			pay_channel, pay_scene, status, amount, paid_at, created_at, updated_at
		) VALUES
			(201, 'PAY-201', 101, NULL, 'ORDER-101', 1001, 1, 'alipay', 'page', 6, 10000, '2026-07-01 10:05:00.000', '2026-07-01 10:03:00.000', '2026-07-04 10:00:00.000'),
			(202, 'PAY-202', 102, 102, 'ORDER-102', 1001, 2, 'alipay', 'page', 1, 5000, NULL, '2026-07-02 10:01:00.000', '2026-07-02 10:01:00.000')`,
		`INSERT INTO refunds (
			id, refund_no, after_sale_id, payment_id, order_id, user_id, merchant_id,
			pay_channel, amount, status, refunded_at, created_at, updated_at
		) VALUES
			(301, 'REFUND-301', 401, 201, 101, 1001, 1, 'alipay', 3000, 2, '2026-07-04 10:00:00.000', '2026-07-04 09:59:00.000', '2026-07-04 10:00:00.000')`,
	}
	for index, statement := range statements {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			t.Fatalf("seed history statement %d: %v", index+1, err)
		}
	}
}

func assertBackfilledState(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	var linkedOrders, trades, allocations, linkedRefunds int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM orders WHERE trade_id IS NOT NULL AND merchant_name IS NOT NULL AND commission_rate_bps = 0 AND commission_amount = 0 AND settlement_amount = payable_amount`).Scan(&linkedOrders); err != nil {
		t.Fatalf("count linked orders: %v", err)
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM trades`).Scan(&trades); err != nil {
		t.Fatalf("count trades: %v", err)
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM payment_allocations`).Scan(&allocations); err != nil {
		t.Fatalf("count allocations: %v", err)
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM refunds WHERE trade_id IS NOT NULL AND payment_allocation_id IS NOT NULL`).Scan(&linkedRefunds); err != nil {
		t.Fatalf("count linked refunds: %v", err)
	}
	if linkedOrders != 3 || trades != 3 || allocations != 2 || linkedRefunds != 1 {
		t.Fatalf("unexpected linked counts: orders=%d trades=%d allocations=%d refunds=%d", linkedOrders, trades, allocations, linkedRefunds)
	}

	statuses := make(map[int64]int)
	rows, err := db.QueryContext(ctx, `SELECT o.id, t.status FROM orders o JOIN trades t ON t.id = o.trade_id ORDER BY o.id`)
	if err != nil {
		t.Fatalf("query trade statuses: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var orderID int64
		var status int
		if err := rows.Scan(&orderID, &status); err != nil {
			t.Fatalf("scan trade status: %v", err)
		}
		statuses[orderID] = status
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate trade statuses: %v", err)
	}
	if statuses[101] != tradeStatusPartiallyRefunded || statuses[102] != tradeStatusPendingPayment || statuses[103] != tradeStatusClosed {
		t.Fatalf("unexpected trade statuses: %+v", statuses)
	}

	var activeTradeMatches int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM payments WHERE id = 202 AND active_trade_id = trade_id AND trade_id IS NOT NULL`).Scan(&activeTradeMatches); err != nil {
		t.Fatalf("verify active trade: %v", err)
	}
	if activeTradeMatches != 1 {
		t.Fatal("pending payment active_trade_id was not backfilled")
	}
	var refundedAmount int64
	if err := db.QueryRowContext(ctx, `SELECT refunded_amount FROM payment_allocations WHERE payment_id = 201`).Scan(&refundedAmount); err != nil {
		t.Fatalf("read allocation refunded amount: %v", err)
	}
	if refundedAmount != 3000 {
		t.Fatalf("allocation refunded_amount=%d, expected 3000", refundedAmount)
	}
}

func checkFailed(report ValidationReport, name string) bool {
	for _, check := range report.Checks {
		if check.Name == name {
			return check.Count > 0
		}
	}
	return false
}
