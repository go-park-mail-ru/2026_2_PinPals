package images

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHandlerGet(t *testing.T) {
	dir := t.TempDir()

	imageData := []byte{
		0x89, 0x50, 0x4E, 0x47,
	}

	imagePath := filepath.Join(dir, "image.png")
	require.NoError(t, os.WriteFile(imagePath, imageData, 0644))

	handler := NewHandler(dir)

	req := httptest.NewRequest(http.MethodGet, "/images/image.png", nil)
	req.SetPathValue("filename", "image.png")

	rec := httptest.NewRecorder()

	handler.Get(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "image/png", rec.Header().Get("Content-Type"))
	require.Equal(t, imageData, rec.Body.Bytes())
}

func TestHandlerGetNotFound(t *testing.T) {
	handler := NewHandler(t.TempDir())

	req := httptest.NewRequest(http.MethodGet, "/images/missing.png", nil)
	req.SetPathValue("filename", "missing.png")

	rec := httptest.NewRecorder()

	handler.Get(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestHandlerGetDirectory(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "images"), 0755))

	handler := NewHandler(dir)

	req := httptest.NewRequest(http.MethodGet, "/images/images", nil)
	req.SetPathValue("filename", "images")

	rec := httptest.NewRecorder()

	handler.Get(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandlerGetInvalidFilename(t *testing.T) {
	handler := NewHandler(t.TempDir())

	tests := []string{
		"",
		"../image.png",
		"..\\image.png",
		"image/test.png",
		"image\\test.png",
		"image.txt",
		"image.exe",
	}

	for _, filename := range tests {
		t.Run(filename, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/images/"+filename, nil)
			req.SetPathValue("filename", filename)

			rec := httptest.NewRecorder()

			handler.Get(rec, req)

			require.Equal(t, http.StatusBadRequest, rec.Code)
		})
	}
}

func TestHandlerGetSupportedExtensions(t *testing.T) {
	dir := t.TempDir()

	files := map[string]string{
		"image.jpg":  "image/jpeg",
		"image.jpeg": "image/jpeg",
		"image.png":  "image/png",
		"image.gif":  "image/gif",
		"image.webp": "image/webp",
	}

	for filename := range files {
		require.NoError(t, os.WriteFile(
			filepath.Join(dir, filename),
			[]byte("test image"),
			0644,
		))
	}

	handler := NewHandler(dir)

	for filename, contentType := range files {
		t.Run(filename, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/images/"+filename, nil)
			req.SetPathValue("filename", filename)

			rec := httptest.NewRecorder()

			handler.Get(rec, req)

			require.Equal(t, http.StatusOK, rec.Code)
			require.Equal(t, contentType, rec.Header().Get("Content-Type"))
			require.Equal(t, []byte("test image"), rec.Body.Bytes())
		})
	}
}
