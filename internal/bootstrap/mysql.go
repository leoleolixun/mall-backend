package bootstrap

import (
	"fmt"
	"go-mall/internal/config"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func InitMySQL(cfg config.MySQLConfig) (*gorm.DB, error) {
	charset := cfg.Charset
	if charset == "" {
		charset = "utf8mb4"
	}

	loc := cfg.Loc
	if loc == "" {
		loc = "Local"
	}

	parseTime := "False"
	if cfg.ParseTime {
		parseTime = "True"
	}

	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=%s&loc=%s",
		cfg.User,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.Database,
		charset,
		parseTime,
		loc,
	)

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("open mysql failed: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get sql db failed: %w", err)
	}

	if err := sqlDB.Ping(); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("ping mysql failed: %w", err)
	}

	// 设置连接池
	sqlDB.SetMaxIdleConns(10)           // 最多空闲连接数
	sqlDB.SetMaxOpenConns(100)          // 最大连接数
	sqlDB.SetConnMaxLifetime(time.Hour) // 连接最大生命周期

	return db, nil
}
