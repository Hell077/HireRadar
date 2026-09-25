package httpapi

import (
	"context"
	"errors"
	"strings"

	resumeapp "github.com/Hell077/HireRadar/apps/backend/internal/resume/application"
	"github.com/Hell077/HireRadar/apps/backend/internal/resume/domain"
	user "github.com/Hell077/HireRadar/apps/backend/internal/user/domain"
	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
)

type ResumeService interface {
	UploadURL(context.Context, user.UserID, string, string, int64) (resumeapp.Upload, error)
	Complete(context.Context, user.UserID, domain.ID) (domain.Resume, error)
	Get(context.Context, user.UserID, domain.ID) (resumeapp.Download, error)
	List(context.Context, user.UserID) ([]domain.Resume, error)
	Delete(context.Context, user.UserID, domain.ID) error
	Analysis(context.Context, user.UserID, domain.ID) (domain.ParsedResume, []domain.Suggestion, error)
	ReviewSuggestion(context.Context, user.UserID, domain.ID, string, bool) error
}

type resumeAuthInput struct {
	Authorization string `header:"Authorization" required:"false"`
}
type resumeUploadInput struct {
	Authorization string `header:"Authorization" required:"false"`
	Body          struct {
		FileName    string `json:"file_name"`
		ContentType string `json:"content_type"`
		Size        int64  `json:"size"`
	}
}
type resumeIDInput struct {
	Authorization string `header:"Authorization" required:"false"`
	ID            string `path:"id"`
}
type resumeSuggestionInput struct {
	Authorization string `header:"Authorization" required:"false"`
	ID            string `path:"id"`
	SuggestionID  string `path:"suggestionID"`
}
type resumeUploadOutput struct {
	Body struct {
		Resume    domain.Resume `json:"resume"`
		UploadURL string        `json:"upload_url"`
	}
}
type resumeOutput struct{ Body domain.Resume }
type resumeDownloadOutput struct {
	Body struct {
		Resume      domain.Resume `json:"resume"`
		DownloadURL string        `json:"download_url,omitempty"`
	}
}
type resumesOutput struct {
	Body struct {
		Resumes []domain.Resume `json:"resumes"`
	}
}
type emptyOutput struct{}
type resumeAnalysisOutput struct {
	Body struct {
		Analysis    domain.ParsedResume `json:"analysis"`
		Suggestions []domain.Suggestion `json:"suggestions"`
	}
}

func resumeID(raw string) (domain.ID, error) {
	if _, err := uuid.Parse(raw); err != nil {
		return "", huma.Error404NotFound("resume not found")
	}
	return domain.ID(raw), nil
}

