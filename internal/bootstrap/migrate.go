package bootstrap

import (
	"fmt"

	"go-mall/internal/model"
	"go-mall/internal/pricing"

	"gorm.io/gorm"
)

func AutoMigrate(db *gorm.DB) error {
	if err := db.AutoMigrate(
		&model.Merchant{},
		&model.MerchantAccount{},
		&model.Category{},
		&model.Product{},
		&model.ProductSKU{},
		&model.InventoryLog{},
		&model.User{},
		&model.UserAuth{},
		&model.Address{},
		&model.Order{},
		&model.OrderItem{},
		&model.Shipment{},
		&model.Payment{},
		&model.AfterSale{},
		&model.Refund{},
		&model.Coupon{},
		&model.UserCoupon{},
		&model.FavoriteProduct{},
	); err != nil {
		return err
	}
	if db.Migrator().HasIndex(&model.AfterSale{}, "idx_after_sales_active_item") {
		if err := db.Migrator().DropIndex(&model.AfterSale{}, "idx_after_sales_active_item"); err != nil {
			return err
		}
	}
	if err := backfillOrderItemAmounts(db); err != nil {
		return err
	}
	return backfillPaymentActiveOrderIDs(db)
}

func backfillOrderItemAmounts(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		var orders []model.Order
		if err := tx.Select("id", "goods_amount", "freight_amount", "discount_amount", "payable_amount").
			Order("id ASC").Find(&orders).Error; err != nil {
			return err
		}

		for _, order := range orders {
			var items []model.OrderItem
			if err := tx.Select("id", "subtotal", "discount_amount", "payable_amount").
				Where("order_id = ?", order.ID).Order("id ASC").Find(&items).Error; err != nil {
				return err
			}

			subtotals := make([]int64, len(items))
			for i := range items {
				subtotals[i] = items[i].Subtotal
			}

			amounts, err := pricing.AllocateOrderItems(subtotals, order.DiscountAmount)
			if err != nil {
				return fmt.Errorf("订单 %d 优惠分摊失败: %w", order.ID, err)
			}
			goodsAmount, err := pricing.SumSubtotals(subtotals)
			if err != nil {
				return fmt.Errorf("订单 %d 商品总额校验失败: %w", order.ID, err)
			}
			if goodsAmount != order.GoodsAmount {
				return fmt.Errorf("订单 %d 商品明细总额 %d 与订单商品总额 %d 不一致", order.ID, goodsAmount, order.GoodsAmount)
			}
			if order.FreightAmount < 0 || order.PayableAmount < order.FreightAmount || order.PayableAmount-order.FreightAmount != order.GoodsAmount-order.DiscountAmount {
				return fmt.Errorf("订单 %d 应付金额快照不一致", order.ID)
			}
			for i := range items {
				if items[i].DiscountAmount == amounts[i].DiscountAmount && items[i].PayableAmount == amounts[i].PayableAmount {
					continue
				}
				if err := tx.Model(&model.OrderItem{}).Where("id = ?", items[i].ID).UpdateColumns(map[string]interface{}{
					"discount_amount": amounts[i].DiscountAmount,
					"payable_amount":  amounts[i].PayableAmount,
				}).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func backfillPaymentActiveOrderIDs(db *gorm.DB) error {
	type duplicateOrder struct {
		OrderID int64 `gorm:"column:order_id"`
		Count   int64 `gorm:"column:count"`
	}

	return db.Transaction(func(tx *gorm.DB) error {
		var duplicates []duplicateOrder
		if err := tx.Unscoped().Model(&model.Payment{}).
			Select("order_id, COUNT(*) AS count").
			Where("status = ? AND deleted_at IS NULL AND order_id IS NOT NULL", model.PaymentStatusPending).
			Group("order_id").
			Having("COUNT(*) > 1").
			Limit(10).
			Scan(&duplicates).Error; err != nil {
			return err
		}
		if len(duplicates) > 0 {
			orderIDs := make([]int64, 0, len(duplicates))
			for _, duplicate := range duplicates {
				orderIDs = append(orderIDs, duplicate.OrderID)
			}
			return fmt.Errorf("存在重复待支付单，请先完成支付对账，订单 ID: %v", orderIDs)
		}

		if err := tx.Unscoped().Model(&model.Payment{}).
			Where("status <> ? OR deleted_at IS NOT NULL", model.PaymentStatusPending).
			Update("active_order_id", nil).Error; err != nil {
			return err
		}

		var pending []model.Payment
		if err := tx.Where("status = ? AND order_id IS NOT NULL", model.PaymentStatusPending).
			Order("order_id ASC, id ASC").
			Find(&pending).Error; err != nil {
			return err
		}
		for _, payment := range pending {
			if payment.OrderID == nil {
				continue
			}
			if payment.ActiveOrderID != nil && *payment.ActiveOrderID == *payment.OrderID {
				continue
			}
			if err := tx.Model(&model.Payment{}).
				Where("id = ? AND status = ?", payment.ID, model.PaymentStatusPending).
				Update("active_order_id", payment.OrderID).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
