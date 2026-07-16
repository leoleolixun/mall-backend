package migration

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"

	migrationfiles "go-mall/migrations"
)

const migrationIntegrationDatabase = "go_mall_migration_test"

func TestRunnerRoundTripOnMySQL(t *testing.T) {
	if os.Getenv("MALL_ALLOW_MIGRATION_INTEGRATION") != "1" {
		t.Skip("set MALL_ALLOW_MIGRATION_INTEGRATION=1 to run destructive migration integration test")
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
	db.SetMaxOpenConns(2)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("ping MySQL: %v", err)
	}
	var databaseName string
	if err := db.QueryRowContext(ctx, `SELECT DATABASE()`).Scan(&databaseName); err != nil {
		t.Fatalf("read database name: %v", err)
	}
	if databaseName != migrationIntegrationDatabase {
		t.Fatalf("refusing destructive integration test on database %q; expected %q", databaseName, migrationIntegrationDatabase)
	}

	resetMigrationIntegrationSchema(t, ctx, db)
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		resetMigrationIntegrationSchema(t, cleanupCtx, db)
	})
	createLegacyMigrationFixture(t, ctx, db)

	items, err := LoadFiles(migrationfiles.Files)
	if err != nil {
		t.Fatalf("load embedded migrations: %v", err)
	}
	runner, err := NewRunner(db, items)
	if err != nil {
		t.Fatalf("new runner: %v", err)
	}

	applied, err := runner.Up(ctx)
	if err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	if len(applied) != len(items) {
		t.Fatalf("applied %d migrations, expected %d", len(applied), len(items))
	}
	assertM8SchemaApplied(t, ctx, db)
	if err := runner.Verify(ctx); err != nil {
		t.Fatalf("verify applied migrations: %v", err)
	}
	statuses, err := runner.Status(ctx)
	if err != nil {
		t.Fatalf("read migration status: %v", err)
	}
	for _, status := range statuses {
		if !status.Applied {
			t.Fatalf("migration %06d was not marked applied", status.Migration.Version)
		}
	}

	secondRun, err := runner.Up(ctx)
	if err != nil {
		t.Fatalf("repeat migrations: %v", err)
	}
	if len(secondRun) != 0 {
		t.Fatalf("repeat up applied migrations again: %+v", secondRun)
	}

	rolledBack, err := runner.Down(ctx, len(items))
	if err != nil {
		t.Fatalf("roll back migrations: %v", err)
	}
	if len(rolledBack) != len(items) || rolledBack[0].Version != items[len(items)-1].Version || rolledBack[len(rolledBack)-1].Version != items[0].Version {
		t.Fatalf("unexpected rollback order: %+v", rolledBack)
	}
	assertM8SchemaRolledBack(t, ctx, db)

	reapplied, err := runner.Up(ctx)
	if err != nil {
		t.Fatalf("reapply migrations after rollback: %v", err)
	}
	if len(reapplied) != len(items) {
		t.Fatalf("reapplied %d migrations, expected %d", len(reapplied), len(items))
	}
	assertM8SchemaApplied(t, ctx, db)
}

func resetMigrationIntegrationSchema(t *testing.T, ctx context.Context, db *sql.DB) {
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
		"merchants",
		"product_skus",
		"products",
		"categories",
	}
	for _, table := range tables {
		if _, err := db.ExecContext(ctx, "DROP TABLE IF EXISTS `"+table+"`"); err != nil {
			t.Fatalf("drop table %s: %v", table, err)
		}
	}
}

