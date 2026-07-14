package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go-mall/internal/dto"
	"go-mall/internal/model"
	"go-mall/internal/repository"
)

type CouponService interface {
	Available(ctx context.Context, userID int64) ([]dto.CouponResponse, error)
	Claim(ctx context.Context, userID, couponID int64) (*dto.UserCouponResponse, error)
	Mine(ctx context.Context, userID int64, status int) ([]dto.UserCouponResponse, error)
	MerchantList(ctx context.Context, merchantID int64, req dto.CouponListRequest) (*dto.PageResponse[dto.CouponResponse], error)
	MerchantCreate(ctx context.Context, merchantID int64, req dto.CouponRequest) (*dto.CouponResponse, error)
	MerchantUpdate(ctx context.Context, merchantID, id int64, req dto.CouponRequest) (*dto.CouponResponse, error)
}

type couponService struct{ repo repository.CouponRepository }

func NewCouponService(repo repository.CouponRepository) CouponService {
	return &couponService{repo: repo}
}

func couponStatusText(status int) string {
	switch status {
	case model.CouponStatusDraft:
		return "草稿"
	case model.CouponStatusActive:
		return "发放中"
	case model.CouponStatusDisabled:
		return "已停用"
	default:
		return "未知状态"
	}
}
func userCouponStatusText(status int) string {
	switch status {
	case model.UserCouponStatusUnused:
		return "未使用"
	case model.UserCouponStatusUsed:
		return "已使用"
	case model.UserCouponStatusExpired:
		return "已过期"
	default:
		return "未知状态"
	}
}
func couponResponse(value model.Coupon) dto.CouponResponse {
	return dto.CouponResponse{ID: value.ID, MerchantID: value.MerchantID, Name: value.Name, ThresholdAmount: value.ThresholdAmount, DiscountAmount: value.DiscountAmount, TotalQuantity: value.TotalQuantity, ClaimedQuantity: value.ClaimedQuantity, UsedQuantity: value.UsedQuantity, PerUserLimit: value.PerUserLimit, Status: value.Status, StatusText: couponStatusText(value.Status), StartAt: value.StartAt.Format(time.RFC3339), EndAt: value.EndAt.Format(time.RFC3339), CreatedAt: value.CreatedAt.Format(time.RFC3339), UpdatedAt: value.UpdatedAt.Format(time.RFC3339)}
}
func userCouponResponse(value model.UserCoupon, coupon model.Coupon) dto.UserCouponResponse {
	return dto.UserCouponResponse{ID: value.ID, Status: value.Status, StatusText: userCouponStatusText(value.Status), ClaimedAt: value.ClaimedAt.Format(time.RFC3339), UsedAt: stringTime(value.UsedAt), OrderID: value.OrderID, Coupon: couponResponse(coupon)}
}

