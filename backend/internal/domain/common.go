package domain

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
)

type JSONMap map[string]interface{}

func (j *JSONMap) Scan(value interface{}) error {
	if value == nil {
		*j = make(JSONMap)
		return nil
	}
	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return errors.New("unsupported type for JSONMap scan")
	}
	return json.Unmarshal(bytes, j)
}

func (j JSONMap) Value() (driver.Value, error) {
	if j == nil {
		return json.Marshal(make(map[string]interface{}))
	}
	return json.Marshal(j)
}

type Pagination struct {
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
}

func DefaultPagination() Pagination {
	return Pagination{Page: 1, PageSize: 20}
}

func (p Pagination) Offset() int {
	return (p.Page - 1) * p.PageSize
}

func (p Pagination) Limit() int {
	return p.PageSize
}

type PaginatedResult[T any] struct {
	Data       []T   `json:"data"`
	Total      int64 `json:"total"`
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	TotalPages int   `json:"total_pages"`
}

func NewPaginatedResult[T any](data []T, total int64, pagination Pagination) PaginatedResult[T] {
	totalPages := int(total) / pagination.PageSize
	if int(total)%pagination.PageSize > 0 {
		totalPages++
	}
	return PaginatedResult[T]{
		Data:       data,
		Total:      total,
		Page:       pagination.Page,
		PageSize:   pagination.PageSize,
		TotalPages: totalPages,
	}
}
