package service

import (
	"context"
	"testing"

	"go-mall/internal/config"
	"go-mall/internal/dto"
	"go-mall/internal/model"
)

func TestPaymentServiceRejectsMockPaymentWhenDisabled(t *testing.T) {
	service := NewPaymentService(nil, config.PaymentConfig{})

	if _, err := service.Create(context.Background(), 1, dto.CreatePaymentRequest{
		OrderID:    1,
		PayChannel: model.PayChannelMock,
	}); err == nil {
		t.Fatal("expected mock payment creation to be disabled")
	}
	if _, err := service.MockComplete(context.Background(), 1, "PAY001"); err == nil {
		t.Fatal("expected mock payment completion to be disabled")
	}
	if _, err := service.MockPayOrder(context.Background(), 1, 1); err == nil {
		t.Fatal("expected legacy mock payment to be disabled")
	}
}

func TestNormalizePayChannelRequiresExplicitChannel(t *testing.T) {
	if _, err := normalizePayChannel(""); err == nil {
		t.Fatal("expected empty payment channel to be rejected")
	}
}

func TestPaymentServiceRejectsUnimplementedWechatPayment(t *testing.T) {
	service := NewPaymentService(nil, config.PaymentConfig{})

	if _, err := service.Create(context.Background(), 1, dto.CreatePaymentRequest{
		OrderID:    1,
		PayChannel: model.PayChannelWechat,
	}); err == nil {
		t.Fatal("expected unimplemented WeChat payment to be rejected")
	}
}
