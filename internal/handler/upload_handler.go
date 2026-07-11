package handler

import (
	"errors"
	"net/http"

	"go-mall/internal/middleware"
	"go-mall/internal/service"
	"go-mall/pkg/response"

	"github.com/gin-gonic/gin"
)

const multipartOverheadSize int64 = 1024 * 1024

type UploadHandler struct {
	uploadService service.UploadService
}

func NewUploadHandler(uploadService service.UploadService) *UploadHandler {
	return &UploadHandler{uploadService: uploadService}
}

func (h *UploadHandler) Image(c *gin.Context) {
	_, userOK := middleware.CurrentUserID(c)
	_, merchantOK := middleware.CurrentMerchant(c)
	if !userOK && !merchantOK {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "未登录")
		return
	}

	c.Request.Body = http.MaxBytesReader(
		c.Writer,
		c.Request.Body,
		h.uploadService.MaxImageSize()+multipartOverheadSize,
	)
	fileHeader, err := c.FormFile("file")
	if err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "上传图片超过大小限制")
			return
		}
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "请选择要上传的图片")
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "读取上传图片失败")
		return
	}
	defer file.Close()

	result, err := h.uploadService.UploadImage(c.Request.Context(), service.UploadImageInput{
		Reader:       file,
		Size:         fileHeader.Size,
		OriginalName: fileHeader.Filename,
		Scene:        c.PostForm("scene"),
	})
	if err != nil {
		if errors.Is(err, service.ErrInvalidImage) {
			response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
			return
		}
		response.Error(c, http.StatusInternalServerError, response.CodeInternalError, err.Error())
		return
	}

	response.Success(c, result)
}
