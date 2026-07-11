package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go-mall/internal/dto"
	"go-mall/internal/model"
	"go-mall/internal/repository"
)

type MerchantInventoryService interface {
	List(
		ctx context.Context,
		merchantID int64,
		req dto.MerchantInventoryLogListRequest,
	) (*dto.PageResponse[dto.MerchantInventoryLogResponse], error)
	ListAlerts(
		ctx context.Context,
		merchantID int64,
		req dto.MerchantInventoryAlertListRequest,
	) (*dto.PageResponse[dto.MerchantInventoryAlertResponse], error)
	AdjustStock(
		ctx context.Context,
		merchantID int64,
		operatorID int64,
		skuID int64,
		req dto.MerchantStockAdjustmentRequest,
	) (*dto.MerchantStockResponse, error)
}

func (s *merchantInventoryService) validateMerchant(ctx context.Context, merchantID int64) error {
	if merchantID <= 0 {
		return fmt.Errorf("商户身份不合法")
	}
	merchant, err := s.repo.FindMerchantByID(ctx, merchantID)
	if err != nil || merchant.Status != model.StatusEnabled {
		return fmt.Errorf("商户不可用")
	}
	return nil
}

func toMerchantInventoryAlertResponse(alert repository.MerchantInventoryAlertRecord) dto.MerchantInventoryAlertResponse {
	severity := "low_stock"
	if alert.Stock == 0 {
		severity = "out_of_stock"
	}
	return dto.MerchantInventoryAlertResponse{
		MerchantID:        alert.MerchantID,
		ProductID:         alert.ProductID,
		SKUID:             alert.SKUID,
		ProductName:       alert.ProductName,
		SKUName:           alert.SKUName,
		Image:             alert.Image,
		Stock:             alert.Stock,
		LowStockThreshold: alert.LowStockThreshold,
		Severity:          severity,
		UpdatedAt:         alert.UpdatedAt.Format(time.RFC3339),
	}
}

type merchantInventoryService struct {
	repo repository.MerchantInventoryRepository
}

func NewMerchantInventoryService(repo repository.MerchantInventoryRepository) MerchantInventoryService {
	return &merchantInventoryService{repo: repo}
}

func validInventoryChangeType(changeType string) bool {
	switch changeType {
	case "",
		model.InventoryChangeOrderCreate,
		model.InventoryChangeOrderCancel,
		model.InventoryChangeOrderTimeout,
		model.InventoryChangeMerchantInit,
		model.InventoryChangeMerchantAdjustment:
		return true
	default:
		return false
	}
}

func toMerchantInventoryLogResponse(log model.InventoryLog) dto.MerchantInventoryLogResponse {
	return dto.MerchantInventoryLogResponse{
		ID:            log.ID,
		MerchantID:    log.MerchantID,
		ProductID:     log.ProductID,
		SKUID:         log.SKUID,
		ProductName:   log.ProductName,
		SKUName:       log.SKUName,
		ChangeType:    log.ChangeType,
		Quantity:      log.Quantity,
		BeforeStock:   log.BeforeStock,
		AfterStock:    log.AfterStock,
		ReferenceType: log.ReferenceType,
		ReferenceID:   log.ReferenceID,
		OperatorType:  log.OperatorType,
		OperatorID:    log.OperatorID,
		Remark:        log.Remark,
		CreatedAt:     log.CreatedAt.Format(time.RFC3339),
	}
}

func (s *merchantInventoryService) List(
	ctx context.Context,
	merchantID int64,
	req dto.MerchantInventoryLogListRequest,
) (*dto.PageResponse[dto.MerchantInventoryLogResponse], error) {
	if err := s.validateMerchant(ctx, merchantID); err != nil {
		return nil, err
	}
	if req.ProductID < 0 || req.SKUID < 0 {
		return nil, fmt.Errorf("商品 ID 或 SKU ID 不合法")
	}
	changeType := strings.TrimSpace(req.ChangeType)
	if !validInventoryChangeType(changeType) {
		return nil, fmt.Errorf("change_type 参数不合法")
	}
	page, pageSize := normalizeMerchantProductPage(req.Page, req.PageSize)
	logs, total, err := s.repo.List(
		ctx,
		merchantID,
		req.ProductID,
		req.SKUID,
		changeType,
		(page-1)*pageSize,
		pageSize,
	)
	if err != nil {
		return nil, err
	}
	list := make([]dto.MerchantInventoryLogResponse, 0, len(logs))
	for _, log := range logs {
		list = append(list, toMerchantInventoryLogResponse(log))
	}
	return &dto.PageResponse[dto.MerchantInventoryLogResponse]{
		List: list, Page: page, PageSize: pageSize, Total: total,
	}, nil
}

