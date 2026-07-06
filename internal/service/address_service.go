package service

import (
	"context"
	"fmt"
	"go-mall/internal/dto"
	"go-mall/internal/model"
	"go-mall/internal/repository"
	"strings"
)

type AddressService interface {
	List(ctx context.Context, userID int64) ([]dto.AddressResponse, error)
	Create(ctx context.Context, userID int64, req dto.AddressRequest) (*dto.AddressResponse, error)
	Update(ctx context.Context, userID int64, id int64, req dto.AddressRequest) (*dto.AddressResponse, error)
	Delete(ctx context.Context, userID int64, id int64) error
	SetDefault(ctx context.Context, userID int64, id int64) error
}

type addressService struct {
	addressRepo repository.AddressRepository
}

func NewAddressService(addressRepo repository.AddressRepository) AddressService {
	return &addressService{
		addressRepo: addressRepo,
	}
}

func (s *addressService) List(ctx context.Context, userID int64) ([]dto.AddressResponse, error) {
	addresses, err := s.addressRepo.ListByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	result := make([]dto.AddressResponse, 0, len(addresses))
	for _, address := range addresses {
		result = append(result, toAddressResponse(address))
	}

	return result, nil
}

func (s *addressService) Create(ctx context.Context, userID int64, req dto.AddressRequest) (*dto.AddressResponse, error) {
	if err := validateAddressRequest(req); err != nil {
		return nil, err
	}

	address := &model.Address{
		UserID:        userID,
		ReceiverName:  strings.TrimSpace(req.ReceiverName),
		ReceiverPhone: strings.TrimSpace(req.ReceiverPhone),
		Province:      strings.TrimSpace(req.Province),
		City:          strings.TrimSpace(req.City),
		District:      strings.TrimSpace(req.District),
		Detail:        strings.TrimSpace(req.Detail),
		IsDefault:     req.IsDefault,
	}

	if err := s.addressRepo.Transaction(ctx, func(repo repository.AddressRepository) error {
		if req.IsDefault {
			if err := repo.ClearDefault(ctx, userID); err != nil {
				return err
			}
		}

		return repo.Create(ctx, address)
	}); err != nil {
		return nil, err
	}

	resp := toAddressResponse(*address)
	return &resp, nil
}

func (s *addressService) Update(ctx context.Context, userID int64, id int64, req dto.AddressRequest) (*dto.AddressResponse, error) {
	if id <= 0 {
		return nil, fmt.Errorf("无效的地址ID")
	}
	if err := validateAddressRequest(req); err != nil {
		return nil, err
	}

	var resp dto.AddressResponse
	if err := s.addressRepo.Transaction(ctx, func(repo repository.AddressRepository) error {
		address, err := repo.FindByIDAndUserID(ctx, id, userID)
		if err != nil {
			return err
		}

		if req.IsDefault {
			if err := repo.ClearDefault(ctx, userID); err != nil {
				return err
			}
		}

		address.ReceiverName = strings.TrimSpace(req.ReceiverName)
		address.ReceiverPhone = strings.TrimSpace(req.ReceiverPhone)
		address.Province = strings.TrimSpace(req.Province)
		address.City = strings.TrimSpace(req.City)
		address.District = strings.TrimSpace(req.District)
		address.Detail = strings.TrimSpace(req.Detail)
		address.IsDefault = req.IsDefault

		if err := repo.Update(ctx, address); err != nil {
			return err
		}

		resp = toAddressResponse(*address)
		return nil
	}); err != nil {
		return nil, err
	}

	return &resp, nil
}

func (s *addressService) Delete(ctx context.Context, userID int64, id int64) error {
	if id <= 0 {
		return fmt.Errorf("无效的地址ID")
	}
	if _, err := s.addressRepo.FindByIDAndUserID(ctx, id, userID); err != nil {
		return err
	}
	return s.addressRepo.DeleteByIDAndUserID(ctx, id, userID)
}

func (s *addressService) SetDefault(ctx context.Context, userID int64, id int64) error {
	if id <= 0 {
		return fmt.Errorf("地址 ID 不合法")
	}

	return s.addressRepo.Transaction(ctx, func(repo repository.AddressRepository) error {
		address, err := repo.FindByIDAndUserID(ctx, id, userID)
		if err != nil {
			return fmt.Errorf("地址不存在")
		}

		if err := repo.ClearDefault(ctx, userID); err != nil {
			return err
		}

		address.IsDefault = true
		return repo.Update(ctx, address)
	})
}

func toAddressResponse(address model.Address) dto.AddressResponse {
	return dto.AddressResponse{
		ID:            address.ID,
		ReceiverName:  address.ReceiverName,
		ReceiverPhone: address.ReceiverPhone,
		Province:      address.Province,
		City:          address.City,
		District:      address.District,
		Detail:        address.Detail,
		IsDefault:     address.IsDefault,
	}
}

func validateAddressRequest(req dto.AddressRequest) error {
	if strings.TrimSpace(req.ReceiverName) == "" {
		return fmt.Errorf("收货人不能为空")
	}
	if strings.TrimSpace(req.ReceiverPhone) == "" {
		return fmt.Errorf("收货人电话不能为空")
	}
	if strings.TrimSpace(req.Province) == "" {
		return fmt.Errorf("省份不能为空")
	}
	if strings.TrimSpace(req.City) == "" {
		return fmt.Errorf("城市不能为空")
	}
	if strings.TrimSpace(req.District) == "" {
		return fmt.Errorf("区域不能为空")
	}
	if strings.TrimSpace(req.Detail) == "" {
		return fmt.Errorf("详细地址不能为空")
	}
	return nil
}
