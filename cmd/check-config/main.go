package main

import (
	"flag"
	"fmt"

	"go-mall/internal/config"
)

func main() {
	configPath := flag.String("config", "config.yaml", "config file path")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		panic(fmt.Errorf("load config failed: %w", err))
	}
	if err := cfg.ValidateForServer(); err != nil {
		panic(fmt.Errorf("server config validation failed: %w", err))
	}

	fmt.Printf("server config validation passed: mode=%s port=%d\n", cfg.Server.Mode, cfg.Server.Port)
}
