package main

import (
	"fmt"
	"go-mall/internal/bootstrap"
	"go-mall/internal/config"
)

func main() {
	cfg, err := config.Load("config.yaml")
	if err != nil {
		panic(fmt.Errorf("load config failed: %w", err))
	}

	db, err := bootstrap.InitMySQL(cfg.MySQL)
	if err != nil {
		panic(fmt.Errorf("init mysql failed: %w", err))
	}

	if err := bootstrap.AutoMigrate(db); err != nil {
		panic(fmt.Errorf("auto migrate failed: %w", err))
	}

	fmt.Println("database migration completed")
}