func createLegacyMigrationFixture(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	statements := []string{
		`CREATE TABLE merchants (
			id BIGINT NOT NULL AUTO_INCREMENT,
			name VARCHAR(100) NOT NULL,
			status TINYINT NOT NULL DEFAULT 1,
			PRIMARY KEY (id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE orders (
			id BIGINT NOT NULL AUTO_INCREMENT,
			merchant_id BIGINT NOT NULL,
			status TINYINT NOT NULL DEFAULT 1,
			payable_amount BIGINT NOT NULL DEFAULT 0,
			PRIMARY KEY (id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE payments (
			id BIGINT NOT NULL AUTO_INCREMENT,
			order_id BIGINT NOT NULL,
			order_no VARCHAR(64) NOT NULL,
			merchant_id BIGINT NOT NULL,
			status TINYINT NOT NULL DEFAULT 1,
			PRIMARY KEY (id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE refunds (
			id BIGINT NOT NULL AUTO_INCREMENT,
			payment_id BIGINT NOT NULL,
			status TINYINT NOT NULL DEFAULT 1,
			PRIMARY KEY (id)
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
			t.Fatalf("create legacy fixture statement %d: %v", index+1, err)
		}
	}
}

func assertM8SchemaApplied(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	for _, table := range []string{"trades", "payment_allocations", "merchant_settlements", "settlement_entries"} {
		assertSchemaObjectCount(t, ctx, db, "table", table, "", 1)
	}
	for _, column := range []struct {
		table string
		name  string
	}{
		{"orders", "trade_id"},
		{"orders", "merchant_name"},
		{"orders", "commission_rate_bps"},
		{"orders", "commission_amount"},
		{"orders", "settlement_amount"},
		{"payments", "trade_id"},
		{"payments", "active_trade_id"},
		{"refunds", "trade_id"},
		{"refunds", "payment_allocation_id"},
		{"merchants", "commission_rate_bps"},
	} {
		assertSchemaObjectCount(t, ctx, db, "column", column.table, column.name, 1)
	}
	for _, column := range []string{"order_id", "order_no", "merchant_id"} {
		assertColumnNullable(t, ctx, db, "payments", column, true)
	}
	for _, index := range []struct {
		table string
		name  string
	}{
		{"orders", "uk_orders_trade_merchant"},
		{"payments", "uk_payments_active_trade"},
		{"products", "idx_products_merchant_status_deleted_id"},
		{"product_skus", "idx_skus_merchant_product_status_deleted"},
		{"categories", "idx_categories_merchant_status_deleted_sort_id"},
	} {
		assertSchemaObjectCount(t, ctx, db, "index", index.table, index.name, 1)
	}
}

func assertM8SchemaRolledBack(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	for _, table := range []string{"trades", "payment_allocations", "merchant_settlements", "settlement_entries"} {
		assertSchemaObjectCount(t, ctx, db, "table", table, "", 0)
	}
	for _, table := range []string{"orders", "payments", "refunds", "merchants", "products", "product_skus", "categories"} {
		assertSchemaObjectCount(t, ctx, db, "table", table, "", 1)
	}
	for _, column := range []struct {
		table string
		name  string
	}{
		{"orders", "trade_id"},
		{"payments", "trade_id"},
		{"refunds", "trade_id"},
		{"merchants", "commission_rate_bps"},
	} {
		assertSchemaObjectCount(t, ctx, db, "column", column.table, column.name, 0)
	}
	for _, column := range []string{"order_id", "order_no", "merchant_id"} {
		assertColumnNullable(t, ctx, db, "payments", column, false)
	}
}

func assertColumnNullable(t *testing.T, ctx context.Context, db *sql.DB, table, column string, nullable bool) {
	t.Helper()
	var value string
	if err := db.QueryRowContext(ctx, `SELECT is_nullable FROM information_schema.columns
		WHERE table_schema = DATABASE() AND table_name = ? AND column_name = ?`, table, column).Scan(&value); err != nil {
		t.Fatalf("read nullability for %s.%s: %v", table, column, err)
	}
	want := "NO"
	if nullable {
		want = "YES"
	}
	if value != want {
		t.Fatalf("column %s.%s is_nullable=%s, expected %s", table, column, value, want)
	}
}

func assertSchemaObjectCount(t *testing.T, ctx context.Context, db *sql.DB, objectType, table, name string, expected int) {
	t.Helper()
	var query string
	var args []any
	switch objectType {
	case "table":
		query = `SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ?`
		args = []any{table}
	case "column":
		query = `SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = ? AND column_name = ?`
		args = []any{table, name}
	case "index":
		query = `SELECT COUNT(DISTINCT index_name) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = ? AND index_name = ?`
		args = []any{table, name}
	default:
		t.Fatalf("unsupported schema object type %q", objectType)
	}
	var count int
	if err := db.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		t.Fatalf("count %s %s.%s: %v", objectType, table, name, err)
	}
	if count != expected {
		t.Fatalf("%s %s.%s count=%d, expected %d", objectType, table, name, count, expected)
	}
}
