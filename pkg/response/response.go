package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const (
	CodeSuccess       = 0
	CodeBadRequest    = 40000
	CodeInternalError = 50000
	CodeUnauthorized  = 401 // 未登录
	CodeForbidden     = 40300
)

type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    CodeSuccess,
		Message: "ok",
		Data:    data,
	})
}

func Error(c *gin.Context, httpStatus int, code int, message string) {
	c.JSON(httpStatus, Response{
		Code:    code,
		Message: message,
	})
}
