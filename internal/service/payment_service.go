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
}

type paymentService struct {
	paymentRepo repository.PaymentRepository
	paymentCfg  config.PaymentConfig
}

func NewPaymentService(paymentRepo repository.PaymentRepository, paymentCfg config.PaymentConfig) PaymentService {
	return &paymentService{
		paymentRepo: paymentRepo,
		paymentCfg:  paymentCfg,
	}
}

func generatePaymentNo() string {
	return fmt.Sprintf("P%s%s", time.Now().Format("20060102150405"), uuid.NewString()[:8])
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
	default:
		return "未知状态"
	}
}

func normalizePayChannel(payChannel string) (string, error) {
	payChannel = strings.TrimSpace(payChannel)
	if payChannel == "" {
		return model.PayChannelMock, nil
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
	if payment.PayChannel == model.PayChannelWechat {
		resp.PayParams = map[string]interface{}{
			"mock":       true,
			"mode":       "qr_code",
			"code_url":   fmt.Sprintf("weixin://wxpay/bizpayurl?pr=%s", payment.PaymentNo),
			"payment_no": payment.PaymentNo,
			"expires_in": 300,
		}
	}
	if payment.PayChannel == model.PayChannelAlipay {
		resp.PayParams = map[string]interface{}{
			"mock":        true,
			"mode":        "qr_code",
			"qr_code_url": fmt.Sprintf("https://openapi.alipay.com/gateway.do?mock_payment_no=%s", payment.PaymentNo),
			"payment_no":  payment.PaymentNo,
			"expires_in":  300,
			"return_mode": "web",
		}
	}

	return resp
}

func (s *paymentService) toPaymentResponse(ctx context.Context, payment model.Payment) (*dto.PaymentResponse, error) {
	resp := toPaymentResponse(payment)
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
	if req.OrderID <= 0 {
		return nil, fmt.Errorf("订单 ID 不合法")
	}

	payChannel, err := normalizePayChannel(req.PayChannel)
	if err != nil {
		return nil, err
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
		order, err := repo.FindOrderByIDAndUserID(ctx, req.OrderID, userID)
		if err != nil {
			return fmt.Errorf("订单不存在")
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

		existing, err := repo.FindLatestByOrderIDUserIDChannelScene(ctx, order.ID, userID, payChannel, payScene)
		if err == nil {
			if existing.Status == model.PaymentStatusPending {
				result = existing
				return nil
			}
			if existing.Status == model.PaymentStatusPaid {
				return fmt.Errorf("订单已支付")
			}
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		payment := &model.Payment{
			PaymentNo:  generatePaymentNo(),
			OrderID:    order.ID,
			OrderNo:    order.OrderNo,
			UserID:     userID,
			MerchantID: order.MerchantID,
			PayChannel: payChannel,
			PayScene:   payScene,
			Status:     model.PaymentStatusPending,
			Amount:     order.PayableAmount,
		}
		if err := repo.Create(ctx, payment); err != nil {
			return err
		}

		result = payment
		return nil
	})
	if err != nil {
		return nil, err
	}

	return s.toPaymentResponse(ctx, *result)
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
	if userID <= 0 {
		return nil, fmt.Errorf("用户未登录")
	}

	paymentNo = strings.TrimSpace(paymentNo)
	if paymentNo == "" {
		return nil, fmt.Errorf("支付单号不能为空")
	}

	var result *model.Payment
	err := s.paymentRepo.Transaction(ctx, func(repo repository.PaymentRepository) error {
		payment, err := repo.FindByPaymentNoAndUserID(ctx, paymentNo, userID)
		if err != nil {
			return fmt.Errorf("支付单不存在")
		}

		if payment.Status == model.PaymentStatusPaid {
			result = payment
			return nil
		}
		if payment.Status != model.PaymentStatusPending {
			return fmt.Errorf("当前支付单状态不能完成支付")
		}

		order, err := repo.FindOrderByIDAndUserID(ctx, payment.OrderID, userID)
		if err != nil {
			return fmt.Errorf("订单不存在")
		}
		if order.Status == model.OrderStatusPaid {
			return fmt.Errorf("订单已支付")
		}
		if order.Status != model.OrderStatusPendingPayment {
			return fmt.Errorf("当前订单状态不能支付")
		}

		now := time.Now()
		transactionID := generateMockTransactionID()
		if err := repo.MarkPaid(ctx, payment.ID, userID, transactionID, now); err != nil {
			return err
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

		payment.Status = model.PaymentStatusPaid
		payment.TransactionID = transactionID
		payment.PaidAt = &now
		result = payment
		return nil
	})
	if err != nil {
		return nil, err
	}

	return toPaymentResponse(*result), nil
}

func (s *paymentService) MockPayOrder(ctx context.Context, userID int64, orderID int64) (*dto.PaymentResponse, error) {
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

		switch order.Status {
		case model.OrderStatusPendingPayment:
		case model.OrderStatusPaid:
			return fmt.Errorf("订单已支付")
		case model.OrderStatusCancelled:
			return fmt.Errorf("已取消订单不能支付")
		default:
			return fmt.Errorf("当前订单状态不能支付")
		}

		now := time.Now()
		transactionID := generateMockTransactionID()

		existing, err := repo.FindLatestByOrderIDUserIDChannelScene(ctx, order.ID, userID, model.PayChannelMock, model.PaySceneMock)
		if err == nil {
			switch existing.Status {
			case model.PaymentStatusPending:
				if err := repo.MarkPaid(ctx, existing.ID, userID, transactionID, now); err != nil {
					return err
				}
				existing.Status = model.PaymentStatusPaid
				existing.TransactionID = transactionID
				existing.PaidAt = &now
				result = existing
			case model.PaymentStatusPaid:
				return fmt.Errorf("订单已支付")
			default:
				payment := &model.Payment{
					PaymentNo:     generatePaymentNo(),
					OrderID:       order.ID,
					OrderNo:       order.OrderNo,
					UserID:        userID,
					MerchantID:    order.MerchantID,
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
		} else if errors.Is(err, gorm.ErrRecordNotFound) {
			payment := &model.Payment{
				PaymentNo:     generatePaymentNo(),
				OrderID:       order.ID,
				OrderNo:       order.OrderNo,
				UserID:        userID,
				MerchantID:    order.MerchantID,
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
		} else {
			return err
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
