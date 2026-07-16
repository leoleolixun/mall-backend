package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go-mall/internal/bootstrap"
	"go-mall/internal/config"
	"go-mall/internal/dto"
	"go-mall/internal/migration"
	"go-mall/internal/model"
	"go-mall/internal/repository"
	migrationfiles "go-mall/migrations"

	"github.com/alicebob/miniredis/v2"
	_ "github.com/go-sql-driver/mysql"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

const tradeIntegrationDatabase = "go_mall_trade_test"

func TestMultiMerchantTradeIntegrationOnMySQL(t *testing.T) {
	if os.Getenv("MALL_ALLOW_TRADE_INTEGRATION") != "1" {
		t.Skip("set MALL_ALLOW_TRADE_INTEGRATION=1 to run destructive trade integration test")
	}
	dsn := os.Getenv("MALL_TRADE_TEST_DSN")
	if dsn == "" {
		t.Fatal("MALL_TRADE_TEST_DSN is required")
	}
	sqlDB, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("open MySQL: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		t.Fatalf("ping MySQL: %v", err)
	}
	var databaseName string
	if err := sqlDB.QueryRowContext(ctx, `SELECT DATABASE()`).Scan(&databaseName); err != nil {
		t.Fatalf("read database name: %v", err)
	}
	if databaseName != tradeIntegrationDatabase {
		t.Fatalf("refusing destructive integration test on database %q; expected %q", databaseName, tradeIntegrationDatabase)
	}
	resetTradeIntegrationSchema(t, ctx, sqlDB)
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		resetTradeIntegrationSchema(t, cleanupCtx, sqlDB)
	})

	db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB}), &gorm.Config{})
	if err != nil {
		t.Fatalf("open GORM: %v", err)
	}
	if err := bootstrap.AutoMigrate(db); err != nil {
		t.Fatalf("create legacy schema: %v", err)
	}
	items, err := migration.LoadFiles(migrationfiles.Files)
	if err != nil {
		t.Fatalf("load M8 migrations: %v", err)
	}
	runner, err := migration.NewRunner(sqlDB, items)
	if err != nil {
		t.Fatalf("new migration runner: %v", err)
	}
	if _, err := runner.Up(ctx); err != nil {
		t.Fatalf("apply M8 migrations: %v", err)
	}
	seedTradeFixture(t, ctx, db)

	redisServer := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() { _ = redisClient.Close() })
	repo := repository.NewTradeRepository(db)
	tradeService := NewTradeService(repo, redisClient, nil)

	t.Run("two merchants coupon idempotency and cancel", func(t *testing.T) {
		previewRequest := dto.TradePreviewRequest{
			AddressID:       1,
			MerchantCoupons: []dto.MerchantCouponSelection{{MerchantID: 1, UserCouponID: 401}},
			Items:           []dto.OrderRequestItem{{SKUID: 222, Quantity: 3}, {SKUID: 111, Quantity: 2}},
		}
		preview, err := tradeService.Preview(ctx, 1, previewRequest)
		if err != nil {
			t.Fatalf("preview trade: %v", err)
		}
		if len(preview.MerchantGroups) != 2 || preview.GoodsAmount != 3500 || preview.DiscountAmount != 100 || preview.PayableAmount != 3400 {
			t.Fatalf("unexpected preview: %+v", preview)
		}
		created, err := tradeService.Create(ctx, 1, dto.CreateTradeRequest{
			AddressID: previewRequest.AddressID, MerchantCoupons: previewRequest.MerchantCoupons,
			Items: previewRequest.Items, IdempotencyToken: preview.IdempotencyToken, Remark: "跨商户测试",
		})
		if err != nil {
			t.Fatalf("create trade: %v", err)
		}
		if len(created.Orders) != 2 || created.PayableAmount != 3400 || created.Orders[0].MerchantID != 1 || created.Orders[1].MerchantID != 2 {
			t.Fatalf("unexpected created trade: %+v", created)
		}
		retried, err := tradeService.Create(ctx, 1, dto.CreateTradeRequest{
			AddressID: previewRequest.AddressID, MerchantCoupons: previewRequest.MerchantCoupons,
			Items: previewRequest.Items, IdempotencyToken: preview.IdempotencyToken, Remark: "跨商户测试",
		})
		if err != nil || retried.ID != created.ID {
			t.Fatalf("idempotent retry returned %+v, err=%v", retried, err)
		}
		assertDatabaseCount(t, ctx, sqlDB, `SELECT COUNT(*) FROM trades WHERE user_id = 1 AND idempotency_key = ?`, 1, preview.IdempotencyToken)
		assertDatabaseCount(t, ctx, sqlDB, `SELECT COUNT(*) FROM orders WHERE trade_id = ?`, 2, created.ID)
		assertSKUStock(t, ctx, sqlDB, 111, 98)
		assertSKUStock(t, ctx, sqlDB, 222, 97)

		cancelled, err := tradeService.Cancel(ctx, 1, created.ID)
		if err != nil {
			t.Fatalf("cancel trade: %v", err)
		}
		if cancelled.Status != model.TradeStatusClosed || cancelled.Orders[0].Status != model.OrderStatusCancelled || cancelled.Orders[1].Status != model.OrderStatusCancelled {
			t.Fatalf("unexpected cancelled trade: %+v", cancelled)
		}
		assertSKUStock(t, ctx, sqlDB, 111, 100)
		assertSKUStock(t, ctx, sqlDB, 222, 100)
		assertDatabaseCount(t, ctx, sqlDB, `SELECT COUNT(*) FROM user_coupons WHERE id = 401 AND status = 1 AND order_id = 0`, 1)
		if _, err := tradeService.Cancel(ctx, 1, created.ID); err != nil {
			t.Fatalf("repeat cancel should be idempotent: %v", err)
		}
		assertSKUStock(t, ctx, sqlDB, 111, 100)
		assertSKUStock(t, ctx, sqlDB, 222, 100)
	})

	t.Run("coupon merchant mismatch", func(t *testing.T) {
		_, err := tradeService.Preview(ctx, 1, dto.TradePreviewRequest{
			AddressID:       1,
			MerchantCoupons: []dto.MerchantCouponSelection{{MerchantID: 2, UserCouponID: 401}},
			Items:           []dto.OrderRequestItem{{SKUID: 222, Quantity: 1}},
		})
		if err == nil || !strings.Contains(err.Error(), "优惠券不可用") {
			t.Fatalf("expected coupon merchant mismatch, got %v", err)
		}
	})

	t.Run("quote change rejects create without partial write", func(t *testing.T) {
		request := dto.TradePreviewRequest{AddressID: 1, Items: []dto.OrderRequestItem{{SKUID: 111, Quantity: 1}, {SKUID: 222, Quantity: 1}}}
		preview, err := tradeService.Preview(ctx, 1, request)
		if err != nil {
			t.Fatalf("preview before price change: %v", err)
		}
		if err := db.Model(&model.ProductSKU{}).Where("id = ?", 222).Update("price", 501).Error; err != nil {
			t.Fatalf("change SKU price: %v", err)
		}
		beforeTrades := databaseCount(t, ctx, sqlDB, `SELECT COUNT(*) FROM trades`)
		_, err = tradeService.Create(ctx, 1, dto.CreateTradeRequest{AddressID: 1, Items: request.Items, IdempotencyToken: preview.IdempotencyToken})
		if err == nil || !errors.Is(err, ErrTradePreviewRequired) {
			t.Fatalf("expected re-preview error, got %v", err)
		}
		assertDatabaseCount(t, ctx, sqlDB, `SELECT COUNT(*) FROM trades`, beforeTrades)
		assertSKUStock(t, ctx, sqlDB, 111, 100)
		assertSKUStock(t, ctx, sqlDB, 222, 100)
		if err := db.Model(&model.ProductSKU{}).Where("id = ?", 222).Update("price", 500).Error; err != nil {
			t.Fatalf("restore SKU price: %v", err)
		}
	})

	t.Run("transaction failure rolls back first merchant", func(t *testing.T) {
		failingService := NewTradeService(&failOrderCreateTradeRepository{TradeRepository: repo, merchantID: 2}, redisClient, nil)
		request := dto.TradePreviewRequest{AddressID: 1, Items: []dto.OrderRequestItem{{SKUID: 111, Quantity: 1}, {SKUID: 222, Quantity: 1}}}
		preview, err := failingService.Preview(ctx, 1, request)
		if err != nil {
			t.Fatalf("preview before injected failure: %v", err)
		}
		beforeTrades := databaseCount(t, ctx, sqlDB, `SELECT COUNT(*) FROM trades`)
		beforeOrders := databaseCount(t, ctx, sqlDB, `SELECT COUNT(*) FROM orders`)
		_, err = failingService.Create(ctx, 1, dto.CreateTradeRequest{AddressID: 1, Items: request.Items, IdempotencyToken: preview.IdempotencyToken})
		if err == nil || !strings.Contains(err.Error(), "injected order failure") {
			t.Fatalf("expected injected failure, got %v", err)
		}
		assertDatabaseCount(t, ctx, sqlDB, `SELECT COUNT(*) FROM trades`, beforeTrades)
		assertDatabaseCount(t, ctx, sqlDB, `SELECT COUNT(*) FROM orders`, beforeOrders)
		assertSKUStock(t, ctx, sqlDB, 111, 100)
		assertSKUStock(t, ctx, sqlDB, 222, 100)
	})

	t.Run("one hundred identical submissions create one trade", func(t *testing.T) {
		request := dto.TradePreviewRequest{AddressID: 1, Items: []dto.OrderRequestItem{{SKUID: 222, Quantity: 1}, {SKUID: 111, Quantity: 1}}}
		preview, err := tradeService.Preview(ctx, 1, request)
		if err != nil {
			t.Fatalf("preview concurrent trade: %v", err)
		}
		createRequest := dto.CreateTradeRequest{AddressID: 1, Items: request.Items, IdempotencyToken: preview.IdempotencyToken}
		var wg sync.WaitGroup
		var unexpected atomic.Int64
		ids := make(chan int64, 100)
		for index := 0; index < 100; index++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				created, err := tradeService.Create(ctx, 1, createRequest)
				if err == nil {
					ids <- created.ID
					return
				}
				if !errors.Is(err, ErrTradeConflict) {
					unexpected.Add(1)
				}
			}()
		}
		wg.Wait()
		close(ids)
		if unexpected.Load() != 0 {
			t.Fatalf("concurrent create returned %d unexpected errors", unexpected.Load())
		}
		var firstID int64
		for id := range ids {
			if firstID == 0 {
				firstID = id
			}
			if id != firstID {
				t.Fatalf("same idempotency token returned trade IDs %d and %d", firstID, id)
			}
		}
		if firstID == 0 {
			t.Fatal("no concurrent request created or returned the trade")
		}
		assertDatabaseCount(t, ctx, sqlDB, `SELECT COUNT(*) FROM trades WHERE user_id = 1 AND idempotency_key = ?`, 1, preview.IdempotencyToken)
		assertDatabaseCount(t, ctx, sqlDB, `SELECT COUNT(*) FROM orders WHERE trade_id = ?`, 2, firstID)
		if _, err := tradeService.Cancel(ctx, 1, firstID); err != nil {
			t.Fatalf("cancel concurrent test trade: %v", err)
		}
	})

	t.Run("one hundred users cannot oversell ten units", func(t *testing.T) {
		if err := db.Model(&model.ProductSKU{}).Where("id = ?", 111).Update("stock", 10).Error; err != nil {
			t.Fatalf("set limited stock: %v", err)
		}
		users := make([]model.User, 100)
		addresses := make([]model.Address, 100)
		for index := 0; index < 100; index++ {
			userID := int64(1000 + index)
			users[index] = model.User{ID: userID, Nickname: fmt.Sprintf("并发用户%d", index), Status: model.StatusEnabled}
			addresses[index] = model.Address{ID: userID, UserID: userID, ReceiverName: "并发用户", ReceiverPhone: "13800000000", Province: "浙江省", City: "杭州市", District: "西湖区", Detail: fmt.Sprintf("测试路%d号", index)}
		}
		if err := db.Create(&users).Error; err != nil {
			t.Fatalf("create concurrent users: %v", err)
		}
		if err := db.Create(&addresses).Error; err != nil {
			t.Fatalf("create concurrent addresses: %v", err)
		}
		tokens := make([]string, 100)
		for index := 0; index < 100; index++ {
			userID := int64(1000 + index)
			preview, err := tradeService.Preview(ctx, userID, dto.TradePreviewRequest{AddressID: userID, Items: []dto.OrderRequestItem{{SKUID: 111, Quantity: 1}}})
			if err != nil {
				t.Fatalf("preview limited stock for user %d: %v", userID, err)
			}
			tokens[index] = preview.IdempotencyToken
		}
		var wg sync.WaitGroup
		var succeeded atomic.Int64
		var unexpected atomic.Int64
		for index := 0; index < 100; index++ {
			index := index
			wg.Add(1)
			go func() {
				defer wg.Done()
				userID := int64(1000 + index)
				_, err := tradeService.Create(ctx, userID, dto.CreateTradeRequest{AddressID: userID, IdempotencyToken: tokens[index], Items: []dto.OrderRequestItem{{SKUID: 111, Quantity: 1}}})
				if err == nil {
					succeeded.Add(1)
					return
				}
				if !strings.Contains(err.Error(), "库存不足") {
					unexpected.Add(1)
				}
			}()
		}
		wg.Wait()
		if succeeded.Load() != 10 || unexpected.Load() != 0 {
			t.Fatalf("oversell result succeeded=%d unexpected=%d", succeeded.Load(), unexpected.Load())
		}
		assertSKUStock(t, ctx, sqlDB, 111, 0)
		assertDatabaseCount(t, ctx, sqlDB, `SELECT COUNT(*) FROM trades WHERE user_id >= 1000 AND user_id < 1100`, 10)
		assertDatabaseCount(t, ctx, sqlDB, `SELECT COUNT(*) FROM orders WHERE user_id >= 1000 AND user_id < 1100`, 10)
	})

	t.Run("trade payment creates one intent and immutable allocations", func(t *testing.T) {
		if err := db.Model(&model.ProductSKU{}).Where("id IN ?", []int64{111, 222}).Update("stock", 100).Error; err != nil {
			t.Fatalf("reset payment test stock: %v", err)
		}
		request := dto.TradePreviewRequest{
			AddressID: 1,
			Items:     []dto.OrderRequestItem{{SKUID: 111, Quantity: 1}, {SKUID: 222, Quantity: 2}},
		}
		preview, err := tradeService.Preview(ctx, 1, request)
		if err != nil {
			t.Fatalf("preview payment trade: %v", err)
		}
		created, err := tradeService.Create(ctx, 1, dto.CreateTradeRequest{
			AddressID: 1, Items: request.Items, IdempotencyToken: preview.IdempotencyToken,
		})
		if err != nil {
			t.Fatalf("create payment trade: %v", err)
		}

		paymentService := NewPaymentService(repository.NewPaymentRepository(db), config.PaymentConfig{MockEnabled: true})
		var wg sync.WaitGroup
		var failures atomic.Int64
		paymentNumbers := make(chan string, 50)
		for index := 0; index < 50; index++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				payment, err := paymentService.Create(ctx, 1, dto.CreatePaymentRequest{
					TradeID: created.ID, PayChannel: model.PayChannelMock,
				})
				if err != nil {
					failures.Add(1)
					return
				}
				paymentNumbers <- payment.PaymentNo
			}()
		}
		wg.Wait()
		close(paymentNumbers)
		if failures.Load() != 0 {
			t.Fatalf("concurrent payment creation failures=%d", failures.Load())
		}
		var paymentNo string
		for value := range paymentNumbers {
			if paymentNo == "" {
				paymentNo = value
			}
			if value != paymentNo {
				t.Fatalf("same trade returned payment numbers %s and %s", paymentNo, value)
			}
		}
		assertDatabaseCount(t, ctx, sqlDB, `SELECT COUNT(*) FROM payments WHERE trade_id = ?`, 1, created.ID)
		assertDatabaseCount(t, ctx, sqlDB, `SELECT COUNT(*) FROM payments WHERE trade_id = ? AND order_id IS NULL AND merchant_id IS NULL`, 1, created.ID)
		assertDatabaseCount(t, ctx, sqlDB, `SELECT COUNT(*) FROM payment_allocations WHERE trade_id = ?`, 2, created.ID)
		assertDatabaseCount(t, ctx, sqlDB, `SELECT COUNT(*) FROM (
			SELECT p.id FROM payments p JOIN payment_allocations pa ON pa.payment_id = p.id
			WHERE p.trade_id = ? GROUP BY p.id, p.amount HAVING SUM(pa.amount) = p.amount
		) matched`, 1, created.ID)

		paid, err := paymentService.MockComplete(ctx, 1, paymentNo)
		if err != nil {
			t.Fatalf("complete trade payment: %v", err)
		}
		if paid.Status != model.PaymentStatusPaid {
			t.Fatalf("unexpected paid response: %+v", paid)
		}
		assertDatabaseCount(t, ctx, sqlDB, `SELECT COUNT(*) FROM trades WHERE id = ? AND status = 2`, 1, created.ID)
		assertDatabaseCount(t, ctx, sqlDB, `SELECT COUNT(*) FROM orders WHERE trade_id = ? AND status = 2`, 2, created.ID)
		assertDatabaseCount(t, ctx, sqlDB, `SELECT COUNT(*) FROM payments WHERE trade_id = ? AND status = 2 AND active_trade_id IS NULL`, 1, created.ID)
		completedAt := time.Now().Add(-10 * 24 * time.Hour)
		if err := db.Model(&model.Order{}).Where("trade_id = ?", created.ID).Updates(map[string]any{
			"status": model.OrderStatusCompleted, "completed_at": &completedAt,
		}).Error; err != nil {
			t.Fatalf("complete payment test child orders: %v", err)
		}

		afterSaleService := NewAfterSaleService(repository.NewAfterSaleRepository(db), config.PaymentConfig{MockEnabled: true})
		for index, order := range created.Orders {
			if len(order.Items) != 1 {
				t.Fatalf("payment test order has unexpected items: %+v", order)
			}
			afterSale, err := afterSaleService.Create(ctx, 1, dto.CreateAfterSaleRequest{
				OrderID: order.ID, OrderItemID: order.Items[0].ID,
				Type: model.AfterSaleTypeRefundOnly, Reason: fmt.Sprintf("商户%d退款", order.MerchantID),
			})
			if err != nil {
				t.Fatalf("create merchant %d after-sale: %v", order.MerchantID, err)
			}
			refunded, err := afterSaleService.MerchantApprove(ctx, order.MerchantID, 99, afterSale.ID)
			if err != nil {
				t.Fatalf("approve merchant %d refund: %v", order.MerchantID, err)
			}
			if refunded.Refund == nil || refunded.Refund.Status != model.RefundStatusSucceeded || refunded.Refund.PaymentAllocationID == nil {
				t.Fatalf("merchant %d refund did not bind allocation: %+v", order.MerchantID, refunded)
			}
			expectedPaymentStatus := model.PaymentStatusPartiallyRefunded
			expectedTradeStatus := model.TradeStatusPartiallyRefunded
			if index == len(created.Orders)-1 {
				expectedPaymentStatus = model.PaymentStatusRefunded
				expectedTradeStatus = model.TradeStatusRefunded
			}
			assertDatabaseCount(t, ctx, sqlDB, `SELECT COUNT(*) FROM payments WHERE payment_no = ? AND status = ?`, 1, paymentNo, expectedPaymentStatus)
			assertDatabaseCount(t, ctx, sqlDB, `SELECT COUNT(*) FROM trades WHERE id = ? AND status = ?`, 1, created.ID, expectedTradeStatus)
		}
		assertDatabaseCount(t, ctx, sqlDB, `SELECT COUNT(*) FROM payment_allocations WHERE trade_id = ? AND refunded_amount = amount`, 2, created.ID)
		assertDatabaseCount(t, ctx, sqlDB, `SELECT COUNT(*) FROM settlement_entries se JOIN orders o ON o.id = se.order_id WHERE o.trade_id = ? AND se.entry_type IN ('refund', 'commission_refund')`, 4, created.ID)
		var historicalRefundID int64
		if err := sqlDB.QueryRowContext(ctx, `SELECT MIN(r.id) FROM refunds r JOIN orders o ON o.id = r.order_id WHERE o.trade_id = ? AND r.status = ?`, created.ID, model.RefundStatusSucceeded).Scan(&historicalRefundID); err != nil {
			t.Fatalf("select historical refund for settlement repair: %v", err)
		}
		if err := db.Where("refund_id = ?", historicalRefundID).Delete(&model.SettlementEntry{}).Error; err != nil {
			t.Fatalf("remove refund entries before settlement repair: %v", err)
		}
		assertDatabaseCount(t, ctx, sqlDB, `SELECT COUNT(*) FROM settlement_entries WHERE refund_id = ?`, 0, historicalRefundID)

		settlementService := NewSettlementService(repository.NewSettlementRepository(db), config.SettlementConfig{HoldDays: 7, BatchSize: 100})
		accrual, err := settlementService.AccrueCompletedOrders(ctx)
		if err != nil || accrual.Accrued < 3 {
			t.Fatalf("accrue trade settlement entries: report=%+v err=%v", accrual, err)
		}
		assertDatabaseCount(t, ctx, sqlDB, `SELECT COUNT(*) FROM settlement_entries WHERE refund_id = ?`, 2, historicalRefundID)
		periodEnd := time.Now().UTC().Truncate(time.Millisecond)
		periodStart := periodEnd.Add(-30 * 24 * time.Hour)
		for _, merchantID := range []int64{1, 2} {
			settlement, err := settlementService.Generate(ctx, merchantID, periodStart, periodEnd)
			if err != nil {
				t.Fatalf("generate merchant %d settlement: %v", merchantID, err)
			}
			if settlement.GrossAmount != 1000 || settlement.CommissionAmount != 0 || settlement.RefundAmount != 1000 || settlement.NetAmount != 0 || len(settlement.Entries) != 4 {
				t.Fatalf("merchant %d settlement does not reconcile after full refund: %+v", merchantID, settlement)
			}
		}
	})
}

