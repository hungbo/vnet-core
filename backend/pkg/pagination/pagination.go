package pagination

import (
	"fmt"
	"math"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Params struct {
	Page      int    `form:"page"`
	PageSize  int    `form:"page_size"`
	Sort      string `form:"sort"`
	Order     string `form:"order"`
	Search    string `form:"search"`
	OrderType string `form:"order_type"`
}

type Result struct {
	Items      interface{} `json:"items"`
	Total      int64       `json:"total"`
	Page       int         `json:"page"`
	PageSize   int         `json:"page_size"`
	TotalPages int         `json:"total_pages"`
}

const (
	DefaultPage     = 1
	DefaultPageSize = 20
	MaxPageSize     = 100

	DefaultSort  = "id"
	DefaultOrder = "desc"
)

// GORM interpolates Order(string) verbatim, so the sort column and direction
// arrive in the SQL as written. Both come straight from the query string,
// which makes this the injection boundary for every paginated list endpoint.
var sortColumnPattern = regexp.MustCompile(`^[a-z_][a-z0-9_]*(\.[a-z_][a-z0-9_]*)?$`)

// SafeSort returns sort if it is a plain (optionally table-qualified) column
// identifier, and fallback otherwise.
func SafeSort(sort, fallback string) string {
	if sortColumnPattern.MatchString(sort) {
		return sort
	}
	return fallback
}

// SafeOrder returns "asc" or "desc"; anything else falls back to descending.
func SafeOrder(order string) string {
	switch strings.ToLower(strings.TrimSpace(order)) {
	case "asc":
		return "asc"
	default:
		return "desc"
	}
}

func GetParams(c *gin.Context) *Params {
	p := &Params{
		Page:      DefaultPage,
		PageSize:  DefaultPageSize,
		Sort:      SafeSort(c.DefaultQuery("sort", DefaultSort), DefaultSort),
		Order:     SafeOrder(c.DefaultQuery("order", DefaultOrder)),
		Search:    c.Query("search"),
		OrderType: c.Query("order_type"),
	}

	if page, err := parseInt(c.Query("page"), DefaultPage); err == nil && page > 0 {
		p.Page = page
	}
	if pageSize, err := parseInt(c.Query("page_size"), DefaultPageSize); err == nil && pageSize > 0 {
		if pageSize > MaxPageSize {
			pageSize = MaxPageSize
		}
		p.PageSize = pageSize
	}

	return p
}

func Apply(db *gorm.DB, p *Params) *gorm.DB {
	offset := (p.Page - 1) * p.PageSize
	// Re-validated here rather than only in GetParams: callers also build
	// Params structs directly.
	orderClause := SafeSort(p.Sort, DefaultSort) + " " + SafeOrder(p.Order)
	return db.Offset(offset).Limit(p.PageSize).Order(orderClause)
}

func NewResult(items interface{}, total int64, p *Params) *Result {
	totalPages := int(math.Ceil(float64(total) / float64(p.PageSize)))
	return &Result{
		Items:      items,
		Total:      total,
		Page:       p.Page,
		PageSize:   p.PageSize,
		TotalPages: totalPages,
	}
}

func parseInt(s string, fallback int) (int, error) {
	if s == "" {
		return fallback, nil
	}
	result := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return fallback, fmt.Errorf("not a number")
		}
		result = result*10 + int(c-'0')
	}
	return result, nil
}
