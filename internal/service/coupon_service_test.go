package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"go-mall/internal/dto"
	"go-mall/internal/model"
	"go-mall/internal/repository"
)

type fakeCouponRepository struct {
	coupons           map[int64]model.Coupon
	userCoupons       []model.UserCoupon
	nextUserCouponID  int64
	transactionCalls  int
	findForUpdate     int
	disabledMerchants map[int64]bool
}

func (r *fakeCouponRepository) Transaction(_ context.Context, fn func(repository.CouponRepository) error) error {
	r.transactionCalls++
	return fn(r)
}

func (r *fakeCouponRepository) Create(_ context.Context, coupon *model.Coupon) error {
	if coupon.ID == 0 {
		coupon.ID = int64(len(r.coupons) + 1)
	}
	r.coupons[coupon.ID] = *coupon
	return nil
}

func (r *fakeCouponRepository) Update(_ context.Context, coupon *model.Coupon) error {
	r.coupons[coupon.ID] = *coupon
	return nil
}

func (r *fakeCouponRepository) FindByIDAndMerchantID(_ context.Context, id, merchantID int64) (*model.Coupon, error) {
	coupon, ok := r.coupons[id]
	if !ok || coupon.MerchantID != merchantID {
		return nil, fmt.Errorf("not found")
	}
	return &coupon, nil
}

func (r *fakeCouponRepository) FindForUpdate(_ context.Context, id int64) (*model.Coupon, error) {
	r.findForUpdate++
	coupon, ok := r.coupons[id]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	return &coupon, nil
}

func (r *fakeCouponRepository) IsMerchantEnabled(_ context.Context, merchantID int64) (bool, error) {
	return merchantID > 0 && !r.disabledMerchants[merchantID], nil
}

func (r *fakeCouponRepository) ListByMerchantID(_ context.Context, merchantID int64, offset, limit, status int) ([]model.Coupon, int64, error) {
	values := make([]model.Coupon, 0)
	for _, coupon := range r.coupons {
		if coupon.MerchantID == merchantID && (status < 0 || coupon.Status == status) {
			values = append(values, coupon)
		}
	}
	total := int64(len(values))
	if offset >= len(values) {
		return []model.Coupon{}, total, nil
	}
	end := offset + limit
	if end > len(values) {
		end = len(values)
	}
	return values[offset:end], total, nil
}

func (r *fakeCouponRepository) ListAvailable(_ context.Context, merchantID, userID int64, now time.Time) ([]model.Coupon, map[int64]int64, error) {
	values := make([]model.Coupon, 0)
	counts := make(map[int64]int64)
	for _, coupon := range r.coupons {
		if coupon.MerchantID == merchantID && coupon.Status == model.CouponStatusActive && !now.Before(coupon.StartAt) && now.Before(coupon.EndAt) && coupon.ClaimedQuantity < coupon.TotalQuantity {
			values = append(values, coupon)
		}
	}
	for _, userCoupon := range r.userCoupons {
		if userCoupon.UserID == userID {
			counts[userCoupon.CouponID]++
		}
	}
	return values, counts, nil
}

func (r *fakeCouponRepository) CountClaimedByUser(_ context.Context, couponID, userID int64) (int64, error) {
	var count int64
	for _, userCoupon := range r.userCoupons {
		if userCoupon.CouponID == couponID && userCoupon.UserID == userID {
			count++
		}
	}
	return count, nil
}

func (r *fakeCouponRepository) IncrementClaimed(_ context.Context, couponID int64) error {
	coupon, ok := r.coupons[couponID]
	if !ok || coupon.ClaimedQuantity >= coupon.TotalQuantity {
		return fmt.Errorf("优惠券已领完")
	}
	coupon.ClaimedQuantity++
	r.coupons[couponID] = coupon
	return nil
}

func (r *fakeCouponRepository) CreateUserCoupon(_ context.Context, userCoupon *model.UserCoupon) error {
	r.nextUserCouponID++
	userCoupon.ID = r.nextUserCouponID
	r.userCoupons = append(r.userCoupons, *userCoupon)
	return nil
}

