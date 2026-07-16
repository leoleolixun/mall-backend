package main

import (
	"fmt"

	"go-mall/internal/bootstrap"
	"go-mall/internal/config"
	"go-mall/internal/handler"
	"go-mall/internal/observability"
	"go-mall/internal/repository"
	"go-mall/internal/router"
	"go-mall/internal/service"
	"go-mall/internal/storage"
	"go-mall/pkg/logger"

	"github.com/gin-gonic/gin"
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
	if err := cfg.ValidateForServer(); err != nil {
		log.Fatal("启动配置校验失败: ", zap.Error(err))
	}
	gin.SetMode(cfg.Server.Mode)

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
	metrics := observability.NewMetrics(db)

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
	merchantRepo := repository.NewMerchantRepository(db)
	authRepo := repository.NewAuthRepository(db)
	addressRepo := repository.NewAddressRepository(db)
	orderRepo := repository.NewOrderRepository(db)
	tradeRepo := repository.NewTradeRepository(db)
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
	favoriteRepo := repository.NewFavoriteRepository(db)
	settlementRepo := repository.NewSettlementRepository(db)

	categoryService := service.NewCategoryService(categoryRepo, merchantRepo)
	merchantService := service.NewMerchantService(merchantRepo)
	productService := service.NewProductService(productRepo, merchantRepo, rdb)
	authService := service.NewAuthService(authRepo, rdb, cfg.JWT)
	addressService := service.NewAddressService(addressRepo)
	cartService := service.NewCartService(rdb, productRepo, merchantRepo)
	orderService := service.NewOrderService(orderRepo, addressRepo, productRepo, rdb)
	paymentService := service.NewPaymentService(paymentRepo, cfg.Payment)
	tradeService := service.NewTradeService(tradeRepo, rdb, paymentService)
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
	favoriteService := service.NewFavoriteService(favoriteRepo, merchantRepo)
	settlementService := service.NewSettlementService(settlementRepo, cfg.Settlement)

	categoryHandler := handler.NewCategoryHandler(categoryService)
	productHandler := handler.NewProductHandler(productService)
	merchantHandler := handler.NewMerchantHandler(merchantService, categoryService, productService)
	healthHandler := handler.NewHealthHandler(db, rdb)
	authHandler := handler.NewAuthHandler(authService)
	addressHandler := handler.NewAddressHandler(addressService)
	cartHandler := handler.NewCartHandler(cartService)
	paymentHandler := handler.NewPaymentHandler(paymentService, metrics)
	orderHandler := handler.NewOrderHandler(orderService, tradeService, paymentService)
	tradeHandler := handler.NewTradeHandler(tradeService)
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
	favoriteHandler := handler.NewFavoriteHandler(favoriteService)
	merchantSettlementHandler := handler.NewMerchantSettlementHandler(settlementService)

	r, err := router.NewRouter(healthHandler, categoryHandler, productHandler, merchantHandler, authHandler, addressHandler, cartHandler, orderHandler, tradeHandler, paymentHandler, uploadHandler, afterSaleHandler, couponHandler, favoriteHandler, merchantAuthHandler, merchantAccountHandler, merchantOrderHandler, merchantCatalogHandler, merchantInventoryHandler, merchantDashboardHandler, merchantCustomerHandler, merchantSettlementHandler, merchantAuthRepo, rdb, cfg.JWT, cfg.Server, cfg.Auth, cfg.Payment, cfg.Observability, log, metrics)
	if err != nil {
		log.Fatal("初始化路由失败: ", zap.Error(err))
	}

	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	if err := r.Run(addr); err != nil {
		log.Fatal("启动服务器失败: ", zap.Error(err))
	}

}
