package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go-mall/internal/dto"
	"go-mall/internal/model"
	"go-mall/internal/repository"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type OrderService interface {
	Preview(ctx context.Context, userID int64, req dto.OrderPreviewRequest) (*dto.OrderPreviewResponse, error)
	Create(ctx context.Context, userID int64, req dto.CreateOrderRequest) (*dto.OrderResponse, error)
	List(ctx context.Context, userID int64, req dto.OrderListRequest) (*dto.PageResponse[dto.OrderResponse], error)
	Detail(ctx context.Context, userID int64, id int64) (*dto.OrderResponse, error)
	Cancel(ctx context.Context, userID int64, id int64) error
}

type orderService struct {
	orderRepo   repository.OrderRepository
	addressRepo repository.AddressRepository
	productRepo repository.ProductRepository
	redisClient *redis.Client
}

func NewOrderService(
	orderRepo repository.OrderRepository,
	addressRepo repository.AddressRepository,
	productRepo repository.ProductRepository,
	redisClient *redis.Client,
) OrderService {
	return &orderService{
		orderRepo:   orderRepo,
		addressRepo: addressRepo,
		productRepo: productRepo,
		redisClient: redisClient,
	}
}

func orderIdempotencyKey(userID int64, token string) string {
	return fmt.Sprintf("mall:order:idempotency:%d:%s", userID, token)
}

func generateOrderNo() string {
	return fmt.Sprintf("O%s%s", time.Now().Format("20060102150405"), uuid.NewString()[:8])
}

func orderStatusText(status int) string {
	switch status {
	case model.OrderStatusPendingPayment:
		return "待支付"
	case model.OrderStatusPaid:
		return "已支付"
	case model.OrderStatusShipped:
		return "已发货"
	case model.OrderStatusCompleted:
		return "已完成"
	case model.OrderStatusCancelled:
		return "已取消"
	default:
		return "未知状态"
	}
}

func buildReceiverAddress(address *model.Address) string {
	return address.Province + address.City + address.District + address.Detail
}

func toOrderItemResponse(item model.OrderItem) dto.OrderItemResponse {
	return dto.OrderItemResponse{
		ID:          item.ID,
		ProductID:   item.ProductID,
		SKUID:       item.SKUID,
		ProductName: item.ProductName,
		SKUName:     item.SKUName,
		SKUImage:    item.SKUImage,
		Price:       item.Price,
		Quantity:    item.Quantity,
		Subtotal:    item.Subtotal,
	}
}

func toOrderResponse(order model.Order, items []model.OrderItem) *dto.OrderResponse {
	itemResponses := make([]dto.OrderItemResponse, 0, len(items))
	for _, item := range items {
		itemResponses = append(itemResponses, toOrderItemResponse(item))
	}

	var paidAt *string
	if order.PaidAt != nil {
		value := order.PaidAt.Format(time.RFC3339)
		paidAt = &value
	}

	var cancelledAt *string
	if order.CancelledAt != nil {
		value := order.CancelledAt.Format(time.RFC3339)
		cancelledAt = &value
	}

	return &dto.OrderResponse{
		ID:              order.ID,
		OrderNo:         order.OrderNo,
		UserID:          order.UserID,
		MerchantID:      order.MerchantID,
		MerchantName:    "默认商户",
		Status:          order.Status,
		StatusText:      orderStatusText(order.Status),
		ReceiverName:    order.ReceiverName,
		ReceiverPhone:   order.ReceiverPhone,
		ReceiverAddress: order.ReceiverAddress,
		GoodsAmount:     order.GoodsAmount,
		FreightAmount:   order.FreightAmount,
		DiscountAmount:  order.DiscountAmount,
		PayableAmount:   order.PayableAmount,
		Remark:          order.Remark,
		PaidAt:          paidAt,
		CancelledAt:     cancelledAt,
		Items:           itemResponses,
	}
}

func normalizeOrderPage(page int, pageSize int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 50 {
		pageSize = 50
	}
	return page, pageSize
}

