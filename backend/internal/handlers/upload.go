package handlers

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type UploadHandler struct {
	supabaseURL    string
	supabaseKey    string
	storageBucket  string
}

func NewUploadHandler() *UploadHandler {
	return &UploadHandler{
		supabaseURL:   os.Getenv("SUPABASE_URL"),
		supabaseKey:   os.Getenv("SUPABASE_SERVICE_KEY"),
		storageBucket: "attachments",
	}
}

// Upload handles file uploads to Supabase Storage.
// POST /api/upload (multipart/form-data)
func (h *UploadHandler) Upload(c echo.Context) error {
	if h.supabaseURL == "" || h.supabaseKey == "" {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{
			"error": "SUPABASE_URL or SUPABASE_SERVICE_KEY not configured",
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

	// Read file
	src, err := fileHeader.Open()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to read file"})
	}
	defer src.Close()

	fileBytes, err := io.ReadAll(src)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to read file"})
	}

	// Generate unique filename
	ext := filepath.Ext(fileHeader.Filename)
	if ext == "" {
		ext = "." + strings.Split(contentType, "/")[1]
	}
	filename := fmt.Sprintf("%s/%s%s", time.Now().Format("2006-01"), uuid.New().String(), ext)

	// Upload to Supabase Storage using raw body (not multipart)
	uploadURL := fmt.Sprintf("%s/storage/v1/object/%s/%s", h.supabaseURL, h.storageBucket, filename)
	req, err := http.NewRequest("POST", uploadURL, strings.NewReader(string(fileBytes)))
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to create request"})
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Authorization", "Bearer "+h.supabaseKey)
	req.Header.Set("x-upsert", "true")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return c.JSON(http.StatusBadGateway, map[string]string{"error": "failed to upload to storage"})
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return c.JSON(http.StatusBadGateway, map[string]string{
			"error": fmt.Sprintf("storage upload failed (status %d): %s", resp.StatusCode, string(respBody)),
		})
	}

	// Return public URL
	publicURL := fmt.Sprintf("%s/storage/v1/object/public/%s/%s", h.supabaseURL, h.storageBucket, filename)

	return c.JSON(http.StatusOK, map[string]interface{}{
		"url":         publicURL,
		"filename":    fileHeader.Filename,
		"content_type": contentType,
		"size":        fileHeader.Size,
	})
}
