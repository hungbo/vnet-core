package service

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestSettingsService_List(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewSettingsService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "system_settings" ORDER BY group_name, key`).
		WillReturnRows(sqlmock.NewRows([]string{"group_name", "key", "value"}).
			AddRow("general", "site_name", "VNET").
			AddRow("general", "timezone", "Asia/HCM").
			AddRow("pricing", "vat_rate", "10"))

	result, err := svc.List()
	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Len(t, result["general"], 2)
	assert.Len(t, result["pricing"], 1)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSettingsService_List_Empty(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewSettingsService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "system_settings" ORDER BY group_name, key`).
		WillReturnRows(sqlmock.NewRows([]string{"group_name", "key", "value"}))

	result, err := svc.List()
	require.NoError(t, err)
	assert.Len(t, result, 0)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSettingsService_GetByGroup_Found(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewSettingsService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "system_settings" WHERE group_name = \$1 ORDER BY key`).
		WithArgs("general").
		WillReturnRows(sqlmock.NewRows([]string{"group_name", "key", "value"}).
			AddRow("general", "site_name", "VNET").
			AddRow("general", "timezone", "Asia/HCM"))

	result, err := svc.GetByGroup("general")
	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Nhóm rỗng nay trả về danh sách rỗng, không còn là lỗi: đó là trạng thái lần
// đầu bình thường của một tab cài đặt chưa ai đụng tới.
func TestSettingsService_GetByGroup_NhomRongTraDanhSachRong(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewSettingsService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "system_settings" WHERE group_name = \$1 ORDER BY key`).
		WithArgs("nonexistent").
		WillReturnRows(sqlmock.NewRows([]string{"group_name", "key", "value"}))

	result, err := svc.GetByGroup("nonexistent")
	require.NoError(t, err)
	assert.Empty(t, result)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSettingsService_Update_CreateNew(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewSettingsService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "system_settings" WHERE group_name = \$1 AND key = \$2 ORDER BY "system_settings"."id" LIMIT \$3`).
		WithArgs("general", "new_key", 1).
		WillReturnError(gorm.ErrRecordNotFound)

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "system_settings"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(testUUID))
	mock.ExpectCommit()

	mock.ExpectQuery(`SELECT \* FROM "system_settings" WHERE group_name = \$1 ORDER BY key`).
		WithArgs("general").
		WillReturnRows(sqlmock.NewRows([]string{"group_name", "key", "value"}).
			AddRow("general", "new_key", "test_value"))

	result, err := svc.Update("general", map[string]interface{}{"new_key": "test_value"})
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSettingsService_Update_Existing(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewSettingsService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "system_settings" WHERE group_name = \$1 AND key = \$2 ORDER BY "system_settings"."id" LIMIT \$3`).
		WithArgs("general", "site_name", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "group_name", "key", "value"}).AddRow("s1", "general", "site_name", "VNET"))

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "system_settings" SET`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	mock.ExpectQuery(`SELECT \* FROM "system_settings" WHERE group_name = \$1 ORDER BY key`).
		WithArgs("general").
		WillReturnRows(sqlmock.NewRows([]string{"group_name", "key", "value"}).
			AddRow("general", "site_name", "VNET 2.0"))

	result, err := svc.Update("general", map[string]interface{}{"site_name": "VNET 2.0"})
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// The value column is jsonb, so a bare string was rejected by PostgreSQL and
// saving any text setting always failed.
func TestSettingsService_Update_StoresValidJSON(t *testing.T) {
	db, mock := newMockDB(t)
	svc := NewSettingsService(db, NewAuditService(db))

	mock.ExpectQuery(`SELECT \* FROM "system_settings" WHERE group_name = \$1 AND key = \$2 ORDER BY "system_settings"\."id" LIMIT \$3`).
		WithArgs("general", "store_name", 1).
		WillReturnError(gorm.ErrRecordNotFound)

	// Quoted: "Tiem net ABC" is valid JSON, Tiem net ABC is not.
	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "system_settings"`).
		WithArgs("general", "store_name", `"Tiem net ABC"`, "").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("s1"))
	mock.ExpectCommit()

	mock.ExpectQuery(`SELECT \* FROM "system_settings" WHERE group_name = \$1 ORDER BY key`).
		WithArgs("general").
		WillReturnRows(sqlmock.NewRows([]string{"group_name", "key", "value"}).
			AddRow("general", "store_name", `"Tiem net ABC"`))

	res, err := svc.Update("general", map[string]interface{}{"store_name": "Tiem net ABC"})

	require.NoError(t, err)
	require.Len(t, res, 1)
	// Read back as plain text, so a form shows the value rather than its quotes.
	assert.Equal(t, "Tiem net ABC", res[0].Value)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDecodeSettingValue(t *testing.T) {
	assert.Equal(t, "abc", decodeSettingValue(`"abc"`))
	assert.Equal(t, `{"values":[1,2]}`, decodeSettingValue(`{"values":[1,2]}`))
	assert.Equal(t, "123", decodeSettingValue(`123`))
}

