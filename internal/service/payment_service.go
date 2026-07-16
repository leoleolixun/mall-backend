package service

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"go-mall/internal/config"
	"go-mall/internal/dto"
	"go-mall/internal/model"
	"go-mall/internal/repository"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type PaymentService interface {
	Create(ctx context.Context, userID int64, req dto.CreatePaymentRequest) (*dto.PaymentResponse, error)
	Detail(ctx context.Context, userID int64, paymentNo string) (*dto.PaymentResponse, error)
	AlipayNotify(ctx context.Context, values url.Values) error
	MockComplete(ctx context.Context, userID int64, paymentNo string) (*dto.PaymentResponse, error)
	MockPayOrder(ctx context.Context, userID int64, orderID int64) (*dto.PaymentResponse, error)
	Sync(ctx context.Context, userID int64, paymentNo string) (*dto.PaymentResponse, error)
	PrepareUserOrderForCancel(ctx context.Context, userID int64, orderID int64) error
	PrepareUserTradeForCancel(ctx context.Context, userID int64, tradeID int64) error
	PrepareOrderForTimeoutCancel(ctx context.Context, orderID int64) (bool, error)
}

type paymentService struct {
	paymentRepo repository.PaymentRepository
	paymentCfg  config.PaymentConfig
	alipay      alipayGateway
}

func NewPaymentService(paymentRepo repository.PaymentRepository, paymentCfg config.PaymentConfig) PaymentService {
	return &paymentService{
		paymentRepo: paymentRepo,
		paymentCfg:  paymentCfg,
		alipay:      newSDKAlipayGateway(paymentCfg.Alipay),
	}
}

func (s *paymentService) Sync(ctx context.Context, userID int64, paymentNo string) (*dto.PaymentResponse, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("用户未登录")
	}
	paymentNo = strings.TrimSpace(paymentNo)
	if paymentNo == "" {
		return nil, fmt.Errorf("支付单号不能为空")
	}

	payment, err := s.paymentRepo.FindByPaymentNoAndUserID(ctx, paymentNo, userID)
	if err != nil {
		return nil, fmt.Errorf("支付单不存在")
	}
	if payment.PayChannel != model.PayChannelAlipay {
		return nil, fmt.Errorf("只有支付宝支付单支持主动查询")
	}
	if payment.Status != model.PaymentStatusPending {
		return toPaymentResponse(*payment), nil
	}

	if _, err := s.syncAlipayPayment(ctx, payment); err != nil {
		return nil, err
	}
	latest, err := s.paymentRepo.FindByPaymentNoAndUserID(ctx, paymentNo, userID)
	if err != nil {
		return nil, err
	}
	return s.toPaymentResponse(ctx, *latest)
}

func (s *paymentService) syncAlipayPayment(ctx context.Context, payment *model.Payment) (alipayTradeState, error) {
	trade, err := s.alipay.Query(ctx, *payment)
	if err != nil {
		return alipayTradeStateUnknown, err
	}

	switch trade.State {
	case alipayTradeStateNotExist, alipayTradeStateWaiting:
		return trade.State, nil
	case alipayTradeStateClosed:
		now := time.Now()
		if err := s.paymentRepo.MarkClosed(ctx, payment.ID, payment.UserID, now); err != nil {
			latest, findErr := s.paymentRepo.FindByPaymentNo(ctx, payment.PaymentNo)
			if findErr == nil && latest.Status == model.PaymentStatusClosed {
				return trade.State, nil
			}
			return alipayTradeStateUnknown, err
		}
		return trade.State, nil
	case alipayTradeStatePaid:
		err := s.paymentRepo.Transaction(ctx, func(repo repository.PaymentRepository) error {
			latest, err := repo.FindByPaymentNoForUpdate(ctx, payment.PaymentNo)
			if err != nil {
				return fmt.Errorf("支付单不存在")
			}
			return s.completePayment(ctx, repo, latest, trade.TransactionID, trade.PaidAt)
		})
		if err != nil {
			return alipayTradeStateUnknown, err
		}
		return trade.State, nil
	default:
		return alipayTradeStateUnknown, fmt.Errorf("未知的支付宝交易状态")
	}
}

