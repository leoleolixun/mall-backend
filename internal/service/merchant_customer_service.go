package service

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"go-mall/internal/dto"
	"go-mall/internal/model"
	"go-mall/internal/repository"
)

type MerchantCustomerService interface {
	Overview(ctx context.Context, merchantID int64) (*dto.MerchantCustomerOverviewResponse, error)
	List(ctx context.Context, merchantID int64, req dto.MerchantCustomerListRequest) (*dto.PageResponse[dto.MerchantCustomerListItem], error)
	Detail(ctx context.Context, merchantID int64, userID int64) (*dto.MerchantCustomerDetailResponse, error)
}

type merchantCustomerService struct {
	repo repository.MerchantCustomerRepository
	now  func() time.Time
}

func NewMerchantCustomerService(repo repository.MerchantCustomerRepository) MerchantCustomerService {
	return &merchantCustomerService{repo: repo, now: time.Now}
}

func (s *merchantCustomerService) validateMerchant(ctx context.Context, merchantID int64) error {
	if merchantID <= 0 {
		return fmt.Errorf("商户身份不合法")
	}
	merchant, err := s.repo.FindMerchantByID(ctx, merchantID)
	if err != nil || merchant.Status != model.StatusEnabled {
		return fmt.Errorf("商户不可用")
	}
	return nil
}

func maskCustomerMobile(mobile string) string {
	mobile = strings.TrimSpace(mobile)
	length := utf8.RuneCountInString(mobile)
	if length == 0 {
		return ""
	}
	runes := []rune(mobile)
	if length <= 4 {
		return strings.Repeat("*", length)
	}
	if length <= 7 {
		return string(runes[:2]) + strings.Repeat("*", length-4) + string(runes[length-2:])
	}
	return string(runes[:3]) + strings.Repeat("*", length-7) + string(runes[length-4:])
}

func toMerchantCustomerListItem(record repository.MerchantCustomerRecord) dto.MerchantCustomerListItem {
	nickname := strings.TrimSpace(record.Nickname)
	avatar := record.Avatar
	mobile := record.Mobile
	userStatus := record.UserStatus
	if record.UserDeletedAt != nil {
		nickname = "已注销用户"
		avatar = ""
		mobile = ""
		userStatus = model.StatusDisabled
	}
	if nickname == "" {
		nickname = fmt.Sprintf("用户 %d", record.UserID)
	}
	return dto.MerchantCustomerListItem{
		UserID: record.UserID, Nickname: nickname, Avatar: avatar,
		MobileMasked: maskCustomerMobile(mobile), UserStatus: userStatus,
		PaidOrders: record.PaidOrders, TotalPaidAmount: record.TotalPaidAmount,
		FirstPaidAt: record.FirstPaidAt.Format(time.RFC3339), LastPaidAt: record.LastPaidAt.Format(time.RFC3339),
		RegisteredAt: record.RegisteredAt.Format(time.RFC3339), IsRepeat: record.PaidOrders >= 2,
	}
}

func (s *merchantCustomerService) Overview(ctx context.Context, merchantID int64) (*dto.MerchantCustomerOverviewResponse, error) {
	if err := s.validateMerchant(ctx, merchantID); err != nil {
		return nil, err
	}
	now := s.now()
	since := now.AddDate(0, 0, -30)
	overview, err := s.repo.Overview(ctx, merchantID, since)
	if err != nil {
		return nil, err
	}
	var repeatRateBPS int64
	var averagePaidAmount int64
	if overview.TotalCustomers > 0 {
		repeatRateBPS = overview.RepeatCustomers * 10000 / overview.TotalCustomers
		averagePaidAmount = overview.TotalPaidAmount / overview.TotalCustomers
	}
	return &dto.MerchantCustomerOverviewResponse{
		TotalCustomers: overview.TotalCustomers, RepeatCustomers: overview.RepeatCustomers,
		RepeatRateBPS: repeatRateBPS, NewCustomers30D: overview.NewCustomers30D,
		ActiveCustomers30D: overview.ActiveCustomers30D, TotalPaidAmount: overview.TotalPaidAmount,
		AveragePaidAmount: averagePaidAmount, GeneratedAt: now.Format(time.RFC3339),
	}, nil
}

func (s *merchantCustomerService) List(
	ctx context.Context,
	merchantID int64,
	req dto.MerchantCustomerListRequest,
) (*dto.PageResponse[dto.MerchantCustomerListItem], error) {
	if err := s.validateMerchant(ctx, merchantID); err != nil {
		return nil, err
	}
	keyword := strings.TrimSpace(req.Keyword)
	if utf8.RuneCountInString(keyword) > 100 {
		return nil, fmt.Errorf("keyword 不能超过 100 个字符")
	}
	page, pageSize := normalizeMerchantProductPage(req.Page, req.PageSize)
	records, total, err := s.repo.List(ctx, merchantID, (page-1)*pageSize, pageSize, keyword, req.RepeatOnly)
	if err != nil {
		return nil, err
	}
	list := make([]dto.MerchantCustomerListItem, 0, len(records))
	for _, record := range records {
		list = append(list, toMerchantCustomerListItem(record))
	}
	return &dto.PageResponse[dto.MerchantCustomerListItem]{List: list, Page: page, PageSize: pageSize, Total: total}, nil
}

func (s *merchantCustomerService) Detail(ctx context.Context, merchantID int64, userID int64) (*dto.MerchantCustomerDetailResponse, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("顾客 ID 不合法")
	}
	if err := s.validateMerchant(ctx, merchantID); err != nil {
		return nil, err
	}
	record, err := s.repo.Find(ctx, merchantID, userID)
	if err != nil {
		return nil, fmt.Errorf("顾客不存在")
	}
	orders, err := s.repo.ListRecentOrders(ctx, merchantID, userID, 10)
	if err != nil {
		return nil, err
	}
	recentOrders := make([]dto.MerchantCustomerOrderResponse, 0, len(orders))
	for _, order := range orders {
		paidAt := ""
		if order.PaidAt != nil {
			paidAt = order.PaidAt.Format(time.RFC3339)
		}
		recentOrders = append(recentOrders, dto.MerchantCustomerOrderResponse{
			ID: order.ID, OrderNo: order.OrderNo, Status: order.Status, StatusText: orderStatusText(order.Status),
			PayableAmount: order.PayableAmount, PaidAt: paidAt, CreatedAt: order.CreatedAt.Format(time.RFC3339),
		})
	}
	return &dto.MerchantCustomerDetailResponse{
		Customer: toMerchantCustomerListItem(*record), RecentOrders: recentOrders,
	}, nil
}