type failOrderCreateTradeRepository struct {
	repository.TradeRepository
	merchantID int64
}

func (r *failOrderCreateTradeRepository) Transaction(ctx context.Context, fn func(repository.TradeRepository) error) error {
	return r.TradeRepository.Transaction(ctx, func(txRepo repository.TradeRepository) error {
		return fn(&failOrderCreateTradeRepository{TradeRepository: txRepo, merchantID: r.merchantID})
	})
}

func (r *failOrderCreateTradeRepository) CreateOrder(ctx context.Context, order *model.Order) error {
	if order.MerchantID == r.merchantID {
		return errors.New("injected order failure")
	}
	return r.TradeRepository.CreateOrder(ctx, order)
}

func resetTradeIntegrationSchema(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	if _, err := db.ExecContext(ctx, `SET FOREIGN_KEY_CHECKS = 0`); err != nil {
		t.Fatalf("disable foreign key checks: %v", err)
	}
	rows, err := db.QueryContext(ctx, `SELECT table_name FROM information_schema.tables WHERE table_schema = DATABASE()`)
	if err != nil {
		t.Fatalf("list integration tables: %v", err)
	}
	var tables []string
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			rows.Close()
			t.Fatalf("scan integration table: %v", err)
		}
		tables = append(tables, table)
	}
	if err := rows.Close(); err != nil {
		t.Fatalf("close integration table rows: %v", err)
	}
	for _, table := range tables {
		if _, err := db.ExecContext(ctx, "DROP TABLE `"+table+"`"); err != nil {
			t.Fatalf("drop integration table %s: %v", table, err)
		}
	}
	if _, err := db.ExecContext(ctx, `SET FOREIGN_KEY_CHECKS = 1`); err != nil {
		t.Fatalf("enable foreign key checks: %v", err)
	}
}

