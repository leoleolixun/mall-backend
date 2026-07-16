package service

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"go-mall/internal/config"
	"go-mall/internal/model"
	"go-mall/internal/repository"
)

type refundProcessOutcome int

const (
	refundProcessOutcomeProcessing refundProcessOutcome = iota
	refundProcessOutcomeSucceeded
	refundProcessOutcomeFailed
)

type refundCoordinator struct {
	repo          repository.AfterSaleRepository
	alipay        alipayRefundGateway
	retryInterval time.Duration
}

func newRefundCoordinator(repo repository.AfterSaleRepository, cfg config.PaymentConfig) *refundCoordinator {
	retryMinutes := cfg.Refund.RetryIntervalMinutes
	if retryMinutes <= 0 {
		retryMinutes = 5
	}
	return &refundCoordinator{
		repo:          repo,
		alipay:        newSDKAlipayRefundGateway(cfg.Alipay),
		retryInterval: time.Duration(retryMinutes) * time.Minute,
	}
}

func (c *refundCoordinator) Process(
	ctx context.Context,
	afterSale model.AfterSale,
	payment model.Payment,
	refund model.Refund,
	queryFirst bool,
	now time.Time,
) (refundProcessOutcome, error) {
	if refund.PaymentAllocationID == nil || refund.TradeID == nil {
		return refundProcessOutcomeFailed, fmt.Errorf("退款单缺少交易支付分配")
	}
	allocation, err := c.repo.FindPaymentAllocationByID(ctx, *refund.PaymentAllocationID)
	if err != nil {
		return refundProcessOutcomeFailed, fmt.Errorf("订单支付分配不存在")
	}
	if refund.PaymentID != payment.ID ||
		refund.AfterSaleID != afterSale.ID ||
		payment.TradeID == nil || *payment.TradeID != *refund.TradeID ||
		allocation.PaymentID != payment.ID || allocation.TradeID != *refund.TradeID ||
		allocation.OrderID != refund.OrderID || allocation.MerchantID != refund.MerchantID ||
		refund.MerchantID != afterSale.MerchantID ||
		refund.PayChannel != payment.PayChannel ||
		refund.Amount <= 0 ||
		refund.Amount > allocation.Amount {
		return refundProcessOutcomeFailed, fmt.Errorf("退款单关联数据不一致")
	}

	switch refund.PayChannel {
	case model.PayChannelMock:
		if err := c.finalize(ctx, refund.ID, "MOCK-"+refund.RefundNo, now); err != nil {
			return refundProcessOutcomeFailed, err
		}
		return refundProcessOutcomeSucceeded, nil
	case model.PayChannelAlipay:
		if queryFirst {
			result, err := c.alipay.Query(ctx, payment, refund)
			if err != nil {
				return c.keepUnknown(ctx, refund.ID, err.Error(), now, err)
			}
			switch result.State {
			case refundProviderStateSucceeded:
				return c.finalizeProviderResult(ctx, refund.ID, result, now)
			case refundProviderStateUnknown:
				return c.keepUnknown(ctx, refund.ID, result.Reason, now, nil)
			case refundProviderStateRejected:
				return c.fail(ctx, refund.ID, result.Reason, now)
			case refundProviderStateNotFound:
				// Reusing the same out_request_no makes this retry idempotent at Alipay.
			default:
				return c.keepUnknown(ctx, refund.ID, "支付宝退款查询结果未知", now, nil)
			}
		}

		result, err := c.alipay.Submit(ctx, payment, refund, afterSale.Reason)
		if err != nil {
			return c.keepUnknown(ctx, refund.ID, err.Error(), now, err)
		}
		switch result.State {
		case refundProviderStateSucceeded:
			return c.finalizeProviderResult(ctx, refund.ID, result, now)
		case refundProviderStateRejected:
			return c.fail(ctx, refund.ID, result.Reason, now)
		default:
			return c.keepUnknown(ctx, refund.ID, result.Reason, now, nil)
		}
	case model.PayChannelWechat:
		return c.fail(ctx, refund.ID, "微信退款尚未接入", now)
	default:
		return c.fail(ctx, refund.ID, "支付渠道不支持退款", now)
	}
}

