package images

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type Handler struct {
	rootDir string
}

func NewHandler(rootDir string) *Handler {
	return &Handler{
		rootDir: rootDir,
	}
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	filename := r.PathValue("filename")

	if !isValidFilename(filename) {
		http.Error(w, "invalid image filename", http.StatusBadRequest)
		return
	}

	path := filepath.Join(h.rootDir, filename)

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			http.NotFound(w, r)
			return
		}

		http.Error(w, "failed to read image", http.StatusInternalServerError)
		return
	}

	if info.IsDir() {
		http.NotFound(w, r)
		return
	}

	http.ServeFile(w, r, path)
}

func isValidFilename(filename string) bool {
	if filename == "" {
		return false
	}

	if filepath.Base(filename) != filename {
		return false
	}

	if strings.ContainsAny(filename, `/\`) {
		return false
	}

	switch strings.ToLower(filepath.Ext(filename)) {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp":
		return true
	default:
		return false
	}
}
