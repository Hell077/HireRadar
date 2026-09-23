package health

import (
	"context"
	"testing"
)

func TestServiceCheckReturnsOK(t *testing.T) {
	result, err := NewService().Check(context.Background())
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if result.Status != "ok" {
		t.Fatalf("Check() status = %q, want %q", result.Status, "ok")
	}
}