func (c *refundCoordinator) finalizeProviderResult(
	ctx context.Context,
	refundID int64,
	result refundProviderResult,
	now time.Time,
) (refundProcessOutcome, error) {
	refundedAt := result.RefundedAt
	if refundedAt.IsZero() {
		refundedAt = now
	}
	if strings.TrimSpace(result.TransactionID) == "" {
		return c.keepUnknown(ctx, refundID, "退款成功但渠道交易号为空", now, nil)
	}
	if err := c.finalize(ctx, refundID, result.TransactionID, refundedAt); err != nil {
		return refundProcessOutcomeFailed, err
	}
	return refundProcessOutcomeSucceeded, nil
}

func (c *refundCoordinator) finalize(ctx context.Context, refundID int64, transactionID string, refundedAt time.Time) error {
	return c.repo.Transaction(ctx, func(repo repository.AfterSaleRepository) error {
		snapshot, err := repo.FindRefundByID(ctx, refundID)
		if err != nil {
			return err
		}
		afterSale, err := repo.FindAfterSaleForUpdate(ctx, snapshot.AfterSaleID)
		if err != nil {
			return err
		}
		payment, err := repo.FindPaymentForUpdate(ctx, snapshot.PaymentID)
		if err != nil {
			return fmt.Errorf("订单支付单不存在")
		}
		refund, err := repo.FindRefundForUpdate(ctx, refundID)
		if err != nil {
			return err
		}
		if refund.Status == model.RefundStatusSucceeded && afterSale.Status == model.AfterSaleStatusRefunded {
			return nil
		}
		if refund.PaymentAllocationID == nil || refund.TradeID == nil {
			return fmt.Errorf("退款单缺少交易支付分配")
		}
		allocation, err := repo.FindPaymentAllocationForUpdate(ctx, *refund.PaymentAllocationID)
		if err != nil {
			return fmt.Errorf("订单支付分配不存在")
		}
		if allocation.PaymentID != payment.ID || allocation.TradeID != *refund.TradeID ||
			allocation.OrderID != refund.OrderID || allocation.MerchantID != refund.MerchantID {
			return fmt.Errorf("退款单与支付分配不一致")
		}
		alreadySucceeded := refund.Status == model.RefundStatusSucceeded
		if !alreadySucceeded && refund.Status != model.RefundStatusPending && refund.Status != model.RefundStatusUnknown {
			return fmt.Errorf("当前退款单状态不能完成退款")
		}
		if !alreadySucceeded {
			order, err := repo.FindOrderForUpdate(ctx, refund.OrderID)
			if err != nil {
				return fmt.Errorf("退款子订单不存在")
			}
			if order.MerchantID != refund.MerchantID || order.PayableAmount != allocation.Amount ||
				order.CommissionAmount == nil || order.SettlementAmount == nil ||
				*order.CommissionAmount+*order.SettlementAmount != order.PayableAmount {
				return fmt.Errorf("退款子订单结算快照不一致")
			}
			beforeCommissionRefund, err := proportionalRefundCommission(
				*order.CommissionAmount, allocation.RefundedAmount, allocation.Amount,
			)
			if err != nil {
				return err
			}
			afterCommissionRefund, err := proportionalRefundCommission(
				*order.CommissionAmount, allocation.RefundedAmount+refund.Amount, allocation.Amount,
			)
			if err != nil {
				return err
			}
			if err := repo.IncreaseAllocationRefundedAmount(ctx, allocation.ID, refund.Amount); err != nil {
				return err
			}
			if err := repo.MarkRefundSucceeded(ctx, refund.ID, transactionID, refundedAt); err != nil {
				return err
			}
			orderID := order.ID
			refundIDValue := refund.ID
			entries := []model.SettlementEntry{{
				EntryNo: fmt.Sprintf("REFUND-%d", refund.ID), MerchantID: refund.MerchantID,
				OrderID: &orderID, RefundID: &refundIDValue, EntryType: model.SettlementEntryRefund,
				Amount: -refund.Amount, AvailableAt: refundedAt,
			}}
			if commissionRefund := afterCommissionRefund - beforeCommissionRefund; commissionRefund > 0 {
				entries = append(entries, model.SettlementEntry{
					EntryNo: fmt.Sprintf("COMMISSION-REFUND-%d", refund.ID), MerchantID: refund.MerchantID,
					OrderID: &orderID, RefundID: &refundIDValue, EntryType: model.SettlementEntryCommissionRefund,
					Amount: commissionRefund, AvailableAt: refundedAt,
				})
			}
			if err := repo.CreateSettlementEntries(ctx, entries); err != nil {
				return err
			}
		} else {
			transactionID = refund.TransactionID
			if refund.RefundedAt != nil {
				refundedAt = *refund.RefundedAt
			}
		}
		if afterSale.Status != model.AfterSaleStatusRefunded {
			if err := repo.UpdateStatus(
				ctx,
				afterSale.ID,
				[]int{model.AfterSaleStatusRefunding, model.AfterSaleStatusRefundFailed},
				map[string]interface{}{
					"status":      model.AfterSaleStatusRefunded,
					"refunded_at": &refundedAt,
					"active_key":  afterSale.AfterSaleNo,
				},
			); err != nil {
				return err
			}
		}
		refundedAmount, err := repo.SumSucceededRefunds(ctx, payment.ID)
		if err != nil {
			return err
		}
		status := model.PaymentStatusPartiallyRefunded
		if refundedAmount >= payment.Amount {
			status = model.PaymentStatusRefunded
		}
		if err := repo.UpdatePaymentRefundStatus(ctx, payment.ID, status); err != nil {
			return err
		}
		tradeStatus := model.TradeStatusPartiallyRefunded
		if refundedAmount >= payment.Amount {
			tradeStatus = model.TradeStatusRefunded
		}
		return repo.UpdateTradeRefundStatus(ctx, *refund.TradeID, tradeStatus)
	})
}

