package model

import "time"

type ContainerInfo struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Image     string            `json:"image"`
	Status    string            `json:"status"`
	Ports     []string          `json:"ports"`
	Labels    map[string]string `json:"labels"`
	CreatedAt string            `json:"created_at"`
	ScannedAt string            `json:"scanned_at"`
}

func NewContainerInfo() *ContainerInfo {
	return &ContainerInfo{
		Ports:  []string{},
		Labels: make(map[string]string),
	}
}

func (c *ContainerInfo) SetScannedNow() {
	c.ScannedAt = time.Now().Format(time.RFC3339)
}
