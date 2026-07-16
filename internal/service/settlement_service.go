package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"go-mall/internal/config"
	"go-mall/internal/dto"
	"go-mall/internal/model"
	"go-mall/internal/repository"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type SettlementAccrualReport struct {
	Scanned int
	Accrued int
	Skipped int
}

type SettlementService interface {
	AccrueCompletedOrders(ctx context.Context) (SettlementAccrualReport, error)
	Generate(ctx context.Context, merchantID int64, periodStart, periodEnd time.Time) (*dto.MerchantSettlementResponse, error)
	ListEntries(ctx context.Context, merchantID int64, req dto.SettlementEntryListRequest) (*dto.PageResponse[dto.SettlementEntryResponse], error)
	List(ctx context.Context, merchantID int64, req dto.MerchantSettlementListRequest) (*dto.PageResponse[dto.MerchantSettlementResponse], error)
	Detail(ctx context.Context, merchantID, settlementID int64) (*dto.MerchantSettlementResponse, error)
}

type settlementService struct {
	repo      repository.SettlementRepository
	hold      time.Duration
	batchSize int
	now       func() time.Time
}

func NewSettlementService(repo repository.SettlementRepository, cfg config.SettlementConfig) SettlementService {
	holdDays := cfg.HoldDays
	if holdDays < 0 {
		holdDays = 0
	}
	batchSize := cfg.BatchSize
	if batchSize <= 0 {
		batchSize = 100
	}
	if batchSize > 1000 {
		batchSize = 1000
	}
	return &settlementService{
		repo: repo, hold: time.Duration(holdDays) * 24 * time.Hour,
		batchSize: batchSize, now: time.Now,
	}
}

func (s *settlementService) AccrueCompletedOrders(ctx context.Context) (SettlementAccrualReport, error) {
	var report SettlementAccrualReport
	orders, err := s.repo.ListCompletedOrdersMissingEntries(ctx, s.batchSize)
	if err != nil {
		return report, err
	}
	report.Scanned = len(orders)
	for _, order := range orders {
		if order.CompletedAt == nil || order.CommissionRateBPS == nil || order.CommissionAmount == nil || order.SettlementAmount == nil {
			report.Skipped++
			continue
		}
		if *order.CommissionRateBPS < 0 || *order.CommissionRateBPS > 10000 ||
			*order.CommissionAmount < 0 || *order.SettlementAmount < 0 ||
			*order.CommissionAmount+*order.SettlementAmount != order.PayableAmount {
			return report, fmt.Errorf("订单 %d 结算金额快照不一致", order.ID)
		}
		orderID := order.ID
		availableAt := order.CompletedAt.Add(s.hold)
		entries := []model.SettlementEntry{{
			EntryNo: fmt.Sprintf("SALE-%d", order.ID), MerchantID: order.MerchantID,
			OrderID: &orderID, EntryType: model.SettlementEntrySale,
			Amount: order.PayableAmount, AvailableAt: availableAt,
		}}
		if *order.CommissionAmount > 0 {
			entries = append(entries, model.SettlementEntry{
				EntryNo: fmt.Sprintf("COMMISSION-%d", order.ID), MerchantID: order.MerchantID,
				OrderID: &orderID, EntryType: model.SettlementEntryCommission,
				Amount: -*order.CommissionAmount, AvailableAt: availableAt,
			})
		}
		if err := s.repo.CreateEntries(ctx, entries); err != nil {
			return report, fmt.Errorf("订单 %d 生成结算流水失败: %w", order.ID, err)
		}
		report.Accrued++
	}

	refunds, err := s.repo.ListSucceededRefundsMissingEntries(ctx, s.batchSize)
	if err != nil {
		return report, err
	}
	report.Scanned += len(refunds)
	for _, refund := range refunds {
		if err := s.accrueSucceededRefund(ctx, refund); err != nil {
			return report, fmt.Errorf("退款 %d 生成结算流水失败: %w", refund.ID, err)
		}
		report.Accrued++
	}
	return report, nil
}