func (s *paymentService) completePayment(
	ctx context.Context,
	repo repository.PaymentRepository,
	payment *model.Payment,
	transactionID string,
	paidAt time.Time,
) error {
	if payment.Status != model.PaymentStatusPending &&
		payment.Status != model.PaymentStatusPaid &&
		payment.Status != model.PaymentStatusPartiallyRefunded &&
		payment.Status != model.PaymentStatusRefunded {
		return fmt.Errorf("当前支付单状态不能完成支付")
	}
	if payment.TradeID != nil {
		trade, err := repo.FindTradeByIDAndUserID(ctx, *payment.TradeID, payment.UserID)
		if err != nil {
			return fmt.Errorf("交易不存在")
		}
		orders, err := repo.FindOrdersByTradeID(ctx, trade.ID, payment.UserID)
		if err != nil {
			return err
		}
		allocations, err := repo.FindAllocationsByPaymentID(ctx, payment.ID)
		if err != nil {
			return err
		}
		if err := validateTradePaymentAmounts(trade, orders, *payment, allocations); err != nil {
			return err
		}
		if trade.Status == model.TradeStatusPendingPayment {
			for _, order := range orders {
				if order.Status != model.OrderStatusPendingPayment {
					return fmt.Errorf("子订单 %d 状态与支付结果冲突，请人工处理", order.ID)
				}
			}
			if payment.Status == model.PaymentStatusPending {
				if err := repo.MarkPaid(ctx, payment.ID, payment.UserID, transactionID, paidAt); err != nil {
					return err
				}
			}
			if err := repo.UpdateTradeStatus(ctx, trade.ID, trade.UserID, model.TradeStatusPendingPayment, model.TradeStatusPaid, &paidAt); err != nil {
				return err
			}
			return repo.UpdateTradeOrdersStatus(
				ctx, trade.ID, trade.UserID, int64(len(orders)),
				model.OrderStatusPendingPayment, model.OrderStatusPaid, &paidAt,
			)
		}
		if trade.Status != model.TradeStatusPaid && trade.Status != model.TradeStatusPartiallyRefunded && trade.Status != model.TradeStatusRefunded {
			return fmt.Errorf("交易状态与支付结果冲突，请人工处理")
		}
		if payment.Status == model.PaymentStatusPending {
			return repo.MarkPaid(ctx, payment.ID, payment.UserID, transactionID, paidAt)
		}
		return nil
	}

	if payment.OrderID == nil {
		return fmt.Errorf("支付单缺少订单或交易引用")
	}
	order, err := repo.FindOrderByIDAndUserID(ctx, *payment.OrderID, payment.UserID)
	if err != nil {
		return fmt.Errorf("订单不存在")
	}
	if order.Status == model.OrderStatusPendingPayment {
		if payment.Status == model.PaymentStatusPending {
			if err := repo.MarkPaid(ctx, payment.ID, payment.UserID, transactionID, paidAt); err != nil {
				return err
			}
		}
		return repo.UpdateOrderStatus(
			ctx, order.ID, order.UserID,
			model.OrderStatusPendingPayment, model.OrderStatusPaid, &paidAt,
		)
	}
	if order.Status != model.OrderStatusPaid {
		return fmt.Errorf("订单状态与支付结果冲突，请人工处理")
	}
	if payment.Status == model.PaymentStatusPending {
		return repo.MarkPaid(ctx, payment.ID, payment.UserID, transactionID, paidAt)
	}
	return nil
}

