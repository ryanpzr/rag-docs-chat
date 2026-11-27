package ingestdocument

import (
	"errors"
	"io"
	"net/http"

	"github.com/ryanpzr/rag-docs-chat/pkg/apperrors"
)

// multipartOverhead leaves room for the multipart boundaries and headers on
// top of the file itself.
const multipartOverhead = 1 << 20

type Handler struct {
	usecase        Usecase
	maxUploadBytes int64
}

func NewHandler(uc Usecase, maxUploadBytes int64) *Handler {
	return &Handler{usecase: uc, maxUploadBytes: maxUploadBytes}
}

// Ingest handles POST /documents with a multipart form whose "file" field
// holds a .txt or .md file.
func (h *Handler) Ingest(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, h.maxUploadBytes+multipartOverhead)

	file, header, err := r.FormFile("file")
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			apperrors.HandleError(w, r, apperrors.NewPayloadTooLarge("file is too large"))
			return
		}
		apperrors.HandleError(w, r, apperrors.NewBadRequest(`expected a multipart form with a "file" field`))
		return
	}
	defer file.Close()

	// Read one byte past the limit so the usecase can tell the file is too big.
	content, err := io.ReadAll(io.LimitReader(file, h.maxUploadBytes+1))
	if err != nil {
		apperrors.HandleError(w, r, apperrors.NewBadRequest("failed to read uploaded file"))
		return
	}

	resp, err := h.usecase.Ingest(r.Context(), IngestInput{Filename: header.Filename, Content: content})
	if err != nil {
		apperrors.HandleError(w, r, err)
		return
	}
	apperrors.WriteJSON(w, http.StatusCreated, resp)
}
