package health

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Result is the application-level response for the health check use case.
type Result struct {
	Status string
}

// Checker is the inbound port consumed by the HTTP adapter.
type Checker interface {
	Check(context.Context) (Result, error)
	Ready(context.Context) (Result, error)
}

type Pinger interface {
	Ping(context.Context) error
}

type Dependency struct {
	Name   string
	Pinger Pinger
}

// Service implements the health check use case.
type Service struct {
	dependencies []Dependency
}

func NewService(dependencies ...Dependency) *Service {
	return &Service{dependencies: dependencies}
}

func (*Service) Check(context.Context) (Result, error) {
	return Result{Status: "ok"}, nil
}

func (s *Service) Ready(ctx context.Context) (Result, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	var failures []error
	for _, dependency := range s.dependencies {
		if err := dependency.Pinger.Ping(ctx); err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", dependency.Name, err))
		}
	}
	if len(failures) > 0 {
		return Result{Status: "unavailable"}, errors.Join(failures...)
	}
	return Result{Status: "ok"}, nil
}