func (s *paymentService) PrepareOrderForTimeoutCancel(ctx context.Context, orderID int64) (bool, error) {
	paidPayment, err := s.paymentRepo.FindPaidByOrderID(ctx, orderID)
	if err == nil {
		if err := s.repairOrderFromPaidPayment(ctx, paidPayment); err != nil {
			return false, err
		}
		return true, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return false, err
	}

	payments, err := s.paymentRepo.FindPendingByOrderID(ctx, orderID)
	if err != nil {
		return false, err
	}
	for i := range payments {
		payment := &payments[i]
		if payment.PayChannel != model.PayChannelAlipay {
			continue
		}
		trade, err := s.alipay.Query(ctx, *payment)
		if err != nil {
			return false, err
		}
		if trade.State == alipayTradeStatePaid {
			if _, err := s.syncAlipayPayment(ctx, payment); err != nil {
				return false, err
			}
			return true, nil
		}
		if trade.State == alipayTradeStateClosed {
			continue
		}
		if trade.State == alipayTradeStateWaiting || trade.State == alipayTradeStateNotExist {
			if err := s.alipay.Close(ctx, *payment); err != nil {
				latestTrade, queryErr := s.alipay.Query(ctx, *payment)
				if queryErr == nil && latestTrade.State == alipayTradeStatePaid {
					if _, syncErr := s.syncAlipayPayment(ctx, payment); syncErr != nil {
						return false, syncErr
					}
					return true, nil
				}
				if queryErr == nil && latestTrade.State == alipayTradeStateClosed {
					continue
				}
				return false, err
			}
			continue
		}
		return false, fmt.Errorf("未知的支付宝交易状态")
	}
	return false, nil
}

func (s *paymentService) repairOrderFromPaidPayment(ctx context.Context, payment *model.Payment) error {
	return s.paymentRepo.Transaction(ctx, func(repo repository.PaymentRepository) error {
		latest, err := repo.FindByPaymentNoForUpdate(ctx, payment.PaymentNo)
		if err != nil {
			return fmt.Errorf("支付单不存在")
		}
		paidAt := latest.PaidAt
		if paidAt == nil {
			now := time.Now()
			paidAt = &now
		}
		return s.completePayment(ctx, repo, latest, latest.TransactionID, *paidAt)
	})
}

func (s *paymentService) PrepareUserOrderForCancel(ctx context.Context, userID int64, orderID int64) error {
	if userID <= 0 {
		return fmt.Errorf("用户未登录")
	}
	if orderID <= 0 {
		return fmt.Errorf("订单 ID 不合法")
	}
	order, err := s.paymentRepo.FindOrderByIDAndUserID(ctx, orderID, userID)
	if err != nil {
		return fmt.Errorf("订单不存在")
	}
	if order.Status == model.OrderStatusCancelled {
		return nil
	}
	if order.Status != model.OrderStatusPendingPayment {
		return fmt.Errorf("当前订单状态不能取消")
	}
	paid, err := s.PrepareOrderForTimeoutCancel(ctx, orderID)
	if err != nil {
		return err
	}
	if paid {
		return fmt.Errorf("订单已支付，不能取消")
	}
	return nil
}

func (s *paymentService) PrepareUserTradeForCancel(ctx context.Context, userID int64, tradeID int64) error {
	if userID <= 0 {
		return fmt.Errorf("用户未登录")
	}
	if tradeID <= 0 {
		return fmt.Errorf("交易 ID 不合法")
	}
	trade, err := s.paymentRepo.FindTradeByIDAndUserID(ctx, tradeID, userID)
	if err != nil {
		return fmt.Errorf("交易不存在")
	}
	if trade.Status == model.TradeStatusClosed {
		return nil
	}
	if trade.Status != model.TradeStatusPendingPayment {
		return fmt.Errorf("当前交易状态不能取消")
	}
	paid, err := s.prepareTradeForTimeoutCancel(ctx, tradeID)
	if err != nil {
		return err
	}
	if paid {
		return fmt.Errorf("交易已支付，不能取消")
	}
	return nil
}

func (s *paymentService) prepareTradeForTimeoutCancel(ctx context.Context, tradeID int64) (bool, error) {
	paidPayment, err := s.paymentRepo.FindPaidByTradeID(ctx, tradeID)
	if err == nil {
		if err := s.repairOrderFromPaidPayment(ctx, paidPayment); err != nil {
			return false, err
		}
		return true, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return false, err
	}
	payments, err := s.paymentRepo.FindPendingByTradeID(ctx, tradeID)
	if err != nil {
		return false, err
	}
	for i := range payments {
		payment := &payments[i]
		if payment.PayChannel != model.PayChannelAlipay {
			continue
		}
		providerTrade, err := s.alipay.Query(ctx, *payment)
		if err != nil {
			return false, err
		}
		if providerTrade.State == alipayTradeStatePaid {
			if _, err := s.syncAlipayPayment(ctx, payment); err != nil {
				return false, err
			}
			return true, nil
		}
		if providerTrade.State == alipayTradeStateClosed {
			continue
		}
		if providerTrade.State == alipayTradeStateWaiting || providerTrade.State == alipayTradeStateNotExist {
			if err := s.alipay.Close(ctx, *payment); err != nil {
				latestTrade, queryErr := s.alipay.Query(ctx, *payment)
				if queryErr == nil && latestTrade.State == alipayTradeStatePaid {
					if _, syncErr := s.syncAlipayPayment(ctx, payment); syncErr != nil {
						return false, syncErr
					}
					return true, nil
				}
				if queryErr == nil && latestTrade.State == alipayTradeStateClosed {
					continue
				}
				return false, err
			}
			continue
		}
		return false, fmt.Errorf("未知的支付宝交易状态")
	}
	return false, nil
}

