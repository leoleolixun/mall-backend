package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go-mall/internal/config"
	"go-mall/internal/model"

	"github.com/smartwalle/alipay/v3"
)

type alipayTradeState int

const (
	alipayTradeStateUnknown alipayTradeState = iota
	alipayTradeStateNotExist
	alipayTradeStateWaiting
	alipayTradeStatePaid
	alipayTradeStateClosed
)

type alipayTradeResult struct {
	State         alipayTradeState
	TransactionID string
	PaidAt        time.Time
}

type alipayGateway interface {
	Query(ctx context.Context, payment model.Payment) (*alipayTradeResult, error)
	Close(ctx context.Context, payment model.Payment) error
}

type sdkAlipayGateway struct {
	cfg config.AlipayConfig
}

func newSDKAlipayGateway(cfg config.AlipayConfig) alipayGateway {
	return &sdkAlipayGateway{cfg: cfg}
}

func (g *sdkAlipayGateway) Query(ctx context.Context, payment model.Payment) (*alipayTradeResult, error) {
	client, err := newAlipayClient(g.cfg)
	if err != nil {
		return nil, err
	}
	result, err := client.TradeQuery(ctx, alipay.TradeQuery{OutTradeNo: payment.PaymentNo})
	if err != nil {
		return nil, fmt.Errorf("查询支付宝交易失败: %w", err)
	}
	if !result.IsSuccess() {
		if result.SubCode == "ACQ.TRADE_NOT_EXIST" {
			return &alipayTradeResult{State: alipayTradeStateNotExist}, nil
		}
		return nil, fmt.Errorf("查询支付宝交易失败: %s %s", result.SubCode, result.SubMsg)
	}
	if result.OutTradeNo != payment.PaymentNo {
		return nil, fmt.Errorf("支付宝返回的支付单号不匹配")
	}
	if result.TotalAmount != amountFenToYuan(payment.Amount) {
		return nil, fmt.Errorf("支付宝返回的支付金额不匹配")
	}

	switch result.TradeStatus {
	case alipay.TradeStatusWaitBuyerPay:
		return &alipayTradeResult{State: alipayTradeStateWaiting}, nil
	case alipay.TradeStatusSuccess, alipay.TradeStatusFinished:
		return &alipayTradeResult{
			State:         alipayTradeStatePaid,
			TransactionID: result.TradeNo,
			PaidAt:        parseAlipayPaidAt(result.SendPayDate),
		}, nil
	case alipay.TradeStatusClosed:
		return &alipayTradeResult{State: alipayTradeStateClosed}, nil
	default:
		return nil, fmt.Errorf("支付宝返回未知交易状态: %s", result.TradeStatus)
	}
}

func (g *sdkAlipayGateway) Close(ctx context.Context, payment model.Payment) error {
	client, err := newAlipayClient(g.cfg)
	if err != nil {
		return err
	}
	result, err := client.TradeClose(ctx, alipay.TradeClose{OutTradeNo: payment.PaymentNo})
	if err != nil {
		return fmt.Errorf("关闭支付宝交易失败: %w", err)
	}
	if result.IsSuccess() || result.SubCode == "ACQ.TRADE_NOT_EXIST" {
		return nil
	}
	message := strings.TrimSpace(result.SubMsg)
	if message == "" {
		message = strings.TrimSpace(result.Msg)
	}
	return fmt.Errorf("关闭支付宝交易失败: %s %s", result.SubCode, message)
}