func (s *settlementService) accrueSucceededRefund(ctx context.Context, refund model.Refund) error {
	if refund.PaymentAllocationID == nil || refund.TradeID == nil {
		return fmt.Errorf("缺少交易支付分配，请先完成 M8 历史回填")
	}
	if refund.Amount <= 0 {
		return fmt.Errorf("退款金额不合法")
	}
	order, err := s.repo.FindOrderByID(ctx, refund.OrderID)
	if err != nil {
		return fmt.Errorf("退款子订单不存在")
	}
	allocation, err := s.repo.FindPaymentAllocationByID(ctx, *refund.PaymentAllocationID)
	if err != nil {
		return fmt.Errorf("退款支付分配不存在")
	}
	if order.MerchantID != refund.MerchantID || order.TradeID == nil || *order.TradeID != *refund.TradeID ||
		allocation.PaymentID != refund.PaymentID || allocation.TradeID != *refund.TradeID ||
		allocation.OrderID != order.ID || allocation.MerchantID != refund.MerchantID ||
		allocation.Amount != order.PayableAmount || allocation.RefundedAmount < refund.Amount ||
		order.CommissionAmount == nil || order.SettlementAmount == nil ||
		*order.CommissionAmount+*order.SettlementAmount != order.PayableAmount {
		return fmt.Errorf("退款、支付分配与订单结算快照不一致")
	}
	eventOrderAt := refund.UpdatedAt
	if eventOrderAt.IsZero() {
		eventOrderAt = refund.CreatedAt
	}
	if eventOrderAt.IsZero() {
		return fmt.Errorf("退款缺少可排序时间")
	}
	refundedBefore, err := s.repo.SumSucceededRefundsBefore(ctx, allocation.ID, eventOrderAt, refund.ID)
	if err != nil {
		return err
	}
	refundedAfter := refundedBefore + refund.Amount
	if refundedBefore < 0 || refundedAfter < refundedBefore || refundedAfter > allocation.Amount ||
		refundedAfter > allocation.RefundedAmount {
		return fmt.Errorf("退款累计金额超过支付分配")
	}
	commissionBefore, err := proportionalRefundCommission(*order.CommissionAmount, refundedBefore, allocation.Amount)
	if err != nil {
		return err
	}
	commissionAfter, err := proportionalRefundCommission(*order.CommissionAmount, refundedAfter, allocation.Amount)
	if err != nil {
		return err
	}
	availableAt := eventOrderAt
	if refund.RefundedAt != nil && !refund.RefundedAt.IsZero() {
		availableAt = *refund.RefundedAt
	}
	orderID := order.ID
	refundID := refund.ID
	entries := []model.SettlementEntry{{
		EntryNo: fmt.Sprintf("REFUND-%d", refund.ID), MerchantID: refund.MerchantID,
		OrderID: &orderID, RefundID: &refundID, EntryType: model.SettlementEntryRefund,
		Amount: -refund.Amount, AvailableAt: availableAt,
	}}
	if commissionRefund := commissionAfter - commissionBefore; commissionRefund > 0 {
		entries = append(entries, model.SettlementEntry{
			EntryNo: fmt.Sprintf("COMMISSION-REFUND-%d", refund.ID), MerchantID: refund.MerchantID,
			OrderID: &orderID, RefundID: &refundID, EntryType: model.SettlementEntryCommissionRefund,
			Amount: commissionRefund, AvailableAt: availableAt,
		})
	}
	return s.repo.CreateEntries(ctx, entries)
}