func generatePaymentNo() string {
	return fmt.Sprintf("P%s%s", time.Now().Format("20060102150405"), uuid.NewString()[:8])
}

func activeOrderID(orderID int64) *int64 {
	value := orderID
	return &value
}

func activeTradeID(tradeID int64) *int64 {
	value := tradeID
	return &value
}

func generateMockTransactionID() string {
	return fmt.Sprintf("MOCK%s%s", time.Now().Format("20060102150405"), uuid.NewString()[:8])
}

func paymentStatusText(status int) string {
	switch status {
	case model.PaymentStatusPending:
		return "待支付"
	case model.PaymentStatusPaid:
		return "已支付"
	case model.PaymentStatusClosed:
		return "已关闭"
	case model.PaymentStatusFailed:
		return "支付失败"
	case model.PaymentStatusRefunded:
		return "已退款"
	case model.PaymentStatusPartiallyRefunded:
		return "部分退款"
	default:
		return "未知状态"
	}
}

func normalizePayChannel(payChannel string) (string, error) {
	payChannel = strings.TrimSpace(payChannel)
	if payChannel == "" {
		return "", fmt.Errorf("支付渠道不能为空")
	}

	switch payChannel {
	case model.PayChannelMock, model.PayChannelWechat, model.PayChannelAlipay:
		return payChannel, nil
	default:
		return "", fmt.Errorf("支付渠道不支持")
	}
}

func normalizePayScene(payChannel string, payScene string) (string, error) {
	payScene = strings.TrimSpace(payScene)
	if payChannel == model.PayChannelMock {
		return model.PaySceneMock, nil
	}

	if payChannel == model.PayChannelAlipay {
		if payScene == "" {
			return model.PaySceneAlipayPage, nil
		}
		switch payScene {
		case model.PaySceneAlipayPage, model.PaySceneAlipayWap:
			return payScene, nil
		default:
			return "", fmt.Errorf("支付宝支付场景不支持")
		}
	}

	if payScene == "" {
		return payChannel, nil
	}
	return payScene, nil
}

func effectivePayScene(payChannel string, payScene string) string {
	payScene = strings.TrimSpace(payScene)
	if payScene != "" {
		return payScene
	}
	if payChannel == model.PayChannelMock {
		return model.PaySceneMock
	}
	if payChannel == model.PayChannelAlipay {
		return model.PaySceneAlipayPage
	}
	return payChannel
}

func toPaymentResponse(payment model.Payment) *dto.PaymentResponse {
	var paidAt *string
	if payment.PaidAt != nil {
		value := payment.PaidAt.Format(time.RFC3339)
		paidAt = &value
	}

	var closedAt *string
	if payment.ClosedAt != nil {
		value := payment.ClosedAt.Format(time.RFC3339)
		closedAt = &value
	}

	resp := &dto.PaymentResponse{
		ID:            payment.ID,
		PaymentNo:     payment.PaymentNo,
		TradeID:       payment.TradeID,
		OrderID:       payment.OrderID,
		OrderNo:       payment.OrderNo,
		UserID:        payment.UserID,
		MerchantID:    payment.MerchantID,
		PayChannel:    payment.PayChannel,
		PayScene:      effectivePayScene(payment.PayChannel, payment.PayScene),
		Status:        payment.Status,
		StatusText:    paymentStatusText(payment.Status),
		Amount:        payment.Amount,
		TransactionID: payment.TransactionID,
		FailureReason: payment.FailureReason,
		PaidAt:        paidAt,
		ClosedAt:      closedAt,
	}

	if payment.PayChannel == model.PayChannelMock {
		resp.PayParams = map[string]interface{}{
			"mock":       true,
			"payment_no": payment.PaymentNo,
		}
	}
	return resp
}

