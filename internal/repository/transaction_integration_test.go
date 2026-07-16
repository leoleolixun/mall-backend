package repository_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"go-mall/internal/bootstrap"
	"go-mall/internal/config"
	"go-mall/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func integrationMySQL(t *testing.T) *gorm.DB {
	t.Helper()
	configPath := os.Getenv("MALL_INTEGRATION_CONFIG")
	if configPath == "" {
		t.Skip("set MALL_INTEGRATION_CONFIG to run MySQL integration tests")
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	db, err := bootstrap.InitMySQL(cfg.MySQL)
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func TestPaymentPendingOrderUniqueIndexIntegration(t *testing.T) {
	db := integrationMySQL(t)
	if !db.Migrator().HasIndex(&model.Payment{}, "idx_payments_active_order") {
		t.Fatal("missing idx_payments_active_order unique index")
	}
	var order model.Order
	if err := db.Where("NOT EXISTS (SELECT 1 FROM payments WHERE payments.active_order_id = orders.id)").First(&order).Error; err != nil {
		t.Skipf("no order without an active payment available for integration test: %v", err)
	}

	rollback := errors.New("rollback payment uniqueness integration test")
	err := db.Transaction(func(tx *gorm.DB) error {
		activeOrderID := order.ID
		orderID := order.ID
		merchantID := order.MerchantID
		first := model.Payment{
			PaymentNo:     fmt.Sprintf("IT-PAY-1-%d", time.Now().UnixNano()),
			OrderID:       &orderID,
			ActiveOrderID: &activeOrderID,
			OrderNo:       order.OrderNo,
			UserID:        order.UserID,
			MerchantID:    &merchantID,
			PayChannel:    model.PayChannelMock,
			PayScene:      model.PaySceneMock,
			Status:        model.PaymentStatusPending,
			Amount:        order.PayableAmount,
		}
		if err := tx.Create(&first).Error; err != nil {
			return err
		}
		second := first
		second.ID = 0
		second.PaymentNo = fmt.Sprintf("IT-PAY-2-%d", time.Now().UnixNano())
		if err := tx.Create(&second).Error; err == nil {
			return fmt.Errorf("duplicate pending payment was accepted")
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatalf("unexpected transaction result: %v", err)
	}
}

func TestSKUForUpdateLockIntegration(t *testing.T) {
	db := integrationMySQL(t)
	var sku model.ProductSKU
	if err := db.First(&sku).Error; err != nil {
		t.Skipf("no SKU available for integration test: %v", err)
	}

	locked := make(chan struct{})
	release := make(chan struct{})
	firstDone := make(chan error, 1)
	go func() {
		firstDone <- db.Transaction(func(tx *gorm.DB) error {
			var value model.ProductSKU
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&value, sku.ID).Error; err != nil {
				return err
			}
			close(locked)
			<-release
			return nil
		})
	}()

	select {
	case <-locked:
	case err := <-firstDone:
		t.Fatalf("first transaction failed before locking: %v", err)
	case <-time.After(3 * time.Second):
		close(release)
		t.Fatal("first transaction did not acquire row lock")
	}

	secondDone := make(chan error, 1)
	startedAt := time.Now()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		secondDone <- db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			var value model.ProductSKU
			return tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&value, sku.ID).Error
		})
	}()

	select {
	case err := <-secondDone:
		close(release)
		<-firstDone
		t.Fatalf("second transaction bypassed row lock after %s: %v", time.Since(startedAt), err)
	case <-time.After(150 * time.Millisecond):
	}
	close(release)
	if err := <-firstDone; err != nil {
		t.Fatalf("first transaction failed: %v", err)
	}
	if err := <-secondDone; err != nil {
		t.Fatalf("second transaction failed after lock release: %v", err)
	}
	if elapsed := time.Since(startedAt); elapsed < 150*time.Millisecond {
		t.Fatalf("row lock wait was too short: %s", elapsed)
	}
}
