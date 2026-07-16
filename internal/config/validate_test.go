package config

import (
	"os"
	"path/filepath"
	"testing"
)

func validServerConfig() Config {
	return Config{
		Server: ServerConfig{Port: 8080, Mode: "debug", TrustedProxies: []string{"127.0.0.1"}},
		Log:    LogConfig{Level: "info", Format: "json"},
		Auth: AuthConfig{
			LoginRateLimitPerMinute:   20,
			RefreshRateLimitPerMinute: 60,
		},
		JWT: JWTConfig{
			AccessSecret:             "buyer-secret-0123456789-0123456789",
			AccessTTLMinutes:         120,
			RefreshTTLHours:          168,
			MerchantAccessSecret:     "merchant-secret-0123456789-012345",
			MerchantAccessTTLMinutes: 120,
			MerchantRefreshTTLHours:  168,
		},
		Settlement: SettlementConfig{HoldDays: 7, BatchSize: 100},
	}
}

func TestValidateForServerAcceptsValidDebugAndReleaseConfig(t *testing.T) {
	debugCfg := validServerConfig()
	if err := debugCfg.ValidateForServer(); err != nil {
		t.Fatalf("valid debug config: %v", err)
	}

	releaseCfg := validServerConfig()
	releaseCfg.Server.Mode = "release"
	releaseCfg.Payment.Alipay.Enabled = true
	releaseCfg.Payment.Alipay.Sandbox = false
	if err := releaseCfg.ValidateForServer(); err != nil {
		t.Fatalf("valid release config: %v", err)
	}
}

func TestValidateForServerRejectsInvalidJWTConfiguration(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Config)
	}{
		{name: "empty buyer secret", mutate: func(cfg *Config) { cfg.JWT.AccessSecret = "" }},
		{name: "placeholder buyer secret", mutate: func(cfg *Config) { cfg.JWT.AccessSecret = "change_me" }},
		{name: "short merchant secret", mutate: func(cfg *Config) { cfg.JWT.MerchantAccessSecret = "too-short" }},
		{name: "shared secrets", mutate: func(cfg *Config) { cfg.JWT.MerchantAccessSecret = cfg.JWT.AccessSecret }},
		{name: "buyer ttl", mutate: func(cfg *Config) { cfg.JWT.AccessTTLMinutes = 0 }},
		{name: "merchant ttl", mutate: func(cfg *Config) { cfg.JWT.MerchantRefreshTTLHours = 0 }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := validServerConfig()
			test.mutate(&cfg)
			if err := cfg.ValidateForServer(); err == nil {
				t.Fatal("expected invalid JWT configuration to be rejected")
			}
		})
	}
}

func TestValidateForServerRejectsReleaseOnlyRisks(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Config)
	}{
		{name: "auto migrate", mutate: func(cfg *Config) { cfg.App.AutoMigrate = true }},
		{name: "seed data", mutate: func(cfg *Config) { cfg.App.SeedData = true }},
		{name: "unsafe wechat login", mutate: func(cfg *Config) { cfg.Auth.UnsafeWechatOpenIDLoginEnabled = true }},
		{name: "mock payment", mutate: func(cfg *Config) { cfg.Payment.MockEnabled = true }},
		{name: "alipay sandbox", mutate: func(cfg *Config) {
			cfg.Payment.Alipay.Enabled = true
			cfg.Payment.Alipay.Sandbox = true
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := validServerConfig()
			cfg.Server.Mode = "release"
			test.mutate(&cfg)
			if err := cfg.ValidateForServer(); err == nil {
				t.Fatal("expected unsafe release configuration to be rejected")
			}
		})
	}
}

func TestValidateForServerRejectsInvalidServerConfiguration(t *testing.T) {
	cfg := validServerConfig()
	cfg.Server.Mode = "production"
	if err := cfg.ValidateForServer(); err == nil {
		t.Fatal("expected unsupported server mode to be rejected")
	}

	cfg = validServerConfig()
	cfg.Server.Port = 70000
	if err := cfg.ValidateForServer(); err == nil {
		t.Fatal("expected invalid server port to be rejected")
	}

	tests := []struct {
		name   string
		mutate func(*Config)
	}{
		{name: "wildcard cors", mutate: func(cfg *Config) { cfg.Server.CORSAllowedOrigins = []string{"*"} }},
		{name: "cors path", mutate: func(cfg *Config) { cfg.Server.CORSAllowedOrigins = []string{"https://mall.example.com/path"} }},
		{name: "invalid proxy", mutate: func(cfg *Config) { cfg.Server.TrustedProxies = []string{"not-an-ip"} }},
		{name: "invalid log format", mutate: func(cfg *Config) { cfg.Log.Format = "plain" }},
		{name: "invalid login rate", mutate: func(cfg *Config) { cfg.Auth.LoginRateLimitPerMinute = 0 }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := validServerConfig()
			test.mutate(&cfg)
			if err := cfg.ValidateForServer(); err == nil {
				t.Fatal("expected invalid server security configuration to be rejected")
			}
		})
	}
}

func TestLoadUsesGINModeEnvironmentOverride(t *testing.T) {
	t.Setenv("GIN_MODE", "release")
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("server:\n  mode: debug\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Server.Mode != "release" {
		t.Fatalf("expected GIN_MODE override, got %q", cfg.Server.Mode)
	}
	if cfg.Payment.Refund.ReconcileEnabled || cfg.Payment.Refund.RetryIntervalMinutes != 5 || cfg.Payment.Refund.ReconcileBatchSize != 100 {
		t.Fatalf("unexpected refund reconciliation defaults: %+v", cfg.Payment.Refund)
	}
}
