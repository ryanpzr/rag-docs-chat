package listdocuments

import (
	"context"
	"net/http"
	"time"

	"github.com/ryanpzr/rag-docs-chat/internal/document/domain"
	"github.com/ryanpzr/rag-docs-chat/pkg/apperrors"
)

// Repository is the part of shared.Repository this feature needs.
type Repository interface {
	ListSummaries(ctx context.Context) ([]domain.DocumentSummary, error)
}

type Usecase interface {
	List(ctx context.Context) (ListResponse, error)
}

type DocumentResponse struct {
	ID         int64     `json:"id"`
	Filename   string    `json:"filename"`
	SizeBytes  int       `json:"size_bytes"`
	ChunkCount int       `json:"chunk_count"`
	CreatedAt  time.Time `json:"created_at"`
}

type ListResponse struct {
	Documents []DocumentResponse `json:"documents"`
}

type usecase struct {
	repo Repository
}

func NewUsecase(repo Repository) Usecase {
	return &usecase{repo: repo}
}

func (u *usecase) List(ctx context.Context) (ListResponse, error) {
	docs, err := u.repo.ListSummaries(ctx)
	if err != nil {
		return ListResponse{}, apperrors.NewInternal("failed to list documents", err)
	}
	out := ListResponse{Documents: make([]DocumentResponse, len(docs))}
	for i, d := range docs {
		out.Documents[i] = DocumentResponse{
			ID:         d.ID,
			Filename:   d.Filename,
			SizeBytes:  d.SizeBytes,
			ChunkCount: d.ChunkCount,
			CreatedAt:  d.CreatedAt,
		}
	}
	return out, nil
}

type Handler struct {
	usecase Usecase
}

func NewHandler(uc Usecase) *Handler {
	return &Handler{usecase: uc}
}

// List handles GET /documents.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	resp, err := h.usecase.List(r.Context())
	if err != nil {
		apperrors.HandleError(w, r, err)
		return
	}
	apperrors.WriteJSON(w, http.StatusOK, resp)
}
