package docker

import (
	"testing"
)

func TestNewClient_FromEnv(t *testing.T) {
	// Integration test - requires Docker daemon running
	// Skip in CI environments without Docker
	t.Skip("requires Docker daemon")
}