func registerResumes(api huma.API, service ResumeService, verifier AccessVerifier) {
	huma.Register(api, huma.Operation{OperationID: "resume-analysis", Method: "GET", Path: "/api/v1/resumes/{id}/analysis", Summary: "Get parsed resume and profile suggestions"}, func(ctx context.Context, in *resumeIDInput) (*resumeAnalysisOutput, error) {
		if service == nil {
			return nil, huma.Error503ServiceUnavailable("resume processing unavailable")
		}
		owner, err := profileUser(in.Authorization, verifier)
		if err != nil {
			return nil, err
		}
		id, err := resumeID(in.ID)
		if err != nil {
			return nil, err
		}
		analysis, suggestions, err := service.Analysis(ctx, owner, id)
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				return nil, huma.Error404NotFound("resume not found")
			}
			if errors.Is(err, domain.ErrAnalysisNotReady) {
				return nil, huma.Error409Conflict("resume analysis is not ready")
			}
			return nil, huma.Error500InternalServerError("could not load resume analysis")
		}
		out := &resumeAnalysisOutput{}
		out.Body.Analysis = analysis
		out.Body.Suggestions = suggestions
		return out, nil
	})
	for _, decision := range []struct {
		name   string
		accept bool
	}{{name: "accept", accept: true}, {name: "reject"}} {
		decision := decision
		huma.Register(api, huma.Operation{OperationID: "resume-suggestion-" + decision.name, Method: "POST", Path: "/api/v1/resumes/{id}/suggestions/{suggestionID}/" + decision.name, Summary: strings.Title(decision.name) + " a profile suggestion", DefaultStatus: 204}, func(ctx context.Context, in *resumeSuggestionInput) (*emptyOutput, error) {
			if service == nil {
				return nil, huma.Error503ServiceUnavailable("resume processing unavailable")
			}
			owner, err := profileUser(in.Authorization, verifier)
			if err != nil {
				return nil, err
			}
			id, err := resumeID(in.ID)
			if err != nil {
				return nil, err
			}
			if _, err := uuid.Parse(in.SuggestionID); err != nil {
				return nil, huma.Error404NotFound("suggestion not found")
			}
			err = service.ReviewSuggestion(ctx, owner, id, in.SuggestionID, decision.accept)
			if err != nil {
				if errors.Is(err, domain.ErrNotFound) {
					return nil, huma.Error404NotFound("suggestion not found")
				}
				if errors.Is(err, domain.ErrSuggestionReviewed) {
					return nil, huma.Error409Conflict("suggestion was already reviewed")
				}
				return nil, huma.Error500InternalServerError("could not review suggestion")
			}
			return &emptyOutput{}, nil
		})
	}
	huma.Register(api, huma.Operation{OperationID: "resume-list", Method: "GET", Path: "/api/v1/resumes", Summary: "List candidate resumes"}, func(ctx context.Context, in *resumeAuthInput) (*resumesOutput, error) {
		if service == nil {
			return nil, huma.Error503ServiceUnavailable("resume storage unavailable")
		}
		owner, err := profileUser(in.Authorization, verifier)
		if err != nil {
			return nil, err
		}
		items, err := service.List(ctx, owner)
		if err != nil {
			return nil, huma.Error500InternalServerError("resume lookup failed")
		}
		out := &resumesOutput{}
		out.Body.Resumes = items
		return out, nil
	})
	huma.Register(api, huma.Operation{OperationID: "resume-upload-url", Method: "POST", Path: "/api/v1/resumes/upload-url", Summary: "Create a private resume upload URL"}, func(ctx context.Context, in *resumeUploadInput) (*resumeUploadOutput, error) {
		if service == nil {
			return nil, huma.Error503ServiceUnavailable("resume storage unavailable")
		}
		owner, err := profileUser(in.Authorization, verifier)
		if err != nil {
			return nil, err
		}
		result, err := service.UploadURL(ctx, owner, in.Body.FileName, in.Body.ContentType, in.Body.Size)
		if err != nil {
			if errors.Is(err, domain.ErrInvalidResume) {
				return nil, huma.Error400BadRequest("invalid PDF upload metadata")
			}
			return nil, huma.Error500InternalServerError("could not create upload URL")
		}
		out := &resumeUploadOutput{}
		out.Body.Resume = result.Resume
		out.Body.UploadURL = result.UploadURL
		return out, nil
	})
	huma.Register(api, huma.Operation{OperationID: "resume-complete", Method: "POST", Path: "/api/v1/resumes/{id}/complete", Summary: "Verify a resume upload"}, func(ctx context.Context, in *resumeIDInput) (*resumeOutput, error) {
		if service == nil {
			return nil, huma.Error503ServiceUnavailable("resume storage unavailable")
		}
		owner, err := profileUser(in.Authorization, verifier)
		if err != nil {
			return nil, err
		}
		id, err := resumeID(in.ID)
		if err != nil {
			return nil, err
		}
		item, err := service.Complete(ctx, owner, id)
		if err != nil {
			if errors.Is(err, resumeapp.ErrNotFound) {
				return nil, huma.Error404NotFound("resume not found")
			}
			if errors.Is(err, resumeapp.ErrInvalidObject) || errors.Is(err, resumeapp.ErrNotReady) {
				return nil, huma.Error400BadRequest("uploaded PDF is invalid or incomplete")
			}
			return nil, huma.Error500InternalServerError("could not complete resume upload")
		}
		return &resumeOutput{Body: item}, nil
	})
	huma.Register(api, huma.Operation{OperationID: "resume-get", Method: "GET", Path: "/api/v1/resumes/{id}", Summary: "Get a resume and short-lived download URL"}, func(ctx context.Context, in *resumeIDInput) (*resumeDownloadOutput, error) {
		if service == nil {
			return nil, huma.Error503ServiceUnavailable("resume storage unavailable")
		}
		owner, err := profileUser(in.Authorization, verifier)
		if err != nil {
			return nil, err
		}
		id, err := resumeID(in.ID)
		if err != nil {
			return nil, err
		}
		item, err := service.Get(ctx, owner, id)
		if err != nil {
			if errors.Is(err, resumeapp.ErrNotFound) {
				return nil, huma.Error404NotFound("resume not found")
			}
			return nil, huma.Error500InternalServerError("could not load resume")
		}
		out := &resumeDownloadOutput{}
		out.Body.Resume = item.Resume
		out.Body.DownloadURL = item.DownloadURL
		return out, nil
	})
	huma.Register(api, huma.Operation{OperationID: "resume-delete", Method: "DELETE", Path: "/api/v1/resumes/{id}", Summary: "Delete a resume", DefaultStatus: 204}, func(ctx context.Context, in *resumeIDInput) (*emptyOutput, error) {
		if service == nil {
			return nil, huma.Error503ServiceUnavailable("resume storage unavailable")
		}
		owner, err := profileUser(in.Authorization, verifier)
		if err != nil {
			return nil, err
		}
		id, err := resumeID(in.ID)
		if err != nil {
			return nil, err
		}
		if err := service.Delete(ctx, owner, id); err != nil {
			if errors.Is(err, resumeapp.ErrNotFound) {
				return nil, huma.Error404NotFound("resume not found")
			}
			return nil, huma.Error500InternalServerError("could not delete resume")
		}
		return &emptyOutput{}, nil
	})
}
