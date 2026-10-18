package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Danh sách mong muốn quyết định tệp nào trên desktop của khách bị XOÁ, nên phần lập
// kế hoạch phải đúng ở mọi trường hợp. Mọi hàm ở đây thuần nên chạy được trên Mac/Linux.

// Desktop của khách chỉ có một lối tắt VNET: "Menu Game", mở thanh đang chạy với
// --open=games. Quán ẩn tên đó bằng chính sách thì không tạo (dịch vụ nền xoá ngay),
// nhưng danh sách rỗng vẫn đi đối chiếu để dọn lối tắt game cũ.
func TestDesiredMenuShortcuts(t *testing.T) {
	exe := filepath.Join("C:", "Program Files", "VNET Client", "vnet-client.exe")
	got := desiredMenuShortcuts(exe, nil)
	want := []gameShortcutSpec{{File: "Menu Game.lnk", Desc: "VNET: Menu Game", Target: exe, Args: "--open=games", WorkDir: filepath.Dir(exe)}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("nhận %+v, mong %+v", got, want)
	}
	if moBangTuThamSo(strings.Fields(got[0].Args)) != "games" {
		t.Error("tham số của lối tắt phải mở được menu Game")
	}
	if got := desiredMenuShortcuts(exe, parseHiddenShortcuts("menu game")); len(got) != 0 {
		t.Errorf("tên bị ẩn mà vẫn có lối tắt: %+v", got)
	}
	if got := desiredMenuShortcuts("", nil); len(got) != 0 {
		t.Errorf("không biết tệp .exe mà vẫn có lối tắt: %+v", got)
	}
	// Lối tắt game cũ (của VNET) không còn mong muốn: kế hoạch xoá chúng, giữ của khách.
	old := []desktopLnk{
		{File: "Roblox.lnk", Ours: true, Desc: "VNET: Roblox", Target: `D:\Games\Roblox\x.exe`},
		{File: "Liên Minh Huyền Thoại.lnk", Ours: true, Desc: "VNET: Liên Minh Huyền Thoại"},
		{File: "Zalo.lnk"},
	}
	p := planGameShortcuts(want, old)
	if !reflect.DeepEqual(p.Delete, []string{"Liên Minh Huyền Thoại.lnk", "Roblox.lnk"}) || len(p.Create) != 1 {
		t.Errorf("kế hoạch %+v: phải xoá hai lối tắt game cũ và tạo Menu Game", p)
	}
	// Quán ẩn "Menu Game" sau khi lối tắt đã được tạo: lần đối chiếu sau xoá nó.
	coSan := []desktopLnk{{File: "Menu Game.lnk", Ours: true, Desc: "VNET: Menu Game", Target: exe, Args: "--open=games"}}
	if p := planGameShortcuts(desiredMenuShortcuts(exe, parseHiddenShortcuts("Menu Game")), coSan); !reflect.DeepEqual(p.Delete, []string{"Menu Game.lnk"}) {
		t.Errorf("Menu Game bị ẩn mà không bị xoá: %+v", p)
	}
}

// Lần gõ cửa tới trước khi cửa sổ phụ khởi động xong (khách vừa đăng nhập: cửa sổ dựng
// sẵn và lần tự mở menu Game chạy gần như cùng lúc) không được mất: nhớ màn hình để
// GetWindowMode trả về đúng, và đánh dấu chờ hiện.
func TestSwitchPanel_TruocKhiKhoiDong(t *testing.T) {
	a := &App{windowMode: "panel", panelMode: "blank"}
	a.switchPanel("games")
	if got := a.GetWindowMode(); got != "games" {
		t.Errorf("GetWindowMode = %q, mong games", got)
	}
	if !a.panelChoHien {
		t.Error("phải đánh dấu chờ hiện")
	}
	a.switchPanel("không-có")
	if got := a.GetWindowMode(); got != "games" {
		t.Errorf("màn hình lạ không được đè: %q", got)
	}
}

