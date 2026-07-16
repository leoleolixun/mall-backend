package service

import (
	"context"
	"errors"
	"go-mall/internal/dto"
	"go-mall/internal/repository"

	"gorm.io/gorm"
)

const defaultMerchantID int64 = 1

type CategoryService interface {
	List(ctx context.Context) ([]dto.CategoryResponse, error)
	ListByMerchant(ctx context.Context, merchantID int64) ([]dto.CategoryResponse, error)
}

type categoryService struct {
	categoryRepo repository.CategoryRepository
	merchantRepo repository.MerchantRepository
}

func NewCategoryService(categoryRepo repository.CategoryRepository, merchantRepo repository.MerchantRepository) CategoryService {
	return &categoryService{
		categoryRepo: categoryRepo,
		merchantRepo: merchantRepo,
	}
}

func (s *categoryService) List(ctx context.Context) ([]dto.CategoryResponse, error) {
	return s.ListByMerchant(ctx, defaultMerchantID)
}

func (s *categoryService) ListByMerchant(ctx context.Context, merchantID int64) ([]dto.CategoryResponse, error) {
	if _, err := s.merchantRepo.FindEnabledByID(ctx, merchantID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPublicMerchantNotFound
		}
		return nil, err
	}
	categories, err := s.categoryRepo.ListEnabled(ctx, merchantID)
	if err != nil {
		return nil, err
	}

	result := make([]dto.CategoryResponse, 0, len(categories))
	for _, category := range categories {
		result = append(result, dto.CategoryResponse{
			ID:         category.ID,
			MerchantID: category.MerchantID,
			ParentID:   category.ParentID,
			Name:       category.Name,
			Sort:       category.Sort,
		})
	}

	return result, nil
}
