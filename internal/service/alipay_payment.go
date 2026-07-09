package service

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"go-mall/internal/config"
	"go-mall/internal/model"
	"go-mall/internal/repository"

	"github.com/smartwalle/alipay/v3"
)

const (
	alipayPageProductCode = "FAST_INSTANT_TRADE_PAY"
	alipayWapProductCode  = "QUICK_WAP_WAY"
)

func loadTextConfig(name string, value string, path string) (string, error) {
	value = strings.TrimSpace(strings.ReplaceAll(value, `\n`, "\n"))
	if value != "" {
		return value, nil
	}

	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("%s 未配置", name)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("读取 %s 失败: %w", name, err)
	}

	text := strings.TrimSpace(string(data))
	if text == "" {
		return "", fmt.Errorf("%s 内容为空", name)
	}
	return text, nil
}

func newAlipayClient(cfg config.AlipayConfig) (*alipay.Client, error) {
	if !cfg.Enabled {
		return nil, fmt.Errorf("支付宝支付未启用")
	}
	if strings.TrimSpace(cfg.AppID) == "" {
		return nil, fmt.Errorf("支付宝 app_id 未配置")
	}

	privateKey, err := loadTextConfig("支付宝应用私钥", cfg.AppPrivateKey, cfg.AppPrivateKeyPath)
	if err != nil {
		return nil, err
	}

	client, err := alipay.New(cfg.AppID, privateKey, !cfg.Sandbox)
	if err != nil {
		return nil, fmt.Errorf("初始化支付宝客户端失败: %w", err)
	}

	if cfg.AppCertPublicKeyPath != "" || cfg.AlipayRootCertPath != "" || cfg.AlipayCertPublicKeyPath != "" {
		if cfg.AppCertPublicKeyPath == "" || cfg.AlipayRootCertPath == "" || cfg.AlipayCertPublicKeyPath == "" {
			return nil, fmt.Errorf("支付宝证书模式需要同时配置 app_cert_public_key_path、alipay_root_cert_path、alipay_cert_public_key_path")
		}
		if err := client.LoadAppCertPublicKeyFromFile(cfg.AppCertPublicKeyPath); err != nil {
			return nil, fmt.Errorf("加载支付宝应用公钥证书失败: %w", err)
		}
		if err := client.LoadAliPayRootCertFromFile(cfg.AlipayRootCertPath); err != nil {
			return nil, fmt.Errorf("加载支付宝根证书失败: %w", err)
		}
		if err := client.LoadAlipayCertPublicKeyFromFile(cfg.AlipayCertPublicKeyPath); err != nil {
			return nil, fmt.Errorf("加载支付宝公钥证书失败: %w", err)
		}
		return client, nil
	}

	publicKey, err := loadTextConfig("支付宝公钥", cfg.AlipayPublicKey, cfg.AlipayPublicKeyPath)
	if err != nil {
		return nil, err
	}
	if err := client.LoadAliPayPublicKey(publicKey); err != nil {
		return nil, fmt.Errorf("加载支付宝公钥失败: %w", err)
	}

	return client, nil
}

func amountFenToYuan(amount int64) string {
	return fmt.Sprintf("%d.%02d", amount/100, amount%100)
}

func alipayPaymentSubject(payment model.Payment) string {
	return fmt.Sprintf("go-mall 订单 %s", payment.OrderNo)
}

func (s *paymentService) validateAlipayPayScene(payScene string) error {
	cfg := s.paymentCfg.Alipay
	if strings.TrimSpace(cfg.NotifyURL) == "" {
		return fmt.Errorf("支付宝 notify_url 未配置")
	}
	if _, err := newAlipayClient(cfg); err != nil {
		return err
	}

	switch payScene {
	case model.PaySceneAlipayPage:
		if !cfg.Page.Enabled {
			return fmt.Errorf("支付宝网页支付未启用")
		}
	case model.PaySceneAlipayWap:
		if !cfg.Wap.Enabled {
			return fmt.Errorf("支付宝 H5 支付未启用")
		}
	default:
		return fmt.Errorf("支付宝支付场景不支持")
	}

	return nil
}

