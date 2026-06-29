package handler

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/vnet/core/pkg/response"
)

type UploadHandler struct {
	uploadDir   string
	maxFileSize int64
}

func NewUploadHandler(uploadDir string, maxFileSize int64) *UploadHandler {
	return &UploadHandler{uploadDir: uploadDir, maxFileSize: maxFileSize}
}

func (h *UploadHandler) Upload(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		response.BadRequest(c, "Không tìm thấy file")
		return
	}

	if file.Size > h.maxFileSize {
		response.BadRequest(c, fmt.Sprintf("Kích thước file vượt quá %dMB", h.maxFileSize>>20))
		return
	}

	ext := strings.ToLower(filepath.Ext(file.Filename))
	allowed := map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true}
	if !allowed[ext] {
		response.BadRequest(c, "Định dạng file không hợp lệ. Chỉ chấp nhận: jpg, jpeg, png, gif, webp")
		return
	}

	dateDir := time.Now().Format("2006/01/02")
	saveDir := filepath.Join(h.uploadDir, dateDir)
	if err := os.MkdirAll(saveDir, 0755); err != nil {
		response.InternalError(c, "Không thể tạo thư mục upload")
		return
	}

	filename := fmt.Sprintf("%d%s", time.Now().UnixNano(), ext)
	dst := filepath.Join(saveDir, filename)

	src, err := file.Open()
	if err != nil {
		response.InternalError(c, "Không thể mở file")
		return
	}
	defer src.Close()

	out, err := os.Create(dst)
	if err != nil {
		response.InternalError(c, "Không thể lưu file")
		return
	}
	defer out.Close()

	if _, err := io.Copy(out, src); err != nil {
		response.InternalError(c, "Không thể ghi file")
		return
	}

	response.Success(c, gin.H{
		"url":      "/" + dst,
		"filename": filename,
		"size":     file.Size,
	})
}
