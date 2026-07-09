package main

import (
	"fmt"
	"go-mall/internal/bootstrap"
	"go-mall/internal/config"
	"go-mall/internal/handler"
	"go-mall/internal/repository"
	"go-mall/internal/router"
	"go-mall/internal/service"
	"go-mall/pkg/logger"

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

	categoryRepo := repository.NewCategoryRepository(db)
	productRepo := repository.NewProductRepository(db)
	authRepo := repository.NewAuthRepository(db)
	addressRepo := repository.NewAddressRepository(db)
	orderRepo := repository.NewOrderRepository(db)
	paymentRepo := repository.NewPaymentRepository(db)

	categoryService := service.NewCategoryService(categoryRepo)
	productService := service.NewProductService(productRepo, rdb)
	authService := service.NewAuthService(authRepo, rdb, cfg.JWT)
	addressService := service.NewAddressService(addressRepo)
	cartService := service.NewCartService(rdb, productRepo)
	orderService := service.NewOrderService(orderRepo, addressRepo, productRepo, rdb)
	paymentService := service.NewPaymentService(paymentRepo, cfg.Payment)

	categoryHandler := handler.NewCategoryHandler(categoryService)
	productHandler := handler.NewProductHandler(productService)
	healthHandler := handler.NewHealthHandler(db, rdb)
	authHandler := handler.NewAuthHandler(authService)
	addressHandler := handler.NewAddressHandler(addressService)
	cartHandler := handler.NewCartHandler(cartService)
	paymentHandler := handler.NewPaymentHandler(paymentService)
	orderHandler := handler.NewOrderHandler(orderService, paymentService)

	r := router.NewRouter(healthHandler, categoryHandler, productHandler, authHandler, addressHandler, cartHandler, orderHandler, paymentHandler, cfg.JWT)

	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	if err := r.Run(addr); err != nil {
		log.Fatal("启动服务器失败: ", zap.Error(err))
	}

}