func (s *paymentService) toPaymentResponse(ctx context.Context, payment model.Payment) (*dto.PaymentResponse, error) {
	resp := toPaymentResponse(payment)
	if payment.TradeID != nil {
		trade, err := s.paymentRepo.FindTradeByIDAndUserID(ctx, *payment.TradeID, payment.UserID)
		if err != nil {
			return nil, fmt.Errorf("交易不存在")
		}
		resp.TradeNo = trade.TradeNo
		allocations, err := s.paymentRepo.FindAllocationsByPaymentID(ctx, payment.ID)
		if err != nil {
			return nil, err
		}
		resp.Allocations = make([]dto.PaymentAllocationResponse, 0, len(allocations))
		for _, allocation := range allocations {
			resp.Allocations = append(resp.Allocations, dto.PaymentAllocationResponse{
				OrderID: allocation.OrderID, MerchantID: allocation.MerchantID, Amount: allocation.Amount,
			})
		}
	}
	if payment.PayChannel != model.PayChannelAlipay || payment.Status != model.PaymentStatusPending {
		return resp, nil
	}

	payParams, err := s.buildAlipayPayParams(ctx, payment)
	if err != nil {
		return nil, err
	}
	resp.PayParams = payParams
	return resp, nil
}

func (s *paymentService) Create(ctx context.Context, userID int64, req dto.CreatePaymentRequest) (*dto.PaymentResponse, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("用户未登录")
	}
	if (req.OrderID > 0) == (req.TradeID > 0) {
		return nil, fmt.Errorf("order_id 和 trade_id 必须且只能填写一个")
	}

	payChannel, err := normalizePayChannel(req.PayChannel)
	if err != nil {
		return nil, err
	}
	if payChannel == model.PayChannelMock && !s.paymentCfg.MockEnabled {
		return nil, fmt.Errorf("模拟支付未启用")
	}
	if payChannel == model.PayChannelWechat {
		return nil, fmt.Errorf("微信支付尚未接入")
	}
	payScene, err := normalizePayScene(payChannel, req.PayScene)
	if err != nil {
		return nil, err
	}
	if payChannel == model.PayChannelAlipay {
		if err := s.validateAlipayPayScene(payScene); err != nil {
			return nil, err
		}
	}

	var result *model.Payment
	err = s.paymentRepo.Transaction(ctx, func(repo repository.PaymentRepository) error {
		if req.TradeID > 0 {
			payment, err := s.createTradePayment(ctx, repo, userID, req.TradeID, payChannel, payScene)
			result = payment
			return err
		}
		payment, err := s.createLegacyOrderPayment(ctx, repo, userID, req.OrderID, payChannel, payScene)
		result = payment
		return err
	})
	if err != nil {
		return nil, err
	}
	return s.toPaymentResponse(ctx, *result)
}

