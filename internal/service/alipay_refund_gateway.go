package service

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"time"

	"go-mall/internal/config"
	"go-mall/internal/model"

	"github.com/smartwalle/alipay/v3"
)

type refundProviderState int

const (
	refundProviderStateUnknown refundProviderState = iota
	refundProviderStateNotFound
	refundProviderStateSucceeded
	refundProviderStateRejected
)

type refundProviderResult struct {
	State         refundProviderState
	TransactionID string
	RefundedAt    time.Time
	Reason        string
}

type alipayRefundGateway interface {
	Submit(ctx context.Context, payment model.Payment, refund model.Refund, reason string) (refundProviderResult, error)
	Query(ctx context.Context, payment model.Payment, refund model.Refund) (refundProviderResult, error)
}

type sdkAlipayRefundGateway struct {
	cfg config.AlipayConfig
}

func newSDKAlipayRefundGateway(cfg config.AlipayConfig) alipayRefundGateway {
	return &sdkAlipayRefundGateway{cfg: cfg}
}

func (g *sdkAlipayRefundGateway) Submit(
	ctx context.Context,
	payment model.Payment,
	refund model.Refund,
	reason string,
) (refundProviderResult, error) {
	client, err := newAlipayClient(g.cfg)
	if err != nil {
		return refundProviderResult{}, err
	}
	result, err := client.TradeRefund(ctx, alipay.TradeRefund{
		OutTradeNo:   payment.PaymentNo,
		RefundAmount: amountFenToYuan(refund.Amount),
		RefundReason: reason,
		OutRequestNo: refund.RefundNo,
	})
	if err != nil {
		return refundProviderResult{}, fmt.Errorf("提交支付宝退款失败: %w", err)
	}

	message := alipayResultMessage(result.SubCode, result.SubMsg, result.Msg)
	if result.Code != alipay.CodeSuccess {
		if isRetryableAlipayCode(result.Code) {
			return refundProviderResult{State: refundProviderStateUnknown, Reason: message}, nil
		}
		return refundProviderResult{State: refundProviderStateRejected, Reason: message}, nil
	}
	if result.OutTradeNo != "" && result.OutTradeNo != payment.PaymentNo {
		return refundProviderResult{}, fmt.Errorf("支付宝返回的支付单号不匹配")
	}
	if result.RefundFee != "" && !yuanAmountMatches(result.RefundFee, refund.Amount) {
		return refundProviderResult{}, fmt.Errorf("支付宝返回的退款金额不匹配")
	}
	if !strings.EqualFold(strings.TrimSpace(result.FundChange), "Y") {
		return refundProviderResult{
			State:  refundProviderStateUnknown,
			Reason: "支付宝已受理退款，但尚未确认资金变更",
		}, nil
	}

	return refundProviderResult{
		State:         refundProviderStateSucceeded,
		TransactionID: result.TradeNo,
		RefundedAt:    time.Now(),
	}, nil
}

func (g *sdkAlipayRefundGateway) Query(
	ctx context.Context,
	payment model.Payment,
	refund model.Refund,
) (refundProviderResult, error) {
	client, err := newAlipayClient(g.cfg)
	if err != nil {
		return refundProviderResult{}, err
	}
	result, err := client.TradeFastPayRefundQuery(ctx, alipay.TradeFastPayRefundQuery{
		OutTradeNo:   payment.PaymentNo,
		OutRequestNo: refund.RefundNo,
	})
	if err != nil {
		return refundProviderResult{}, fmt.Errorf("查询支付宝退款失败: %w", err)
	}

	if result.Code != alipay.CodeSuccess {
		if isAlipayRefundNotFound(result.SubCode) {
			return refundProviderResult{State: refundProviderStateNotFound}, nil
		}
		return refundProviderResult{
			State:  refundProviderStateUnknown,
			Reason: alipayResultMessage(result.SubCode, result.SubMsg, result.Msg),
		}, nil
	}
	if strings.TrimSpace(result.RefundStatus) == "" {
		return refundProviderResult{State: refundProviderStateNotFound}, nil
	}
	if result.OutTradeNo != payment.PaymentNo || result.OutRequestNo != refund.RefundNo {
		return refundProviderResult{}, fmt.Errorf("支付宝返回的退款单号不匹配")
	}
	if result.RefundStatus != "REFUND_SUCCESS" {
		return refundProviderResult{
			State:  refundProviderStateUnknown,
			Reason: fmt.Sprintf("支付宝返回未知退款状态: %s", result.RefundStatus),
		}, nil
	}
	if !yuanAmountMatches(result.RefundAmount, refund.Amount) {
		return refundProviderResult{}, fmt.Errorf("支付宝返回的退款金额不匹配")
	}

	refundedAt := parseAlipayRefundedAt(result.GMTRefundPay)
	return refundProviderResult{
		State:         refundProviderStateSucceeded,
		TransactionID: result.TradeNo,
		RefundedAt:    refundedAt,
	}, nil
}

func isRetryableAlipayCode(code alipay.Code) bool {
	return code == alipay.CodeUnknowError ||
		code == alipay.CodeCallLimited ||
		code == alipay.CodeOrderSuccessPayInProcess
}

func isAlipayRefundNotFound(subCode string) bool {
	switch strings.TrimSpace(subCode) {
	case "ACQ.REFUND_NOT_EXIST", "ACQ.TRADE_NOT_EXIST":
		return true
	default:
		return false
	}
}

func alipayResultMessage(subCode, subMessage, message string) string {
	parts := make([]string, 0, 2)
	if value := strings.TrimSpace(subCode); value != "" {
		parts = append(parts, value)
	}
	if value := strings.TrimSpace(subMessage); value != "" {
		parts = append(parts, value)
	} else if value := strings.TrimSpace(message); value != "" {
		parts = append(parts, value)
	}
	if len(parts) == 0 {
		return "支付宝未返回明确的退款结果"
	}
	return strings.Join(parts, " ")
}

func yuanAmountMatches(value string, expectedFen int64) bool {
	amount, ok := new(big.Rat).SetString(strings.TrimSpace(value))
	if !ok {
		return false
	}
	amount.Mul(amount, big.NewRat(100, 1))
	return amount.Denom().Cmp(big.NewInt(1)) == 0 && amount.Num().IsInt64() && amount.Num().Int64() == expectedFen
}

func parseAlipayRefundedAt(value string) time.Time {
	value = strings.TrimSpace(value)
	for _, layout := range []string{"2006-01-02 15:04:05", time.RFC3339} {
		if parsed, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return parsed
		}
	}
	return time.Now()
}
