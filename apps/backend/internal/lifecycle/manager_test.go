package lifecycle

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestManagerControlsAndReportsServiceState(t *testing.T) {
	manager := New()
	started := make(chan struct{}, 2)
	if err := manager.Register("fixture", true, "", func(ctx context.Context) error {
		started <- struct{}{}
		<-ctx.Done()
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := manager.Register("optional", false, "not configured", nil); err != nil {
		t.Fatal(err)
	}
	root, cancelRoot := context.WithCancel(context.Background())
	defer cancelRoot()
	manager.StartAll(root)
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("service did not start")
	}
	if got := stateOf(t, manager, "fixture").State; got != "running" {
		t.Fatalf("state=%q want running", got)
	}
	if err := manager.Start(context.Background(), "optional"); err == nil {
		t.Fatal("starting disabled service succeeded")
	}
	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := manager.Stop(stopCtx, "fixture"); err != nil {
		t.Fatal(err)
	}
	if got := stateOf(t, manager, "fixture").State; got != "stopped" {
		t.Fatalf("state=%q want stopped", got)
	}
	if err := manager.Restart(context.Background(), "fixture"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("service did not restart")
	}
	if err := manager.StopAll(stopCtx); err != nil {
		t.Fatal(err)
	}
}

func TestManagerRecordsUnexpectedExitAndCanRestart(t *testing.T) {
	manager := New()
	if err := manager.Register("fixture", true, "", func(context.Context) error { return errors.New("dependency unavailable") }); err != nil {
		t.Fatal(err)
	}
	if err := manager.Start(context.Background(), "fixture"); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		status := stateOf(t, manager, "fixture")
		if status.State == "failed" {
			if !strings.Contains(status.LastError, "dependency unavailable") {
				t.Fatalf("last error=%q", status.LastError)
			}
			if status.StartCount != 1 {
				t.Fatalf("start count=%d want 1", status.StartCount)
			}
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("service failure was not reported")
}

func stateOf(t *testing.T, manager *Manager, name string) Status {
	t.Helper()
	for _, status := range manager.List() {
		if status.Name == name {
			return status
		}
	}
	t.Fatalf("service %q is missing", name)
	return Status{}
}
