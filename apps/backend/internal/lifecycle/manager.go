package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"sort"
	"sync"
	"time"

	"log/slog"
)

type Runner func(context.Context) error

type Status struct {
	Name       string     `json:"name"`
	State      string     `json:"state"`
	Enabled    bool       `json:"enabled"`
	Reason     string     `json:"reason,omitempty"`
	StartedAt  *time.Time `json:"started_at,omitempty"`
	StoppedAt  *time.Time `json:"stopped_at,omitempty"`
	LastError  string     `json:"last_error,omitempty"`
	StartCount uint64     `json:"start_count"`
}

type service struct {
	status Status
	runner Runner
	cancel context.CancelFunc
	done   chan struct{}
	gen    uint64
}

// Manager supervises in-process services. Unexpected exits are recorded and
// surfaced to operators; restart is explicit so a failing dependency cannot
// create an invisible crash loop.
type Manager struct {
	mu       sync.Mutex
	services map[string]*service
	root     context.Context
}

func New() *Manager { return &Manager{services: make(map[string]*service)} }

func (m *Manager) Register(name string, enabled bool, reason string, runner Runner) error {
	if name == "" || (enabled && runner == nil) {
		return fmt.Errorf("service name and enabled runner are required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.services[name]; exists {
		return fmt.Errorf("service %q is already registered", name)
	}
	state := "stopped"
	if !enabled {
		state = "disabled"
	}
	m.services[name] = &service{status: Status{Name: name, State: state, Enabled: enabled, Reason: reason}, runner: runner}
	return nil
}

func (m *Manager) StartAll(ctx context.Context) {
	m.mu.Lock()
	m.root = ctx
	m.mu.Unlock()
	for _, status := range m.List() {
		if status.Enabled {
			if err := m.Start(ctx, status.Name); err != nil {
				slog.Error("service start failed", "service", status.Name, "error", err)
			}
		}
	}
}

func (m *Manager) StopAll(ctx context.Context) error {
	var errs []error
	for _, status := range m.List() {
		if status.Enabled {
			if err := m.Stop(ctx, status.Name); err != nil {
				errs = append(errs, fmt.Errorf("stop %s: %w", status.Name, err))
			}
		}
	}
	return errorsJoin(errs)
}

func (m *Manager) List() []Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]Status, 0, len(m.services))
	for _, item := range m.services {
		result = append(result, item.status)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

func (m *Manager) ReportError(name string, err error) {
	if err == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if item := m.services[name]; item != nil {
		item.status.LastError = err.Error()
	}
}

func (m *Manager) ReportSuccess(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if item := m.services[name]; item != nil {
		item.status.LastError = ""
	}
}

func (m *Manager) Start(parent context.Context, name string) error {
	m.mu.Lock()
	item := m.services[name]
	if item == nil {
		m.mu.Unlock()
		return fmt.Errorf("unknown service %q", name)
	}
	if !item.status.Enabled {
		reason := item.status.Reason
		m.mu.Unlock()
		return fmt.Errorf("service %q is disabled: %s", name, reason)
	}
	if item.status.State == "running" || item.status.State == "starting" || item.status.State == "stopping" {
		m.mu.Unlock()
		return fmt.Errorf("service %q is %s", name, item.status.State)
	}
	if m.root != nil {
		parent = m.root
	}
	ctx, cancel := context.WithCancel(parent)
	done := make(chan struct{})
	item.gen++
	gen := item.gen
	item.cancel, item.done = cancel, done
	item.status.State = "starting"
	item.status.LastError = ""
	item.status.StartCount++
	slog.Info("service starting", "service", name, "start_count", item.status.StartCount)
	m.mu.Unlock()

	go func() {
		started := time.Now().UTC()
		m.mu.Lock()
		if current := m.services[name]; current != nil && current.gen == gen {
			if current.status.State == "starting" {
				current.status.State = "running"
				current.status.StartedAt = &started
			}
		}
		m.mu.Unlock()
		slog.Info("service running", "service", name)
		err := runSafely(ctx, item.runner)
		stopped := time.Now().UTC()
		m.mu.Lock()
		if current := m.services[name]; current != nil && current.gen == gen {
			current.status.StoppedAt = &stopped
			current.cancel, current.done = nil, nil
			if err != nil {
				current.status.State = "failed"
				current.status.LastError = err.Error()
			} else {
				current.status.State = "stopped"
			}
		}
		close(done)
		m.mu.Unlock()
		if err != nil {
			slog.Error("service exited unexpectedly", "service", name, "error", err)
		} else {
			slog.Info("service stopped", "service", name)
		}
	}()
	return nil
}

func (m *Manager) Stop(ctx context.Context, name string) error {
	m.mu.Lock()
	item := m.services[name]
	if item == nil {
		m.mu.Unlock()
		return fmt.Errorf("unknown service %q", name)
	}
	if item.status.State == "stopped" || item.status.State == "disabled" || item.status.State == "failed" {
		m.mu.Unlock()
		return nil
	}
	item.status.State = "stopping"
	cancel, done := item.cancel, item.done
	m.mu.Unlock()
	cancel()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (m *Manager) Restart(ctx context.Context, name string) error {
	if err := m.Stop(ctx, name); err != nil {
		return err
	}
	return m.Start(ctx, name)
}

func runSafely(ctx context.Context, runner Runner) (err error) {
	defer func() {
		if value := recover(); value != nil {
			err = fmt.Errorf("panic: %v\n%s", value, debug.Stack())
		}
	}()
	return runner(ctx)
}

func errorsJoin(errs []error) error {
	return errors.Join(errs...)
}
