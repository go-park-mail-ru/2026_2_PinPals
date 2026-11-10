package images

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestStorageSaveAndDelete(t *testing.T) {
	dir := t.TempDir()
	storage := NewStorage(dir)
	name, err := storage.Save(bytes.NewReader([]byte("\x89PNG\r\n\x1a\ncontent")))
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Ext(name) != ".png" {
		t.Fatalf("unexpected extension: %s", name)
	}
	if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
		t.Fatal(err)
	}
	if err := storage.Delete(name); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, name)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("image still exists: %v", err)
	}
}

func TestStorageRejectsInvalidAndOversizedImages(t *testing.T) {
	storage := NewStorage(t.TempDir())
	if _, err := storage.Save(bytes.NewReader([]byte("not an image"))); !errors.Is(err, ErrInvalidImage) {
		t.Fatalf("expected invalid image, got %v", err)
	}
	if _, err := storage.Save(bytes.NewReader(bytes.Repeat([]byte{0}, int(MaxUploadSize+1)))); !errors.Is(err, ErrImageTooLarge) {
		t.Fatalf("expected oversized image, got %v", err)
	}
}
