package history

import (
	"context"
	"net/url"
	"strconv"
	"time"
)

type ListParams struct {
	Limit  int
	Offset int
	From   *time.Time
	To     *time.Time
}

type ListResponse struct {
	Items []Item `json:"items"`
	Total int    `json:"total"`
}

func (s *Service) List(ctx context.Context, userID string, params ListParams) (ListResponse, error) {
	items, total, err := s.repo.ListByUser(ctx, userID, params.Limit, params.Offset, params.From, params.To)
	if err != nil {
		return ListResponse{}, err
	}
	return ListResponse{Items: items, Total: total}, nil
}

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Calendar(ctx context.Context, userID string, year, month int) ([]CalendarDay, error) {
	return s.repo.CalendarByMonth(ctx, userID, year, month)
}

func ParseListParams(q url.Values) ListParams {
	params := ListParams{Limit: 50, Offset: 0}
	if v := q.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 100 {
			params.Limit = n
		}
	}
	if v := q.Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			params.Offset = n
		}
	}
	if v := q.Get("from"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			params.From = &t
		}
	}
	if v := q.Get("to"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			params.To = &t
		}
	}
	return params
}