func (s *paymentService) createLegacyOrderPayment(
	ctx context.Context,
	repo repository.PaymentRepository,
	userID, orderID int64,
	payChannel, payScene string,
) (*model.Payment, error) {
	order, err := repo.FindOrderByIDAndUserID(ctx, orderID, userID)
	if err != nil {
		return nil, fmt.Errorf("订单不存在")
	}
	if order.TradeID != nil {
		orders, err := repo.FindOrdersByTradeID(ctx, *order.TradeID, userID)
		if err != nil {
			return nil, err
		}
		if len(orders) != 1 || orders[0].ID != order.ID {
			return nil, fmt.Errorf("该订单属于多商户交易，请使用 trade_id 创建合并支付")
		}
		return s.createTradePayment(ctx, repo, userID, *order.TradeID, payChannel, payScene)
	}
	switch order.Status {
	case model.OrderStatusPendingPayment:
	case model.OrderStatusPaid:
		return nil, fmt.Errorf("订单已支付")
	case model.OrderStatusCancelled:
		return nil, fmt.Errorf("已取消订单不能支付")
	default:
		return nil, fmt.Errorf("当前订单状态不能支付")
	}
	if _, err := repo.FindPaidByOrderID(ctx, order.ID); err == nil {
		return nil, fmt.Errorf("订单已支付")
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	pendingPayments, err := repo.FindPendingByOrderID(ctx, order.ID)
	if err != nil {
		return nil, err
	}
	if len(pendingPayments) > 1 {
		return nil, fmt.Errorf("订单存在多个待支付单，请先完成支付对账")
	}
	if len(pendingPayments) == 1 {
		existing := pendingPayments[0]
		if existing.UserID != userID {
			return nil, fmt.Errorf("待支付单归属异常，请联系管理员")
		}
		if existing.PayChannel != payChannel || effectivePayScene(existing.PayChannel, existing.PayScene) != payScene {
			return nil, fmt.Errorf("订单已有其他支付方式的待支付单，请继续使用原支付方式或等待支付单超时关闭")
		}
		return &existing, nil
	}
	orderIDValue := order.ID
	merchantIDValue := order.MerchantID
	payment := &model.Payment{
		PaymentNo:     generatePaymentNo(),
		OrderID:       &orderIDValue,
		ActiveOrderID: activeOrderID(order.ID),
		OrderNo:       order.OrderNo,
		UserID:        userID,
		MerchantID:    &merchantIDValue,
		PayChannel:    payChannel,
		PayScene:      payScene,
		Status:        model.PaymentStatusPending,
		Amount:        order.PayableAmount,
	}
	if err := repo.Create(ctx, payment); err != nil {
		return nil, err
	}
	return payment, nil
}

func (s *paymentService) createTradePayment(
	ctx context.Context,
	repo repository.PaymentRepository,
	userID, tradeID int64,
	payChannel, payScene string,
) (*model.Payment, error) {
	trade, err := repo.FindTradeByIDAndUserID(ctx, tradeID, userID)
	if err != nil {
		return nil, fmt.Errorf("交易不存在")
	}
	switch trade.Status {
	case model.TradeStatusPendingPayment:
	case model.TradeStatusPaid:
		return nil, fmt.Errorf("交易已支付")
	case model.TradeStatusClosed:
		return nil, fmt.Errorf("已关闭交易不能支付")
	default:
		return nil, fmt.Errorf("当前交易状态不能支付")
	}
	if trade.PayableAmount <= 0 {
		return nil, fmt.Errorf("交易应付金额不合法")
	}
	if _, err := repo.FindPaidByTradeID(ctx, trade.ID); err == nil {
		return nil, fmt.Errorf("交易已支付")
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	orders, err := repo.FindOrdersByTradeID(ctx, trade.ID, userID)
	if err != nil {
		return nil, err
	}
	if len(orders) == 0 {
		return nil, fmt.Errorf("交易没有子订单")
	}
	for _, order := range orders {
		if order.Status != model.OrderStatusPendingPayment {
			return nil, fmt.Errorf("子订单 %d 状态不能支付", order.ID)
		}
	}
	pendingPayments, err := repo.FindPendingByTradeID(ctx, trade.ID)
	if err != nil {
		return nil, err
	}
	if len(pendingPayments) > 1 {
		return nil, fmt.Errorf("交易存在多个待支付单，请先完成支付对账")
	}
	if len(pendingPayments) == 1 {
		existing := pendingPayments[0]
		if existing.UserID != userID {
			return nil, fmt.Errorf("待支付单归属异常，请联系管理员")
		}
		if existing.PayChannel != payChannel || effectivePayScene(existing.PayChannel, existing.PayScene) != payScene {
			return nil, fmt.Errorf("交易已有其他支付方式的待支付单，请继续使用原支付方式或等待支付单超时关闭")
		}
		allocations, err := repo.FindAllocationsByPaymentID(ctx, existing.ID)
		if err != nil {
			return nil, err
		}
		if err := validateTradePaymentAmounts(trade, orders, existing, allocations); err != nil {
			return nil, err
		}
		return &existing, nil
	}

	tradeIDValue := trade.ID
	payment := &model.Payment{
		PaymentNo:     generatePaymentNo(),
		TradeID:       &tradeIDValue,
		ActiveTradeID: activeTradeID(trade.ID),
		UserID:        userID,
		PayChannel:    payChannel,
		PayScene:      payScene,
		Status:        model.PaymentStatusPending,
		Amount:        trade.PayableAmount,
	}
	if err := repo.Create(ctx, payment); err != nil {
		return nil, err
	}
	allocations := make([]model.PaymentAllocation, 0, len(orders))
	for _, order := range orders {
		allocations = append(allocations, model.PaymentAllocation{
			PaymentID: payment.ID, TradeID: trade.ID, OrderID: order.ID,
			MerchantID: order.MerchantID, Amount: order.PayableAmount,
		})
	}
	if err := validateTradePaymentAmounts(trade, orders, *payment, allocations); err != nil {
		return nil, err
	}
	if err := repo.CreateAllocations(ctx, allocations); err != nil {
		return nil, err
	}
	return payment, nil
}

func validateTradePaymentAmounts(
	trade *model.Trade,
	orders []model.Order,
	payment model.Payment,
	allocations []model.PaymentAllocation,
) error {
	if payment.TradeID == nil || *payment.TradeID != trade.ID || payment.Amount != trade.PayableAmount {
		return fmt.Errorf("支付单与交易金额不一致")
	}
	if len(orders) == 0 || len(allocations) != len(orders) {
		return fmt.Errorf("支付分配数量与子订单不一致")
	}
	ordersByID := make(map[int64]model.Order, len(orders))
	var total int64
	for _, order := range orders {
		if order.TradeID == nil || *order.TradeID != trade.ID || order.PayableAmount <= 0 {
			return fmt.Errorf("子订单 %d 金额或交易引用不合法", order.ID)
		}
		if total > trade.PayableAmount-order.PayableAmount {
			return fmt.Errorf("子订单金额总和超过交易金额")
		}
		total += order.PayableAmount
		ordersByID[order.ID] = order
	}
	if total != trade.PayableAmount {
		return fmt.Errorf("子订单金额总和与交易金额不一致")
	}
	var allocated int64
	seen := make(map[int64]struct{}, len(allocations))
	for _, allocation := range allocations {
		order, ok := ordersByID[allocation.OrderID]
		if !ok || allocation.TradeID != trade.ID || allocation.PaymentID != payment.ID ||
			allocation.MerchantID != order.MerchantID || allocation.Amount != order.PayableAmount {
			return fmt.Errorf("子订单 %d 支付分配不一致", allocation.OrderID)
		}
		if _, exists := seen[allocation.OrderID]; exists {
			return fmt.Errorf("子订单 %d 存在重复支付分配", allocation.OrderID)
		}
		seen[allocation.OrderID] = struct{}{}
		allocated += allocation.Amount
	}
	if allocated != payment.Amount {
		return fmt.Errorf("支付分配总额与支付金额不一致")
	}
	return nil
}

func (s *paymentService) Detail(ctx context.Context, userID int64, paymentNo string) (*dto.PaymentResponse, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("用户未登录")
	}

	paymentNo = strings.TrimSpace(paymentNo)
	if paymentNo == "" {
		return nil, fmt.Errorf("支付单号不能为空")
	}

	payment, err := s.paymentRepo.FindByPaymentNoAndUserID(ctx, paymentNo, userID)
	if err != nil {
		return nil, fmt.Errorf("支付单不存在")
	}

	return s.toPaymentResponse(ctx, *payment)
}

func (s *paymentService) MockComplete(ctx context.Context, userID int64, paymentNo string) (*dto.PaymentResponse, error) {
	if !s.paymentCfg.MockEnabled {
		return nil, fmt.Errorf("模拟支付未启用")
	}
	if userID <= 0 {
		return nil, fmt.Errorf("用户未登录")
	}

	paymentNo = strings.TrimSpace(paymentNo)
	if paymentNo == "" {
		return nil, fmt.Errorf("支付单号不能为空")
	}

	var result *model.Payment
	err := s.paymentRepo.Transaction(ctx, func(repo repository.PaymentRepository) error {
		payment, err := repo.FindByPaymentNoForUpdate(ctx, paymentNo)
		if err != nil {
			return fmt.Errorf("支付单不存在")
		}
		if payment.UserID != userID {
			return fmt.Errorf("支付单不存在")
		}

		if payment.Status == model.PaymentStatusPaid {
			result = payment
			return nil
		}
		if payment.Status != model.PaymentStatusPending {
			return fmt.Errorf("当前支付单状态不能完成支付")
		}
		if payment.PayChannel != model.PayChannelMock {
			return fmt.Errorf("只有模拟支付单可以使用模拟完成接口")
		}

		now := time.Now()
		transactionID := generateMockTransactionID()
		if err := s.completePayment(ctx, repo, payment, transactionID, now); err != nil {
			return err
		}

		payment.Status = model.PaymentStatusPaid
		payment.ActiveOrderID = nil
		payment.ActiveTradeID = nil
		payment.TransactionID = transactionID
		payment.PaidAt = &now
		result = payment
		return nil
	})
	if err != nil {
		return nil, err
	}

	return s.toPaymentResponse(ctx, *result)
}

func (s *paymentService) MockPayOrder(ctx context.Context, userID int64, orderID int64) (*dto.PaymentResponse, error) {
	if !s.paymentCfg.MockEnabled {
		return nil, fmt.Errorf("模拟支付未启用")
	}
	if userID <= 0 {
		return nil, fmt.Errorf("用户未登录")
	}
	if orderID <= 0 {
		return nil, fmt.Errorf("订单 ID 不合法")
	}

	var result *model.Payment
	err := s.paymentRepo.Transaction(ctx, func(repo repository.PaymentRepository) error {
		order, err := repo.FindOrderByIDAndUserID(ctx, orderID, userID)
		if err != nil {
			return fmt.Errorf("订单不存在")
		}
		if order.TradeID != nil {
			return fmt.Errorf("该订单属于交易，不能使用旧模拟支付入口")
		}

		switch order.Status {
		case model.OrderStatusPendingPayment:
		case model.OrderStatusPaid:
			return fmt.Errorf("订单已支付")
		case model.OrderStatusCancelled:
			return fmt.Errorf("已取消订单不能支付")
		default:
			return fmt.Errorf("当前订单状态不能支付")
		}

		if _, err := repo.FindPaidByOrderID(ctx, order.ID); err == nil {
			return fmt.Errorf("订单已支付")
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		now := time.Now()
		transactionID := generateMockTransactionID()
		pendingPayments, err := repo.FindPendingByOrderID(ctx, order.ID)
		if err != nil {
			return err
		}
		if len(pendingPayments) > 1 {
			return fmt.Errorf("订单存在多个待支付单，请先完成支付对账")
		}
		if len(pendingPayments) == 1 {
			existing := pendingPayments[0]
			if existing.UserID != userID {
				return fmt.Errorf("待支付单归属异常，请联系管理员")
			}
			if existing.PayChannel != model.PayChannelMock || effectivePayScene(existing.PayChannel, existing.PayScene) != model.PaySceneMock {
				return fmt.Errorf("订单已有其他支付方式的待支付单，不能使用模拟支付")
			}
			if err := repo.MarkPaid(ctx, existing.ID, userID, transactionID, now); err != nil {
				return err
			}
			existing.Status = model.PaymentStatusPaid
			existing.ActiveOrderID = nil
			existing.TransactionID = transactionID
			existing.PaidAt = &now
			result = &existing
		} else {
			orderIDValue := order.ID
			merchantIDValue := order.MerchantID
			payment := &model.Payment{
				PaymentNo:     generatePaymentNo(),
				OrderID:       &orderIDValue,
				OrderNo:       order.OrderNo,
				UserID:        userID,
				MerchantID:    &merchantIDValue,
				PayChannel:    model.PayChannelMock,
				PayScene:      model.PaySceneMock,
				Status:        model.PaymentStatusPaid,
				Amount:        order.PayableAmount,
				TransactionID: transactionID,
				PaidAt:        &now,
			}
			if err := repo.Create(ctx, payment); err != nil {
				return err
			}
			result = payment
		}

		if err := repo.UpdateOrderStatus(
			ctx,
			order.ID,
			userID,
			model.OrderStatusPendingPayment,
			model.OrderStatusPaid,
			&now,
		); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return toPaymentResponse(*result), nil
}