func (r *fakeCouponRepository) ListUserCoupons(_ context.Context, userID int64, status int) ([]model.UserCoupon, error) {
	values := make([]model.UserCoupon, 0)
	for _, value := range r.userCoupons {
		if value.UserID == userID && (status <= 0 || value.Status == status) {
			values = append(values, value)
		}
	}
	return values, nil
}

func (r *fakeCouponRepository) ExpireUserCoupons(_ context.Context, userID int64, now time.Time) error {
	for index, value := range r.userCoupons {
		coupon := r.coupons[value.CouponID]
		if value.UserID == userID && value.Status == model.UserCouponStatusUnused && !now.Before(coupon.EndAt) {
			r.userCoupons[index].Status = model.UserCouponStatusExpired
		}
	}
	return nil
}

func (r *fakeCouponRepository) FindCoupon(_ context.Context, id int64) (*model.Coupon, error) {
	coupon, ok := r.coupons[id]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	return &coupon, nil
}

func activeCoupon(now time.Time) model.Coupon {
	now = now.Truncate(time.Second)
	return model.Coupon{
		ID: 1, MerchantID: defaultMerchantID, Name: "满 100 减 10",
		ThresholdAmount: 10000, DiscountAmount: 1000, TotalQuantity: 10,
		PerUserLimit: 1, Status: model.CouponStatusActive,
		StartAt: now.Add(-time.Hour), EndAt: now.Add(time.Hour),
		CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Hour),
	}
}

func newCouponServiceForTest(now time.Time) (*couponService, *fakeCouponRepository) {
	coupon := activeCoupon(now)
	repo := &fakeCouponRepository{coupons: map[int64]model.Coupon{coupon.ID: coupon}}
	return NewCouponService(repo).(*couponService), repo
}

func TestCouponClaimUpdatesResponseAndStoredQuantity(t *testing.T) {
	now := time.Now()
	service, repo := newCouponServiceForTest(now)

	result, err := service.Claim(context.Background(), 7, 1)
	if err != nil {
		t.Fatalf("claim coupon: %v", err)
	}
	if result.ID == 0 || result.Status != model.UserCouponStatusUnused {
		t.Fatalf("unexpected user coupon: %+v", result)
	}
	if result.Coupon.ClaimedQuantity != 1 || repo.coupons[1].ClaimedQuantity != 1 {
		t.Fatalf("claim quantity is inconsistent: response=%d stored=%d", result.Coupon.ClaimedQuantity, repo.coupons[1].ClaimedQuantity)
	}
	if repo.transactionCalls != 1 || repo.findForUpdate != 1 {
		t.Fatalf("claim must lock in a transaction: transactions=%d locks=%d", repo.transactionCalls, repo.findForUpdate)
	}
}

func TestCouponClaimRejectsPerUserLimit(t *testing.T) {
	now := time.Now()
	service, repo := newCouponServiceForTest(now)
	if _, err := service.Claim(context.Background(), 7, 1); err != nil {
		t.Fatalf("first claim: %v", err)
	}
	if _, err := service.Claim(context.Background(), 7, 1); err == nil {
		t.Fatal("expected second claim to exceed per-user limit")
	}
	if repo.coupons[1].ClaimedQuantity != 1 || len(repo.userCoupons) != 1 {
		t.Fatalf("rejected claim changed state: coupon=%+v user_coupons=%d", repo.coupons[1], len(repo.userCoupons))
	}
}

func TestCouponAvailableAndClaimSupportNonDefaultMerchant(t *testing.T) {
	now := time.Now()
	service, repo := newCouponServiceForTest(now)
	coupon := activeCoupon(now)
	coupon.ID = 2
	coupon.MerchantID = 2
	repo.coupons[coupon.ID] = coupon

	available, err := service.Available(context.Background(), 7, 2)
	if err != nil {
		t.Fatalf("list merchant coupons: %v", err)
	}
	if len(available) != 1 || available[0].MerchantID != 2 {
		t.Fatalf("unexpected merchant coupons: %+v", available)
	}
	claimed, err := service.Claim(context.Background(), 7, coupon.ID)
	if err != nil {
		t.Fatalf("claim non-default merchant coupon: %v", err)
	}
	if claimed.Coupon.MerchantID != 2 {
		t.Fatalf("unexpected claimed coupon: %+v", claimed)
	}
}

