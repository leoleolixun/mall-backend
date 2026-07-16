package observability

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestHTTPMetricsUseNormalizedRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	metrics := NewMetrics(nil)
	engine := gin.New()
	engine.Use(metrics.Middleware())
	engine.GET("/products/:id", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	engine.GET("/metrics", gin.WrapH(metrics.Handler()))

	productRecorder := httptest.NewRecorder()
	engine.ServeHTTP(productRecorder, httptest.NewRequest(http.MethodGet, "/products/123", nil))
	metricsRecorder := httptest.NewRecorder()
	engine.ServeHTTP(metricsRecorder, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	body := metricsRecorder.Body.String()
	if !strings.Contains(body, `go_mall_http_requests_total{method="GET",route="/products/:id",status="204"} 1`) {
		t.Fatalf("normalized request metric not found:\n%s", body)
	}
	if strings.Contains(body, `route="/products/123"`) {
		t.Fatal("raw resource id leaked into a metric label")
	}
}
