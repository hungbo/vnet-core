package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// Danh sách này quyết định tệp nào bị xoá khỏi desktop của khách, nên tên nào
// không phải tên tệp trơn thì bỏ hẳn chứ không sửa lại cho "hợp lệ".
func TestChuanHoaLoiTat(t *testing.T) {
	cases := []struct{ ten, vao, ra string }{
		{"rỗng", "", ""},
		{"chỉ khoảng trắng và dấu phẩy", " , ,\n", ""},
		{"bỏ đuôi .lnk/.url, không phân biệt hoa thường", "Game Menu.LNK, Riot Client.url", "Game Menu,Riot Client"},
		{"bỏ trùng không phân biệt hoa thường, giữ bản đầu", "Game Menu.lnk, game menu,GAME MENU.url", "Game Menu"},
		{"xuống dòng cũng là dấu ngăn", "A\r\nB\nC", "A,B,C"},
		{"giữ thứ tự và chữ hoa", "Teamfight Tactics,Game Menu", "Teamfight Tactics,Game Menu"},
		{"tên có dấu tiếng Việt", "Trò chơi Ván Cờ", "Trò chơi Ván Cờ"},
		{"cắt khoảng trắng quanh tên và trước đuôi", "  Game Menu .lnk ", "Game Menu"},
		{"dấu gạch chéo", "a/b,ok", "ok"},
		{"dấu gạch ngược", `..\evil,ok`, "ok"},
		{"ổ đĩa", `C:\x,ok`, "ok"},
		{"ký tự cấm của Windows", `a*b,a?b,a<b,a>b,a"b,a|b,ok`, "ok"},
		{"ký tự điều khiển", "a\x00b,a\tb,a\x7fb,ok", "ok"},
		{"một và hai chấm", ".,..,ok", "ok"},
		{"chỉ có đuôi", ".lnk,.URL,ok", "ok"},
		{"đã chuẩn thì giữ nguyên", "A,B", "A,B"},
	}
	for _, c := range cases {
		t.Run(c.ten, func(t *testing.T) {
			got := chuanHoaLoiTat(c.vao)
			if got != c.ra {
				t.Errorf("chuanHoaLoiTat(%q) = %q, mong %q", c.vao, got, c.ra)
			}
			if again := chuanHoaLoiTat(got); again != got {
				t.Errorf("không idempotent: %q → %q", got, again)
			}
		})
	}
}

func TestChuanHoaLoiTat_GioiHan(t *testing.T) {
	// Đúng 100 ký tự thì giữ; 101 thì bỏ (cắt đi là thành tên khác, có thể xoá nhầm).
	ok := strings.Repeat("é", maxShortcutName)
	if got := chuanHoaLoiTat(ok + "," + ok + "x"); got != ok {
		t.Errorf("tên dài đúng giới hạn: nhận %q", got)
	}

	var names []string
	for i := 0; i < maxShortcutNames+10; i++ {
		names = append(names, fmt.Sprintf("lối tắt %02d", i))
	}
	got := strings.Split(chuanHoaLoiTat(strings.Join(names, ",")), ",")
	if len(got) != maxShortcutNames {
		t.Fatalf("%d tên, mong tối đa %d", len(got), maxShortcutNames)
	}
	if got[0] != names[0] || got[maxShortcutNames-1] != names[maxShortcutNames-1] {
		t.Errorf("phải giữ %d tên đầu theo thứ tự", maxShortcutNames)
	}
}

func TestParseHiddenShortcuts(t *testing.T) {
	if got := parseHiddenShortcuts(""); len(got) != 0 {
		t.Errorf("rỗng mà có %v", got)
	}
	got := parseHiddenShortcuts("Game Menu.lnk, Teamfight Tactics,../x")
	want := map[string]bool{"game menu": true, "teamfight tactics": true}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("nhận %v, mong %v", got, want)
	}
}

