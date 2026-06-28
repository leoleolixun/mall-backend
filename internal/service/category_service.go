package service

import (
	"context"
	"go-mall/internal/dto"
	"go-mall/internal/repository"
)

const defaultMerchantID int64 = 1

type CategoryService interface {
	List(ctx context.Context) ([]dto.CategoryResponse, error)
}

type categoryService struct {
	categoryRepo repository.CategoryRepository
}

func NewCategoryService(categoryRepo repository.CategoryRepository) CategoryService {
	return &categoryService{
		categoryRepo: categoryRepo,
	}
}

func (s *categoryService) List(ctx context.Context) ([]dto.CategoryResponse, error) {
	categories, err := s.categoryRepo.ListEnabled(ctx, defaultMerchantID)
	if err != nil {
		return nil, err
	}

	result := make([]dto.CategoryResponse, 0, len(categories))
	for _, category := range categories {
		result = append(result, dto.CategoryResponse{
			ID:   category.ID,
			Name: category.Name,
			Sort: category.Sort,
		})
	}

	return result, nil
}