func TestMoBangTuThamSo(t *testing.T) {
	cases := map[string]struct {
		args []string
		want string
	}{
		"lối tắt":           {[]string{"--open=games"}, "games"},
		"tách đôi":          {[]string{"--open", "games"}, "games"},
		"một gạch":          {[]string{"-open=Games"}, "games"},
		"không có":          {nil, ""},
		"lối tắt VNET":      {[]string{}, ""},
		"màn khác":          {[]string{"--open=order"}, ""},
		"thiếu giá trị":     {[]string{"--open"}, ""},
		"cờ khác":           {[]string{"--window", "games"}, ""},
		"tên giống tiền tố": {[]string{"--opener=games"}, ""},
	}
	for ten, c := range cases {
		if got := moBangTuThamSo(c.args); got != c.want {
			t.Errorf("%s: %v → %q, mong %q", ten, c.args, got, c.want)
		}
	}
}

func fileNames(specs []gameShortcutSpec) []string {
	out := []string{}
	for _, s := range specs {
		out = append(out, s.File)
	}
	return out
}

func TestPlanGameShortcuts(t *testing.T) {
	spec := func(file, args string) gameShortcutSpec {
		return gameShortcutSpec{File: file, Desc: gameLnkPrefix + strings.TrimSuffix(file, ".lnk"),
			Target: `D:\Games\x\Game.exe`, Args: args, WorkDir: `D:\Games\x`}
	}
	ours := func(s gameShortcutSpec) desktopLnk {
		return desktopLnk{File: s.File, Ours: true, Desc: s.Desc, Target: s.Target, Args: s.Args, WorkDir: s.WorkDir}
	}

	same := spec("Giống.lnk", "-a")
	changedArgs := spec("ĐổiThamSố.lnk", "-b")
	changedTarget := spec("ĐổiĐích.lnk", "")
	fresh := spec("Mới.lnk", "")
	taken := spec("BịChiếm.lnk", "")
	oldTarget := ours(changedTarget)
	oldTarget.Target = `D:\Games\y\Other.exe`
	oldArgs := ours(changedArgs)
	oldArgs.Args = "-a"

	existing := []desktopLnk{
		ours(same),
		oldArgs,
		oldTarget,
		{File: "BịChiếm.lnk"}, // lối tắt của người khác
		ours(spec("Cũ.lnk", "")),
		{File: "Của khách.lnk"}, // của người khác, không được mong muốn: không đụng
	}
	p := planGameShortcuts([]gameShortcutSpec{same, changedArgs, changedTarget, fresh, taken}, existing)

	if got := fileNames(p.Create); !reflect.DeepEqual(got, []string{"Mới.lnk"}) {
		t.Errorf("Create = %v", got)
	}
	if got := fileNames(p.Update); !reflect.DeepEqual(got, []string{"ĐổiThamSố.lnk", "ĐổiĐích.lnk"}) {
		t.Errorf("Update = %v", got)
	}
	if !reflect.DeepEqual(p.Delete, []string{"Cũ.lnk"}) {
		t.Errorf("Delete = %v, chỉ lối tắt của VNET không còn mong muốn mới bị xoá", p.Delete)
	}
	if !reflect.DeepEqual(p.Skip, []string{"BịChiếm.lnk"}) {
		t.Errorf("Skip = %v", p.Skip)
	}
}

func TestPlanGameShortcuts_KhongDoiThiKhongGhi(t *testing.T) {
	s := gameShortcutSpec{File: "Game.lnk", Desc: "VNET: Game", Target: `D:\Games\g\Game.exe`, Args: "-x", WorkDir: `D:\Games\g`}
	// Windows không phân biệt hoa thường: ổ đĩa/thư mục khác hoa thường không phải thay đổi.
	have := desktopLnk{File: "game.lnk", Ours: true, Desc: s.Desc, Target: `d:\games\g\game.exe`, Args: "-x", WorkDir: `D:\GAMES\g`}
	if p := planGameShortcuts([]gameShortcutSpec{s}, []desktopLnk{have}); len(p.Create)+len(p.Update)+len(p.Delete)+len(p.Skip) != 0 {
		t.Errorf("lối tắt đã đúng mà vẫn có việc: %+v", p)
	}
	if p := planGameShortcuts(nil, nil); len(p.Create)+len(p.Update)+len(p.Delete)+len(p.Skip) != 0 {
		t.Errorf("không có gì mà vẫn có việc: %+v", p)
	}
}

