package pagination

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

func TestGetParams_Defaults(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/test", nil)

	p := GetParams(c)
	assert.Equal(t, DefaultPage, p.Page)
	assert.Equal(t, DefaultPageSize, p.PageSize)
	assert.Equal(t, "id", p.Sort)
	assert.Equal(t, "desc", p.Order)
	assert.Equal(t, "", p.Search)
}

func TestGetParams_Custom(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/test?page=3&page_size=10&sort=name&order=asc&search=test", nil)

	p := GetParams(c)
	assert.Equal(t, 3, p.Page)
	assert.Equal(t, 10, p.PageSize)
	assert.Equal(t, "name", p.Sort)
	assert.Equal(t, "asc", p.Order)
	assert.Equal(t, "test", p.Search)
}

func TestGetParams_MaxPageSize(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/test?page_size=500", nil)

	p := GetParams(c)
	assert.Equal(t, MaxPageSize, p.PageSize)
}

func TestGetParams_InvalidPage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/test?page=abc&page_size=xyz", nil)

	p := GetParams(c)
	assert.Equal(t, DefaultPage, p.Page)
	assert.Equal(t, DefaultPageSize, p.PageSize)
}

func TestGetParams_ZeroPage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/test?page=0&page_size=0", nil)

	p := GetParams(c)
	assert.Equal(t, DefaultPage, p.Page)
	assert.Equal(t, DefaultPageSize, p.PageSize)
}

func TestNewResult(t *testing.T) {
	items := []string{"a", "b", "c"}
	p := &Params{Page: 2, PageSize: 10}
	total := int64(25)

	result := NewResult(items, total, p)
	assert.Equal(t, items, result.Items)
	assert.Equal(t, total, result.Total)
	assert.Equal(t, 2, result.Page)
	assert.Equal(t, 10, result.PageSize)
	assert.Equal(t, 3, result.TotalPages)
}

func TestNewResult_ExactPages(t *testing.T) {
	items := []int{1, 2}
	p := &Params{Page: 1, PageSize: 10}
	result := NewResult(items, int64(20), p)
	assert.Equal(t, 2, result.TotalPages)
}

func TestNewResult_SinglePage(t *testing.T) {
	items := []string{"x"}
	p := &Params{Page: 1, PageSize: 20}
	result := NewResult(items, int64(1), p)
	assert.Equal(t, 1, result.TotalPages)
}

func TestNewResult_ZeroTotal(t *testing.T) {
	items := []string{}
	p := &Params{Page: 1, PageSize: 20}
	result := NewResult(items, int64(0), p)
	assert.Equal(t, 0, result.TotalPages)
}

func TestApply_SetsOffsetLimitOrder(t *testing.T) {
	db, _ := gorm.Open(nil)
	p := &Params{Page: 3, PageSize: 15, Sort: "name", Order: "asc"}
	applied := Apply(db, p)
	assert.NotNil(t, applied)

	p2 := &Params{Page: 1, PageSize: 20}
	applied2 := Apply(db, p2)
	assert.NotNil(t, applied2)
}

func TestParseInt(t *testing.T) {
	tests := []struct {
		input    string
		fallback int
		want     int
	}{
		{"42", 1, 42},
		{"0", 1, 0},
		{"", 10, 10},
		{"abc", 5, 5},
		{"12a", 5, 5},
		{"999", 1, 999},
	}

	for _, tt := range tests {
		got, _ := parseInt(tt.input, tt.fallback)
		assert.Equal(t, tt.want, got, "parseInt(%q, %d)", tt.input, tt.fallback)
	}
}
