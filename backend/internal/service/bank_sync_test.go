package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vnet/core/internal/model"
)

// Giao dịch thật đầu tiên qua ThueAPI, 06/11/2026.
const noiDungThat = "VNBT3YHK- Ma GD ACSP/ 3i361321"

func TestQRCodesIn(t *testing.T) {
	assert.Equal(t, []string{"VNBT3YHK"}, qrCodesIn(noiDungThat))
	assert.Equal(t, []string{"VN7K2Q9M"}, qrCodesIn("chuyen tien VN7K 2Q9M"))
	assert.Equal(t, []string{"VN7K2Q9M"}, qrCodesIn("APP123.vn7k2q9m.CT"))
	// 0/O, 1/I/L không có trong bảng chữ của mã.
	assert.Empty(t, qrCodesIn("VN012345 VNOIL123"))
}

func TestNotificationFrom(t *testing.T) {
	now := time.Date(2026, 11, 6, 7, 47, 35, 0, time.UTC)
	s := &QRPayService{now: func() time.Time { return now }}
	n := s.notificationFrom(hubBankEvent{
		TxID: "149883", Bank: "MBB", BankRef: "FT26310812890", Amount: 2000, Content: noiDungThat,
		Direction: "IN", TransactionDate: "2026-11-06 07:47:34",
	})
	assert.Equal(t, "thueapi", n.Package)
	assert.Equal(t, "mb", n.Bank)
	assert.EqualValues(t, 2000, n.Amount)
	assert.True(t, n.Incoming)
	assert.Equal(t, "FT26310812890", n.BankRef)
	assert.Equal(t, noiDungThat, n.Content)
	assert.Equal(t, model.BankNotifUnmatched, n.Status)
	assert.Equal(t, bankEventHash("149883"), n.Hash)
	assert.NotEqual(t, bankEventHash("149884"), n.Hash)
	require.NotNil(t, n.PostedAt)
	assert.True(t, n.PostedAt.Equal(time.Date(2026, 11, 6, 0, 47, 34, 0, time.UTC)), "giờ giao dịch là giờ Việt Nam")
}

func TestIngestBankEvent(t *testing.T) {
	const daCo = `SELECT count\(\*\) FROM "bank_notifications" WHERE \(package <> \$1 AND amount = \$2 AND content = \$3\)`
	const luu = `INSERT INTO "bank_notifications" .* ON CONFLICT \("hash"\) DO NOTHING`
	e := hubBankEvent{TxID: "1", Bank: "MBB", Amount: 2000, Content: "chuyen khoan", Direction: "IN"}

	t.Run("tien ra bo qua", func(t *testing.T) {
		db, mock := newMockDB(t)
		s := NewQRPayService(db, nil, nil, nil, nil)
		out := e
		out.Direction = "OUT"
		require.NoError(t, s.ingestBankEvent(out))
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("khoan da ghi tu dien thoai khong ghi lai", func(t *testing.T) {
		db, mock := newMockDB(t)
		s := NewQRPayService(db, nil, nil, nil, nil)
		mock.ExpectQuery(daCo).WithArgs("thueapi", int64(2000), "chuyen khoan").
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
		require.NoError(t, s.ingestBankEvent(e))
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("doc lai cung giao dich khong khop lan hai", func(t *testing.T) {
		db, mock := newMockDB(t)
		s := NewQRPayService(db, nil, nil, nil, nil)
		mock.ExpectQuery(daCo).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
		mock.ExpectBegin()
		mock.ExpectQuery(luu).WillReturnRows(sqlmock.NewRows([]string{"id"}))
		mock.ExpectCommit()
		require.NoError(t, s.ingestBankEvent(e))
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("khong co ma thi nam o Tien vao", func(t *testing.T) {
		db, mock := newMockDB(t)
		s := NewQRPayService(db, nil, nil, nil, nil)
		mock.ExpectQuery(daCo).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
		mock.ExpectBegin()
		mock.ExpectQuery(luu).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("n1"))
		mock.ExpectCommit()
		require.NoError(t, s.ingestBankEvent(e))
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestFetchBankEvents(t *testing.T) {
	var gotKey, gotPath string
	status := http.StatusOK
	hub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey, gotPath = r.Header.Get(GameCatalogKeyHeader), r.URL.String()
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"events":[{"seq":7,"tx_id":"149883","bank":"MBB","amount":2000,"content":"x","direction":"IN"}]}`))
	}))
	defer hub.Close()

	s := (&QRPayService{}).WithHubAccess(func() (string, string) { return hub.URL, "khoa-quan" })
	got, err := s.fetchBankEvents(context.Background(), 6)
	require.NoError(t, err)
	assert.Equal(t, "khoa-quan", gotKey)
	assert.Equal(t, "/api/bank-events?after=6", gotPath)
	require.Len(t, got, 1)
	assert.EqualValues(t, 7, got[0].Seq)
	assert.EqualValues(t, 2000, got[0].Amount)

	status = http.StatusNotFound
	_, err = s.fetchBankEvents(context.Background(), 6)
	assert.ErrorContains(t, err, "404")

	chuaNoi := (&QRPayService{}).WithHubAccess(func() (string, string) { return "", "" })
	_, err = chuaNoi.fetchBankEvents(context.Background(), 0)
	assert.ErrorContains(t, err, "chưa nối Hub")
}