// fakeDesktop giả lớp COM: ghi lối tắt thành tệp thường và nhớ siêu dữ liệu để
// "đọc lại". Phần còn lại (Lstat, xoá, quyết định bỏ qua) chạy thật trên thư mục tạm.
type fakeDesktop struct {
	meta   map[string]desktopLnk // theo tên tệp viết thường
	writes []string
}

func newFakeDesktop() *fakeDesktop { return &fakeDesktop{meta: map[string]desktopLnk{}} }

func (f *fakeDesktop) scan(dir string) ([]desktopLnk, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []desktopLnk
	for _, e := range entries {
		if !strings.EqualFold(filepath.Ext(e.Name()), ".lnk") {
			continue
		}
		l := desktopLnk{File: e.Name()}
		if fi, err := os.Lstat(filepath.Join(dir, e.Name())); err == nil && fi.Mode().IsRegular() {
			if m, ok := f.meta[strings.ToLower(e.Name())]; ok {
				l = m
				l.File = e.Name()
			}
		}
		out = append(out, l)
	}
	return out, nil
}

func (f *fakeDesktop) write(path string, s gameShortcutSpec) error {
	f.writes = append(f.writes, filepath.Base(path))
	f.meta[strings.ToLower(s.File)] = desktopLnk{File: s.File, Ours: true, Desc: s.Desc, Target: s.Target, Args: s.Args, WorkDir: s.WorkDir}
	return os.WriteFile(path, []byte(s.Desc+"|"+s.Args), 0o644)
}

func TestReconcileDesktop(t *testing.T) {
	dir := t.TempDir()
	fd := newFakeDesktop()
	put := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	read := func(name string) string {
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return "<không có>"
		}
		return string(b)
	}

	mk := func(file, args string) gameShortcutSpec {
		return gameShortcutSpec{File: file, Desc: gameLnkPrefix + strings.TrimSuffix(file, ".lnk"),
			Target: `D:\Games\x\Game.exe`, Args: args, WorkDir: `D:\Games\x`}
	}
	// Lần 1: desktop có sẵn tệp của khách, một lối tắt xa lạ chiếm tên, và một lối tắt cũ của VNET.
	put("Ghi chú.txt", "của khách")
	put("Người khác.lnk", "lối tắt của người khác")
	put("Cũ.lnk", "bản cũ")
	fd.meta["cũ.lnk"] = desktopLnk{File: "Cũ.lnk", Ours: true, Desc: "VNET: Cũ"}
	put("Giữ.lnk", "giữ")
	keep := mk("Giữ.lnk", "")
	fd.meta["giữ.lnk"] = desktopLnk{File: "Giữ.lnk", Ours: true, Desc: keep.Desc, Target: keep.Target, WorkDir: keep.WorkDir}

	desired := []gameShortcutSpec{keep, mk("Mới.lnk", "-a"), mk("Người khác.lnk", "")}
	skipped, err := reconcileDesktop(dir, desired, fd.scan, fd.write)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(skipped, []string{"Người khác.lnk"}) {
		t.Errorf("skipped = %v", skipped)
	}
	if got := read("Ghi chú.txt"); got != "của khách" {
		t.Errorf("tệp của khách bị đụng: %q", got)
	}
	if got := read("Người khác.lnk"); got != "lối tắt của người khác" {
		t.Errorf("lối tắt của người khác bị ghi đè: %q", got)
	}
	if got := read("Cũ.lnk"); got != "<không có>" {
		t.Errorf("lối tắt cũ của VNET phải bị xoá, còn %q", got)
	}
	if got := read("Mới.lnk"); got != "VNET: Mới|-a" {
		t.Errorf("Mới.lnk = %q", got)
	}
	if !reflect.DeepEqual(fd.writes, []string{"Mới.lnk"}) {
		t.Errorf("chỉ Mới.lnk được ghi (Giữ.lnk đã đúng), nhưng ghi %v", fd.writes)
	}

	// Lần 2 cùng mong muốn: không ghi gì thêm. Đổi tham số một game: chỉ game đó được ghi lại.
	fd.writes = nil
	if _, err := reconcileDesktop(dir, desired, fd.scan, fd.write); err != nil {
		t.Fatal(err)
	}
	if len(fd.writes) != 0 {
		t.Errorf("không đổi mà vẫn ghi %v", fd.writes)
	}
	desired[1] = mk("Mới.lnk", "-b")
	if _, err := reconcileDesktop(dir, desired, fd.scan, fd.write); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(fd.writes, []string{"Mới.lnk"}) || read("Mới.lnk") != "VNET: Mới|-b" {
		t.Errorf("cập nhật: ghi %v, nội dung %q", fd.writes, read("Mới.lnk"))
	}

	// Game không còn đủ điều kiện: lối tắt của VNET biến mất, của khách thì còn.
	if _, err := reconcileDesktop(dir, nil, fd.scan, fd.write); err != nil {
		t.Fatal(err)
	}
	if read("Mới.lnk") != "<không có>" || read("Giữ.lnk") != "<không có>" {
		t.Error("lối tắt VNET phải bị xoá khi hết game")
	}
	if read("Người khác.lnk") == "<không có>" || read("Ghi chú.txt") == "<không có>" {
		t.Error("tệp không phải của VNET bị xoá")
	}
}