func seedTradeFixture(t *testing.T, ctx context.Context, db *gorm.DB) {
	t.Helper()
	now := time.Now()
	values := []any{
		&[]model.Merchant{{ID: 1, Name: "商户一", Status: model.StatusEnabled, CommissionRateBPS: 1000}, {ID: 2, Name: "商户二", Status: model.StatusEnabled, CommissionRateBPS: 500}},
		&model.User{ID: 1, Nickname: "测试买家", Status: model.StatusEnabled},
		&model.Address{ID: 1, UserID: 1, ReceiverName: "测试买家", ReceiverPhone: "13800000000", Province: "浙江省", City: "杭州市", District: "西湖区", Detail: "测试路 1 号", IsDefault: true},
		&[]model.Product{{ID: 11, MerchantID: 1, CategoryID: 1, Name: "商户一商品", Status: model.ProductStatusOnSale}, {ID: 22, MerchantID: 2, CategoryID: 2, Name: "商户二商品", Status: model.ProductStatusOnSale}},
		&[]model.ProductSKU{{ID: 111, MerchantID: 1, ProductID: 11, Name: "规格一", Price: 1000, Stock: 100, Status: model.StatusEnabled}, {ID: 222, MerchantID: 2, ProductID: 22, Name: "规格二", Price: 500, Stock: 100, Status: model.StatusEnabled}},
		&model.Coupon{ID: 301, MerchantID: 1, Name: "商户一满减券", ThresholdAmount: 1000, DiscountAmount: 100, TotalQuantity: 100, ClaimedQuantity: 1, Status: model.CouponStatusActive, StartAt: now.Add(-time.Hour), EndAt: now.Add(time.Hour)},
		&model.UserCoupon{ID: 401, CouponID: 301, UserID: 1, MerchantID: 1, Status: model.UserCouponStatusUnused, ClaimedAt: now},
	}
	for index, value := range values {
		if err := db.WithContext(ctx).Create(value).Error; err != nil {
			t.Fatalf("seed trade fixture value %d: %v", index+1, err)
		}
	}
}

func databaseCount(t *testing.T, ctx context.Context, db *sql.DB, query string, args ...any) int64 {
	t.Helper()
	var count int64
	if err := db.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		t.Fatalf("count database rows: %v", err)
	}
	return count
}

func assertDatabaseCount(t *testing.T, ctx context.Context, db *sql.DB, query string, expected int64, args ...any) {
	t.Helper()
	if count := databaseCount(t, ctx, db, query, args...); count != expected {
		t.Fatalf("database count=%d, expected %d for query %s", count, expected, query)
	}
}

func assertSKUStock(t *testing.T, ctx context.Context, db *sql.DB, skuID int64, expected int) {
	t.Helper()
	var stock int
	if err := db.QueryRowContext(ctx, `SELECT stock FROM product_skus WHERE id = ?`, skuID).Scan(&stock); err != nil {
		t.Fatalf("read SKU %d stock: %v", skuID, err)
	}
	if stock != expected {
		t.Fatalf("SKU %d stock=%d, expected %d", skuID, stock, expected)
	}
}