func proportionalRefundCommission(totalCommission, refundedAmount, allocationAmount int64) (int64, error) {
	if totalCommission < 0 || refundedAmount < 0 || allocationAmount <= 0 ||
		totalCommission > allocationAmount || refundedAmount > allocationAmount {
		return 0, fmt.Errorf("退款佣金分摊参数不合法")
	}
	numerator := new(big.Int).Mul(big.NewInt(totalCommission), big.NewInt(refundedAmount))
	value := numerator.Quo(numerator, big.NewInt(allocationAmount))
	if !value.IsInt64() {
		return 0, fmt.Errorf("退款佣金分摊溢出")
	}
	return value.Int64(), nil
}

func (c *refundCoordinator) fail(
	ctx context.Context,
	refundID int64,
	reason string,
	now time.Time,
) (refundProcessOutcome, error) {
	reason = limitedRefundMessage(reason)
	err := c.repo.Transaction(ctx, func(repo repository.AfterSaleRepository) error {
		snapshot, err := repo.FindRefundByID(ctx, refundID)
		if err != nil {
			return err
		}
		afterSale, err := repo.FindAfterSaleForUpdate(ctx, snapshot.AfterSaleID)
		if err != nil {
			return err
		}
		refund, err := repo.FindRefundForUpdate(ctx, refundID)
		if err != nil {
			return err
		}
		if refund.Status == model.RefundStatusFailed {
			return nil
		}
		if refund.Status == model.RefundStatusSucceeded {
			return fmt.Errorf("退款单已经成功，不能标记失败")
		}
		if err := repo.MarkRefundFailed(ctx, refund.ID, reason, now); err != nil {
			return err
		}
		if afterSale.Status == model.AfterSaleStatusRefundFailed {
			return nil
		}
		return repo.UpdateStatus(ctx, afterSale.ID, []int{model.AfterSaleStatusRefunding}, map[string]interface{}{
			"status": model.AfterSaleStatusRefundFailed,
		})
	})
	if err != nil {
		return refundProcessOutcomeFailed, err
	}
	return refundProcessOutcomeFailed, fmt.Errorf("退款被支付渠道拒绝: %s", reason)
}

