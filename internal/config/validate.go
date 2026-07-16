package config

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

const minimumJWTSecretLength = 32

func validateJWTSecret(name, value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("%s 未配置", name)
	}

	normalized := strings.ToLower(value)
	if strings.HasPrefix(normalized, "change_me") ||
		strings.HasPrefix(normalized, "replace_with") ||
		normalized == "changeme" ||
		normalized == "secret" {
		return fmt.Errorf("%s 仍在使用示例密钥", name)
	}
	if len(value) < minimumJWTSecretLength {
		return fmt.Errorf("%s 长度不能少于 %d 个字节", name, minimumJWTSecretLength)
	}
	return nil
}

// ValidateForServer validates configuration that is required by the HTTP server.
// It intentionally runs before external dependencies are initialized.
func (c Config) ValidateForServer() error {
	mode := strings.ToLower(strings.TrimSpace(c.Server.Mode))
	if mode != "debug" && mode != "release" && mode != "test" {
		return fmt.Errorf("server.mode 只支持 debug、release 或 test")
	}
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("server.port 必须在 1 到 65535 之间")
	}
	for _, proxy := range c.Server.TrustedProxies {
		proxy = strings.TrimSpace(proxy)
		if proxy == "" {
			return fmt.Errorf("server.trusted_proxies 不能包含空值")
		}
		if net.ParseIP(proxy) == nil {
			if _, _, err := net.ParseCIDR(proxy); err != nil {
				return fmt.Errorf("server.trusted_proxies 包含无效地址 %q", proxy)
			}
		}
	}
	for _, origin := range c.Server.CORSAllowedOrigins {
		origin = strings.TrimSpace(origin)
		if origin == "*" {
			return fmt.Errorf("server.cors_allowed_origins 不允许使用通配符")
		}
		parsed, err := url.Parse(origin)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil || (parsed.Path != "" && parsed.Path != "/") || parsed.RawQuery != "" || parsed.Fragment != "" {
			return fmt.Errorf("server.cors_allowed_origins 包含无效来源 %q", origin)
		}
	}
	logFormat := strings.ToLower(strings.TrimSpace(c.Log.Format))
	if logFormat != "" && logFormat != "json" && logFormat != "console" {
		return fmt.Errorf("log.format 只支持 json 或 console")
	}
	if c.Auth.LoginRateLimitPerMinute <= 0 || c.Auth.RefreshRateLimitPerMinute <= 0 {
		return fmt.Errorf("登录和刷新限流次数必须大于 0")
	}
	if c.Settlement.HoldDays < 0 || c.Settlement.HoldDays > 365 {
		return fmt.Errorf("settlement.hold_days 必须在 0 到 365 之间")
	}
	if c.Settlement.BatchSize <= 0 || c.Settlement.BatchSize > 1000 {
		return fmt.Errorf("settlement.batch_size 必须在 1 到 1000 之间")
	}

	if err := validateJWTSecret("jwt.access_secret", c.JWT.AccessSecret); err != nil {
		return err
	}
	if err := validateJWTSecret("jwt.merchant_access_secret", c.JWT.MerchantAccessSecret); err != nil {
		return err
	}
	if strings.TrimSpace(c.JWT.AccessSecret) == strings.TrimSpace(c.JWT.MerchantAccessSecret) {
		return fmt.Errorf("买家和商家 JWT 必须使用不同密钥")
	}
	if c.JWT.AccessTTLMinutes <= 0 || c.JWT.RefreshTTLHours <= 0 {
		return fmt.Errorf("买家 JWT 过期时间必须大于 0")
	}
	if c.JWT.MerchantAccessTTLMinutes <= 0 || c.JWT.MerchantRefreshTTLHours <= 0 {
		return fmt.Errorf("商家 JWT 过期时间必须大于 0")
	}

	if mode == "release" {
		if logFormat != "json" {
			return fmt.Errorf("release 模式必须使用 JSON 结构化日志")
		}
		if c.App.AutoMigrate || c.App.SeedData {
			return fmt.Errorf("release 模式不能启用 app.auto_migrate 或 app.seed_data")
		}
		if c.Auth.UnsafeWechatOpenIDLoginEnabled {
			return fmt.Errorf("release 模式不能启用不安全的微信 open_id 登录")
		}
		if c.Payment.MockEnabled {
			return fmt.Errorf("release 模式不能启用模拟支付")
		}
		if c.Payment.Alipay.Enabled && c.Payment.Alipay.Sandbox {
			return fmt.Errorf("release 模式启用支付宝时不能使用沙箱配置")
		}
	}

	return nil
}