func (s *couponService) Available(ctx context.Context, userID int64) ([]dto.CouponResponse, error) {
	values, counts, err := s.repo.ListAvailable(ctx, defaultMerchantID, userID, time.Now())
	if err != nil {
		return nil, err
	}
	list := make([]dto.CouponResponse, 0, len(values))
	for _, value := range values {
		item := couponResponse(value)
		item.Claimed = counts[value.ID] >= int64(value.PerUserLimit)
		list = append(list, item)
	}
	return list, nil
}
func (s *couponService) Claim(ctx context.Context, userID, couponID int64) (*dto.UserCouponResponse, error) {
	if userID <= 0 || couponID <= 0 {
		return nil, fmt.Errorf("优惠券参数不合法")
	}
	var userCoupon *model.UserCoupon
	var coupon *model.Coupon
	err := s.repo.Transaction(ctx, func(repo repository.CouponRepository) error {
		value, err := repo.FindForUpdate(ctx, couponID)
		if err != nil {
			return fmt.Errorf("优惠券不存在")
		}
		now := time.Now()
		if value.MerchantID != defaultMerchantID || value.Status != model.CouponStatusActive || now.Before(value.StartAt) || !now.Before(value.EndAt) {
			return fmt.Errorf("优惠券当前不可领取")
		}
		count, err := repo.CountClaimedByUser(ctx, value.ID, userID)
		if err != nil {
			return err
		}
		if count >= int64(value.PerUserLimit) {
			return fmt.Errorf("已达到每人领取上限")
		}
		if err := repo.IncrementClaimed(ctx, value.ID); err != nil {
			return err
		}
		userCoupon = &model.UserCoupon{CouponID: value.ID, UserID: userID, MerchantID: value.MerchantID, Status: model.UserCouponStatusUnused, ClaimedAt: now}
		if err := repo.CreateUserCoupon(ctx, userCoupon); err != nil {
			return err
		}
		coupon = value
		return nil
	})
	if err != nil {
		return nil, err
	}
	result := userCouponResponse(*userCoupon, *coupon)
	return &result, nil
}
func (s *couponService) Mine(ctx context.Context, userID int64, status int) ([]dto.UserCouponResponse, error) {
	if err := s.repo.ExpireUserCoupons(ctx, userID, time.Now()); err != nil {
		return nil, err
	}
	values, err := s.repo.ListUserCoupons(ctx, userID, status)
	if err != nil {
		return nil, err
	}
	list := make([]dto.UserCouponResponse, 0, len(values))
	now := time.Now()
	for _, value := range values {
		coupon, err := s.repo.FindCoupon(ctx, value.CouponID)
		if err != nil {
			return nil, err
		}
		if value.Status == model.UserCouponStatusUnused && !now.Before(coupon.EndAt) {
			value.Status = model.UserCouponStatusExpired
		}
		list = append(list, userCouponResponse(value, *coupon))
	}
	return list, nil
}
func parseCouponRequest(req dto.CouponRequest) (*model.Coupon, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" || len([]rune(name)) > 100 {
		return nil, fmt.Errorf("优惠券名称不能为空且不能超过 100 个字符")
	}
	if req.DiscountAmount <= 0 || req.ThresholdAmount < req.DiscountAmount {
		return nil, fmt.Errorf("优惠金额必须大于 0 且不能超过使用门槛")
	}
	if req.TotalQuantity <= 0 {
		return nil, fmt.Errorf("发行数量必须大于 0")
	}
	if req.PerUserLimit <= 0 {
		req.PerUserLimit = 1
	}
	if req.Status < model.CouponStatusDraft || req.Status > model.CouponStatusDisabled {
		return nil, fmt.Errorf("优惠券状态不合法")
	}
	startAt, err := time.Parse(time.RFC3339, req.StartAt)
	if err != nil {
		return nil, fmt.Errorf("开始时间格式不合法")
	}
	endAt, err := time.Parse(time.RFC3339, req.EndAt)
	if err != nil || !endAt.After(startAt) {
		return nil, fmt.Errorf("结束时间必须晚于开始时间")
	}
	return &model.Coupon{Name: name, ThresholdAmount: req.ThresholdAmount, DiscountAmount: req.DiscountAmount, TotalQuantity: req.TotalQuantity, PerUserLimit: req.PerUserLimit, Status: req.Status, StartAt: startAt, EndAt: endAt}, nil
}
func (s *couponService) MerchantList(ctx context.Context, merchantID int64, req dto.CouponListRequest) (*dto.PageResponse[dto.CouponResponse], error) {
	page, pageSize := normalizePage(req.Page, req.PageSize)
	values, total, err := s.repo.ListByMerchantID(ctx, merchantID, (page-1)*pageSize, pageSize, req.Status)
	if err != nil {
		return nil, err
	}
	list := make([]dto.CouponResponse, 0, len(values))
	for _, value := range values {
		list = append(list, couponResponse(value))
	}
	return &dto.PageResponse[dto.CouponResponse]{List: list, Page: page, PageSize: pageSize, Total: total}, nil
}
func (s *couponService) MerchantCreate(ctx context.Context, merchantID int64, req dto.CouponRequest) (*dto.CouponResponse, error) {
	value, err := parseCouponRequest(req)
	if err != nil {
		return nil, err
	}
	value.MerchantID = merchantID
	if err := s.repo.Create(ctx, value); err != nil {
		return nil, err
	}
	result := couponResponse(*value)
	return &result, nil
}
func (s *couponService) MerchantUpdate(ctx context.Context, merchantID, id int64, req dto.CouponRequest) (*dto.CouponResponse, error) {
	input, err := parseCouponRequest(req)
	if err != nil {
		return nil, err
	}
	value, err := s.repo.FindByIDAndMerchantID(ctx, id, merchantID)
	if err != nil {
		return nil, fmt.Errorf("优惠券不存在")
	}
	if input.TotalQuantity < value.ClaimedQuantity {
		return nil, fmt.Errorf("发行数量不能小于已领取数量")
	}
	if value.ClaimedQuantity > 0 && (input.ThresholdAmount != value.ThresholdAmount || input.DiscountAmount != value.DiscountAmount || input.PerUserLimit != value.PerUserLimit || !input.StartAt.Equal(value.StartAt)) {
		return nil, fmt.Errorf("优惠券已有领取记录，不能修改门槛、优惠金额、限领数量和开始时间")
	}
	value.Name, value.ThresholdAmount, value.DiscountAmount, value.TotalQuantity, value.PerUserLimit, value.Status, value.StartAt, value.EndAt = input.Name, input.ThresholdAmount, input.DiscountAmount, input.TotalQuantity, input.PerUserLimit, input.Status, input.StartAt, input.EndAt
	if err := s.repo.Update(ctx, value); err != nil {
		return nil, err
	}
	result := couponResponse(*value)
	return &result, nil
}