func TestLaLoiTatCanXoa(t *testing.T) {
	hidden := parseHiddenShortcuts("Game Menu,Riot Client,VNET")
	cases := []struct {
		file string
		xoa  bool
	}{
		{"Game Menu.lnk", true},
		{"game menu.LNK", true}, // hoa thường khác
		{"GAME MENU.lnk", true},
		{"Riot Client.url", true}, // lối tắt Internet
		{"Game Menu.txt", false},  // đuôi khác là của khách
		{"Game Menu.lnk.txt", false},
		{"Game Menu", false}, // không đuôi
		{"Game Menu 2.lnk", false},
		{"Other.lnk", false},
		{"VNET.lnk", false}, // được bảo vệ dù có trong danh sách
		{"vnet.LNK", false},
		{"VNET.url", false},
		{".lnk", false},
	}
	for _, c := range cases {
		if got := laLoiTatCanXoa(c.file, hidden); got != c.xoa {
			t.Errorf("laLoiTatCanXoa(%q) = %v, mong %v", c.file, got, c.xoa)
		}
	}
	if laLoiTatCanXoa("Game Menu.lnk", nil) || laLoiTatCanXoa("Game Menu.lnk", map[string]bool{}) {
		t.Error("danh sách rỗng mà vẫn xoá")
	}
}

