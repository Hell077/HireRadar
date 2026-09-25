package httpapi

import (
	"context"
	"crypto/subtle"
	"fmt"

	"github.com/Hell077/HireRadar/apps/backend/internal/lifecycle"
	"github.com/danielgtaylor/huma/v2"
)

type ServiceController interface {
	List() []lifecycle.Status
	Start(context.Context, string) error
	Stop(context.Context, string) error
	Restart(context.Context, string) error
}

type adminServicesInput struct {
	Token string `header:"X-Operator-Token" required:"false"`
}
type adminServiceActionInput struct {
	Token  string `header:"X-Operator-Token" required:"false"`
	Name   string `path:"name"`
	Action string `json:"action" enum:"start,stop,restart"`
}
type adminServicesOutput struct {
	Body struct {
		Services []lifecycle.Status `json:"services"`
	}
}
type adminServiceActionOutput struct {
	Body struct {
		Status  string           `json:"status"`
		Service lifecycle.Status `json:"service"`
	}
}

func registerServices(api huma.API, controller ServiceController, token string) {
	validToken := func(value string) bool {
		return token != "" && len(value) == len(token) && subtle.ConstantTimeCompare([]byte(value), []byte(token)) == 1
	}
	unauthorized := func(value string) error {
		if validToken(value) {
			return nil
		}
		if token == "" {
			return huma.Error503ServiceUnavailable("service controls are unavailable")
		}
		return huma.Error401Unauthorized("operator token required")
	}
	huma.Register(api, huma.Operation{OperationID: "admin-services-list", Method: "GET", Path: "/api/v1/admin/services", Summary: "Read background service status"}, func(ctx context.Context, input *adminServicesInput) (*adminServicesOutput, error) {
		if err := unauthorized(input.Token); err != nil {
			return nil, err
		}
		if controller == nil {
			return nil, huma.Error503ServiceUnavailable("service controls are unavailable")
		}
		out := &adminServicesOutput{}
		out.Body.Services = controller.List()
		return out, nil
	})
	huma.Register(api, huma.Operation{OperationID: "admin-service-action", Method: "POST", Path: "/api/v1/admin/services/{name}", Summary: "Start, stop or restart a background service"}, func(ctx context.Context, input *adminServiceActionInput) (*adminServiceActionOutput, error) {
		if err := unauthorized(input.Token); err != nil {
			return nil, err
		}
		if controller == nil {
			return nil, huma.Error503ServiceUnavailable("service controls are unavailable")
		}
		var err error
		switch input.Action {
		case "start":
			err = controller.Start(ctx, input.Name)
		case "stop":
			err = controller.Stop(ctx, input.Name)
		case "restart":
			err = controller.Restart(ctx, input.Name)
		default:
			return nil, huma.Error400BadRequest("action must be start, stop or restart")
		}
		if err != nil {
			return nil, huma.Error400BadRequest(fmt.Sprintf("service action failed: %v", err))
		}
		out := &adminServiceActionOutput{}
		out.Body.Status = input.Action
		for _, status := range controller.List() {
			if status.Name == input.Name {
				out.Body.Service = status
				break
			}
		}
		return out, nil
	})
}
