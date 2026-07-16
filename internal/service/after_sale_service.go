package service

import (
	"context"
	"encoding/json"
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

type AfterSaleService interface {
	Create(ctx context.Context, userID int64, req dto.CreateAfterSaleRequest) (*dto.AfterSaleResponse, error)
	List(ctx context.Context, userID int64, req dto.AfterSaleListRequest) (*dto.PageResponse[dto.AfterSaleResponse], error)
	Detail(ctx context.Context, userID, id int64) (*dto.AfterSaleResponse, error)
	Cancel(ctx context.Context, userID, id int64) error
	MerchantList(ctx context.Context, merchantID int64, req dto.AfterSaleListRequest) (*dto.PageResponse[dto.AfterSaleResponse], error)
	MerchantApprove(ctx context.Context, merchantID, accountID, id int64) (*dto.AfterSaleResponse, error)
	MerchantReject(ctx context.Context, merchantID, accountID, id int64, reason string) (*dto.AfterSaleResponse, error)
	MerchantSyncRefund(ctx context.Context, merchantID, id int64) (*dto.AfterSaleResponse, error)
}

type afterSaleService struct {
	repo    repository.AfterSaleRepository
	refunds *refundCoordinator
}

func NewAfterSaleService(repo repository.AfterSaleRepository, paymentCfg config.PaymentConfig) AfterSaleService {
	return &afterSaleService{repo: repo, refunds: newRefundCoordinator(repo, paymentCfg)}
}

func generateAfterSaleNo() string {
	return fmt.Sprintf("A%s%s", time.Now().Format("20060102150405"), uuid.NewString()[:8])
}
func generateRefundNo() string {
	return fmt.Sprintf("R%s%s", time.Now().Format("20060102150405"), uuid.NewString()[:8])
}

func afterSaleStatusText(status int) string {
	switch status {
	case model.AfterSaleStatusPending:
		return "待商家审核"
	case model.AfterSaleStatusRefunding:
		return "退款处理中"
	case model.AfterSaleStatusRefunded:
		return "退款成功"
	case model.AfterSaleStatusRejected:
		return "商家已拒绝"
	case model.AfterSaleStatusCancelled:
		return "用户已取消"
	case model.AfterSaleStatusRefundFailed:
		return "退款失败"
	default:
		return "未知状态"
	}
}

func afterSaleTypeText(value string) string {
	if value == model.AfterSaleTypeReturnRefund {
		return "退货退款"
	}
	return "仅退款"
}

func refundStatusText(status int) string {
	switch status {
	case model.RefundStatusPending:
		return "退款处理中"
	case model.RefundStatusSucceeded:
		return "退款成功"
	case model.RefundStatusFailed:
		return "退款失败"
	case model.RefundStatusUnknown:
		return "退款结果确认中"
	default:
		return "未知状态"
	}
}

func normalizePage(page, pageSize int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 50 {
		pageSize = 50
	}
	return page, pageSize
}

func stringTime(value *time.Time) *string {
	if value == nil {
		return nil
	}
	text := value.Format(time.RFC3339)
	return &text
}

func (s *afterSaleService) buildResponse(ctx context.Context, value model.AfterSale) (*dto.AfterSaleResponse, error) {
	order, err := s.repo.FindOrder(ctx, value.OrderID)
	if err != nil {
		return nil, err
	}
	item, err := s.repo.FindItem(ctx, value.OrderItemID)
	if err != nil {
		return nil, err
	}
	images := []string{}
	if value.Images != "" {
		_ = json.Unmarshal([]byte(value.Images), &images)
	}
	resp := &dto.AfterSaleResponse{
		ID: value.ID, AfterSaleNo: value.AfterSaleNo, OrderID: value.OrderID, OrderNo: order.OrderNo,
		OrderItemID: value.OrderItemID, ProductName: item.ProductName, SKUName: item.SKUName, SKUImage: item.SKUImage,
		UserID: value.UserID, MerchantID: value.MerchantID, Type: value.Type, TypeText: afterSaleTypeText(value.Type),
		Status: value.Status, StatusText: afterSaleStatusText(value.Status), Reason: value.Reason, Description: value.Description,
		Images: images, RefundAmount: value.RefundAmount, RejectReason: value.RejectReason,
		ReviewedAt: stringTime(value.ReviewedAt), CancelledAt: stringTime(value.CancelledAt), RefundedAt: stringTime(value.RefundedAt),
		CreatedAt: value.CreatedAt.Format(time.RFC3339), UpdatedAt: value.UpdatedAt.Format(time.RFC3339),
	}
	refund, err := s.repo.FindRefundByAfterSaleID(ctx, value.ID)
	if err == nil {
		resp.Refund = &dto.RefundResponse{
			TradeID:             refund.TradeID,
			PaymentAllocationID: refund.PaymentAllocationID,
			RefundNo:            refund.RefundNo,
			PayChannel:          refund.PayChannel,
			Amount:              refund.Amount,
			Status:              refund.Status,
			StatusText:          refundStatusText(refund.Status),
			TransactionID:       refund.TransactionID,
			FailureReason:       refund.FailureReason,
			LastError:           refund.LastError,
			RetryCount:          refund.RetryCount,
			LastAttemptAt:       stringTime(refund.LastAttemptAt),
			NextRetryAt:         stringTime(refund.NextRetryAt),
			RefundedAt:          stringTime(refund.RefundedAt),
		}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	return resp, nil
}

func (s *afterSaleService) Create(ctx context.Context, userID int64, req dto.CreateAfterSaleRequest) (*dto.AfterSaleResponse, error) {
	if userID <= 0 || req.OrderID <= 0 || req.OrderItemID <= 0 {
		return nil, fmt.Errorf("订单和商品参数不合法")
	}
	if req.Type != model.AfterSaleTypeRefundOnly && req.Type != model.AfterSaleTypeReturnRefund {
		return nil, fmt.Errorf("售后类型不支持")
	}
	reason := strings.TrimSpace(req.Reason)
	if reason == "" || len([]rune(reason)) > 100 {
		return nil, fmt.Errorf("售后原因不能为空且不能超过 100 个字符")
	}
	description := strings.TrimSpace(req.Description)
	if len([]rune(description)) > 500 || len(req.Images) > 9 {
		return nil, fmt.Errorf("售后说明不能超过 500 个字符，图片不能超过 9 张")
	}
	order, err := s.repo.FindOrderByIDAndUserID(ctx, req.OrderID, userID)
	if err != nil {
		return nil, fmt.Errorf("订单不存在")
	}
	if order.Status != model.OrderStatusPaid && order.Status != model.OrderStatusShipped && order.Status != model.OrderStatusCompleted {
		return nil, fmt.Errorf("当前订单状态不能申请售后")
	}
	item, err := s.repo.FindOrderItem(ctx, order.ID, req.OrderItemID)
	if err != nil {
		return nil, fmt.Errorf("订单商品不存在")
	}
	existing, err := s.repo.FindLatestByOrderItem(ctx, item.ID)
	if err == nil && existing.Status != model.AfterSaleStatusRejected && existing.Status != model.AfterSaleStatusCancelled {
		return nil, fmt.Errorf("该订单商品已有进行中的售后")
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	images, _ := json.Marshal(req.Images)
	if item.Subtotal < 0 || item.DiscountAmount < 0 || item.DiscountAmount > item.Subtotal || item.PayableAmount != item.Subtotal-item.DiscountAmount {
		return nil, fmt.Errorf("订单商品金额快照不完整")
	}
	refundAmount := item.PayableAmount
	if refundAmount <= 0 || refundAmount > order.PayableAmount {
		return nil, fmt.Errorf("可退款金额不合法")
	}
	value := &model.AfterSale{AfterSaleNo: generateAfterSaleNo(), OrderID: order.ID, OrderItemID: item.ID, UserID: userID, MerchantID: order.MerchantID, Type: req.Type, Status: model.AfterSaleStatusPending, ActiveKey: fmt.Sprintf("%d:%d", order.ID, item.ID), Reason: reason, Description: description, Images: string(images), RefundAmount: refundAmount}
	if err := s.repo.Create(ctx, value); err != nil {
		return nil, err
	}
	return s.buildResponse(ctx, *value)
}

func (s *afterSaleService) List(ctx context.Context, userID int64, req dto.AfterSaleListRequest) (*dto.PageResponse[dto.AfterSaleResponse], error) {
	page, pageSize := normalizePage(req.Page, req.PageSize)
	values, total, err := s.repo.ListByUserID(ctx, userID, (page-1)*pageSize, pageSize, req.Status)
	if err != nil {
		return nil, err
	}
	return s.buildPage(ctx, values, total, page, pageSize)
}

func (s *afterSaleService) MerchantList(ctx context.Context, merchantID int64, req dto.AfterSaleListRequest) (*dto.PageResponse[dto.AfterSaleResponse], error) {
	page, pageSize := normalizePage(req.Page, req.PageSize)
	values, total, err := s.repo.ListByMerchantID(ctx, merchantID, (page-1)*pageSize, pageSize, req.Status)
	if err != nil {
		return nil, err
	}
	return s.buildPage(ctx, values, total, page, pageSize)
}

func (s *afterSaleService) buildPage(ctx context.Context, values []model.AfterSale, total int64, page, pageSize int) (*dto.PageResponse[dto.AfterSaleResponse], error) {
	list := make([]dto.AfterSaleResponse, 0, len(values))
	for _, value := range values {
		response, err := s.buildResponse(ctx, value)
		if err != nil {
			return nil, err
		}
		list = append(list, *response)
	}
	return &dto.PageResponse[dto.AfterSaleResponse]{List: list, Page: page, PageSize: pageSize, Total: total}, nil
}

func (s *afterSaleService) Detail(ctx context.Context, userID, id int64) (*dto.AfterSaleResponse, error) {
	value, err := s.repo.FindByIDAndUserID(ctx, id, userID)
	if err != nil {
		return nil, fmt.Errorf("售后申请不存在")
	}
	return s.buildResponse(ctx, *value)
}

func (s *afterSaleService) Cancel(ctx context.Context, userID, id int64) error {
	value, err := s.repo.FindByIDAndUserID(ctx, id, userID)
	if err != nil {
		return fmt.Errorf("售后申请不存在")
	}
	now := time.Now()
	return s.repo.UpdateStatus(ctx, value.ID, []int{model.AfterSaleStatusPending}, map[string]interface{}{"status": model.AfterSaleStatusCancelled, "cancelled_at": &now, "active_key": value.AfterSaleNo})
}

func (s *afterSaleService) MerchantReject(ctx context.Context, merchantID, accountID, id int64, reason string) (*dto.AfterSaleResponse, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" || len([]rune(reason)) > 255 {
		return nil, fmt.Errorf("拒绝原因不能为空且不能超过 255 个字符")
	}
	value, err := s.repo.FindByIDAndMerchantID(ctx, id, merchantID)
	if err != nil {
		return nil, fmt.Errorf("售后申请不存在")
	}
	now := time.Now()
	if err := s.repo.UpdateStatus(ctx, value.ID, []int{model.AfterSaleStatusPending}, map[string]interface{}{"status": model.AfterSaleStatusRejected, "reject_reason": reason, "reviewed_by": accountID, "reviewed_at": &now, "active_key": value.AfterSaleNo}); err != nil {
		return nil, err
	}
	value, _ = s.repo.FindByIDAndMerchantID(ctx, id, merchantID)
	return s.buildResponse(ctx, *value)
}

func (s *afterSaleService) MerchantApprove(ctx context.Context, merchantID, accountID, id int64) (*dto.AfterSaleResponse, error) {
	var afterSale *model.AfterSale
	var payment *model.Payment
	var refund *model.Refund
	queryFirst := false
	err := s.repo.Transaction(ctx, func(repo repository.AfterSaleRepository) error {
		value, err := repo.FindForUpdateByIDAndMerchantID(ctx, id, merchantID)
		if err != nil {
			return fmt.Errorf("售后申请不存在")
		}
		if value.Status == model.AfterSaleStatusRefunded {
			afterSale = value
			return nil
		}
		if value.Status != model.AfterSaleStatusPending && value.Status != model.AfterSaleStatusRefundFailed && value.Status != model.AfterSaleStatusRefunding {
			return fmt.Errorf("当前售后状态不能同意")
		}
		currentRefund, findErr := repo.FindRefundByAfterSaleID(ctx, value.ID)
		var pay *model.Payment
		var allocation *model.PaymentAllocation
		if errors.Is(findErr, gorm.ErrRecordNotFound) {
			pay, allocation, err = repo.FindPaidPaymentAllocationByOrderID(ctx, value.OrderID)
			if err != nil {
				return fmt.Errorf("订单支付分配不存在，请先完成 M8 迁移")
			}
			reserved, err := repo.SumReservedRefundsByAllocation(ctx, allocation.ID, 0)
			if err != nil {
				return err
			}
			if value.RefundAmount <= 0 || reserved > allocation.Amount-value.RefundAmount {
				return fmt.Errorf("退款金额超过子订单剩余可退金额")
			}
			tradeID := allocation.TradeID
			allocationID := allocation.ID
			currentRefund = &model.Refund{
				TradeID: &tradeID, RefundNo: generateRefundNo(), AfterSaleID: value.ID,
				PaymentID: pay.ID, PaymentAllocationID: &allocationID, OrderID: value.OrderID,
				UserID: value.UserID, MerchantID: value.MerchantID, PayChannel: pay.PayChannel,
				Amount: value.RefundAmount, Status: model.RefundStatusPending,
			}
			if err := repo.CreateRefund(ctx, currentRefund); err != nil {
				return err
			}
		} else if findErr != nil {
			return findErr
		} else {
			if currentRefund.PaymentAllocationID == nil || currentRefund.TradeID == nil {
				return fmt.Errorf("退款单缺少交易支付分配，请先完成 M8 回填")
			}
			allocation, err = repo.FindPaymentAllocationForUpdate(ctx, *currentRefund.PaymentAllocationID)
			if err != nil {
				return fmt.Errorf("订单支付分配不存在")
			}
			pay, err = repo.FindPaymentByID(ctx, currentRefund.PaymentID)
			if err != nil {
				return fmt.Errorf("订单支付单不存在")
			}
			switch currentRefund.Status {
			case model.RefundStatusSucceeded:
			case model.RefundStatusFailed:
				reserved, err := repo.SumReservedRefundsByAllocation(ctx, allocation.ID, currentRefund.ID)
				if err != nil {
					return err
				}
				if currentRefund.Amount <= 0 || reserved > allocation.Amount-currentRefund.Amount {
					return fmt.Errorf("退款金额超过子订单剩余可退金额")
				}
				if err := repo.MarkRefundPending(ctx, currentRefund.ID); err != nil {
					return err
				}
				currentRefund.Status = model.RefundStatusPending
				currentRefund.FailureReason = ""
				queryFirst = true
			case model.RefundStatusPending, model.RefundStatusUnknown:
				queryFirst = true
			default:
				return fmt.Errorf("退款单状态不合法")
			}
		}
		if value.Status != model.AfterSaleStatusRefunding {
			now := time.Now()
			if err := repo.UpdateStatus(ctx, value.ID, []int{value.Status}, map[string]interface{}{"status": model.AfterSaleStatusRefunding, "reviewed_by": accountID, "reviewed_at": &now, "reject_reason": ""}); err != nil {
				return err
			}
		}
		value.Status = model.AfterSaleStatusRefunding
		afterSale, payment, refund = value, pay, currentRefund
		return nil
	})
	if err != nil {
		return nil, err
	}
	if afterSale.Status == model.AfterSaleStatusRefunded && refund == nil {
		return s.buildResponse(ctx, *afterSale)
	}
	now := time.Now()
	if refund.Status == model.RefundStatusSucceeded {
		refundedAt := now
		if refund.RefundedAt != nil {
			refundedAt = *refund.RefundedAt
		}
		err = s.refunds.finalize(ctx, refund.ID, refund.TransactionID, refundedAt)
	} else {
		var outcome refundProcessOutcome
		outcome, err = s.refunds.Process(ctx, *afterSale, *payment, *refund, queryFirst, now)
		if outcome == refundProcessOutcomeFailed {
			return nil, err
		}
	}
	if err != nil {
		// The provider result is uncertain, but Process has persisted it for reconciliation.
		if latest, findErr := s.repo.FindByIDAndMerchantID(ctx, id, merchantID); findErr == nil {
			return s.buildResponse(ctx, *latest)
		}
		return nil, err
	}
	value, _ := s.repo.FindByIDAndMerchantID(ctx, id, merchantID)
	return s.buildResponse(ctx, *value)
}

func (s *afterSaleService) MerchantSyncRefund(ctx context.Context, merchantID, id int64) (*dto.AfterSaleResponse, error) {
	afterSale, err := s.repo.FindByIDAndMerchantID(ctx, id, merchantID)
	if err != nil {
		return nil, fmt.Errorf("售后申请不存在")
	}
	refund, err := s.repo.FindRefundByAfterSaleID(ctx, afterSale.ID)
	if err != nil {
		return nil, fmt.Errorf("退款单不存在")
	}
	if refund.Status == model.RefundStatusFailed {
		return nil, fmt.Errorf("退款已明确失败，请重新同意售后后发起")
	}
	if refund.Status == model.RefundStatusSucceeded && afterSale.Status == model.AfterSaleStatusRefunded {
		return s.buildResponse(ctx, *afterSale)
	}
	payment, err := s.repo.FindPaymentByID(ctx, refund.PaymentID)
	if err != nil {
		return nil, fmt.Errorf("订单支付单不存在")
	}

	now := time.Now()
	if refund.Status == model.RefundStatusSucceeded {
		refundedAt := now
		if refund.RefundedAt != nil {
			refundedAt = *refund.RefundedAt
		}
		err = s.refunds.finalize(ctx, refund.ID, refund.TransactionID, refundedAt)
	} else {
		outcome, processErr := s.refunds.Process(ctx, *afterSale, *payment, *refund, true, now)
		if outcome == refundProcessOutcomeFailed {
			return nil, processErr
		}
		err = processErr
	}
	if err != nil {
		if latest, findErr := s.repo.FindByIDAndMerchantID(ctx, id, merchantID); findErr == nil {
			return s.buildResponse(ctx, *latest)
		}
		return nil, err
	}
	latest, err := s.repo.FindByIDAndMerchantID(ctx, id, merchantID)
	if err != nil {
		return nil, err
	}
	return s.buildResponse(ctx, *latest)
}
