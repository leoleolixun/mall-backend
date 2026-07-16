package service

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"go-mall/internal/dto"
	"go-mall/internal/model"
	"go-mall/internal/repository"

	"gorm.io/gorm"
)

type fakeMerchantOrderRepository struct {
	merchant            model.Merchant
	orders              []model.Order
	items               []model.OrderItem
	shipment            *model.Shipment
	createShipmentCalls int
}

func (r *fakeMerchantOrderRepository) Transaction(_ context.Context, fn func(repository.MerchantOrderRepository) error) error {
	return fn(r)
}

func (r *fakeMerchantOrderRepository) ListByMerchantID(
	_ context.Context,
	merchantID int64,
	offset int,
	limit int,
	status int,
	keyword string,
) ([]model.Order, int64, error) {
	filtered := make([]model.Order, 0)
	for _, order := range r.orders {
		matchesKeyword := keyword == "" || strings.Contains(order.OrderNo, keyword) || strings.Contains(order.ReceiverName, keyword) || strings.Contains(order.ReceiverPhone, keyword)
		if order.MerchantID == merchantID && (status == 0 || order.Status == status) && matchesKeyword {
			filtered = append(filtered, order)
		}
	}
	total := int64(len(filtered))
	if offset >= len(filtered) {
		return []model.Order{}, total, nil
	}
	end := offset + limit
	if end > len(filtered) {
		end = len(filtered)
	}
	return append([]model.Order(nil), filtered[offset:end]...), total, nil
}

func (r *fakeMerchantOrderRepository) FindByIDAndMerchantID(_ context.Context, orderID int64, merchantID int64) (*model.Order, error) {
	return r.findOrder(orderID, merchantID)
}

func (r *fakeMerchantOrderRepository) FindByIDAndMerchantIDForUpdate(_ context.Context, orderID int64, merchantID int64) (*model.Order, error) {
	return r.findOrder(orderID, merchantID)
}

