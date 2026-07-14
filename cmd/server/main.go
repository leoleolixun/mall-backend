package main

import (
	"fmt"
	"go-mall/internal/bootstrap"
	"go-mall/internal/config"
	"go-mall/internal/handler"
	"go-mall/internal/repository"
	"go-mall/internal/router"
	"go-mall/internal/service"
	"go-mall/internal/storage"
	"go-mall/pkg/logger"
	"strings"

	"go.uber.org/zap"
)

func main() {
	cfg, err := config.Load("config.yaml")
	if err != nil {
		panic(err)
	}

	log, err := logger.New(cfg.Log)
	if err != nil {
		panic(err)
	}
	if strings.TrimSpace(cfg.JWT.MerchantAccessSecret) == "" {
		log.Fatal("启动失败: jwt.merchant_access_secret 未配置")
	}

	db, err := bootstrap.InitMySQL(cfg.MySQL)
	if err != nil {
		log.Fatal("初始化 MySQL 失败: ", zap.Error(err))
	}

	if cfg.App.AutoMigrate {
		if err := bootstrap.AutoMigrate(db); err != nil {
			log.Fatal("自动迁移失败: ", zap.Error(err))
		}
	}

	if cfg.App.SeedData {
		if err := bootstrap.SeedDefaultData(db); err != nil {
			log.Fatal("初始化默认数据失败: ", zap.Error(err))
		}
	}

	rdb, err := bootstrap.InitRedis(cfg.Redis)
	if err != nil {
		log.Fatal("初始化 Redis 失败: ", zap.Error(err))
	}

	objectStorage, err := storage.New(cfg.Storage)
	if err != nil {
		log.Fatal("初始化图片存储失败: ", zap.Error(err))
	}

	categoryRepo := repository.NewCategoryRepository(db)
	productRepo := repository.NewProductRepository(db)
	authRepo := repository.NewAuthRepository(db)
	addressRepo := repository.NewAddressRepository(db)
	orderRepo := repository.NewOrderRepository(db)
	paymentRepo := repository.NewPaymentRepository(db)
	merchantAuthRepo := repository.NewMerchantAuthRepository(db)
	merchantAccountRepo := repository.NewMerchantAccountRepository(db)
	merchantOrderRepo := repository.NewMerchantOrderRepository(db)
	merchantCatalogRepo := repository.NewMerchantCatalogRepository(db)
	merchantInventoryRepo := repository.NewMerchantInventoryRepository(db)
	merchantDashboardRepo := repository.NewMerchantDashboardRepository(db)
	merchantCustomerRepo := repository.NewMerchantCustomerRepository(db)
	afterSaleRepo := repository.NewAfterSaleRepository(db)
	couponRepo := repository.NewCouponRepository(db)

	categoryService := service.NewCategoryService(categoryRepo)
	productService := service.NewProductService(productRepo, rdb)
	authService := service.NewAuthService(authRepo, rdb, cfg.JWT)
	addressService := service.NewAddressService(addressRepo)
	cartService := service.NewCartService(rdb, productRepo)
	orderService := service.NewOrderService(orderRepo, addressRepo, productRepo, rdb)
	paymentService := service.NewPaymentService(paymentRepo, cfg.Payment)
	uploadService := service.NewUploadService(objectStorage, cfg.Storage)
	merchantAuthService := service.NewMerchantAuthService(merchantAuthRepo, rdb, cfg.JWT)
	merchantAccountService := service.NewMerchantAccountService(merchantAccountRepo, rdb)
	merchantOrderService := service.NewMerchantOrderService(merchantOrderRepo)
	merchantCatalogService := service.NewMerchantCatalogService(merchantCatalogRepo)
	merchantInventoryService := service.NewMerchantInventoryService(merchantInventoryRepo)
	merchantDashboardService := service.NewMerchantDashboardService(merchantDashboardRepo)
	merchantCustomerService := service.NewMerchantCustomerService(merchantCustomerRepo)
	afterSaleService := service.NewAfterSaleService(afterSaleRepo, cfg.Payment)
	couponService := service.NewCouponService(couponRepo)

	categoryHandler := handler.NewCategoryHandler(categoryService)
	productHandler := handler.NewProductHandler(productService)
	healthHandler := handler.NewHealthHandler(db, rdb)
	authHandler := handler.NewAuthHandler(authService)
	addressHandler := handler.NewAddressHandler(addressService)
	cartHandler := handler.NewCartHandler(cartService)
	paymentHandler := handler.NewPaymentHandler(paymentService)
	orderHandler := handler.NewOrderHandler(orderService, paymentService)
	uploadHandler := handler.NewUploadHandler(uploadService)
	merchantAuthHandler := handler.NewMerchantAuthHandler(merchantAuthService)
	merchantAccountHandler := handler.NewMerchantAccountHandler(merchantAccountService)
	merchantOrderHandler := handler.NewMerchantOrderHandler(merchantOrderService)
	merchantCatalogHandler := handler.NewMerchantCatalogHandler(merchantCatalogService)
	merchantInventoryHandler := handler.NewMerchantInventoryHandler(merchantInventoryService)
	merchantDashboardHandler := handler.NewMerchantDashboardHandler(merchantDashboardService)
	merchantCustomerHandler := handler.NewMerchantCustomerHandler(merchantCustomerService)
	afterSaleHandler := handler.NewAfterSaleHandler(afterSaleService)
	couponHandler := handler.NewCouponHandler(couponService)

	r := router.NewRouter(healthHandler, categoryHandler, productHandler, authHandler, addressHandler, cartHandler, orderHandler, paymentHandler, uploadHandler, afterSaleHandler, couponHandler, merchantAuthHandler, merchantAccountHandler, merchantOrderHandler, merchantCatalogHandler, merchantInventoryHandler, merchantDashboardHandler, merchantCustomerHandler, merchantAuthRepo, rdb, cfg.JWT)

	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	if err := r.Run(addr); err != nil {
		log.Fatal("启动服务器失败: ", zap.Error(err))
	}

}
