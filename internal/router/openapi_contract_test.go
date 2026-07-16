package router

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"go-mall/internal/config"
	"go-mall/internal/handler"
	"go-mall/internal/observability"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.yaml.in/yaml/v3"
)

var ginParameterPattern = regexp.MustCompile(`:([a-zA-Z0-9_]+)`)

func TestImplementedRoutesExistInOpenAPI(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine, err := NewRouter(
		handler.NewHealthHandler(nil, nil),
		handler.NewCategoryHandler(nil),
		handler.NewProductHandler(nil),
		handler.NewMerchantHandler(nil, nil, nil),
		handler.NewAuthHandler(nil),
		handler.NewAddressHandler(nil),
		handler.NewCartHandler(nil),
		handler.NewOrderHandler(nil, nil, nil),
		handler.NewTradeHandler(nil),
		handler.NewPaymentHandler(nil, nil),
		handler.NewUploadHandler(nil),
		handler.NewAfterSaleHandler(nil),
		handler.NewCouponHandler(nil),
		handler.NewFavoriteHandler(nil),
		handler.NewMerchantAuthHandler(nil),
		handler.NewMerchantAccountHandler(nil),
		handler.NewMerchantOrderHandler(nil),
		handler.NewMerchantCatalogHandler(nil),
		handler.NewMerchantInventoryHandler(nil),
		handler.NewMerchantDashboardHandler(nil),
		handler.NewMerchantCustomerHandler(nil),
		handler.NewMerchantSettlementHandler(nil),
		nil,
		nil,
		config.JWTConfig{},
		config.ServerConfig{},
		config.AuthConfig{LoginRateLimitPerMinute: 20, RefreshRateLimitPerMinute: 60},
		config.PaymentConfig{},
		config.ObservabilityConfig{},
		zap.NewNop(),
		observability.NewMetrics(nil),
	)
	if err != nil {
		t.Fatalf("create router: %v", err)
	}

	document, err := os.ReadFile("../../docs/openapi.yaml")
	if err != nil {
		t.Fatalf("read OpenAPI: %v", err)
	}
	var spec struct {
		Paths map[string]map[string]any `yaml:"paths"`
	}
	if err := yaml.Unmarshal(document, &spec); err != nil {
		t.Fatalf("parse OpenAPI: %v", err)
	}

	for _, route := range engine.Routes() {
		if route.Method == "HEAD" {
			continue
		}
		path := ginParameterPattern.ReplaceAllString(route.Path, `{$1}`)
		if path == "/swagger/*any" {
			path = "/swagger/index.html"
		}
		operations, ok := spec.Paths[path]
		if !ok {
			t.Errorf("implemented route is missing from OpenAPI: %s %s", route.Method, path)
			continue
		}
		if _, ok := operations[strings.ToLower(route.Method)]; !ok {
			t.Errorf("implemented method is missing from OpenAPI: %s %s", route.Method, path)
		}
	}
}