func TestApplyGameShortcutPlan_KhongXoaTepKhongPhaiThuong(t *testing.T) {
	dir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "ngoai.txt")
	if err := os.WriteFile(outside, []byte("quan trọng"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "Sym.lnk")); err != nil {
		t.Skipf("không tạo được liên kết mềm: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "Thư mục.lnk", "con"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Thường.lnk"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := applyGameShortcutPlan(dir, gameShortcutPlan{Delete: []string{"Sym.lnk", "Thư mục.lnk", "Thường.lnk", "Không có.lnk"}},
		func(string, gameShortcutSpec) error { t.Error("không được ghi"); return nil })
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(dir, "Sym.lnk")); err != nil {
		t.Error("liên kết mềm bị xoá")
	}
	if b, _ := os.ReadFile(outside); string(b) != "quan trọng" {
		t.Error("tệp đích của liên kết bị động tới")
	}
	if _, err := os.Stat(filepath.Join(dir, "Thư mục.lnk", "con")); err != nil {
		t.Error("thư mục trùng tên bị xoá")
	}
	if _, err := os.Lstat(filepath.Join(dir, "Thường.lnk")); err == nil {
		t.Error("tệp thường phải bị xoá")
	}

	// Ghi: không ghi đè tệp vừa xuất hiện (Create) và không ghi xuyên liên kết (Update).
	var wrote []string
	write := func(p string, s gameShortcutSpec) error { wrote = append(wrote, filepath.Base(p)); return nil }
	err = applyGameShortcutPlan(dir, gameShortcutPlan{
		Create: []gameShortcutSpec{{File: "Sym.lnk"}, {File: "Mới.lnk"}},
		Update: []gameShortcutSpec{{File: "Sym.lnk"}},
	}, write)
	if err == nil {
		t.Error("Create/Update lên liên kết mềm phải báo lỗi")
	}
	if !reflect.DeepEqual(wrote, []string{"Mới.lnk"}) {
		t.Errorf("đã ghi %v, chỉ Mới.lnk hợp lệ", wrote)
	}
}
