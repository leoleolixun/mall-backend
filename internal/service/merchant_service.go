package service

import (
	"context"
	"errors"
	"fmt"

	"go-mall/internal/dto"
	"go-mall/internal/model"
	"go-mall/internal/repository"

	"gorm.io/gorm"
)

var ErrPublicMerchantNotFound = errors.New("商户不存在或已停用")

type MerchantService interface {
	List(ctx context.Context, req dto.PublicMerchantListRequest) (*dto.PageResponse[dto.PublicMerchantResponse], error)
	Detail(ctx context.Context, id int64) (*dto.PublicMerchantResponse, error)
	FindByIDs(ctx context.Context, ids []int64) (map[int64]model.Merchant, error)
}

type merchantService struct {
	repo repository.MerchantRepository
}

func NewMerchantService(repo repository.MerchantRepository) MerchantService {
	return &merchantService{repo: repo}
}

func toPublicMerchantResponse(merchant model.Merchant) dto.PublicMerchantResponse {
	return dto.PublicMerchantResponse{ID: merchant.ID, Name: merchant.Name, Logo: merchant.Logo, Status: merchant.Status}
}

func (s *merchantService) List(ctx context.Context, req dto.PublicMerchantListRequest) (*dto.PageResponse[dto.PublicMerchantResponse], error) {
	page, pageSize := normalizePage(req.Page, req.PageSize)
	merchants, total, err := s.repo.ListEnabled(ctx, (page-1)*pageSize, pageSize)
	if err != nil {
		return nil, err
	}
	items := make([]dto.PublicMerchantResponse, 0, len(merchants))
	for _, merchant := range merchants {
		items = append(items, toPublicMerchantResponse(merchant))
	}
	return &dto.PageResponse[dto.PublicMerchantResponse]{List: items, Page: page, PageSize: pageSize, Total: total}, nil
}

func (s *merchantService) Detail(ctx context.Context, id int64) (*dto.PublicMerchantResponse, error) {
	if id <= 0 {
		return nil, fmt.Errorf("商户 ID 不合法")
	}
	merchant, err := s.repo.FindEnabledByID(ctx, id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrPublicMerchantNotFound
	}
	if err != nil {
		return nil, err
	}
	result := toPublicMerchantResponse(*merchant)
	return &result, nil
}

func (s *merchantService) FindByIDs(ctx context.Context, ids []int64) (map[int64]model.Merchant, error) {
	merchants, err := s.repo.FindByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]model.Merchant, len(merchants))
	for _, merchant := range merchants {
		result[merchant.ID] = merchant
	}
	return result, nil
}
