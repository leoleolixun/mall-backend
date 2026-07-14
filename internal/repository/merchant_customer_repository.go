package repository

import (
	"context"
	"time"

	"go-mall/internal/model"

	"gorm.io/gorm"
)

type MerchantCustomerRecord struct {
	UserID          int64
	Nickname        string
	Avatar          string
	Mobile          string
	UserStatus      int
	UserDeletedAt   *time.Time
	RegisteredAt    time.Time
	PaidOrders      int64
	TotalPaidAmount int64
	FirstPaidAt     time.Time
	LastPaidAt      time.Time
}

type MerchantCustomerOverview struct {
	TotalCustomers     int64
	RepeatCustomers    int64
	NewCustomers30D    int64
	ActiveCustomers30D int64
	TotalPaidAmount    int64
}

type MerchantCustomerRepository interface {
	FindMerchantByID(ctx context.Context, merchantID int64) (*model.Merchant, error)
	Overview(ctx context.Context, merchantID int64, since time.Time) (*MerchantCustomerOverview, error)
	List(ctx context.Context, merchantID int64, offset int, limit int, keyword string, repeatOnly bool) ([]MerchantCustomerRecord, int64, error)
	Find(ctx context.Context, merchantID int64, userID int64) (*MerchantCustomerRecord, error)
	ListRecentOrders(ctx context.Context, merchantID int64, userID int64, limit int) ([]model.Order, error)
}

type merchantCustomerRepository struct {
	db *gorm.DB
}

func NewMerchantCustomerRepository(db *gorm.DB) MerchantCustomerRepository {
	return &merchantCustomerRepository{db: db}
}

func merchantPaidOrderStatsQuery(db *gorm.DB, merchantID int64) *gorm.DB {
	paidStatuses := []int{model.OrderStatusPaid, model.OrderStatusShipped, model.OrderStatusCompleted}
	return db.Table("orders").
		Select(`user_id,
			COUNT(*) AS paid_orders,
			COALESCE(SUM(payable_amount), 0) AS total_paid_amount,
			MIN(paid_at) AS first_paid_at,
			MAX(paid_at) AS last_paid_at`).
		Where("merchant_id = ? AND deleted_at IS NULL AND status IN ? AND paid_at IS NOT NULL", merchantID, paidStatuses).
		Group("user_id")
}

func merchantCustomerQuery(db *gorm.DB, merchantID int64) *gorm.DB {
	stats := merchantPaidOrderStatsQuery(db, merchantID)
	return db.Table("(?) AS customer_stats", stats).
		Joins("JOIN users ON users.id = customer_stats.user_id").
		Select(`users.id AS user_id,
			users.nickname,
			users.avatar,
			users.mobile,
			users.status AS user_status,
			users.deleted_at AS user_deleted_at,
			users.created_at AS registered_at,
			customer_stats.paid_orders,
			customer_stats.total_paid_amount,
			customer_stats.first_paid_at,
			customer_stats.last_paid_at`)
}

func (r *merchantCustomerRepository) FindMerchantByID(ctx context.Context, merchantID int64) (*model.Merchant, error) {
	var merchant model.Merchant
	if err := r.db.WithContext(ctx).Where("id = ?", merchantID).First(&merchant).Error; err != nil {
		return nil, err
	}
	return &merchant, nil
}

func (r *merchantCustomerRepository) Overview(ctx context.Context, merchantID int64, since time.Time) (*MerchantCustomerOverview, error) {
	stats := merchantPaidOrderStatsQuery(r.db.WithContext(ctx), merchantID)
	var result MerchantCustomerOverview
	err := r.db.WithContext(ctx).Table("(?) AS customer_stats", stats).
		Select(`COUNT(*) AS total_customers,
			COALESCE(SUM(CASE WHEN paid_orders >= 2 THEN 1 ELSE 0 END), 0) AS repeat_customers,
			COALESCE(SUM(CASE WHEN first_paid_at >= ? THEN 1 ELSE 0 END), 0) AS new_customers30_d,
			COALESCE(SUM(CASE WHEN last_paid_at >= ? THEN 1 ELSE 0 END), 0) AS active_customers30_d,
			COALESCE(SUM(total_paid_amount), 0) AS total_paid_amount`, since, since).
		Scan(&result).Error
	return &result, err
}

func (r *merchantCustomerRepository) List(
	ctx context.Context,
	merchantID int64,
	offset int,
	limit int,
	keyword string,
	repeatOnly bool,
) ([]MerchantCustomerRecord, int64, error) {
	query := merchantCustomerQuery(r.db.WithContext(ctx), merchantID)
	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("users.nickname LIKE ? OR users.mobile LIKE ?", like, like)
	}
	if repeatOnly {
		query = query.Where("customer_stats.paid_orders >= 2")
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var records []MerchantCustomerRecord
	if err := query.Order("customer_stats.last_paid_at DESC, users.id DESC").Offset(offset).Limit(limit).Scan(&records).Error; err != nil {
		return nil, 0, err
	}
	return records, total, nil
}

func (r *merchantCustomerRepository) Find(ctx context.Context, merchantID int64, userID int64) (*MerchantCustomerRecord, error) {
	var record MerchantCustomerRecord
	if err := merchantCustomerQuery(r.db.WithContext(ctx), merchantID).
		Where("customer_stats.user_id = ?", userID).
		Take(&record).Error; err != nil {
		return nil, err
	}
	return &record, nil
}

func (r *merchantCustomerRepository) ListRecentOrders(ctx context.Context, merchantID int64, userID int64, limit int) ([]model.Order, error) {
	var orders []model.Order
	err := r.db.WithContext(ctx).
		Where("merchant_id = ? AND user_id = ?", merchantID, userID).
		Order("id DESC").
		Limit(limit).
		Find(&orders).Error
	return orders, err
}