func ghiTep(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func tonTai(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

func tenTep(paths []string) []string {
	var out []string
	for _, p := range paths {
		out = append(out, filepath.Base(p))
	}
	sort.Strings(out)
	return out
}

func TestShortcutsToDelete(t *testing.T) {
	dir := t.TempDir()
	ngoai := t.TempDir() // thư mục "bên ngoài desktop"
	ghiTep(t, filepath.Join(ngoai, "that.lnk"))

	for _, f := range []string{"Game Menu.lnk", "GAME MENU.URL", "Game Menu.txt", "Other.lnk", "VNET.lnk"} {
		ghiTep(t, filepath.Join(dir, f))
	}
	// Thư mục trùng tên một lối tắt: không phải tệp thường, không được xoá.
	if err := os.Mkdir(filepath.Join(dir, "Riot Client.lnk"), 0o755); err != nil {
		t.Fatal(err)
	}
	ghiTep(t, filepath.Join(dir, "Riot Client.lnk", "con.txt"))
	// Thư mục con có lối tắt trùng tên: không đi vào.
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	ghiTep(t, filepath.Join(dir, "sub", "Game Menu.lnk"))
	// Liên kết mềm tên trùng danh sách, trỏ ra ngoài.
	if err := os.Symlink(filepath.Join(ngoai, "that.lnk"), filepath.Join(dir, "Teamfight Tactics.lnk")); err != nil {
		t.Logf("không tạo được liên kết mềm (%v) — bỏ qua phần liên kết", err)
	}

	hidden := parseHiddenShortcuts("Game Menu,Riot Client,Teamfight Tactics,VNET")
	got := tenTep(shortcutsToDelete(dir, hidden))
	want := []string{"GAME MENU.URL", "Game Menu.lnk"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("cần xoá %v, mong %v", got, want)
	}

	if got := shortcutsToDelete(dir, nil); got != nil {
		t.Errorf("danh sách rỗng mà cần xoá %v", got)
	}
	if got := shortcutsToDelete(filepath.Join(dir, "khong-co"), hidden); got != nil {
		t.Errorf("thư mục không tồn tại mà cần xoá %v", got)
	}
}

// Một liên kết thay chỗ thư mục Desktop không được dẫn dịch vụ ra ngoài.
func TestShortcutsToDelete_ThuMucLaLienKet(t *testing.T) {
	that := t.TempDir()
	ghiTep(t, filepath.Join(that, "Game Menu.lnk"))
	link := filepath.Join(t.TempDir(), "Desktop")
	if err := os.Symlink(that, link); err != nil {
		t.Skipf("không tạo được liên kết mềm: %v", err)
	}
	if got := shortcutsToDelete(link, parseHiddenShortcuts("Game Menu")); got != nil {
		t.Errorf("đi theo liên kết thư mục: %v", got)
	}
}

func TestSweepShortcuts(t *testing.T) {
	pub, user, ngoai := t.TempDir(), t.TempDir(), t.TempDir()
	dich := filepath.Join(ngoai, "that.lnk")
	ghiTep(t, dich)

	xoa := []string{filepath.Join(pub, "Game Menu.lnk"), filepath.Join(user, "teamfight tactics.URL")}
	giu := []string{
		filepath.Join(pub, "VNET.lnk"), filepath.Join(pub, "Game Menu.txt"), filepath.Join(user, "Chrome.lnk"),
	}
	for _, p := range append(append([]string{}, xoa...), giu...) {
		ghiTep(t, p)
	}
	if err := os.Symlink(dich, filepath.Join(user, "Riot Client.lnk")); err == nil {
		giu = append(giu, filepath.Join(user, "Riot Client.lnk"))
	}

	// "VNET" có trong danh sách nhưng không bao giờ bị xoá.
	sweepShortcuts([]string{pub, user, filepath.Join(pub, "khong-co")},
		parseHiddenShortcuts("Game Menu,Teamfight Tactics,Riot Client,VNET"))

	for _, p := range xoa {
		if tonTai(p) {
			t.Errorf("%s phải bị xoá", p)
		}
	}
	for _, p := range giu {
		if !tonTai(p) {
			t.Errorf("%s phải còn", p)
		}
	}
	if !tonTai(dich) {
		t.Error("tệp đích của liên kết bị xoá")
	}

	// Danh sách rỗng: không đụng gì.
	ghiTep(t, filepath.Join(pub, "Game Menu.lnk"))
	sweepShortcuts([]string{pub}, parseHiddenShortcuts(""))
	if !tonTai(filepath.Join(pub, "Game Menu.lnk")) {
		t.Error("danh sách rỗng mà vẫn xoá")
	}
}

func TestUserDesktopDirs(t *testing.T) {
	root := t.TempDir()
	for _, d := range []string{"alice", "Bob", "PUBLIC", "all users", "DEFAULT", "Default User"} {
		if err := os.Mkdir(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	ghiTep(t, filepath.Join(root, "desktop.ini")) // tệp, không phải hồ sơ
	if err := os.Symlink(t.TempDir(), filepath.Join(root, "lienket")); err != nil {
		t.Logf("không tạo được liên kết mềm (%v)", err)
	}

	got := userDesktopDirs(root)
	sort.Strings(got)
	want := []string{filepath.Join(root, "Bob", "Desktop"), filepath.Join(root, "alice", "Desktop")}
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("nhận %v, mong %v", got, want)
	}
	if got := userDesktopDirs(filepath.Join(root, "khong-co")); got != nil {
		t.Errorf("thư mục gốc không tồn tại mà nhận %v", got)
	}
}

// Chính sách đi qua normalizePolicy ở cả hai cổng vào (nhịp tim và policy.json),
// và đi tròn qua JSON mà không đổi.
func TestClientPolicy_HiddenShortcuts(t *testing.T) {
	p := normalizePolicy(clientPolicy{HiddenShortcuts: " Game Menu.lnk,game menu,../x,Riot Client "})
	if p.HiddenShortcuts != "Game Menu,Riot Client" {
		t.Errorf("HiddenShortcuts = %q", p.HiddenShortcuts)
	}

	data, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"hidden_shortcuts":"Game Menu,Riot Client"`) {
		t.Errorf("JSON thiếu hidden_shortcuts: %s", data)
	}
	var back clientPolicy
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatal(err)
	}
	if normalizePolicy(back) != p {
		t.Errorf("đi tròn JSON làm đổi chính sách: %+v ≠ %+v", normalizePolicy(back), p)
	}

	// Máy chủ bản cũ không gửi trường này: tính năng tắt.
	var old clientPolicy
	if err := json.Unmarshal([]byte(`{"tamper_action":"restart"}`), &old); err != nil {
		t.Fatal(err)
	}
	if normalizePolicy(old).HiddenShortcuts != "" {
		t.Error("thiếu trường mà không rỗng")
	}
}
