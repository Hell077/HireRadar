package health

import "context"

// Result is the application-level response for the health check use case.
type Result struct {
	Status string
}

// Checker is the inbound port consumed by the HTTP adapter.
type Checker interface {
	Check(context.Context) (Result, error)
}

// Service implements the health check use case.
type Service struct{}

func NewService() *Service {
	return &Service{}
}

func (*Service) Check(context.Context) (Result, error) {
	return Result{Status: "ok"}, nil
}