func (s *orderService) Preview(
	ctx context.Context,
	userID int64,
	req dto.OrderPreviewRequest,
) (*dto.OrderPreviewResponse, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("用户未登录")
	}

	if req.AddressID <= 0 {
		return nil, fmt.Errorf("收货地址不能为空")
	}

	if len(req.Items) == 0 {
		return nil, fmt.Errorf("订单商品不能为空")
	}

	// 查询收货地址
	address, err := s.addressRepo.FindByIDAndUserID(ctx, req.AddressID, userID)
	if err != nil {
		return nil, fmt.Errorf("收货地址不存在")
	}

	// 整理 SKU ID 和数量
	quantityMap := make(map[int64]int)
	skuIDs := make([]int64, 0, len(req.Items))

	for _, item := range req.Items {
		if item.SKUID <= 0 {
			return nil, fmt.Errorf("sku_id 不能为空")
		}
		if item.Quantity <= 0 {
			return nil, fmt.Errorf("购买数量必须大于 0")
		}

		if _, exists := quantityMap[item.SKUID]; !exists {
			skuIDs = append(skuIDs, item.SKUID)
		}

		quantityMap[item.SKUID] += item.Quantity
	}

	// 批量查询 SKU

	skus, err := s.productRepo.FindSKUsByIDs(ctx, defaultMerchantID, skuIDs)
	if err != nil {
		return nil, err
	}

	skuMap := make(map[int64]model.ProductSKU)
	productIDSet := make(map[int64]struct{})

	for _, sku := range skus {
		skuMap[sku.ID] = sku
		productIDSet[sku.ProductID] = struct{}{}
	}

	// 批量查询商品
	productIDs := make([]int64, 0, len(productIDSet))
	for productID := range productIDSet {
		productIDs = append(productIDs, productID)
	}

	products, err := s.productRepo.FindProductsByIDs(ctx, defaultMerchantID, productIDs)
	if err != nil {
		return nil, err
	}

	productMap := make(map[int64]model.Product)
	for _, product := range products {
		productMap[product.ID] = product
	}

	// 校验商品并组装明细
	items := make([]dto.OrderItemResponse, 0, len(quantityMap))
	var goodsAmount int64

	for skuID, quantity := range quantityMap {
		sku, ok := skuMap[skuID]
		if !ok {
			return nil, fmt.Errorf("SKU 不存在")
		}

		if sku.Status != model.StatusEnabled {
			return nil, fmt.Errorf("SKU 已下架")
		}

		if sku.Stock < quantity {
			return nil, fmt.Errorf("库存不足")
		}

		product, ok := productMap[sku.ProductID]
		if !ok {
			return nil, fmt.Errorf("商品不存在")
		}

		if product.Status != model.ProductStatusOnSale {
			return nil, fmt.Errorf("商品已下架")
		}

		subtotal := sku.Price * int64(quantity)
		goodsAmount += subtotal

		items = append(items, dto.OrderItemResponse{
			ProductID:   product.ID,
			SKUID:       sku.ID,
			ProductName: product.Name,
			SKUName:     sku.Name,
			SKUImage:    sku.Image,
			Price:       sku.Price,
			Quantity:    quantity,
			Subtotal:    subtotal,
		})
	}

	// 生成幂等 token
	idempotencyToken := uuid.NewString()

	if err := s.redisClient.Set(
		ctx,
		orderIdempotencyKey(userID, idempotencyToken),
		"pending",
		15*time.Minute,
	).Err(); err != nil {
		return nil, err
	}

	// 返回预览结果
	freightAmount := int64(0)
	discountAmount := int64(0)
	payableAmount := goodsAmount + freightAmount - discountAmount

	return &dto.OrderPreviewResponse{
		IdempotencyToken: idempotencyToken,
		MerchantID:       defaultMerchantID,
		MerchantName:     "默认商户",
		Address: dto.AddressResponse{
			ID:            address.ID,
			ReceiverName:  address.ReceiverName,
			ReceiverPhone: address.ReceiverPhone,
			Province:      address.Province,
			City:          address.City,
			District:      address.District,
			Detail:        address.Detail,
			IsDefault:     address.IsDefault,
		},
		Items:          items,
		GoodsAmount:    goodsAmount,
		FreightAmount:  freightAmount,
		DiscountAmount: discountAmount,
		PayableAmount:  payableAmount,
	}, nil
}

