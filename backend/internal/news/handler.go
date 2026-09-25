package news

import (
	"net/http"
	"slices"
	"strconv"

	"github.com/fmolinar/arium/backend/pkg/response"
)

const (
	defaultLimit = 20
	maxLimit     = 100
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// List serves GET /api/v1/news?tag=<slug>&limit=<n>&cursor=<nextCursor>.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	q, msg := parseListQuery(r)
	if msg != "" {
		response.Error(w, http.StatusBadRequest, msg)
		return
	}

	result, err := h.service.List(r.Context(), q)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to list news")
		return
	}

	response.JSON(w, http.StatusOK, result)
}

// parseListQuery reads the list parameters, returning an error message for
// the client when one is invalid.
func parseListQuery(r *http.Request) (ListQuery, string) {
	params := r.URL.Query()
	q := ListQuery{Tag: params.Get("tag"), Limit: defaultLimit}

	if q.Tag != "" && !slices.Contains(Topics, q.Tag) {
		return q, "tag must be one of devops, sre, gitops, devsecops"
	}

	if v := params.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > maxLimit {
			return q, "limit must be between 1 and 100"
		}
		q.Limit = n
	}

	if v := params.Get("cursor"); v != "" {
		c, err := DecodeCursor(v)
		if err != nil {
			return q, "invalid cursor"
		}
		q.After = &c
	}

	return q, ""
}
