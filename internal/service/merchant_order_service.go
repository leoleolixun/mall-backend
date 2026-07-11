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

	"gorm.io/gorm"
)

type MerchantOrderService interface {
	List(ctx context.Context, merchantID int64, req dto.MerchantOrderListRequest) (*dto.PageResponse[dto.OrderResponse], error)
	Detail(ctx context.Context, merchantID int64, orderID int64) (*dto.OrderResponse, error)
	Ship(ctx context.Context, merchantID int64, orderID int64, req dto.ShipOrderRequest) (*dto.OrderResponse, error)
}

type merchantOrderService struct {
	repo repository.MerchantOrderRepository
}

func NewMerchantOrderService(repo repository.MerchantOrderRepository) MerchantOrderService {
	return &merchantOrderService{repo: repo}
}

func toShipmentResponse(shipment *model.Shipment) *dto.ShipmentResponse {
	if shipment == nil {
		return nil
	}
	return &dto.ShipmentResponse{
		ID:               shipment.ID,
		OrderID:          shipment.OrderID,
		DeliveryType:     shipment.DeliveryType,
		LogisticsCompany: shipment.LogisticsCompany,
		TrackingNo:       shipment.TrackingNo,
		ShippedAt:        shipment.ShippedAt.Format(time.RFC3339),
	}
}

func buildMerchantOrderResponse(
	order model.Order,
	items []model.OrderItem,
	shipment *model.Shipment,
	merchantName string,
) dto.OrderResponse {
	response := toOrderResponse(order, items)
	response.MerchantName = merchantName
	response.Shipment = toShipmentResponse(shipment)
	return *response
}

func validateMerchantOrderStatus(status int) error {
	if status < 0 || status > model.OrderStatusCancelled {
		return fmt.Errorf("status 参数不合法")
	}
	return nil
}

func (s *merchantOrderService) activeMerchant(ctx context.Context, merchantID int64) (*model.Merchant, error) {
	if merchantID <= 0 {
		return nil, fmt.Errorf("商户身份不合法")
	}
	merchant, err := s.repo.FindMerchantByID(ctx, merchantID)
	if err != nil || merchant.Status != model.StatusEnabled {
		return nil, fmt.Errorf("商户不可用")
	}
	return merchant, nil
}

func (s *merchantOrderService) List(
	ctx context.Context,
	merchantID int64,
	req dto.MerchantOrderListRequest,
) (*dto.PageResponse[dto.OrderResponse], error) {
	merchant, err := s.activeMerchant(ctx, merchantID)
	if err != nil {
		return nil, err
	}
	if err := validateMerchantOrderStatus(req.Status); err != nil {
		return nil, err
	}
	page, pageSize := normalizeOrderPage(req.Page, req.PageSize)
	orders, total, err := s.repo.ListByMerchantID(ctx, merchantID, (page-1)*pageSize, pageSize, req.Status)
	if err != nil {
		return nil, err
	}

	orderIDs := make([]int64, 0, len(orders))
	for _, order := range orders {
		orderIDs = append(orderIDs, order.ID)
	}
	items, err := s.repo.FindItemsByOrderIDs(ctx, orderIDs)
	if err != nil {
		return nil, err
	}
	shipments, err := s.repo.FindShipmentsByOrderIDs(ctx, orderIDs)
	if err != nil {
		return nil, err
	}

	itemsByOrderID := make(map[int64][]model.OrderItem, len(orderIDs))
	for _, item := range items {
		itemsByOrderID[item.OrderID] = append(itemsByOrderID[item.OrderID], item)
	}
	shipmentsByOrderID := make(map[int64]*model.Shipment, len(shipments))
	for i := range shipments {
		shipmentsByOrderID[shipments[i].OrderID] = &shipments[i]
	}

	list := make([]dto.OrderResponse, 0, len(orders))
	for _, order := range orders {
		list = append(list, buildMerchantOrderResponse(
			order,
			itemsByOrderID[order.ID],
			shipmentsByOrderID[order.ID],
			merchant.Name,
		))
	}
	return &dto.PageResponse[dto.OrderResponse]{
		List:     list,
		Page:     page,
		PageSize: pageSize,
		Total:    total,
	}, nil
}

func (s *merchantOrderService) Detail(
	ctx context.Context,
	merchantID int64,
	orderID int64,
) (*dto.OrderResponse, error) {
	if orderID <= 0 {
		return nil, fmt.Errorf("订单 ID 不合法")
	}
	merchant, err := s.activeMerchant(ctx, merchantID)
	if err != nil {
		return nil, err
	}
	order, err := s.repo.FindByIDAndMerchantID(ctx, orderID, merchantID)
	if err != nil {
		return nil, fmt.Errorf("订单不存在")
	}
	items, err := s.repo.FindItemsByOrderID(ctx, order.ID)
	if err != nil {
		return nil, err
	}
	shipment, err := s.repo.FindShipmentByOrderID(ctx, order.ID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	response := buildMerchantOrderResponse(*order, items, shipment, merchant.Name)
	return &response, nil
}

func (s *merchantOrderService) Ship(
	ctx context.Context,
	merchantID int64,
	orderID int64,
	req dto.ShipOrderRequest,
) (*dto.OrderResponse, error) {
	if orderID <= 0 {
		return nil, fmt.Errorf("订单 ID 不合法")
	}
	deliveryType := strings.TrimSpace(req.DeliveryType)
	if deliveryType == "" {
		deliveryType = model.DeliveryTypeExpress
	}
	company := strings.TrimSpace(req.LogisticsCompany)
	trackingNo := strings.TrimSpace(req.TrackingNo)
	switch deliveryType {
	case model.DeliveryTypeExpress:
		if company == "" || trackingNo == "" {
			return nil, fmt.Errorf("普通快递必须填写物流公司和运单号")
		}
	case model.DeliveryTypeSelfDelivery:
		company = "商家配送"
		trackingNo = ""
	default:
		return nil, fmt.Errorf("配送类型不支持")
	}
	if len([]rune(company)) > 100 || len(trackingNo) > 100 {
		return nil, fmt.Errorf("物流公司或运单号过长")
	}
	if _, err := s.activeMerchant(ctx, merchantID); err != nil {
		return nil, err
	}

	err := s.repo.Transaction(ctx, func(repo repository.MerchantOrderRepository) error {
		order, err := repo.FindByIDAndMerchantIDForUpdate(ctx, orderID, merchantID)
		if err != nil {
			return fmt.Errorf("订单不存在")
		}
		if order.Status == model.OrderStatusShipped {
			shipment, err := repo.FindShipmentByOrderID(ctx, order.ID)
			if err == nil && shipment.DeliveryType == deliveryType && shipment.LogisticsCompany == company && shipment.TrackingNo == trackingNo {
				return nil
			}
			return fmt.Errorf("订单已发货，不能修改物流信息")
		}
		if order.Status != model.OrderStatusPaid {
			return fmt.Errorf("只有已支付订单可以发货")
		}

		shipment := &model.Shipment{
			OrderID:          order.ID,
			MerchantID:       merchantID,
			DeliveryType:     deliveryType,
			LogisticsCompany: company,
			TrackingNo:       trackingNo,
			ShippedAt:        time.Now(),
		}
		if err := repo.CreateShipment(ctx, shipment); err != nil {
			return err
		}
		return repo.MarkShipped(ctx, order.ID, merchantID)
	})
	if err != nil {
		return nil, err
	}
	return s.Detail(ctx, merchantID, orderID)
}
