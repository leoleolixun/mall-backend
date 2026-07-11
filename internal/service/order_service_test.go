package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"go-mall/internal/dto"
	"go-mall/internal/model"
	"go-mall/internal/repository"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

type fakeOrderRepository struct {
	order         *model.Order
	items         []model.OrderItem
	shipment      *model.Shipment
	inventoryLogs []model.InventoryLog
	createCalls   int
	completeErr   error
}

func (r *fakeOrderRepository) Transaction(_ context.Context, fn func(repository.OrderRepository) error) error {
	return fn(r)
}

func (r *fakeOrderRepository) Create(_ context.Context, order *model.Order) error {
	r.createCalls++
	order.ID = 42
	copy := *order
	r.order = &copy
	return nil
}

func (r *fakeOrderRepository) CreateItems(_ context.Context, items []model.OrderItem) error {
	r.items = append([]model.OrderItem(nil), items...)
	return nil
}

func (r *fakeOrderRepository) Update(_ context.Context, order *model.Order) error {
	copy := *order
	r.order = &copy
	return nil
}

func (r *fakeOrderRepository) UpdateStatus(_ context.Context, _ int64, _ int64, _ int, nextStatus int, _ *time.Time, _ *time.Time) error {
	if r.order != nil {
		r.order.Status = nextStatus
	}
	return nil
}

func (r *fakeOrderRepository) FindByIDAndUserID(_ context.Context, id int64, userID int64) (*model.Order, error) {
	if r.order == nil || r.order.ID != id || r.order.UserID != userID {
		return nil, context.Canceled
	}
	copy := *r.order
	return &copy, nil
}

func (r *fakeOrderRepository) FindItemsByOrderID(_ context.Context, _ int64) ([]model.OrderItem, error) {
	return append([]model.OrderItem(nil), r.items...), nil
}

func (r *fakeOrderRepository) FindShipmentByOrderID(_ context.Context, orderID int64) (*model.Shipment, error) {
	if r.shipment == nil || r.shipment.OrderID != orderID {
		return nil, context.Canceled
	}
	copy := *r.shipment
	return &copy, nil
}

func (r *fakeOrderRepository) Complete(_ context.Context, id int64, userID int64, completedAt time.Time) error {
	if r.order == nil || r.order.ID != id || r.order.UserID != userID || r.order.Status != model.OrderStatusShipped {
		return context.Canceled
	}
	r.order.Status = model.OrderStatusCompleted
	r.order.CompletedAt = &completedAt
	return r.completeErr
}

func (r *fakeOrderRepository) ListByUserID(_ context.Context, _ int64, _ int, _ int, _ int) ([]model.Order, int64, error) {
	return nil, 0, nil
}

func (r *fakeOrderRepository) DecreaseSKUStock(_ context.Context, _ int64, _ int64, quantity int) (int, error) {
	return 10 - quantity, nil
}

func (r *fakeOrderRepository) IncreaseSKUStock(_ context.Context, _ int64, _ int64, quantity int) (int, error) {
	return 10 + quantity, nil
}

func (r *fakeOrderRepository) CreateInventoryLogs(_ context.Context, logs []model.InventoryLog) error {
	r.inventoryLogs = append(r.inventoryLogs, logs...)
	return nil
}

func (r *fakeOrderRepository) ClosePendingPayments(_ context.Context, _ int64, _ time.Time) error {
	return nil
}

type fakeAddressRepository struct {
	address model.Address
}

func (r *fakeAddressRepository) ListByUserID(_ context.Context, _ int64) ([]model.Address, error) {
	return []model.Address{r.address}, nil
}

func (r *fakeAddressRepository) FindByIDAndUserID(_ context.Context, id int64, userID int64) (*model.Address, error) {
	if r.address.ID != id || r.address.UserID != userID {
		return nil, context.Canceled
	}
	copy := r.address
	return &copy, nil
}

func (r *fakeAddressRepository) Create(_ context.Context, address *model.Address) error {
	r.address = *address
	return nil
}

func (r *fakeAddressRepository) Update(_ context.Context, address *model.Address) error {
	r.address = *address
	return nil
}

func (r *fakeAddressRepository) DeleteByIDAndUserID(_ context.Context, _ int64, _ int64) error {
	return nil
}