func (s *paymentService) buildAlipayPayParams(ctx context.Context, payment model.Payment) (map[string]interface{}, error) {
	cfg := s.paymentCfg.Alipay
	if strings.TrimSpace(cfg.NotifyURL) == "" {
		return nil, fmt.Errorf("支付宝 notify_url 未配置")
	}

	client, err := newAlipayClient(cfg)
	if err != nil {
		return nil, err
	}

	payScene := effectivePayScene(payment.PayChannel, payment.PayScene)
	switch payScene {
	case model.PaySceneAlipayPage:
		if !cfg.Page.Enabled {
			return nil, fmt.Errorf("支付宝网页支付未启用")
		}

		productCode := strings.TrimSpace(cfg.Page.ProductCode)
		if productCode == "" {
			productCode = alipayPageProductCode
		}

		timeoutExpress := strings.TrimSpace(cfg.Page.TimeoutExpress)
		if timeoutExpress == "" {
			timeoutExpress = "15m"
		}

		payURL, err := client.TradePagePay(alipay.TradePagePay{
			Trade: alipay.Trade{
				NotifyURL:      cfg.NotifyURL,
				ReturnURL:      cfg.Page.ReturnURL,
				Subject:        alipayPaymentSubject(payment),
				OutTradeNo:     payment.PaymentNo,
				TotalAmount:    amountFenToYuan(payment.Amount),
				ProductCode:    productCode,
				Body:           payment.OrderNo,
				TimeoutExpress: timeoutExpress,
			},
		})
		if err != nil {
			return nil, fmt.Errorf("生成支付宝网页支付链接失败: %w", err)
		}

		return map[string]interface{}{
			"provider":    model.PayChannelAlipay,
			"scene":       payScene,
			"method":      "redirect",
			"pay_url":     payURL.String(),
			"payment_no":  payment.PaymentNo,
			"expires_in":  900,
			"total_yuan":  amountFenToYuan(payment.Amount),
			"notify_url":  cfg.NotifyURL,
			"return_url":  cfg.Page.ReturnURL,
			"sdk_request": "alipay.trade.page.pay",
		}, nil

	case model.PaySceneAlipayWap:
		if !cfg.Wap.Enabled {
			return nil, fmt.Errorf("支付宝 H5 支付未启用")
		}

		productCode := strings.TrimSpace(cfg.Wap.ProductCode)
		if productCode == "" {
			productCode = alipayWapProductCode
		}

		timeoutExpress := strings.TrimSpace(cfg.Wap.TimeoutExpress)
		if timeoutExpress == "" {
			timeoutExpress = "15m"
		}

		payURL, err := client.TradeWapPay(alipay.TradeWapPay{
			Trade: alipay.Trade{
				NotifyURL:      cfg.NotifyURL,
				ReturnURL:      cfg.Wap.ReturnURL,
				Subject:        alipayPaymentSubject(payment),
				OutTradeNo:     payment.PaymentNo,
				TotalAmount:    amountFenToYuan(payment.Amount),
				ProductCode:    productCode,
				Body:           payment.OrderNo,
				TimeoutExpress: timeoutExpress,
			},
			QuitURL: cfg.Wap.QuitURL,
		})
		if err != nil {
			return nil, fmt.Errorf("生成支付宝 H5 支付链接失败: %w", err)
		}

		return map[string]interface{}{
			"provider":    model.PayChannelAlipay,
			"scene":       payScene,
			"method":      "redirect",
			"pay_url":     payURL.String(),
			"payment_no":  payment.PaymentNo,
			"expires_in":  900,
			"total_yuan":  amountFenToYuan(payment.Amount),
			"notify_url":  cfg.NotifyURL,
			"return_url":  cfg.Wap.ReturnURL,
			"quit_url":    cfg.Wap.QuitURL,
			"sdk_request": "alipay.trade.wap.pay",
		}, nil

	default:
		return nil, fmt.Errorf("支付宝支付场景不支持")
	}
}

func parseAlipayPaidAt(value string) time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Now()
	}

	paidAt, err := time.ParseInLocation("2006-01-02 15:04:05", value, time.Local)
	if err != nil {
		return time.Now()
	}
	return paidAt
}

func (s *paymentService) AlipayNotify(ctx context.Context, values url.Values) error {
	cfg := s.paymentCfg.Alipay
	client, err := newAlipayClient(cfg)
	if err != nil {
		return err
	}

	notification, err := client.DecodeNotification(ctx, values)
	if err != nil {
		return fmt.Errorf("支付宝通知验签失败: %w", err)
	}
	if notification.AppId != cfg.AppID {
		return fmt.Errorf("支付宝 app_id 不匹配")
	}
	if notification.TradeStatus != alipay.TradeStatusSuccess && notification.TradeStatus != alipay.TradeStatusFinished {
		return nil
	}

	paymentNo := strings.TrimSpace(notification.OutTradeNo)
	if paymentNo == "" {
		return fmt.Errorf("支付宝通知缺少 out_trade_no")
	}

	return s.paymentRepo.Transaction(ctx, func(repo repository.PaymentRepository) error {
		payment, err := repo.FindByPaymentNo(ctx, paymentNo)
		if err != nil {
			return fmt.Errorf("支付单不存在")
		}
		if payment.PayChannel != model.PayChannelAlipay {
			return fmt.Errorf("支付渠道不匹配")
		}
		if notification.TotalAmount != amountFenToYuan(payment.Amount) {
			return fmt.Errorf("支付宝通知金额不匹配")
		}
		if payment.Status == model.PaymentStatusPaid {
			return nil
		}
		if payment.Status != model.PaymentStatusPending {
			return fmt.Errorf("当前支付单状态不能完成支付")
		}

		order, err := repo.FindOrderByIDAndUserID(ctx, payment.OrderID, payment.UserID)
		if err != nil {
			return fmt.Errorf("订单不存在")
		}
		if order.Status == model.OrderStatusPaid {
			return nil
		}
		if order.Status != model.OrderStatusPendingPayment {
			return fmt.Errorf("当前订单状态不能支付")
		}

		paidAt := parseAlipayPaidAt(notification.GmtPayment)
		if err := repo.MarkPaid(ctx, payment.ID, payment.UserID, notification.TradeNo, paidAt); err != nil {
			return err
		}
		if err := repo.UpdateOrderStatus(
			ctx,
			order.ID,
			payment.UserID,
			model.OrderStatusPendingPayment,
			model.OrderStatusPaid,
			&paidAt,
		); err != nil {
			return err
		}

		return nil
	})
}
