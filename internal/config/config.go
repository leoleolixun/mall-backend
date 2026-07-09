package config

import "github.com/spf13/viper"

type Config struct {
	App     AppConfig     `mapstructure:"app"`
	Server  ServerConfig  `mapstructure:"server"`
	MySQL   MySQLConfig   `mapstructure:"mysql"`
	Redis   RedisConfig   `mapstructure:"redis"`
	Log     LogConfig     `mapstructure:"log"`
	JWT     JWTConfig     `mapstructure:"jwt"`
	Payment PaymentConfig `mapstructure:"payment"`
}

type AppConfig struct {
	AutoMigrate bool `mapstructure:"auto_migrate"`
	SeedData    bool `mapstructure:"seed_data"`
}

type ServerConfig struct {
	Port int    `mapstructure:"port"`
	Mode string `mapstructure:"mode"`
}

type MySQLConfig struct {
	Host      string `mapstructure:"host"`
	Port      int    `mapstructure:"port"`
	User      string `mapstructure:"user"`
	Password  string `mapstructure:"password"`
	Database  string `mapstructure:"database"`
	Charset   string `mapstructure:"charset"`
	ParseTime bool   `mapstructure:"parse_time"`
	Loc       string `mapstructure:"loc"`
}

type RedisConfig struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

type LogConfig struct {
	Level string `mapstructure:"level"`
}

type JWTConfig struct {
	AccessSecret     string `mapstructure:"access_secret"`
	AccessTTLMinutes int    `mapstructure:"access_ttl_minutes"`
	RefreshTTLHours  int    `mapstructure:"refresh_ttl_hours"`
}

type PaymentConfig struct {
	Alipay AlipayConfig `mapstructure:"alipay"`
}

type AlipayConfig struct {
	Enabled                 bool               `mapstructure:"enabled"`
	Sandbox                 bool               `mapstructure:"sandbox"`
	AppID                   string             `mapstructure:"app_id"`
	AppPrivateKeyPath       string             `mapstructure:"app_private_key_path"`
	AppPrivateKey           string             `mapstructure:"app_private_key"`
	AlipayPublicKeyPath     string             `mapstructure:"alipay_public_key_path"`
	AlipayPublicKey         string             `mapstructure:"alipay_public_key"`
	AppCertPublicKeyPath    string             `mapstructure:"app_cert_public_key_path"`
	AlipayRootCertPath      string             `mapstructure:"alipay_root_cert_path"`
	AlipayCertPublicKeyPath string             `mapstructure:"alipay_cert_public_key_path"`
	NotifyURL               string             `mapstructure:"notify_url"`
	Page                    AlipayPageConfig   `mapstructure:"page"`
	Wap                     AlipayWapPayConfig `mapstructure:"wap"`
}

type AlipayPageConfig struct {
	Enabled        bool   `mapstructure:"enabled"`
	ReturnURL      string `mapstructure:"return_url"`
	ProductCode    string `mapstructure:"product_code"`
	TimeoutExpress string `mapstructure:"timeout_express"`
}

type AlipayWapPayConfig struct {
	Enabled        bool   `mapstructure:"enabled"`
	ReturnURL      string `mapstructure:"return_url"`
	QuitURL        string `mapstructure:"quit_url"`
	ProductCode    string `mapstructure:"product_code"`
	TimeoutExpress string `mapstructure:"timeout_express"`
}

func Load(path string) (*Config, error) {
	// 使用 Viper 加载配置文件
	v := viper.New()
	v.SetConfigFile(path)
	v.SetDefault("app.auto_migrate", false)
	v.SetDefault("app.seed_data", false)
	v.SetDefault("payment.alipay.sandbox", true)
	v.SetDefault("payment.alipay.page.product_code", "FAST_INSTANT_TRADE_PAY")
	v.SetDefault("payment.alipay.page.timeout_express", "15m")
	v.SetDefault("payment.alipay.wap.product_code", "QUICK_WAP_WAY")
	v.SetDefault("payment.alipay.wap.timeout_express", "15m")

	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
