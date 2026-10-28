package service

import (
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vnet/core/internal/model"
)

// Combo giới hạn theo nhóm máy: gói chỉ mở được máy thuộc các nhóm đã chọn,
// không chọn nhóm nào thì dùng được mọi máy.

func TestKiemTraNhomMayCuaCombo(t *testing.T) {
	db, mock := newMockDB(t)
	vip, thuong := "g-vip", "g-thuong"

	// Gói không giới hạn: máy nào cũng được, không cần đọc gì thêm.
	require.NoError(t, kiemTraNhomMayCuaCombo(db, &model.Combo{Name: "sang"}, &model.Machine{MachineCode: "MAY01"}))

	goiVip := &model.Combo{Name: "sangvip", MachineGroupIDs: model.StringArray{vip}}
	require.NoError(t, kiemTraNhomMayCuaCombo(db, goiVip, &model.Machine{MachineCode: "VIP01", GroupID: &vip}))

	// Sai nhóm, và máy chưa gán nhóm, đều bị từ chối với tên nhóm hợp lệ.
	for _, may := range []*model.Machine{{MachineCode: "MAY01", GroupID: &thuong}, {MachineCode: "MAY02"}} {
		mock.ExpectQuery(`SELECT "name" FROM "machine_groups" WHERE id IN \(\$1\) AND "machine_groups"\."deleted_at" IS NULL ORDER BY sort_order`).
			WithArgs(vip).
			WillReturnRows(sqlmock.NewRows([]string{"name"}).AddRow("VIP"))
		err := kiemTraNhomMayCuaCombo(db, goiVip, may)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "gói sangvip chỉ dùng được ở nhóm máy VIP")
		assert.Contains(t, err.Error(), may.MachineCode)
	}
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Kích hoạt gói VIP lên máy thường bị chặn trước khi mở phiên.
func TestComboService_Activate_SaiNhomMay(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewComboService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "combo_purchases" WHERE id = \$1`).
		WithArgs("p1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "combo_id", "member_id", "activated", "remaining_minutes"}).
			AddRow("p1", "c1", "mem1", false, 180))
	mock.ExpectQuery(`SELECT \* FROM "combos" WHERE id = \$1`).
		WithArgs("c1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "type", "total_minutes", "machine_group_ids"}).
			AddRow("c1", "sangvip", "prepaid", 180, `["g-vip"]`))
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT \* FROM "combo_purchases" WHERE id = \$1 .*FOR UPDATE`).
		WithArgs("p1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "combo_id", "member_id", "activated", "remaining_minutes"}).
			AddRow("p1", "c1", "mem1", false, 180))
	mock.ExpectQuery(`SELECT \* FROM "machines" WHERE \(id = \$1 AND is_active = \$2\) .*FOR UPDATE`).
		WithArgs("m1", true, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "machine_code", "status", "group_id"}).
			AddRow("m1", "MAY01", "available", "g-thuong"))
	mock.ExpectQuery(`SELECT "name" FROM "machine_groups"`).
		WillReturnRows(sqlmock.NewRows([]string{"name"}).AddRow("VIP"))
	mock.ExpectRollback()

	_, err := svc.Activate("p1", &ActivateComboRequest{MachineID: "m1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "chỉ dùng được ở nhóm máy VIP")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestComboService_Create_NhomMay(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewComboService(db, NewAuditService(db))

	// Bỏ trùng và id rỗng trước khi kiểm.
	mock.ExpectQuery(`SELECT count\(\*\) FROM "machine_groups" WHERE id IN \(\$1\) AND "machine_groups"\."deleted_at" IS NULL`).
		WithArgs("g-vip").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "combos" .*"machine_group_ids"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("c1"))
	mock.ExpectCommit()
	mock.ExpectQuery(`SELECT "id","name" FROM "machine_groups" WHERE id IN \(\$1\)`).
		WithArgs("g-vip").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow("g-vip", "VIP"))

	result, err := svc.Create(&CreateComboRequest{
		Name: "sangvip", Type: "prepaid", TotalMinutes: 180, MachineGroupIDs: []string{"g-vip", "", "g-vip"},
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"g-vip"}, result.MachineGroupIDs)
	assert.Equal(t, []string{"VIP"}, result.MachineGroupNames)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestComboService_Create_NhomMayKhongTonTai(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewComboService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT count\(\*\) FROM "machine_groups"`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	_, err := svc.Create(&CreateComboRequest{Name: "sangvip", Type: "prepaid", TotalMinutes: 180, MachineGroupIDs: []string{"g-xoa"}})
	assert.EqualError(t, err, "không tìm thấy nhóm máy đã chọn")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Gửi [] là bỏ giới hạn: cột về NULL, gói lại dùng được mọi máy.
func TestComboService_Update_BoNhomMay(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewComboService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "combos" WHERE id = \$1`).
		WithArgs("c1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "machine_group_ids"}).AddRow("c1", "sangvip", `["g-vip"]`))
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "combos" SET "machine_group_ids"=\$1 WHERE`).
		WithArgs(nil, "c1").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	mock.ExpectQuery(`SELECT \* FROM "combos" WHERE id = \$1`).
		WithArgs("c1", "c1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "machine_group_ids"}).AddRow("c1", "sangvip", nil))

	result, err := svc.Update("c1", &UpdateComboRequest{MachineGroupIDs: []string{}})
	require.NoError(t, err)
	assert.Equal(t, []string{}, result.MachineGroupIDs)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Đổi máy sang máy ngoài nhóm của gói phải bị chặn TRƯỚC khi phiên cũ bị đóng:
// không có câu UPDATE nào chạy, khách vẫn ngồi máy cũ.
func TestSessionService_SwitchMachine_SaiNhomMay_GiuPhienCu(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewSessionService(db, nil, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "machines" WHERE \(id = \$1 AND is_active = \$2\)`).
		WithArgs("may01", true, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "machine_code", "status", "group_id"}).
			AddRow("may01", "MAY01", "available", "g-thuong"))
	mock.ExpectQuery(`SELECT \* FROM "machine_sessions" WHERE id = \$1`).
		WithArgs("s1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "machine_id", "combo_id", "is_active"}).
			AddRow("s1", "vip01", "p1", true))
	mock.ExpectQuery(`SELECT \* FROM "combo_purchases" WHERE id = \$1`).
		WithArgs("p1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "combo_id"}).AddRow("p1", "c1"))
	mock.ExpectQuery(`SELECT \* FROM "combos" WHERE id = \$1`).
		WithArgs("c1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "machine_group_ids"}).AddRow("c1", "sangvip", `["g-vip"]`))
	mock.ExpectQuery(`SELECT "name" FROM "machine_groups"`).
		WillReturnRows(sqlmock.NewRows([]string{"name"}).AddRow("VIP"))

	_, err := svc.SwitchMachine("s1", "may01")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "chỉ dùng được ở nhóm máy VIP")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Xoá nhóm mà combo còn giới hạn ở đó thì gói không mở được máy nào: phải chặn.
func TestMachineService_DeleteGroup_TuChoiKhiComboConDung(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewMachineService(db, nil, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "machine_groups" WHERE id = \$1`).
		WithArgs("g-vip", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow("g-vip", "VIP"))
	for _, bang := range []string{"machines", "machine_prices", "time_based_pricings", "website_rule_mappings"} {
		mock.ExpectQuery(`SELECT count\(\*\) FROM "` + bang + `"`).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	}
	mock.ExpectQuery(`SELECT count\(\*\) FROM "combos" WHERE machine_group_ids @> \$1::jsonb AND "combos"\."deleted_at" IS NULL`).
		WithArgs(`["g-vip"]`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))

	err := svc.DeleteGroup("g-vip")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrRangBuoc))
	assert.Contains(t, err.Error(), "còn 2 combo giới hạn ở nhóm máy này")
	assert.NoError(t, mock.ExpectationsWereMet())
}