func (r *fakeMerchantOrderRepository) findOrder(orderID int64, merchantID int64) (*model.Order, error) {
	for i := range r.orders {
		if r.orders[i].ID == orderID && r.orders[i].MerchantID == merchantID {
			return &r.orders[i], nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (r *fakeMerchantOrderRepository) FindItemsByOrderID(_ context.Context, orderID int64) ([]model.OrderItem, error) {
	return filterOrderItems(r.items, map[int64]struct{}{orderID: {}}), nil
}

func (r *fakeMerchantOrderRepository) FindItemsByOrderIDs(_ context.Context, orderIDs []int64) ([]model.OrderItem, error) {
	ids := make(map[int64]struct{}, len(orderIDs))
	for _, orderID := range orderIDs {
		ids[orderID] = struct{}{}
	}
	return filterOrderItems(r.items, ids), nil
}

func filterOrderItems(items []model.OrderItem, orderIDs map[int64]struct{}) []model.OrderItem {
	result := make([]model.OrderItem, 0)
	for _, item := range items {
		if _, ok := orderIDs[item.OrderID]; ok {
			result = append(result, item)
		}
	}
	return result
}

func (r *fakeMerchantOrderRepository) FindShipmentByOrderID(_ context.Context, orderID int64) (*model.Shipment, error) {
	if r.shipment == nil || r.shipment.OrderID != orderID {
		return nil, gorm.ErrRecordNotFound
	}
	copy := *r.shipment
	return &copy, nil
}

func (r *fakeMerchantOrderRepository) FindShipmentsByOrderIDs(_ context.Context, orderIDs []int64) ([]model.Shipment, error) {
	if r.shipment == nil {
		return []model.Shipment{}, nil
	}
	for _, orderID := range orderIDs {
		if r.shipment.OrderID == orderID {
			return []model.Shipment{*r.shipment}, nil
		}
	}
	return []model.Shipment{}, nil
}

func (r *fakeMerchantOrderRepository) FindMerchantByID(_ context.Context, merchantID int64) (*model.Merchant, error) {
	if r.merchant.ID != merchantID {
		return nil, gorm.ErrRecordNotFound
	}
	copy := r.merchant
	return &copy, nil
}

func (r *fakeMerchantOrderRepository) CreateShipment(_ context.Context, shipment *model.Shipment) error {
	if r.shipment != nil {
		return fmt.Errorf("duplicate shipment")
	}
	r.createShipmentCalls++
	shipment.ID = 21
	copy := *shipment
	r.shipment = &copy
	return nil
}

func (r *fakeMerchantOrderRepository) MarkShipped(_ context.Context, orderID int64, merchantID int64) error {
	order, err := r.findOrder(orderID, merchantID)
	if err != nil {
		return err
	}
	if order.Status != model.OrderStatusPaid {
		return fmt.Errorf("订单状态已变更")
	}
	order.Status = model.OrderStatusShipped
	return nil
}

func newMerchantOrderServiceForTest(status int) (MerchantOrderService, *fakeMerchantOrderRepository) {
	now := time.Now()
	repo := &fakeMerchantOrderRepository{
		merchant: model.Merchant{ID: 1, Name: "测试商户", Status: model.StatusEnabled},
		orders: []model.Order{
			{
				ID:              10,
				OrderNo:         "O202607100001",
				UserID:          7,
				MerchantID:      1,
				Status:          status,
				ReceiverName:    "测试用户",
				ReceiverPhone:   "13800000000",
				ReceiverAddress: "测试地址",
				GoodsAmount:     1990,
				PayableAmount:   1990,
				CreatedAt:       now,
				UpdatedAt:       now,
			},
			{ID: 11, OrderNo: "OTHER", MerchantID: 2, Status: model.OrderStatusPaid, CreatedAt: now, UpdatedAt: now},
		},
		items: []model.OrderItem{
			{ID: 1, OrderID: 10, ProductID: 2, SKUID: 3, ProductName: "测试商品", SKUName: "默认规格", Price: 1990, Quantity: 1, Subtotal: 1990},
			{ID: 2, OrderID: 11, ProductID: 9, SKUID: 9, ProductName: "其他商户商品", SKUName: "规格", Price: 100, Quantity: 1, Subtotal: 100},
		},
	}
	return NewMerchantOrderService(repo), repo
}

func TestMerchantOrderListIsScopedByMerchant(t *testing.T) {
	service, _ := newMerchantOrderServiceForTest(model.OrderStatusPaid)
	result, err := service.List(context.Background(), 1, dto.MerchantOrderListRequest{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("list returned error: %v", err)
	}
	if result.Total != 1 || len(result.List) != 1 || result.List[0].MerchantID != 1 {
		t.Fatalf("unexpected merchant order page: %+v", result)
	}
	if len(result.List[0].Items) != 1 || result.List[0].Items[0].ProductName != "测试商品" {
		t.Fatalf("unexpected order items: %+v", result.List[0].Items)
	}
}

func TestMerchantOrderListFiltersByKeyword(t *testing.T) {
	service, _ := newMerchantOrderServiceForTest(model.OrderStatusPaid)

	for _, keyword := range []string{"O202607", "测试用户", "1380000"} {
		result, err := service.List(context.Background(), 1, dto.MerchantOrderListRequest{Page: 1, PageSize: 10, Keyword: keyword})
		if err != nil {
			t.Fatalf("keyword %q returned error: %v", keyword, err)
		}
		if result.Total != 1 || len(result.List) != 1 || result.List[0].OrderNo != "O202607100001" {
			t.Fatalf("keyword %q returned unexpected page: %+v", keyword, result)
		}
	}

	result, err := service.List(context.Background(), 1, dto.MerchantOrderListRequest{Page: 1, PageSize: 10, Keyword: "不存在"})
	if err != nil {
		t.Fatalf("missing keyword returned error: %v", err)
	}
	if result.Total != 0 || len(result.List) != 0 {
		t.Fatalf("missing keyword should return an empty page: %+v", result)
	}
}

func TestMerchantOrderShipAndRetry(t *testing.T) {
	service, repo := newMerchantOrderServiceForTest(model.OrderStatusPaid)
	req := dto.ShipOrderRequest{LogisticsCompany: "顺丰速运", TrackingNo: "SF1234567890"}

	result, err := service.Ship(context.Background(), 1, 10, req)
	if err != nil {
		t.Fatalf("ship returned error: %v", err)
	}
	if result.Status != model.OrderStatusShipped || result.Shipment == nil || result.Shipment.TrackingNo != req.TrackingNo {
		t.Fatalf("unexpected shipped order: %+v", result)
	}
	if repo.createShipmentCalls != 1 {
		t.Fatalf("expected one shipment insert, got %d", repo.createShipmentCalls)
	}

	if _, err := service.Ship(context.Background(), 1, 10, req); err != nil {
		t.Fatalf("idempotent retry returned error: %v", err)
	}
	if repo.createShipmentCalls != 1 {
		t.Fatalf("retry inserted another shipment, calls=%d", repo.createShipmentCalls)
	}

	_, err = service.Ship(context.Background(), 1, 10, dto.ShipOrderRequest{
		LogisticsCompany: "中通快递",
		TrackingNo:       "ZT0001",
	})
	if err == nil {
		t.Fatal("expected changing shipped logistics to fail")
	}
}

func TestMerchantOrderShipRejectsUnpaidOrder(t *testing.T) {
	service, repo := newMerchantOrderServiceForTest(model.OrderStatusPendingPayment)
	_, err := service.Ship(context.Background(), 1, 10, dto.ShipOrderRequest{
		LogisticsCompany: "顺丰速运",
		TrackingNo:       "SF1234567890",
	})
	if err == nil {
		t.Fatal("expected unpaid order to be rejected")
	}
	if repo.createShipmentCalls != 0 {
		t.Fatal("unpaid order created a shipment")
	}
}

func TestMerchantOrderSupportsSelfDeliveryWithoutTrackingNo(t *testing.T) {
	service, repo := newMerchantOrderServiceForTest(model.OrderStatusPaid)
	result, err := service.Ship(context.Background(), 1, 10, dto.ShipOrderRequest{
		DeliveryType: model.DeliveryTypeSelfDelivery,
	})
	if err != nil {
		t.Fatalf("self delivery returned error: %v", err)
	}
	if result.Shipment == nil || result.Shipment.DeliveryType != model.DeliveryTypeSelfDelivery {
		t.Fatalf("unexpected self delivery shipment: %+v", result.Shipment)
	}
	if result.Shipment.LogisticsCompany != "商家配送" || result.Shipment.TrackingNo != "" {
		t.Fatalf("unexpected normalized self delivery fields: %+v", result.Shipment)
	}
	if repo.createShipmentCalls != 1 {
		t.Fatalf("expected one shipment insert, got %d", repo.createShipmentCalls)
	}
}

func TestMerchantOrderExpressRequiresTrackingFields(t *testing.T) {
	service, repo := newMerchantOrderServiceForTest(model.OrderStatusPaid)
	_, err := service.Ship(context.Background(), 1, 10, dto.ShipOrderRequest{
		DeliveryType: model.DeliveryTypeExpress,
	})
	if err == nil {
		t.Fatal("expected missing express tracking fields to fail")
	}
	if repo.createShipmentCalls != 0 {
		t.Fatal("invalid express request created a shipment")
	}
}

func TestMerchantOrderDetailRejectsAnotherMerchant(t *testing.T) {
	service, _ := newMerchantOrderServiceForTest(model.OrderStatusPaid)
	if _, err := service.Detail(context.Background(), 1, 11); err == nil {
		t.Fatal("expected another merchant order to be hidden")
	}
}