func (s *merchantInventoryService) ListAlerts(
	ctx context.Context,
	merchantID int64,
	req dto.MerchantInventoryAlertListRequest,
) (*dto.PageResponse[dto.MerchantInventoryAlertResponse], error) {
	if err := s.validateMerchant(ctx, merchantID); err != nil {
		return nil, err
	}
	if req.ProductID < 0 || req.SKUID < 0 {
		return nil, fmt.Errorf("商品 ID 或 SKU ID 不合法")
	}
	keyword := strings.TrimSpace(req.Keyword)
	if len([]rune(keyword)) > 100 {
		return nil, fmt.Errorf("搜索关键词不能超过 100 个字符")
	}
	page, pageSize := normalizeMerchantProductPage(req.Page, req.PageSize)
	alerts, total, err := s.repo.ListAlerts(
		ctx,
		merchantID,
		req.ProductID,
		req.SKUID,
		keyword,
		(page-1)*pageSize,
		pageSize,
	)
	if err != nil {
		return nil, err
	}
	list := make([]dto.MerchantInventoryAlertResponse, 0, len(alerts))
	for _, alert := range alerts {
		list = append(list, toMerchantInventoryAlertResponse(alert))
	}
	return &dto.PageResponse[dto.MerchantInventoryAlertResponse]{
		List: list, Page: page, PageSize: pageSize, Total: total,
	}, nil
}

func (s *merchantInventoryService) AdjustStock(
	ctx context.Context,
	merchantID int64,
	operatorID int64,
	skuID int64,
	req dto.MerchantStockAdjustmentRequest,
) (*dto.MerchantStockResponse, error) {
	if err := s.validateMerchant(ctx, merchantID); err != nil {
		return nil, err
	}
	if operatorID <= 0 || skuID <= 0 {
		return nil, fmt.Errorf("操作人或 SKU ID 不合法")
	}
	if req.Stock == nil {
		return nil, fmt.Errorf("SKU 库存不能为空")
	}
	if *req.Stock < 0 {
		return nil, fmt.Errorf("SKU 库存不能小于 0")
	}
	if req.LowStockThreshold != nil && *req.LowStockThreshold < 0 {
		return nil, fmt.Errorf("低库存预警阈值不能小于 0")
	}
	remark := strings.TrimSpace(req.Remark)
	if len([]rune(remark)) > 255 {
		return nil, fmt.Errorf("库存调整备注不能超过 255 个字符")
	}
	if remark == "" {
		remark = "商家后台调整库存"
	}

	var updatedSKU *model.ProductSKU
	err := s.repo.Transaction(ctx, func(repo repository.MerchantInventoryRepository) error {
		sku, err := repo.FindSKUForUpdate(ctx, merchantID, skuID)
		if err != nil {
			return fmt.Errorf("SKU 不存在")
		}
		product, err := repo.FindProductByID(ctx, merchantID, sku.ProductID)
		if err != nil {
			return fmt.Errorf("商品不存在")
		}
		beforeStock := sku.Stock
		sku.Stock = *req.Stock
		if req.LowStockThreshold != nil {
			sku.LowStockThreshold = *req.LowStockThreshold
		}
		if err := repo.UpdateSKUStock(ctx, sku); err != nil {
			return err
		}
		if beforeStock != sku.Stock {
			if err := repo.CreateInventoryLog(ctx, &model.InventoryLog{
				MerchantID: merchantID, ProductID: sku.ProductID, SKUID: sku.ID,
				ProductName: product.Name, SKUName: sku.Name,
				ChangeType: model.InventoryChangeMerchantAdjustment,
				Quantity:   sku.Stock - beforeStock, BeforeStock: beforeStock, AfterStock: sku.Stock,
				ReferenceType: model.InventoryReferenceSKU, ReferenceID: sku.ID,
				OperatorType: model.InventoryOperatorMerchant, OperatorID: operatorID, Remark: remark,
			}); err != nil {
				return err
			}
		}
		updatedSKU = sku
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &dto.MerchantStockResponse{
		MerchantID: updatedSKU.MerchantID, ProductID: updatedSKU.ProductID, SKUID: updatedSKU.ID,
		Stock: updatedSKU.Stock, LowStockThreshold: updatedSKU.LowStockThreshold,
	}, nil
}