func (s *orderService) Create(
	ctx context.Context,
	userID int64,
	req dto.CreateOrderRequest,
) (*dto.OrderResponse, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("用户未登录")
	}

	if req.AddressID <= 0 {
		return nil, fmt.Errorf("收货地址不能为空")
	}

	if req.IdempotencyToken == "" {
		return nil, fmt.Errorf("幂等 token 不能为空")
	}

	if len(req.Items) == 0 {
		return nil, fmt.Errorf("订单商品不能为空")
	}

	tokenKey := orderIdempotencyKey(userID, req.IdempotencyToken)

	tokenValue, err := s.redisClient.GetDel(ctx, tokenKey).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, fmt.Errorf("订单预览已过期，请重新预览订单")
		}
		return nil, fmt.Errorf("订单预览校验失败，请稍后重试")
	}

	if tokenValue != "pending" {
		return nil, fmt.Errorf("订单已提交，请勿重复提交")
	}

	quantityMap := make(map[int64]int)
	skuIDs := make([]int64, 0, len(req.Items))

	for _, item := range req.Items {
		if item.SKUID <= 0 {
			return nil, fmt.Errorf("sku_id 不能为空")
		}
		if item.Quantity <= 0 {
			return nil, fmt.Errorf("购买数量必须大于 0")
		}

		if _, exists := quantityMap[item.SKUID]; !exists {
			skuIDs = append(skuIDs, item.SKUID)
		}

		quantityMap[item.SKUID] += item.Quantity
	}

	var createdOrder *model.Order
	var createdItems []model.OrderItem

	err = s.orderRepo.Transaction(ctx, func(repo repository.OrderRepository) error {
		address, err := s.addressRepo.FindByIDAndUserID(ctx, req.AddressID, userID)
		if err != nil {
			return fmt.Errorf("收货地址不存在")
		}

		skus, err := s.productRepo.FindSKUsByIDs(ctx, defaultMerchantID, skuIDs)
		if err != nil {
			return err
		}

		skuMap := make(map[int64]model.ProductSKU)
		productIDSet := make(map[int64]struct{})

		for _, sku := range skus {
			skuMap[sku.ID] = sku
			productIDSet[sku.ProductID] = struct{}{}
		}

		productIDs := make([]int64, 0, len(productIDSet))
		for productID := range productIDSet {
			productIDs = append(productIDs, productID)
		}

		products, err := s.productRepo.FindProductsByIDs(ctx, defaultMerchantID, productIDs)
		if err != nil {
			return err
		}

		productMap := make(map[int64]model.Product)
		for _, product := range products {
			productMap[product.ID] = product
		}

		orderNo := generateOrderNo()
		order := &model.Order{
			OrderNo:         orderNo,
			UserID:          userID,
			MerchantID:      defaultMerchantID,
			Status:          model.OrderStatusPendingPayment,
			ReceiverName:    address.ReceiverName,
			ReceiverPhone:   address.ReceiverPhone,
			ReceiverAddress: buildReceiverAddress(address),
			Remark:          req.Remark,
		}

		items := make([]model.OrderItem, 0, len(quantityMap))
		var goodsAmount int64

		for skuID, quantity := range quantityMap {
			sku, ok := skuMap[skuID]
			if !ok {
				return fmt.Errorf("SKU 不存在")
			}

			if sku.Status != model.StatusEnabled {
				return fmt.Errorf("SKU 已下架")
			}

			product, ok := productMap[sku.ProductID]
			if !ok {
				return fmt.Errorf("商品不存在")
			}

			if product.Status != model.ProductStatusOnSale {
				return fmt.Errorf("商品已下架")
			}

			if err := repo.DecreaseSKUStock(ctx, defaultMerchantID, sku.ID, quantity); err != nil {
				return err
			}

			subtotal := sku.Price * int64(quantity)
			goodsAmount += subtotal

			items = append(items, model.OrderItem{
				ProductID:   product.ID,
				SKUID:       sku.ID,
				ProductName: product.Name,
				SKUName:     sku.Name,
				SKUImage:    sku.Image,
				Price:       sku.Price,
				Quantity:    quantity,
				Subtotal:    subtotal,
			})
		}

		order.GoodsAmount = goodsAmount
		order.FreightAmount = 0
		order.DiscountAmount = 0
		order.PayableAmount = goodsAmount

		if err := repo.Create(ctx, order); err != nil {
			return err
		}

		for i := range items {
			items[i].OrderID = order.ID
		}

		if err := repo.CreateItems(ctx, items); err != nil {
			return err
		}

		createdOrder = order
		createdItems = items
		return nil
	})
	if err != nil {
		return nil, err
	}

	for skuID := range quantityMap {
		_ = s.redisClient.HDel(ctx, cartKey(userID), fmt.Sprintf("%d", skuID)).Err()
	}

	return toOrderResponse(*createdOrder, createdItems), nil
}

