package main

import (
	"fmt"
	"go-mall/internal/bootstrap"
	"go-mall/internal/config"
	"go-mall/internal/handler"
	"go-mall/internal/router"
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

	rdb, err := bootstrap.InitRedis(cfg.Redis)
	if err != nil {
		log.Fatal("初始化 Redis 失败: ", zap.Error(err))
	}

	healthHandler := handler.NewHealthHandler(db, rdb)
	r := router.NewRouter(healthHandler)

	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	if err := r.Run(addr); err != nil {
		log.Fatal("启动服务器失败: ", zap.Error(err))
	}

}
