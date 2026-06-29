package service

import (
	"database/sql/driver"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func newMockDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()

	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)

	db, err := gorm.Open(postgres.New(postgres.Config{
		Conn: mockDB,
	}), &gorm.Config{})
	require.NoError(t, err)

	return db, mock
}

type anyTime struct{}

func (a anyTime) Match(v driver.Value) bool {
	_, ok := v.(time.Time)
	return ok
}

type anyUUID struct{}

func (a anyUUID) Match(v driver.Value) bool {
	s, ok := v.(string)
	if !ok {
		return false
	}
	return len(s) == 36
}

func mustParseTime(layout, value string) time.Time {
	t, err := time.Parse(layout, value)
	if err != nil {
		panic(err)
	}
	return t
}

func strPtr(s string) *string {
	return &s
}

func intPtr(i int) *int {
	return &i
}

var (
	testUUID     = "550e8400-e29b-41d4-a716-446655440000"
	testStoreID  = "660e8400-e29b-41d4-a716-446655440001"
	testUserID   = "770e8400-e29b-41d4-a716-446655440002"
	testNow      = time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)
	testNowStr   = "2025-06-15T10:30:00Z"
	deletedAtPtr = &time.Time{}
)

func init() {
	*deletedAtPtr = time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)
}

func newSQLMockRows(columns []string, rows ...[]driver.Value) *sqlmock.Rows {
	r := sqlmock.NewRows(columns)
	for _, row := range rows {
		r.AddRow(row...)
	}
	return r
}
