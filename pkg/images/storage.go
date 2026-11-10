package images

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const MaxUploadSize int64 = 10 << 20 // 10 MiB

var ErrInvalidImage = errors.New("unsupported or invalid image")
var ErrImageTooLarge = errors.New("image exceeds 10 MiB")

type Storage struct{ rootDir string }

func NewStorage(rootDir string) *Storage { return &Storage{rootDir: rootDir} }

// Save writes a validated image to the configured image directory and returns its safe filename.
func (s *Storage) Save(source io.Reader) (string, error) {
	if s.rootDir == "" {
		return "", errors.New("image directory is not configured")
	}
	if err := os.MkdirAll(s.rootDir, 0755); err != nil {
		return "", fmt.Errorf("create image directory: %w", err)
	}
	tmp, err := os.CreateTemp(s.rootDir, ".upload-*")
	if err != nil {
		return "", fmt.Errorf("create temporary image: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	n, err := io.Copy(tmp, io.LimitReader(source, MaxUploadSize+1))
	if err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("write image: %w", err)
	}
	if n == 0 {
		_ = tmp.Close()
		return "", ErrInvalidImage
	}
	if n > MaxUploadSize {
		_ = tmp.Close()
		return "", ErrImageTooLarge
	}
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		_ = tmp.Close()
		return "", err
	}
	header := make([]byte, 12)
	headerN, readErr := tmp.Read(header)
	if readErr != nil && readErr != io.EOF {
		_ = tmp.Close()
		return "", readErr
	}
	ext := imageExtension(header[:headerN])
	if ext == "" {
		_ = tmp.Close()
		return "", ErrInvalidImage
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}

	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return "", fmt.Errorf("generate image name: %w", err)
	}
	id[6] = (id[6] & 0x0f) | 0x40 // версия 4
	id[8] = (id[8] & 0x3f) | 0x80 // вариант 10
	filename := fmt.Sprintf("%x-%x-%x-%x-%x", id[0:4], id[4:6], id[6:8], id[8:10], id[10:16]) + ext
	if err := os.Rename(tmpName, filepath.Join(s.rootDir, filename)); err != nil {
		return "", fmt.Errorf("store image: %w", err)
	}
	return filename, nil
}

func (s *Storage) Delete(filename string) error {
	if !validImageFilename(filename) {
		return ErrInvalidImage
	}
	err := os.Remove(filepath.Join(s.rootDir, filename))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func imageExtension(b []byte) string {
	switch {
	case len(b) >= 3 && b[0] == 0xff && b[1] == 0xd8 && b[2] == 0xff:
		return ".jpg"
	case len(b) >= 8 && string(b[:8]) == "\x89PNG\r\n\x1a\n":
		return ".png"
	case len(b) >= 6 && (string(b[:6]) == "GIF87a" || string(b[:6]) == "GIF89a"):
		return ".gif"
	case len(b) >= 12 && string(b[:4]) == "RIFF" && string(b[8:12]) == "WEBP":
		return ".webp"
	default:
		return ""
	}
}

func validImageFilename(filename string) bool {
	if filename == "" || filepath.Base(filename) != filename || strings.ContainsAny(filename, `/\\`) {
		return false
	}
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp":
		return true
	default:
		return false
	}
}