func (c *refundCoordinator) keepUnknown(
	ctx context.Context,
	refundID int64,
	reason string,
	now time.Time,
	providerErr error,
) (refundProcessOutcome, error) {
	reason = limitedRefundMessage(reason)
	nextRetryAt := now.Add(c.retryInterval)
	err := c.repo.Transaction(ctx, func(repo repository.AfterSaleRepository) error {
		snapshot, err := repo.FindRefundByID(ctx, refundID)
		if err != nil {
			return err
		}
		afterSale, err := repo.FindAfterSaleForUpdate(ctx, snapshot.AfterSaleID)
		if err != nil {
			return err
		}
		refund, err := repo.FindRefundForUpdate(ctx, refundID)
		if err != nil {
			return err
		}
		if refund.Status == model.RefundStatusSucceeded {
			return nil
		}
		if refund.Status != model.RefundStatusPending && refund.Status != model.RefundStatusUnknown {
			return fmt.Errorf("当前退款单状态不能进入待确认状态")
		}
		if err := repo.MarkRefundUnknown(ctx, refund.ID, reason, now, nextRetryAt); err != nil {
			return err
		}
		if afterSale.Status == model.AfterSaleStatusRefundFailed {
			return repo.UpdateStatus(ctx, afterSale.ID, []int{model.AfterSaleStatusRefundFailed}, map[string]interface{}{
				"status": model.AfterSaleStatusRefunding,
			})
		}
		return nil
	})
	if err != nil {
		return refundProcessOutcomeProcessing, err
	}
	return refundProcessOutcomeProcessing, providerErr
}

func limitedRefundMessage(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		value = "支付渠道未返回明确的退款结果"
	}
	runes := []rune(value)
	if len(runes) > 255 {
		value = string(runes[:255])
	}
	return value
}

type RefundReconciliationReport struct {
	Scanned    int
	Succeeded  int
	Processing int
	Failed     int
	Errors     int
}

type RefundReconciliationService interface {
	Run(ctx context.Context, now time.Time) (RefundReconciliationReport, error)
}

type refundReconciliationService struct {
	repo        repository.AfterSaleRepository
	coordinator *refundCoordinator
	batchSize   int
}

func NewRefundReconciliationService(repo repository.AfterSaleRepository, cfg config.PaymentConfig) RefundReconciliationService {
	batchSize := cfg.Refund.ReconcileBatchSize
	if batchSize <= 0 {
		batchSize = 100
	}
	if batchSize > 1000 {
		batchSize = 1000
	}
	return &refundReconciliationService{
		repo:        repo,
		coordinator: newRefundCoordinator(repo, cfg),
		batchSize:   batchSize,
	}
}

func (s *refundReconciliationService) Run(ctx context.Context, now time.Time) (RefundReconciliationReport, error) {
	var report RefundReconciliationReport
	ids, err := s.repo.ListRefundIDsForReconciliation(ctx, now, s.batchSize)
	if err != nil {
		return report, err
	}
	report.Scanned = len(ids)

	var failures []error
	for _, id := range ids {
		refund, err := s.repo.FindRefundByID(ctx, id)
		if err != nil {
			report.Errors++
			failures = append(failures, fmt.Errorf("退款单 %d 查询失败: %w", id, err))
			continue
		}
		afterSale, err := s.repo.FindByIDAndMerchantID(ctx, refund.AfterSaleID, refund.MerchantID)
		if err != nil {
			report.Errors++
			failures = append(failures, fmt.Errorf("退款单 %s 缺少售后申请: %w", refund.RefundNo, err))
			continue
		}
		payment, err := s.repo.FindPaymentByID(ctx, refund.PaymentID)
		if err != nil {
			report.Errors++
			failures = append(failures, fmt.Errorf("退款单 %s 缺少支付单: %w", refund.RefundNo, err))
			continue
		}

		outcome, processErr := s.coordinator.Process(ctx, *afterSale, *payment, *refund, true, now)
		switch outcome {
		case refundProcessOutcomeSucceeded:
			report.Succeeded++
		case refundProcessOutcomeFailed:
			report.Failed++
		default:
			report.Processing++
		}
		if processErr != nil {
			report.Errors++
			failures = append(failures, fmt.Errorf("退款单 %s 对账失败: %w", refund.RefundNo, processErr))
		}
	}

	return report, errors.Join(failures...)
}