// Giá trị trong cột jsonb được decodeSettingValue trả về dạng CHUỖI. Kiểm bằng
// truthiness thì "false" — một chuỗi khác rỗng — vẫn ra "bật", và công tắc ở
// trang quản trị sẽ không bao giờ tắt được gì.
func TestSettingBool(t *testing.T) {
	cases := []struct {
		name string
		m    map[string]string
		def  bool
		want bool
	}{
		{"thiếu khoá thì lấy mặc định", map[string]string{}, true, true},
		{"rỗng thì lấy mặc định", map[string]string{"k": ""}, true, true},
		{"rác thì lấy mặc định", map[string]string{"k": "bật"}, true, true},
		{`chuỗi "false" phải TẮT`, map[string]string{"k": "false"}, true, false},
		{`chuỗi "true" phải BẬT`, map[string]string{"k": "true"}, false, true},
		{"có khoảng trắng vẫn đọc được", map[string]string{"k": " false "}, true, false},
		{"chấp nhận 0/1", map[string]string{"k": "0"}, true, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, settingBool(c.m, "k", c.def))
		})
	}
}

// Chính sách máy trạm: thiếu cài đặt thì dùng mặc định an toàn, và con số vô lý
// phải bị ép về khoảng hợp lệ — sai ở đây là khoá hoặc khởi động lại oan cả
// phòng máy.
func TestClientPolicyFrom(t *testing.T) {
	macDinh := clientPolicyFrom(map[string]string{})
	assert.Equal(t, ClientPolicy{TamperAction: "restart", OfflineLockSeconds: 60, OfflineRebootSeconds: 300, IdleShutdownMinutes: 5}, macDinh)

	p := clientPolicyFrom(map[string]string{
		"tamper_action": "shutdown", "offline_lock_seconds": "90", "offline_reboot_seconds": "600", IdleShutdownKey: "15",
	})
	assert.Equal(t, ClientPolicy{TamperAction: "shutdown", OfflineLockSeconds: 90, OfflineRebootSeconds: 600, IdleShutdownMinutes: 15}, p)

	// Tự tắt máy khi không ai đăng nhập: 0 là tắt tính năng, số âm cũng vậy,
	// quá một ngày thì kẹp lại.
	assert.Equal(t, int64(0), clientPolicyFrom(map[string]string{IdleShutdownKey: "0"}).IdleShutdownMinutes)
	assert.Equal(t, int64(0), clientPolicyFrom(map[string]string{IdleShutdownKey: "-3"}).IdleShutdownMinutes)
	assert.Equal(t, int64(1440), clientPolicyFrom(map[string]string{IdleShutdownKey: "99999"}).IdleShutdownMinutes)

	// Giá trị lạ cho hành động thì về mặc định, không truyền nguyên xuống máy trạm.
	assert.Equal(t, "restart", clientPolicyFrom(map[string]string{"tamper_action": "format-c"}).TamperAction)

	// Khoá quá sớm: một nhịp tim rớt cũng đủ khoá máy.
	assert.Equal(t, int64(30), clientPolicyFrom(map[string]string{"offline_lock_seconds": "5"}).OfflineLockSeconds)

	// Khởi động lại phải cách bậc khoá ít nhất một phút.
	p = clientPolicyFrom(map[string]string{"offline_lock_seconds": "60", "offline_reboot_seconds": "70"})
	assert.Equal(t, int64(120), p.OfflineRebootSeconds)

	// 0 là không bao giờ khởi động lại.
	assert.Equal(t, int64(0), clientPolicyFrom(map[string]string{"offline_reboot_seconds": "0"}).OfflineRebootSeconds)
}

// Danh sách ứng dụng chặn đi xuống mọi máy trạm: nhân viên gõ thế nào cũng phải
// ra cùng một chuỗi gọn, và thứ không phải tên tiến trình thì bị bỏ.
func TestChuanHoaUngDungChan(t *testing.T) {
	assert.Equal(t, "", chuanHoaUngDungChan(""))
	assert.Equal(t, "chrome,steam", chuanHoaUngDungChan("Chrome.EXE\n steam, ,chrome\r\n"))
	assert.Equal(t, "leagueclient,valorant", chuanHoaUngDungChan("Valorant; LeagueClient.exe"))
	// Máy trạm không nhận dấu cách trong tên tiến trình, nên máy chủ cũng không.
	assert.Equal(t, "", chuanHoaUngDungChan("League of Legends"))
	// Đường dẫn và ký tự lệnh không phải tên tiến trình.
	assert.Equal(t, "zalo", chuanHoaUngDungChan("C:\\x\\a.exe, zalo, rm -rf /, a&b"))

	// Luôn có trong chính sách, kể cả rỗng: rỗng là bỏ chặn hết.
	assert.Equal(t, "zalo", clientPolicyFrom(map[string]string{BlockedAppsKey: "Zalo.exe"}).BlockedApps)
}

