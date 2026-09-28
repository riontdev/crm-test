package handlers

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

const defaultStorageBucket = "attachments"

type minioStore struct {
	client  *minio.Client
	bucket  string
	enabled bool
}

func newMinioStore() *minioStore {
	endpoint := os.Getenv("MINIO_ENDPOINT")
	accessKey := os.Getenv("MINIO_ACCESS_KEY")
	secretKey := os.Getenv("MINIO_SECRET_KEY")
	bucket := os.Getenv("MINIO_BUCKET")
	if bucket == "" {
		bucket = defaultStorageBucket
	}

	if endpoint == "" || accessKey == "" || secretKey == "" {
		return &minioStore{enabled: false}
	}

	useSSL := strings.EqualFold(os.Getenv("MINIO_USE_SSL"), "true")

	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return &minioStore{enabled: false}
	}

	return &minioStore{
		client:  client,
		bucket:  bucket,
		enabled: true,
	}
}

type UploadHandler struct {
	store *minioStore
}

func NewUploadHandler() *UploadHandler {
	return &UploadHandler{store: newMinioStore()}
}

// Upload handles file uploads to local MinIO storage.
// POST /api/upload (multipart/form-data)
func (h *UploadHandler) Upload(c echo.Context) error {
	if !h.store.enabled {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{
			"error": "MINIO_ENDPOINT, MINIO_ACCESS_KEY and MINIO_SECRET_KEY not configured",
		})
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "no file provided"})
	}

	// Validate file type
	allowedTypes := map[string]bool{
		"image/jpeg": true, "image/png": true, "image/gif": true, "image/webp": true,
		"video/mp4": true, "video/quicktime": true, "video/webm": true,
		"audio/mpeg": true, "audio/ogg": true,
		"application/pdf": true,
	}

	contentType := fileHeader.Header.Get("Content-Type")
	if !allowedTypes[contentType] {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": fmt.Sprintf("file type %s not allowed", contentType),
		})
	}

	// Max 10MB
	if fileHeader.Size > 10*1024*1024 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "file too large (max 10MB)"})
	}

	src, err := fileHeader.Open()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to read file"})
	}
	defer src.Close()

	fileBytes, err := io.ReadAll(src)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to read file"})
	}

	// Generate unique object key
	ext := filepath.Ext(fileHeader.Filename)
	if ext == "" {
		ext = "." + strings.Split(contentType, "/")[1]
	}
	objectKey := fmt.Sprintf("%s/%s%s", time.Now().Format("2006-01"), uuid.New().String(), ext)

	putCtx := c.Request().Context()
	_, err = h.store.client.PutObject(putCtx, h.store.bucket, objectKey, bytes.NewReader(fileBytes), int64(len(fileBytes)), minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return c.JSON(http.StatusBadGateway, map[string]string{"error": "failed to upload to storage"})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"url":          fmt.Sprintf("/api/files/%s/%s", h.store.bucket, objectKey),
		"filename":     fileHeader.Filename,
		"content_type": contentType,
		"size":         fileHeader.Size,
	})
}

// FilesHandler serves files stored in MinIO through /api/files/*.
type FilesHandler struct {
	store *minioStore
}

func NewFilesHandler() *FilesHandler {
	return &FilesHandler{store: newMinioStore()}
}

// Serve streams a stored object.
// GET /api/files/:bucket/:prefix... (wildcard param)
func (h *FilesHandler) Serve(c echo.Context) error {
	if !h.store.enabled {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{
			"error": "MINIO_ENDPOINT, MINIO_ACCESS_KEY and MINIO_SECRET_KEY not configured",
		})
	}

	path := c.Param("*")
	if path == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "missing object path"})
	}

	parts := strings.SplitN(path, "/", 2)
	if len(parts) != 2 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid object path"})
	}
	bucket, objectKey := parts[0], parts[1]

	obj, err := h.store.client.GetObject(c.Request().Context(), bucket, objectKey, minio.GetObjectOptions{})
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "file not found"})
	}
	defer obj.Close()

	stat, err := obj.Stat()
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "file not found"})
	}

	contentType := stat.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	c.Response().Header().Set("Content-Type", contentType)
	c.Response().Header().Set("Cache-Control", "public, max-age=86400")
	return c.Stream(http.StatusOK, contentType, obj)
}
