package repository

import (
	"context"
	"fmt"
	"time"

	"go-mall/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type SettlementRepository interface {
	Transaction(ctx context.Context, fn func(repo SettlementRepository) error) error
	ListCompletedOrdersMissingEntries(ctx context.Context, limit int) ([]model.Order, error)
	ListSucceededRefundsMissingEntries(ctx context.Context, limit int) ([]model.Refund, error)
	FindOrderByID(ctx context.Context, orderID int64) (*model.Order, error)
	FindPaymentAllocationByID(ctx context.Context, allocationID int64) (*model.PaymentAllocation, error)
	SumSucceededRefundsBefore(ctx context.Context, allocationID int64, updatedAt time.Time, refundID int64) (int64, error)
	CreateEntries(ctx context.Context, entries []model.SettlementEntry) error
	FindMerchantByID(ctx context.Context, merchantID int64) (*model.Merchant, error)
	ListEntriesByMerchantID(ctx context.Context, merchantID int64, entryType string, offset, limit int) ([]model.SettlementEntry, int64, error)
	ListSettlementsByMerchantID(ctx context.Context, merchantID int64, status, offset, limit int) ([]model.MerchantSettlement, int64, error)
	FindSettlementByIDAndMerchantID(ctx context.Context, id, merchantID int64) (*model.MerchantSettlement, error)
	FindSettlementByPeriod(ctx context.Context, merchantID int64, start, end time.Time) (*model.MerchantSettlement, error)
	FindEntriesBySettlementID(ctx context.Context, merchantID, settlementID int64) ([]model.SettlementEntry, error)
	FindEligibleEntriesForUpdate(ctx context.Context, merchantID int64, periodEnd, completedBefore time.Time) ([]model.SettlementEntry, error)
	CreateSettlement(ctx context.Context, settlement *model.MerchantSettlement) error
	AttachEntries(ctx context.Context, entryIDs []int64, settlementID int64) error
}

type settlementRepository struct{ db *gorm.DB }

func NewSettlementRepository(db *gorm.DB) SettlementRepository {
	return &settlementRepository{db: db}
}

func (r *settlementRepository) Transaction(ctx context.Context, fn func(repo SettlementRepository) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&settlementRepository{db: tx})
	})
}

func (r *settlementRepository) ListCompletedOrdersMissingEntries(ctx context.Context, limit int) ([]model.Order, error) {
	var orders []model.Order
	err := r.db.WithContext(ctx).Table("orders AS o").
		Select("o.*").
		Joins("LEFT JOIN settlement_entries sale_entry ON sale_entry.entry_no = CONCAT('SALE-', o.id)").
		Joins("LEFT JOIN settlement_entries commission_entry ON commission_entry.entry_no = CONCAT('COMMISSION-', o.id)").
		Where("o.status = ? AND o.completed_at IS NOT NULL", model.OrderStatusCompleted).
		Where("o.commission_rate_bps IS NOT NULL AND o.commission_amount IS NOT NULL AND o.settlement_amount IS NOT NULL").
		Where("sale_entry.id IS NULL OR (o.commission_amount > 0 AND commission_entry.id IS NULL)").
		Order("o.completed_at ASC, o.id ASC").
		Limit(limit).
		Find(&orders).Error
	return orders, err
}

func (r *settlementRepository) ListSucceededRefundsMissingEntries(ctx context.Context, limit int) ([]model.Refund, error) {
	var refunds []model.Refund
	err := r.db.WithContext(ctx).Table("refunds AS r").
		Select("r.*").
		Joins("LEFT JOIN settlement_entries refund_entry ON refund_entry.entry_no = CONCAT('REFUND-', r.id)").
		Where("r.status = ? AND refund_entry.id IS NULL", model.RefundStatusSucceeded).
		Order("r.updated_at ASC, r.id ASC").
		Limit(limit).
		Find(&refunds).Error
	return refunds, err
}

func (r *settlementRepository) FindOrderByID(ctx context.Context, orderID int64) (*model.Order, error) {
	var order model.Order
	if err := r.db.WithContext(ctx).First(&order, orderID).Error; err != nil {
		return nil, err
	}
	return &order, nil
}

func (r *settlementRepository) FindPaymentAllocationByID(ctx context.Context, allocationID int64) (*model.PaymentAllocation, error) {
	var allocation model.PaymentAllocation
	if err := r.db.WithContext(ctx).First(&allocation, allocationID).Error; err != nil {
		return nil, err
	}
	return &allocation, nil
}

