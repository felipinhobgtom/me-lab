package http

import (
	"net/http"
	"strconv"
)

const (
	defaultPage     = 1
	defaultPageSize = 20
	maxPageSize     = 100
)

// Pagination holds parsed pagination parameters.
type Pagination struct {
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
}

// Offset returns the SQL OFFSET for the current page.
func (p Pagination) Offset() int {
	return (p.Page - 1) * p.PageSize
}

// PaginationResponse is included in list responses.
type PaginationResponse struct {
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
	Total    int `json:"total"`
}

// PaginatedResponse wraps a data slice with pagination metadata.
type PaginatedResponse struct {
	Data       any                `json:"data"`
	Pagination PaginationResponse `json:"pagination"`
}

// ParsePagination extracts page and page_size query parameters with defaults
// and bounds checking.
func ParsePagination(r *http.Request) Pagination {
	page := parseIntParam(r, "page", defaultPage)
	pageSize := parseIntParam(r, "page_size", defaultPageSize)

	if page < 1 {
		page = defaultPage
	}
	if pageSize < 1 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}

	return Pagination{
		Page:     page,
		PageSize: pageSize,
	}
}

// NewPaginatedResponse constructs a paginated envelope.
func NewPaginatedResponse(data any, page, pageSize, total int) PaginatedResponse {
	return PaginatedResponse{
		Data: data,
		Pagination: PaginationResponse{
			Page:     page,
			PageSize: pageSize,
			Total:    total,
		},
	}
}

// parseIntParam reads an integer query parameter, returning def if absent or invalid.
func parseIntParam(r *http.Request, key string, def int) int {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return def
	}
	val, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	return val
}
