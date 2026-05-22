package store

import (
	"os"
	"path/filepath"
	"testing"

	"dockerman/internal/model"
)

func TestLoad_NonexistentFile_ReturnsEmpty(t *testing.T) {
	s := NewJSONStore(filepath.Join(t.TempDir(), "nonexistent.json"))
	containers, err := s.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(containers) != 0 {
		t.Fatalf("expected empty list, got %d items", len(containers))
	}
}

func TestSaveAndLoad_RoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "containers.json")
	s := NewJSONStore(path)

	input := []model.ContainerInfo{
		{ID: "abc123", Name: "web", Image: "nginx", Status: "running"},
		{ID: "def456", Name: "db", Image: "postgres", Status: "exited"},
	}

	if err := s.Save(input); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	loaded, err := s.Load()
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("expected 2 containers, got %d", len(loaded))
	}
	if loaded[0].ID != "abc123" || loaded[1].ID != "def456" {
		t.Fatalf("data mismatch: %+v", loaded)
	}
}

func TestGetByID_Found(t *testing.T) {
	path := filepath.Join(t.TempDir(), "containers.json")
	s := NewJSONStore(path)
	if err := s.Save([]model.ContainerInfo{
		{ID: "abc123", Name: "web", Image: "nginx"},
	}); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	c, err := s.GetByID("abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.Name != "web" {
		t.Fatalf("expected web, got %s", c.Name)
	}
}

func TestGetByID_ByName(t *testing.T) {
	path := filepath.Join(t.TempDir(), "containers.json")
	s := NewJSONStore(path)
	if err := s.Save([]model.ContainerInfo{
		{ID: "abc123", Name: "web", Image: "nginx"},
	}); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	c, err := s.GetByID("web")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.ID != "abc123" {
		t.Fatalf("expected abc123, got %s", c.ID)
	}
}

func TestGetByID_NotFound(t *testing.T) {
	s := NewJSONStore(filepath.Join(t.TempDir(), "nonexistent.json"))
	_, err := s.GetByID("missing")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestSave_CreatesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "subdir", "data")
	path := filepath.Join(dir, "containers.json")
	s := NewJSONStore(path)

	err := s.Save([]model.ContainerInfo{{ID: "test"}})
	if err != nil {
		t.Fatalf("save failed: %v", err)
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("file was not created")
	}
}