func (r *fakeAddressRepository) ClearDefault(_ context.Context, _ int64) error {
	return nil
}

func (r *fakeAddressRepository) Transaction(_ context.Context, fn func(repository.AddressRepository) error) error {
	return fn(r)
}

type fakeProductRepository struct {
	product model.Product
	sku     model.ProductSKU
}

func (r *fakeProductRepository) ListOnSale(_ context.Context, _ int64, _ int64, _ string, _ int, _ int) ([]model.Product, int64, error) {
	return []model.Product{r.product}, 1, nil
}

func (r *fakeProductRepository) FindOnSaleByID(_ context.Context, _ int64, _ int64) (*model.Product, error) {
	copy := r.product
	return &copy, nil
}

func (r *fakeProductRepository) ListEnabledSKUs(_ context.Context, _ int64, _ int64) ([]model.ProductSKU, error) {
	return []model.ProductSKU{r.sku}, nil
}

func (r *fakeProductRepository) FindMinPrices(_ context.Context, _ int64, _ []int64) (map[int64]int64, error) {
	return map[int64]int64{r.product.ID: r.sku.Price}, nil
}

func (r *fakeProductRepository) FindSKUsByIDs(_ context.Context, _ int64, _ []int64) ([]model.ProductSKU, error) {
	return []model.ProductSKU{r.sku}, nil
}

func (r *fakeProductRepository) FindProductsByIDs(_ context.Context, _ int64, _ []int64) ([]model.Product, error) {
	return []model.Product{r.product}, nil
}

func newOrderServiceForTest(t *testing.T) (*orderService, *fakeOrderRepository, *miniredis.Miniredis) {
	t.Helper()

	redisServer := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() {
		_ = redisClient.Close()
	})

	orderRepo := &fakeOrderRepository{}
	addressRepo := &fakeAddressRepository{address: model.Address{
		ID:            4,
		UserID:        7,
		ReceiverName:  "测试用户",
		ReceiverPhone: "13800000000",
		Province:      "浙江省",
		City:          "杭州市",
		District:      "西湖区",
		Detail:        "测试地址 1 号",
	}}
	productRepo := &fakeProductRepository{
		product: model.Product{
			ID:         2,
			MerchantID: defaultMerchantID,
			Name:       "测试商品",
			Status:     model.ProductStatusOnSale,
		},
		sku: model.ProductSKU{
			ID:         3,
			MerchantID: defaultMerchantID,
			ProductID:  2,
			Name:       "默认规格",
			Price:      1990,
			Stock:      10,
			Status:     model.StatusEnabled,
		},
	}

	service := NewOrderService(orderRepo, addressRepo, productRepo, redisClient).(*orderService)
	return service, orderRepo, redisServer
}

func TestOrderCreateAcceptsMissingPreviewTokenAndIsIdempotent(t *testing.T) {
	service, orderRepo, redisServer := newOrderServiceForTest(t)
	ctx := context.Background()
	req := dto.CreateOrderRequest{
		AddressID:        4,
		Remark:           "PC 端下单",
		IdempotencyToken: "test-token",
		Items: []dto.OrderRequestItem{
			{SKUID: 3, Quantity: 3},
		},
	}

	created, err := service.Create(ctx, 7, req)
	if err != nil {
		t.Fatalf("first create returned error: %v", err)
	}
	if created.ID != 42 || created.PayableAmount != 5970 {
		t.Fatalf("unexpected created order: %+v", created)
	}
	if orderRepo.createCalls != 1 {
		t.Fatalf("expected one order insert, got %d", orderRepo.createCalls)
	}
	if len(orderRepo.inventoryLogs) != 1 || orderRepo.inventoryLogs[0].Quantity != -3 || orderRepo.inventoryLogs[0].ReferenceID != 42 {
		t.Fatalf("unexpected inventory logs: %+v", orderRepo.inventoryLogs)
	}
	if value, err := redisServer.Get(orderIdempotencyKey(7, req.IdempotencyToken)); err != nil || value != "order:42" {
		t.Fatalf("unexpected idempotency value %q, err=%v", value, err)
	}

	retried, err := service.Create(ctx, 7, req)
	if err != nil {
		t.Fatalf("retry returned error: %v", err)
	}
	if retried.ID != created.ID {
		t.Fatalf("retry returned order %d, want %d", retried.ID, created.ID)
	}
	if orderRepo.createCalls != 1 {
		t.Fatalf("retry inserted another order, create calls=%d", orderRepo.createCalls)
	}
}