func TestCouponClaimRejectsDisabledMerchant(t *testing.T) {
	now := time.Now()
	service, repo := newCouponServiceForTest(now)
	repo.disabledMerchants = map[int64]bool{defaultMerchantID: true}

	if _, err := service.Claim(context.Background(), 7, 1); err == nil {
		t.Fatal("expected disabled merchant coupon claim to fail")
	}
	if repo.coupons[1].ClaimedQuantity != 0 || len(repo.userCoupons) != 0 {
		t.Fatal("rejected claim changed coupon state")
	}
}

func TestCouponMerchantUpdateLocksAndPreservesClaimedQuantity(t *testing.T) {
	now := time.Now()
	service, repo := newCouponServiceForTest(now)
	coupon := repo.coupons[1]
	coupon.ClaimedQuantity = 3
	repo.coupons[1] = coupon

	result, err := service.MerchantUpdate(context.Background(), defaultMerchantID, 1, dto.CouponRequest{
		Name: "暑期满减券", ThresholdAmount: coupon.ThresholdAmount, DiscountAmount: coupon.DiscountAmount,
		TotalQuantity: 20, PerUserLimit: coupon.PerUserLimit, Status: model.CouponStatusDisabled,
		StartAt: coupon.StartAt.Format(time.RFC3339), EndAt: now.Add(48 * time.Hour).Format(time.RFC3339),
	})
	if err != nil {
		t.Fatalf("update coupon: %v", err)
	}
	if result.ClaimedQuantity != 3 || repo.coupons[1].ClaimedQuantity != 3 || result.TotalQuantity != 20 {
		t.Fatalf("unexpected updated coupon: %+v", result)
	}
	if repo.transactionCalls != 1 || repo.findForUpdate != 1 {
		t.Fatalf("update must lock in a transaction: transactions=%d locks=%d", repo.transactionCalls, repo.findForUpdate)
	}
}

func TestCouponMerchantUpdateRejectsTermsAfterClaim(t *testing.T) {
	now := time.Now()
	service, repo := newCouponServiceForTest(now)
	coupon := repo.coupons[1]
	coupon.ClaimedQuantity = 1
	repo.coupons[1] = coupon

	_, err := service.MerchantUpdate(context.Background(), defaultMerchantID, 1, dto.CouponRequest{
		Name: coupon.Name, ThresholdAmount: coupon.ThresholdAmount, DiscountAmount: coupon.DiscountAmount + 100,
		TotalQuantity: coupon.TotalQuantity, PerUserLimit: coupon.PerUserLimit, Status: coupon.Status,
		StartAt: coupon.StartAt.Format(time.RFC3339), EndAt: coupon.EndAt.Format(time.RFC3339),
	})
	if err == nil {
		t.Fatal("expected claimed coupon terms to be immutable")
	}
}

func TestCouponMerchantUpdateRejectsShorterEndTimeAfterClaim(t *testing.T) {
	now := time.Now()
	service, repo := newCouponServiceForTest(now)
	coupon := repo.coupons[1]
	coupon.ClaimedQuantity = 1
	repo.coupons[1] = coupon

	_, err := service.MerchantUpdate(context.Background(), defaultMerchantID, 1, dto.CouponRequest{
		Name: coupon.Name, ThresholdAmount: coupon.ThresholdAmount, DiscountAmount: coupon.DiscountAmount,
		TotalQuantity: coupon.TotalQuantity, PerUserLimit: coupon.PerUserLimit, Status: coupon.Status,
		StartAt: coupon.StartAt.Format(time.RFC3339), EndAt: coupon.EndAt.Add(-time.Minute).Format(time.RFC3339),
	})
	if err == nil {
		t.Fatal("expected claimed coupon end time not to be shortened")
	}
}