// Tên lối tắt đi xuống dịch vụ máy trạm để XOÁ FILE: thứ gì giống đường dẫn hay
// ký tự đại diện phải bị bỏ, thứ hợp lệ phải ra đúng một chuỗi gọn.
func TestChuanHoaLoiTatAn(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{"rỗng là tắt tính năng", "", ""},
		{"chỉ khoảng trắng và dấu phẩy", " , \n ,\r\n", ""},
		{"giữ chữ hoa và thứ tự gõ", "Game Menu,Teamfight Tactics", "Game Menu,Teamfight Tactics"},
		{"xuống dòng cũng là dấu ngăn", "Game Menu\r\nRiot Client\n", "Game Menu,Riot Client"},
		{"bỏ đuôi .lnk .url", "Game Menu.lnk, Steam.URL, Riot Client.Lnk", "Game Menu,Steam,Riot Client"},
		{"đuôi bỏ hết, chuẩn hoá lại không đổi", "x.lnk.lnk", "x"},
		{"đuôi một mình thì bỏ", ".lnk, .url, lnk", "lnk"},
		{"tên có đuôi khác giữ nguyên", "Setup.exe, My.App v2", "Setup.exe,My.App v2"},
		{"trùng khác hoa thường chỉ lấy lần đầu", "Game Menu, game menu, GAME MENU.lnk", "Game Menu"},
		{"dấu cách thừa quanh tên", "  Game Menu  ,  Riot Client .lnk ", "Game Menu,Riot Client"},
		{"tiếng Việt có dấu", "Trò chơi.lnk", "Trò chơi"},
		{"leo thư mục ..\\", `..\evil`, ""},
		{"leo thư mục ../", "../evil", ""},
		{"có dấu gạch chéo", "a/b", ""},
		{"có dấu gạch ngược", `a\b`, ""},
		{"ký tự ổ đĩa", "C:x", ""},
		{"đường dẫn tuyệt đối", `C:\Users\Public\Desktop\Game Menu.lnk`, ""},
		{"luồng dữ liệu NTFS", "x:stream", ""},
		{"ký tự đại diện", "*, a?b, <x>, \"q\", a|b", ""},
		{"một và hai chấm", ".., .", ""},
		{"hai chấm sau khi bỏ đuôi", "...lnk, ..lnk", ""},
		{"ký tự NUL", "a\x00b", ""},
		{"ký tự điều khiển", "a\tb, c\x1fd, e\x7ff", ""},
		{"tên xấu không làm hỏng tên tốt", `..\evil, Game Menu, a/b, Riot Client`, "Game Menu,Riot Client"},
		{"đủ 100 ký tự thì giữ", strings.Repeat("a", 100), strings.Repeat("a", 100)},
		{"quá 100 ký tự thì bỏ chứ không cắt", strings.Repeat("a", 101), ""},
		{"100 ký tự có dấu tính theo rune", strings.Repeat("ế", 100), strings.Repeat("ế", 100)},
		{"101 ký tự có dấu", strings.Repeat("ế", 101), ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, chuanHoaLoiTatAn(c.raw))
		})
	}

	// 60 tên khác nhau → chỉ giữ 50 tên đầu, theo thứ tự gõ.
	var names []string
	for i := 0; i < 60; i++ {
		names = append(names, fmt.Sprintf("App %d", i))
	}
	got := strings.Split(chuanHoaLoiTatAn(strings.Join(names, ",")), ",")
	assert.Len(t, got, 50)
	assert.Equal(t, "App 0", got[0])
	assert.Equal(t, "App 49", got[49])

	// Tên bị bỏ không chiếm chỗ trong 50 suất.
	names = names[:0]
	for i := 0; i < 10; i++ {
		names = append(names, "bad/"+strconv.Itoa(i))
	}
	for i := 0; i < 55; i++ {
		names = append(names, "ok "+strconv.Itoa(i))
	}
	got = strings.Split(chuanHoaLoiTatAn(strings.Join(names, ",")), ",")
	assert.Len(t, got, 50)
	assert.Equal(t, "ok 0", got[0])

	// Luôn có trong chính sách, kể cả rỗng: rỗng là tắt tính năng. Chưa đặt thì
	// mặc định rỗng.
	assert.Equal(t, "Game Menu", clientPolicyFrom(map[string]string{HiddenShortcutsKey: "Game Menu.lnk"}).HiddenShortcuts)
	assert.Equal(t, "", clientPolicyFrom(map[string]string{}).HiddenShortcuts)

	raw, err := json.Marshal(clientPolicyFrom(map[string]string{}))
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"hidden_shortcuts":""`)
}
