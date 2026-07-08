package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"go-mall/internal/dto"
	"go-mall/internal/model"
	"go-mall/internal/repository"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type PaymentService interface {
	Create(ctx context.Context, userID int64, req dto.CreatePaymentRequest) (*dto.PaymentResponse, error)
	Detail(ctx context.Context, userID int64, paymentNo string) (*dto.PaymentResponse, error)
	MockPayOrder(ctx context.Context, userID int64, orderID int64) (*dto.PaymentResponse, error)
}

type paymentService struct {
	paymentRepo repository.PaymentRepository
}

func NewPaymentService(paymentRepo repository.PaymentRepository) PaymentService {
	return &paymentService{
		paymentRepo: paymentRepo,
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

		existing, err := repo.FindLatestByOrderIDUserIDChannel(ctx, order.ID, userID, payChannel)
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

	return toPaymentResponse(*result), nil
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

	return toPaymentResponse(*payment), nil
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

		existing, err := repo.FindLatestByOrderIDUserIDChannel(ctx, order.ID, userID, model.PayChannelMock)
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