func (s *orderService) List(
	ctx context.Context,
	userID int64,
	req dto.OrderListRequest,
) (*dto.PageResponse[dto.OrderResponse], error) {
	if userID <= 0 {
		return nil, fmt.Errorf("用户未登录")
	}

	page, pageSize := normalizeOrderPage(req.Page, req.PageSize)
	offset := (page - 1) * pageSize

	orders, total, err := s.orderRepo.ListByUserID(ctx, userID, offset, pageSize, req.Status)
	if err != nil {
		return nil, err
	}

	list := make([]dto.OrderResponse, 0, len(orders))
	for _, order := range orders {
		items, err := s.orderRepo.FindItemsByOrderID(ctx, order.ID)
		if err != nil {
			return nil, err
		}
		resp := toOrderResponse(order, items)
		list = append(list, *resp)
	}

	return &dto.PageResponse[dto.OrderResponse]{
		List:     list,
		Page:     page,
		PageSize: pageSize,
		Total:    total,
	}, nil
}

func (s *orderService) Detail(ctx context.Context, userID int64, id int64) (*dto.OrderResponse, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("用户未登录")
	}
	if id <= 0 {
		return nil, fmt.Errorf("订单 ID 不合法")
	}

	order, err := s.orderRepo.FindByIDAndUserID(ctx, id, userID)
	if err != nil {
		return nil, fmt.Errorf("订单不存在")
	}

	items, err := s.orderRepo.FindItemsByOrderID(ctx, order.ID)
	if err != nil {
		return nil, err
	}

	return toOrderResponse(*order, items), nil
}

func (s *orderService) Cancel(ctx context.Context, userID int64, id int64) error {
	if userID <= 0 {
		return fmt.Errorf("用户未登录")
	}
	if id <= 0 {
		return fmt.Errorf("订单 ID 不合法")
	}

	return s.orderRepo.Transaction(ctx, func(repo repository.OrderRepository) error {
		order, err := repo.FindByIDAndUserID(ctx, id, userID)
		if err != nil {
			return fmt.Errorf("订单不存在")
		}

		switch order.Status {
		case model.OrderStatusPendingPayment:
		case model.OrderStatusPaid:
			return fmt.Errorf("已支付订单不能取消")
		case model.OrderStatusCancelled:
			return fmt.Errorf("订单已取消")
		default:
			return fmt.Errorf("当前订单状态不能取消")
		}

		items, err := repo.FindItemsByOrderID(ctx, order.ID)
		if err != nil {
			return err
		}

		now := time.Now()
		if err := repo.UpdateStatus(
			ctx,
			order.ID,
			userID,
			model.OrderStatusPendingPayment,
			model.OrderStatusCancelled,
			nil,
			&now,
		); err != nil {
			return err
		}

		for _, item := range items {
			if err := repo.IncreaseSKUStock(ctx, item.SKUID, item.Quantity); err != nil {
				return err
			}
		}

		return nil
	})
}
