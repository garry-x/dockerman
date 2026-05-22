package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"dockerman/internal/model"
)

type JSONStore struct {
	Path string
}

func NewJSONStore(path string) *JSONStore {
	return &JSONStore{Path: path}
}

func (s *JSONStore) Load() ([]model.ContainerInfo, error) {
	data, err := os.ReadFile(s.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return []model.ContainerInfo{}, nil
		}
		return nil, fmt.Errorf("read store file: %w", err)
	}
	if len(data) == 0 {
		return []model.ContainerInfo{}, nil
	}
	var containers []model.ContainerInfo
	if err := json.Unmarshal(data, &containers); err != nil {
		return nil, fmt.Errorf("parse store file: %w", err)
	}
	return containers, nil
}

func (s *JSONStore) Save(containers []model.ContainerInfo) error {
	dir := filepath.Dir(s.Path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create store directory: %w", err)
	}
	data, err := json.MarshalIndent(containers, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal containers: %w", err)
	}
	if err := os.WriteFile(s.Path, data, 0644); err != nil {
		return fmt.Errorf("write store file: %w", err)
	}
	return nil
}

func (s *JSONStore) GetByID(id string) (*model.ContainerInfo, error) {
	containers, err := s.Load()
	if err != nil {
		return nil, err
	}
	for i, c := range containers {
		if c.ID == id || c.Name == id {
			return &containers[i], nil
		}
	}
	return nil, fmt.Errorf("container %q not found in store", id)
}
