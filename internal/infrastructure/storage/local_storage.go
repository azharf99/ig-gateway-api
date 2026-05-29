package storage

import (
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/azharf99/ig-gateway-api/internal/config"
)

type StorageService interface {
	SaveFile(file *multipart.FileHeader) (string, error)
	DeleteFile(path string) error
	GetPublicURL(path string) string
}

type localStorage struct {
	uploadDir string
}

func NewLocalStorage(uploadDir string) StorageService {
	// Ensure directory exists
	if _, err := os.Stat(uploadDir); os.IsNotExist(err) {
		_ = os.MkdirAll(uploadDir, 0755)
	}
	return &localStorage{uploadDir: uploadDir}
}

func (s *localStorage) SaveFile(file *multipart.FileHeader) (string, error) {
	src, err := file.Open()
	if err != nil {
		return "", err
	}
	defer src.Close()

	// Generate unique filename
	ext := filepath.Ext(file.Filename)
	filename := fmt.Sprintf("%d-%s%s", time.Now().UnixNano(), strings.ReplaceAll(filepath.Base(file.Filename), ext, ""), ext)
	relativePath := filepath.Join("uploads", filename)
	destPath := filepath.Join(s.uploadDir, filename)

	out, err := os.Create(destPath)
	if err != nil {
		return "", err
	}
	defer out.Close()

	_, err = io.Copy(out, src)
	if err != nil {
		return "", err
	}

	// Return normalized relative path (e.g. "uploads/12345.jpg")
	return filepath.ToSlash(relativePath), nil
}

func (s *localStorage) DeleteFile(path string) error {
	// path is e.g. "uploads/12345.jpg". We need the filename
	filename := filepath.Base(path)
	fullPath := filepath.Join(s.uploadDir, filename)
	return os.Remove(fullPath)
}

func (s *localStorage) GetPublicURL(path string) string {
	// Normalizes paths like "uploads/12345.jpg" to URL "http://app_url/uploads/12345.jpg"
	cleanPath := filepath.ToSlash(path)
	return fmt.Sprintf("%s/%s", strings.TrimSuffix(config.AppConfig.AppURL, "/"), cleanPath)
}