func TestOrderCreateAcceptsPreviewToken(t *testing.T) {
	service, orderRepo, redisServer := newOrderServiceForTest(t)
	req := dto.CreateOrderRequest{
		AddressID:        4,
		IdempotencyToken: "preview-token",
		Items:            []dto.OrderRequestItem{{SKUID: 3, Quantity: 1}},
	}
	redisServer.Set(orderIdempotencyKey(7, req.IdempotencyToken), "pending")

	if _, err := service.Create(context.Background(), 7, req); err != nil {
		t.Fatalf("create with preview token returned error: %v", err)
	}
	if orderRepo.createCalls != 1 {
		t.Fatalf("expected one order insert, got %d", orderRepo.createCalls)
	}
}

func TestOrderCreateRejectsConcurrentSubmission(t *testing.T) {
	service, orderRepo, redisServer := newOrderServiceForTest(t)
	req := dto.CreateOrderRequest{
		AddressID:        4,
		IdempotencyToken: "processing-token",
		Items:            []dto.OrderRequestItem{{SKUID: 3, Quantity: 1}},
	}
	redisServer.Set(orderIdempotencyKey(7, req.IdempotencyToken), "processing")

	_, err := service.Create(context.Background(), 7, req)
	if err == nil || !strings.Contains(err.Error(), "正在处理") {
		t.Fatalf("expected processing error, got %v", err)
	}
	if orderRepo.createCalls != 0 {
		t.Fatalf("concurrent request inserted an order, create calls=%d", orderRepo.createCalls)
	}
}

func TestOrderLogisticsChecksOwnershipAndReturnsShipment(t *testing.T) {
	service, orderRepo, _ := newOrderServiceForTest(t)
	shippedAt := time.Date(2026, 7, 10, 12, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	orderRepo.order = &model.Order{ID: 42, UserID: 7, Status: model.OrderStatusShipped}
	orderRepo.shipment = &model.Shipment{
		OrderID:          42,
		LogisticsCompany: "顺丰速运",
		TrackingNo:       "SF123",
		ShippedAt:        shippedAt,
	}

	result, err := service.Logistics(context.Background(), 7, 42)
	if err != nil {
		t.Fatalf("logistics returned error: %v", err)
	}
	if result.TrackingNo != "SF123" || len(result.Traces) != 1 || result.Traces[0].Content != "商家已发货" {
		t.Fatalf("unexpected logistics response: %+v", result)
	}

	if _, err := service.Logistics(context.Background(), 8, 42); err == nil || !strings.Contains(err.Error(), "不存在") {
		t.Fatalf("expected ownership rejection, got %v", err)
	}
}

func TestOrderConfirmCompletesShippedOrderAndIsIdempotent(t *testing.T) {
	service, orderRepo, _ := newOrderServiceForTest(t)
	orderRepo.order = &model.Order{
		ID:        42,
		UserID:    7,
		Status:    model.OrderStatusShipped,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	result, err := service.Confirm(context.Background(), 7, 42)
	if err != nil {
		t.Fatalf("confirm returned error: %v", err)
	}
	if result.Status != model.OrderStatusCompleted || result.CompletedAt == nil {
		t.Fatalf("unexpected completed order: %+v", result)
	}

	if _, err := service.Confirm(context.Background(), 7, 42); err != nil {
		t.Fatalf("idempotent confirm returned error: %v", err)
	}
}

func TestOrderConfirmTreatsConcurrentCompletionAsSuccess(t *testing.T) {
	service, orderRepo, _ := newOrderServiceForTest(t)
	orderRepo.order = &model.Order{
		ID:        42,
		UserID:    7,
		Status:    model.OrderStatusShipped,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	orderRepo.completeErr = context.Canceled

	result, err := service.Confirm(context.Background(), 7, 42)
	if err != nil {
		t.Fatalf("concurrent confirm returned error: %v", err)
	}
	if result.Status != model.OrderStatusCompleted {
		t.Fatalf("unexpected concurrent completion result: %+v", result)
	}
}