func (s *settlementService) Generate(
	ctx context.Context,
	merchantID int64,
	periodStart, periodEnd time.Time,
) (*dto.MerchantSettlementResponse, error) {
	if merchantID <= 0 {
		return nil, fmt.Errorf("商户 ID 不合法")
	}
	periodStart = periodStart.UTC().Truncate(time.Millisecond)
	periodEnd = periodEnd.UTC().Truncate(time.Millisecond)
	if periodStart.IsZero() || !periodEnd.After(periodStart) {
		return nil, fmt.Errorf("结算周期不合法")
	}
	if periodEnd.After(s.now().UTC().Add(time.Minute)) {
		return nil, fmt.Errorf("结算周期不能晚于当前时间")
	}
	if _, err := s.repo.FindMerchantByID(ctx, merchantID); err != nil {
		return nil, fmt.Errorf("商户不存在")
	}
	if existing, err := s.repo.FindSettlementByPeriod(ctx, merchantID, periodStart, periodEnd); err == nil {
		return s.Detail(ctx, merchantID, existing.ID)
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	var created *model.MerchantSettlement
	err := s.repo.Transaction(ctx, func(repo repository.SettlementRepository) error {
		if existing, err := repo.FindSettlementByPeriod(ctx, merchantID, periodStart, periodEnd); err == nil {
			created = existing
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		entries, err := repo.FindEligibleEntriesForUpdate(ctx, merchantID, periodEnd, periodEnd.Add(-s.hold))
		if err != nil {
			return err
		}
		if len(entries) == 0 {
			return fmt.Errorf("当前周期没有可结算流水")
		}
		var grossAmount, commissionAmount, refundAmount, adjustmentAmount, netAmount int64
		entryIDs := make([]int64, 0, len(entries))
		for _, entry := range entries {
			entryIDs = append(entryIDs, entry.ID)
			netAmount += entry.Amount
			switch entry.EntryType {
			case model.SettlementEntrySale:
				if entry.Amount <= 0 {
					return fmt.Errorf("销售流水 %d 金额不合法", entry.ID)
				}
				grossAmount += entry.Amount
			case model.SettlementEntryCommission:
				if entry.Amount > 0 {
					return fmt.Errorf("佣金流水 %d 金额不合法", entry.ID)
				}
				commissionAmount -= entry.Amount
			case model.SettlementEntryRefund:
				if entry.Amount > 0 {
					return fmt.Errorf("退款流水 %d 金额不合法", entry.ID)
				}
				refundAmount -= entry.Amount
			case model.SettlementEntryCommissionRefund:
				if entry.Amount < 0 {
					return fmt.Errorf("佣金冲正流水 %d 金额不合法", entry.ID)
				}
				commissionAmount -= entry.Amount
			case model.SettlementEntryAdjustment:
				adjustmentAmount += entry.Amount
			default:
				return fmt.Errorf("未知结算流水类型 %q", entry.EntryType)
			}
		}
		if commissionAmount < 0 || netAmount != grossAmount-commissionAmount-refundAmount+adjustmentAmount {
			return fmt.Errorf("结算金额不满足对账等式")
		}
		settlement := &model.MerchantSettlement{
			SettlementNo: generateSettlementNo(), MerchantID: merchantID,
			PeriodStart: periodStart, PeriodEnd: periodEnd,
			GrossAmount: grossAmount, CommissionAmount: commissionAmount,
			RefundAmount: refundAmount, NetAmount: netAmount,
			Status: model.SettlementStatusPending,
		}
		if err := repo.CreateSettlement(ctx, settlement); err != nil {
			return err
		}
		if err := repo.AttachEntries(ctx, entryIDs, settlement.ID); err != nil {
			return err
		}
		created = settlement
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.Detail(ctx, merchantID, created.ID)
}

func generateSettlementNo() string {
	return fmt.Sprintf("S%s%s", time.Now().Format("20060102150405"), uuid.NewString()[:8])
}

func (s *settlementService) ListEntries(
	ctx context.Context,
	merchantID int64,
	req dto.SettlementEntryListRequest,
) (*dto.PageResponse[dto.SettlementEntryResponse], error) {
	entryType := strings.TrimSpace(req.EntryType)
	if entryType != "" && !validSettlementEntryType(entryType) {
		return nil, fmt.Errorf("结算流水类型不合法")
	}
	page, pageSize := normalizeOrderPage(req.Page, req.PageSize)
	values, total, err := s.repo.ListEntriesByMerchantID(ctx, merchantID, entryType, (page-1)*pageSize, pageSize)
	if err != nil {
		return nil, err
	}
	list := make([]dto.SettlementEntryResponse, 0, len(values))
	for _, value := range values {
		list = append(list, toSettlementEntryResponse(value))
	}
	return &dto.PageResponse[dto.SettlementEntryResponse]{List: list, Page: page, PageSize: pageSize, Total: total}, nil
}

func (s *settlementService) List(
	ctx context.Context,
	merchantID int64,
	req dto.MerchantSettlementListRequest,
) (*dto.PageResponse[dto.MerchantSettlementResponse], error) {
	if req.Status < 0 || req.Status > model.SettlementStatusPaid {
		return nil, fmt.Errorf("结算状态不合法")
	}
	page, pageSize := normalizeOrderPage(req.Page, req.PageSize)
	values, total, err := s.repo.ListSettlementsByMerchantID(ctx, merchantID, req.Status, (page-1)*pageSize, pageSize)
	if err != nil {
		return nil, err
	}
	list := make([]dto.MerchantSettlementResponse, 0, len(values))
	for _, value := range values {
		list = append(list, toMerchantSettlementResponse(value, nil))
	}
	return &dto.PageResponse[dto.MerchantSettlementResponse]{List: list, Page: page, PageSize: pageSize, Total: total}, nil
}

func (s *settlementService) Detail(ctx context.Context, merchantID, settlementID int64) (*dto.MerchantSettlementResponse, error) {
	if merchantID <= 0 || settlementID <= 0 {
		return nil, fmt.Errorf("结算单 ID 不合法")
	}
	value, err := s.repo.FindSettlementByIDAndMerchantID(ctx, settlementID, merchantID)
	if err != nil {
		return nil, fmt.Errorf("结算单不存在")
	}
	entries, err := s.repo.FindEntriesBySettlementID(ctx, merchantID, settlementID)
	if err != nil {
		return nil, err
	}
	return ptrMerchantSettlementResponse(toMerchantSettlementResponse(*value, entries)), nil
}

func validSettlementEntryType(value string) bool {
	switch value {
	case model.SettlementEntrySale, model.SettlementEntryCommission, model.SettlementEntryRefund,
		model.SettlementEntryCommissionRefund, model.SettlementEntryAdjustment:
		return true
	default:
		return false
	}
}

func settlementStatusText(status int) string {
	switch status {
	case model.SettlementStatusPending:
		return "待确认"
	case model.SettlementStatusConfirmed:
		return "已确认"
	case model.SettlementStatusPaid:
		return "已打款"
	default:
		return "未知状态"
	}
}

func toSettlementEntryResponse(value model.SettlementEntry) dto.SettlementEntryResponse {
	return dto.SettlementEntryResponse{
		ID: value.ID, EntryNo: value.EntryNo, MerchantID: value.MerchantID,
		OrderID: value.OrderID, RefundID: value.RefundID, EntryType: value.EntryType,
		Amount: value.Amount, AvailableAt: value.AvailableAt.Format(time.RFC3339),
		SettlementID: value.SettlementID, CreatedAt: value.CreatedAt.Format(time.RFC3339),
	}
}

func toMerchantSettlementResponse(value model.MerchantSettlement, entries []model.SettlementEntry) dto.MerchantSettlementResponse {
	entryResponses := make([]dto.SettlementEntryResponse, 0, len(entries))
	for _, entry := range entries {
		entryResponses = append(entryResponses, toSettlementEntryResponse(entry))
	}
	return dto.MerchantSettlementResponse{
		ID: value.ID, SettlementNo: value.SettlementNo, MerchantID: value.MerchantID,
		PeriodStart: value.PeriodStart.Format(time.RFC3339), PeriodEnd: value.PeriodEnd.Format(time.RFC3339),
		GrossAmount: value.GrossAmount, CommissionAmount: value.CommissionAmount,
		RefundAmount: value.RefundAmount, NetAmount: value.NetAmount,
		Status: value.Status, StatusText: settlementStatusText(value.Status),
		ConfirmedAt: stringTime(value.ConfirmedAt), PaidAt: stringTime(value.PaidAt),
		CreatedAt: value.CreatedAt.Format(time.RFC3339), UpdatedAt: value.UpdatedAt.Format(time.RFC3339),
		Entries: entryResponses,
	}
}

func ptrMerchantSettlementResponse(value dto.MerchantSettlementResponse) *dto.MerchantSettlementResponse {
	return &value
}
