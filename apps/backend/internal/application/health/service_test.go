package health

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type fakePinger struct {
	err error
}

func (f fakePinger) Ping(context.Context) error { return f.err }

func TestServiceCheckReturnsOK(t *testing.T) {
	result, err := NewService().Check(context.Background())
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if result.Status != "ok" {
		t.Fatalf("Check() status = %q, want %q", result.Status, "ok")
	}
}

func TestServiceReadyChecksEachDependency(t *testing.T) {
	service := NewService(
		Dependency{Name: "postgres", Pinger: fakePinger{}},
		Dependency{Name: "redis", Pinger: fakePinger{err: errors.New("down")}},
	)
	result, err := service.Ready(context.Background())
	if result.Status != "unavailable" || err == nil || !strings.Contains(err.Error(), "redis: down") {
		t.Fatalf("Ready() = %+v, %v", result, err)
	}
}

func TestServiceReadySucceedsWhenDependenciesRespond(t *testing.T) {
	service := NewService(Dependency{Name: "postgres", Pinger: fakePinger{}})
	result, err := service.Ready(context.Background())
	if err != nil || result.Status != "ok" {
		t.Fatalf("Ready() = %+v, %v", result, err)
	}
}