func (r *settlementRepository) SumSucceededRefundsBefore(
	ctx context.Context,
	allocationID int64,
	updatedAt time.Time,
	refundID int64,
) (int64, error) {
	var amount int64
	err := r.db.WithContext(ctx).Model(&model.Refund{}).
		Where("payment_allocation_id = ? AND status = ?", allocationID, model.RefundStatusSucceeded).
		Where("updated_at < ? OR (updated_at = ? AND id < ?)", updatedAt, updatedAt, refundID).
		Select("COALESCE(SUM(amount), 0)").
		Scan(&amount).Error
	return amount, err
}

func (r *settlementRepository) CreateEntries(ctx context.Context, entries []model.SettlementEntry) error {
	if len(entries) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&entries).Error
}

func (r *settlementRepository) FindMerchantByID(ctx context.Context, merchantID int64) (*model.Merchant, error) {
	var merchant model.Merchant
	if err := r.db.WithContext(ctx).Where("id = ?", merchantID).First(&merchant).Error; err != nil {
		return nil, err
	}
	return &merchant, nil
}

func (r *settlementRepository) ListEntriesByMerchantID(
	ctx context.Context,
	merchantID int64,
	entryType string,
	offset, limit int,
) ([]model.SettlementEntry, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.SettlementEntry{}).Where("merchant_id = ?", merchantID)
	if entryType != "" {
		query = query.Where("entry_type = ?", entryType)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var entries []model.SettlementEntry
	if err := query.Order("id DESC").Offset(offset).Limit(limit).Find(&entries).Error; err != nil {
		return nil, 0, err
	}
	return entries, total, nil
}

func (r *settlementRepository) ListSettlementsByMerchantID(
	ctx context.Context,
	merchantID int64,
	status, offset, limit int,
) ([]model.MerchantSettlement, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.MerchantSettlement{}).Where("merchant_id = ?", merchantID)
	if status > 0 {
		query = query.Where("status = ?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var settlements []model.MerchantSettlement
	if err := query.Order("period_end DESC, id DESC").Offset(offset).Limit(limit).Find(&settlements).Error; err != nil {
		return nil, 0, err
	}
	return settlements, total, nil
}

func (r *settlementRepository) FindSettlementByIDAndMerchantID(ctx context.Context, id, merchantID int64) (*model.MerchantSettlement, error) {
	var settlement model.MerchantSettlement
	if err := r.db.WithContext(ctx).Where("id = ? AND merchant_id = ?", id, merchantID).First(&settlement).Error; err != nil {
		return nil, err
	}
	return &settlement, nil
}

func (r *settlementRepository) FindSettlementByPeriod(ctx context.Context, merchantID int64, start, end time.Time) (*model.MerchantSettlement, error) {
	var settlement model.MerchantSettlement
	if err := r.db.WithContext(ctx).
		Where("merchant_id = ? AND period_start = ? AND period_end = ?", merchantID, start, end).
		First(&settlement).Error; err != nil {
		return nil, err
	}
	return &settlement, nil
}

func (r *settlementRepository) FindEntriesBySettlementID(ctx context.Context, merchantID, settlementID int64) ([]model.SettlementEntry, error) {
	var entries []model.SettlementEntry
	err := r.db.WithContext(ctx).
		Where("merchant_id = ? AND settlement_id = ?", merchantID, settlementID).
		Order("id ASC").Find(&entries).Error
	return entries, err
}

func (r *settlementRepository) FindEligibleEntriesForUpdate(
	ctx context.Context,
	merchantID int64,
	periodEnd time.Time,
	completedBefore time.Time,
) ([]model.SettlementEntry, error) {
	var entries []model.SettlementEntry
	err := r.db.WithContext(ctx).Table("settlement_entries AS se").
		Select("se.*").
		Joins("JOIN orders o ON o.id = se.order_id").
		Where("se.merchant_id = ? AND se.settlement_id IS NULL", merchantID).
		Where("se.available_at < ?", periodEnd).
		Where("o.status = ? AND o.completed_at IS NOT NULL AND o.completed_at <= ?", model.OrderStatusCompleted, completedBefore).
		Order("se.id ASC").
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Find(&entries).Error
	return entries, err
}

func (r *settlementRepository) CreateSettlement(ctx context.Context, settlement *model.MerchantSettlement) error {
	return r.db.WithContext(ctx).Create(settlement).Error
}

func (r *settlementRepository) AttachEntries(ctx context.Context, entryIDs []int64, settlementID int64) error {
	if len(entryIDs) == 0 {
		return fmt.Errorf("结算流水不能为空")
	}
	result := r.db.WithContext(ctx).Model(&model.SettlementEntry{}).
		Where("id IN ? AND settlement_id IS NULL", entryIDs).
		Update("settlement_id", settlementID)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != int64(len(entryIDs)) {
		return fmt.Errorf("结算流水已被其他结算任务处理")
	}
	return nil
}
