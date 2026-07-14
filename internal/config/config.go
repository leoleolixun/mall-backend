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
	Storage StorageConfig `mapstructure:"storage"`
	Order   OrderConfig   `mapstructure:"order"`
}

type OrderConfig struct {
	CancelExpiredEnabled         bool `mapstructure:"cancel_expired_enabled"`
	PendingPaymentTimeoutMinutes int  `mapstructure:"pending_payment_timeout_minutes"`
	CancelBatchSize              int  `mapstructure:"cancel_batch_size"`
	AutoCompleteEnabled          bool `mapstructure:"auto_complete_enabled"`
	ShippedAutoCompleteDays      int  `mapstructure:"shipped_auto_complete_days"`
	CompleteBatchSize            int  `mapstructure:"complete_batch_size"`
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
	AccessSecret             string `mapstructure:"access_secret"`
	AccessTTLMinutes         int    `mapstructure:"access_ttl_minutes"`
	RefreshTTLHours          int    `mapstructure:"refresh_ttl_hours"`
	MerchantAccessSecret     string `mapstructure:"merchant_access_secret"`
	MerchantAccessTTLMinutes int    `mapstructure:"merchant_access_ttl_minutes"`
	MerchantRefreshTTLHours  int    `mapstructure:"merchant_refresh_ttl_hours"`
}

type StorageConfig struct {
	Provider       string             `mapstructure:"provider"`
	MaxImageSizeMB int64              `mapstructure:"max_image_size_mb"`
	PathPrefix     string             `mapstructure:"path_prefix"`
	PublicBaseURL  string             `mapstructure:"public_base_url"`
	COS            TencentCOSConfig   `mapstructure:"cos"`
	Qiniu          QiniuStorageConfig `mapstructure:"qiniu"`
}

type TencentCOSConfig struct {
	BucketURL string `mapstructure:"bucket_url"`
	SecretID  string `mapstructure:"secret_id"`
	SecretKey string `mapstructure:"secret_key"`
}

type QiniuStorageConfig struct {
	Bucket    string `mapstructure:"bucket"`
	AccessKey string `mapstructure:"access_key"`
	SecretKey string `mapstructure:"secret_key"`
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
	v.SetDefault("storage.max_image_size_mb", 10)
	v.SetDefault("storage.path_prefix", "go-mall")
	v.SetDefault("jwt.merchant_access_ttl_minutes", 120)
	v.SetDefault("jwt.merchant_refresh_ttl_hours", 168)
	v.SetDefault("payment.alipay.sandbox", true)
	v.SetDefault("payment.alipay.page.product_code", "FAST_INSTANT_TRADE_PAY")
	v.SetDefault("payment.alipay.page.timeout_express", "15m")
	v.SetDefault("payment.alipay.wap.product_code", "QUICK_WAP_WAY")
	v.SetDefault("payment.alipay.wap.timeout_express", "15m")
	v.SetDefault("order.pending_payment_timeout_minutes", 15)
	v.SetDefault("order.cancel_batch_size", 100)
	v.SetDefault("order.cancel_expired_enabled", false)
	v.SetDefault("order.auto_complete_enabled", false)
	v.SetDefault("order.shipped_auto_complete_days", 10)
	v.SetDefault("order.complete_batch_size", 100)

	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
