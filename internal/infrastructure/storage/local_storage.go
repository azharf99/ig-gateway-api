package storage

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
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
	if _, err := os.Stat(uploadDir); os.IsNotExist(err) {
		_ = os.MkdirAll(uploadDir, 0755)
	}
	return &localStorage{uploadDir: uploadDir}
}

func (s *localStorage) SaveFile(file *multipart.FileHeader) (string, error) {
	// Generate 8 random hex chars to make filename completely secure
	randBytes := make([]byte, 4)
	if _, err := rand.Read(randBytes); err != nil {
		return "", err
	}
	randHex := hex.EncodeToString(randBytes)

	ext := strings.ToLower(filepath.Ext(file.Filename))
	
	// Fallback detection for files without extension (e.g., blobs/temp files)
	if ext == "" {
		src, err := file.Open()
		if err == nil {
			buffer := make([]byte, 512)
			n, err := src.Read(buffer)
			src.Close()
			if err == nil || err == io.EOF {
				contentType := http.DetectContentType(buffer[:n])
				if contentType == "application/octet-stream" {
					contentType = file.Header.Get("Content-Type")
				}

				switch {
				case strings.HasPrefix(contentType, "image/jpeg") || contentType == "image/jpg":
					ext = ".jpg"
				case strings.HasPrefix(contentType, "image/png"):
					ext = ".png"
				case strings.HasPrefix(contentType, "image/webp"):
					ext = ".webp"
				case strings.HasPrefix(contentType, "video/mp4"):
					ext = ".mp4"
				case strings.HasPrefix(contentType, "video/quicktime"):
					ext = ".mov"
				case strings.HasPrefix(contentType, "video/x-msvideo"):
					ext = ".avi"
				case strings.HasPrefix(contentType, "video/x-matroska"):
					ext = ".mkv"
				case strings.HasPrefix(contentType, "audio/mpeg") || contentType == "audio/mp3":
					ext = ".mp3"
				case strings.HasPrefix(contentType, "audio/wav"):
					ext = ".wav"
				case strings.HasPrefix(contentType, "audio/ogg"):
					ext = ".ogg"
				case strings.HasPrefix(contentType, "audio/aac"):
					ext = ".aac"
				case contentType == "text/plain" || strings.HasPrefix(contentType, "text/"):
					ext = ".srt"
				}
			}
		}
	}

	// Restrict to safe formats
	allowedExts := map[string]bool{
		".jpg": true, ".jpeg": true, ".png": true, ".webp": true,
		".mp4": true, ".mov": true, ".avi": true, ".mkv": true,
		".mp3": true, ".wav": true, ".ogg": true, ".aac": true,
		".srt": true, ".vtt": true,
	}
	if !allowedExts[ext] {
		return "", fmt.Errorf("unsupported file extension: %s", ext)
	}

	filename := fmt.Sprintf("%d-%s%s", time.Now().UnixNano(), randHex, ext)
	relativePath := filepath.Join("uploads", filename)
	destPath := filepath.Join(s.uploadDir, filename)

	// Ensure destination remains inside upload directory (path traversal check using absolute paths)
	cleanUploadDir, err := filepath.Abs(s.uploadDir)
	if err != nil {
		return "", err
	}
	cleanDestPath, err := filepath.Abs(destPath)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(cleanDestPath, cleanUploadDir) {
		return "", fmt.Errorf("path traversal attempt detected")
	}

	src, err := file.Open()
	if err != nil {
		return "", err
	}
	defer src.Close()

	out, err := os.Create(cleanDestPath)
	if err != nil {
		return "", err
	}
	defer out.Close()

	_, err = io.Copy(out, src)
	if err != nil {
		return "", err
	}

	return filepath.ToSlash(relativePath), nil
}

func (s *localStorage) DeleteFile(path string) error {
	filename := filepath.Base(path)
	fullPath := filepath.Join(s.uploadDir, filename)

	// Path traversal protection using absolute paths
	cleanUploadDir, err := filepath.Abs(s.uploadDir)
	if err != nil {
		return err
	}
	cleanFullPath, err := filepath.Abs(fullPath)
	if err != nil {
		return err
	}
	if !strings.HasPrefix(cleanFullPath, cleanUploadDir) {
		return fmt.Errorf("path traversal attempt detected")
	}

	return os.Remove(cleanFullPath)
}

func (s *localStorage) GetPublicURL(path string) string {
	cleanPath := filepath.ToSlash(path)
	return fmt.Sprintf("%s/%s", strings.TrimSuffix(config.AppConfig.AppURL, "/"), cleanPath)
}


