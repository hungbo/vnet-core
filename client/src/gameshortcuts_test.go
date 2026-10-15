package main

import (
	"bytes"
	"errors"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

// Tên tệp lối tắt đi vào hệ thống tệp của khách, và danh sách mong muốn quyết
// định tệp nào bị XOÁ: hai việc đó phải đúng ở mọi dữ liệu máy chủ có thể trả,
// kể cả tên đã hỏng bảng mã. Mọi hàm ở đây thuần nên chạy được trên Mac/Linux.
func TestCleanShortcutName(t *testing.T) {
	cases := []struct{ ten, vao, ra string }{
		{"tên thường giữ nguyên", "Liên Minh Huyền Thoại (Riot)", "Liên Minh Huyền Thoại (Riot)"},
		{"lọc ký tự cấm, giữ khoảng trắng", "Counter-Strike: GO", "Counter-Strike GO"},
		{"mọi ký tự cấm", `a<b>c:d"e/f\g|h?i*j`, "abcdefghij"},
		{"ký tự điều khiển", "a\x00b\tc\nd", "abcd"},
		{"bỏ khoảng trắng và dấu chấm cuối", "  Game .. . ", "Game"},
		{"U+FFFD bị lọc", "Li\uFFFDn", "Lin"},
		{"chỉ có ký tự cấm", `:*?`, ""},
		{"chỉ toàn dấu chấm", "...", ""},
		{"rỗng", "", ""},
		{"CON", "CON", ""},
		{"con viết thường", "con", ""},
		{"thiết bị có đuôi", "nul.txt", ""},
		{"thiết bị có khoảng trắng", "AUX  ", ""},
		{"PRN", "PRN", ""},
		{"COM1", "COM1", ""},
		{"lpt9 có đuôi", "lpt9.x", ""},
		{"COM10 không phải thiết bị", "COM10", "COM10"},
		{"COM0 không phải thiết bị", "COM0", "COM0"},
		{"CONSOLE không phải thiết bị", "CONSOLE", "CONSOLE"},
		{"chứa CON ở giữa", "Icon", "Icon"},
		{"tên VNET không bao giờ dùng", "VNET", ""},
		{"vnet viết thường", " vnet ", ""},
		{"VNET Game thì được", "VNET Game", "VNET Game"},
	}
	for _, c := range cases {
		t.Run(c.ten, func(t *testing.T) {
			if got := cleanShortcutName(c.vao); got != c.ra {
				t.Errorf("cleanShortcutName(%q) = %q, mong %q", c.vao, got, c.ra)
			}
		})
	}
}

func TestCleanShortcutName_GioiHan(t *testing.T) {
	// Cắt theo ký tự: chữ có dấu là nhiều byte, cắt theo byte sẽ chặt ngang một ký tự.
	got := cleanShortcutName(strings.Repeat("é", maxGameLnkStem+20))
	if want := strings.Repeat("é", maxGameLnkStem); got != want {
		t.Errorf("tên dài: %d ký tự, mong %d", len([]rune(got)), maxGameLnkStem)
	}
	// Chỗ bị cắt rơi vào khoảng trắng/dấu chấm thì không để lại đuôi như vậy.
	in := strings.Repeat("a", maxGameLnkStem-1) + ". tail"
	if got := cleanShortcutName(in); got != strings.Repeat("a", maxGameLnkStem-1) {
		t.Errorf("cắt giữa dấu chấm: %q", got)
	}
}

func TestShortcutStem_VaLabel(t *testing.T) {
	// Đúng chuỗi hỏng đang nằm trong DB: "Li" + U+FFFD + "n Minh Huy?n Tho?i (Riot)".
	hong := "Li\uFFFDn Minh Huy?n Tho?i (Riot)"
	cases := []struct{ ten, display, slug, stem, label string }{
		{"tên tốt", "Liên Minh", "lol", "Liên Minh", "Liên Minh"},
		{"tên hỏng bảng mã dùng slug", hong, "lol", "lol", "lol"},
		{"tên rỗng dùng slug", "", "lol", "lol", "lol"},
		{"chỉ khoảng trắng dùng slug", "   ", "lol", "lol", "lol"},
		{"tên là thiết bị: tên tệp dùng slug, nhãn giữ tên", "CON", "csgo", "csgo", "CON"},
		{"tên là VNET: tên tệp dùng slug", "VNET", "vg", "vg", "VNET"},
		{"chỉ ký tự cấm: tên tệp dùng slug", "???", "valorant", "valorant", "???"},
		{"nhãn không có ký tự điều khiển", "A\nB", "ab", "AB", "A B"},
		{"slug cũng là thiết bị thì không có tên", "NUL", "nul", "", "NUL"},
	}
	for _, c := range cases {
		t.Run(c.ten, func(t *testing.T) {
			if got := shortcutStem(c.display, c.slug); got != c.stem {
				t.Errorf("shortcutStem(%q,%q) = %q, mong %q", c.display, c.slug, got, c.stem)
			}
			if got := shortcutLabel(c.display, c.slug); got != c.label {
				t.Errorf("shortcutLabel(%q,%q) = %q, mong %q", c.display, c.slug, got, c.label)
			}
		})
	}
	if strings.ContainsRune(shortcutLabel(hong, "lol"), '\uFFFD') || strings.ContainsRune(shortcutStem(hong, "lol"), '\uFFFD') {
		t.Error("tên hỏng lọt vào tên tệp hoặc nhãn")
	}
}

func menuReady(name, display string) gameMenuItem {
	return gameMenuItem{Name: name, DisplayName: display, Launcher: `Bin\Game.exe`, Status: gameStatusReady}
}

func testMenu(items ...gameMenuItem) *gameMenu { return &gameMenu{Root: `D:\Games`, Items: items} }

func allFiles(string) bool { return true }

func fileNames(specs []gameShortcutSpec) []string {
	out := []string{}
	for _, s := range specs {
		out = append(out, s.File)
	}
	return out
}

func TestDesiredGameShortcuts_NoiDung(t *testing.T) {
	lol := gameMenuItem{
		Name: "lol", DisplayName: "Liên Minh Huyền Thoại (Riot)", Status: gameStatusReady,
		Launcher: `Riot Client/RiotClientServices.exe`, LaunchArgs: "  --launch-product=league_of_legends  ",
	}
	got := desiredGameShortcuts(testMenu(lol), nil, allFiles)
	want := []gameShortcutSpec{{
		File:    "Liên Minh Huyền Thoại (Riot).lnk",
		Desc:    "VNET: Liên Minh Huyền Thoại (Riot)",
		Target:  `D:\Games\lol\Riot Client\RiotClientServices.exe`,
		Args:    "--launch-product=league_of_legends",
		WorkDir: `D:\Games\lol\Riot Client`,
	}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v\nwant %+v", got, want)
	}
}

func TestDesiredGameShortcuts_DieuKien(t *testing.T) {
	hong := menuReady("lol2", "Li\uFFFDn Minh Huy?n Tho?i (Riot)")
	dang := menuReady("dang", "Đang cập nhật")
	dang.Status = "downloading"
	thieu := menuReady("thieu", "Thiếu tệp")
	xau := menuReady("xau", "Launcher xấu")
	xau.Launcher = `..\x.exe`
	chuaKhaiBao := menuReady("trong", "Chưa khai báo")
	chuaKhaiBao.Launcher = ""
	nul := menuReady("nul", "Tham số NUL")
	nul.LaunchArgs = "a\x00b"
	tot := menuReady("tot", "Game tốt")

	isFile := func(p string) bool { return !strings.Contains(p, `\thieu\`) }
	got := fileNames(desiredGameShortcuts(testMenu(hong, dang, thieu, xau, chuaKhaiBao, nul, tot), nil, isFile))
	// Tên hỏng bảng mã dùng slug; còn lại chỉ game ready, dựng được đường dẫn và có tệp chạy.
	want := []string{"Game tốt.lnk", "lol2.lnk"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	specs := desiredGameShortcuts(testMenu(hong), nil, allFiles)
	if len(specs) != 1 || specs[0].Desc != "VNET: lol2" {
		t.Errorf("Description của game tên hỏng = %+v, mong dùng slug", specs)
	}
}

func TestDesiredGameShortcuts_TrungTen(t *testing.T) {
	a, b, c := menuReady("dota", "Dota"), menuReady("dota2", "dota"), menuReady("dota3", "DOTA")
	// Mỗi game giữ cách viết tên của riêng nó; chỉ game sau (slug lớn hơn) bị thêm slug.
	want := []string{"dota (dota2).lnk", "DOTA (dota3).lnk", "Dota.lnk"}

	items := []gameMenuItem{a, b, c}
	got := fileNames(desiredGameShortcuts(testMenu(items...), nil, allFiles))
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	// Máy chủ trả thứ tự nào thì game giữ tên gốc vẫn là game có slug nhỏ nhất.
	rev := []gameMenuItem{c, b, a}
	if got2 := fileNames(desiredGameShortcuts(testMenu(rev...), nil, allFiles)); !reflect.DeepEqual(got2, want) {
		t.Errorf("đảo thứ tự: got %v, want %v", got2, want)
	}

	// Tên "thêm slug" lại trùng một game khác thì thêm số, không ghi đè lên nhau.
	x := menuReady("a", "Foo")
	y := menuReady("b", "Foo")
	z := menuReady("c", "Foo (b)")
	files := fileNames(desiredGameShortcuts(testMenu(x, y, z), nil, allFiles))
	seen := map[string]bool{}
	for _, f := range files {
		if seen[strings.ToLower(f)] {
			t.Errorf("tên tệp trùng: %v", files)
		}
		seen[strings.ToLower(f)] = true
	}
	if len(files) != 3 {
		t.Errorf("mong 3 lối tắt, got %v", files)
	}
}

func TestDesiredGameShortcuts_TenCamVaChinhSach(t *testing.T) {
	hidden := parseHiddenShortcuts("Teamfight Tactics, Riot Client")
	tft := menuReady("tft", "Teamfight Tactics")
	riot := menuReady("riot", "riot client")
	vnet := menuReady("vg", "VNET")
	ok := menuReady("ok", "Game Khác")
	got := fileNames(desiredGameShortcuts(testMenu(tft, riot, vnet, ok), hidden, allFiles))
	// Tên trong danh sách ẩn thì bỏ (dịch vụ nền sẽ xoá ngay); VNET thì dùng slug.
	want := []string{"Game Khác.lnk", "vg.lnk"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	// Hai game cùng tên mà tên đó bị ẩn: cả hai đều bỏ, không để game sau lọt qua
	// bằng tên "Foo (slug)".
	f1, f2 := menuReady("f1", "Foo"), menuReady("f2", "Foo")
	if got := desiredGameShortcuts(testMenu(f1, f2), parseHiddenShortcuts("Foo"), allFiles); len(got) != 0 {
		t.Errorf("tên bị ẩn mà vẫn có lối tắt: %v", fileNames(got))
	}
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

func TestGameShortcutsKey(t *testing.T) {
	a := gameShortcutSpec{File: "A.lnk", Desc: "VNET: A", Target: `D:\a.exe`, Args: "-x", WorkDir: `D:\`}
	b := gameShortcutSpec{File: "B.lnk", Desc: "VNET: B", Target: `D:\b.exe`, WorkDir: `D:\`}
	k := gameShortcutsKey([]gameShortcutSpec{a, b})
	if k == "" || k != gameShortcutsKey([]gameShortcutSpec{a, b}) {
		t.Fatal("khoá phải ổn định và không rỗng")
	}
	if gameShortcutsKey(nil) == "" {
		t.Error("tập rỗng phải có khoá khác chuỗi rỗng, để lần đầu vẫn dọn lối tắt cũ")
	}
	a2 := a
	a2.Args = "-y"
	if k == gameShortcutsKey([]gameShortcutSpec{a2, b}) {
		t.Error("đổi tham số mà khoá không đổi")
	}
	// Ranh giới trường: chuyển một ký tự từ trường này sang trường kế không được cho cùng khoá.
	c1 := gameShortcutSpec{File: "A.lnk", Desc: "VNET: A", Target: `D:\a.exe`, Args: "x", WorkDir: "y"}
	c2 := gameShortcutSpec{File: "A.lnk", Desc: "VNET: A", Target: `D:\a.exe`, Args: "", WorkDir: "xy"}
	if gameShortcutsKey([]gameShortcutSpec{c1}) == gameShortcutsKey([]gameShortcutSpec{c2}) {
		t.Error("hai tập khác nhau mà cùng khoá")
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

// ---- gameShortcutSyncer

type syncFake struct {
	mu       sync.Mutex
	calls    int
	running  int
	maxRun   int
	lastDone []string // tên tệp của lần apply gần nhất
	err      error
	block    chan struct{} // có thì apply chờ kênh này
	entered  chan struct{}
	panicNow bool
}

func (f *syncFake) apply(dir string, desired []gameShortcutSpec) ([]string, error) {
	f.mu.Lock()
	f.calls++
	f.running++
	if f.running > f.maxRun {
		f.maxRun = f.running
	}
	f.lastDone = fileNames(desired)
	err, block, entered, doPanic := f.err, f.block, f.entered, f.panicNow
	f.mu.Unlock()

	if entered != nil {
		entered <- struct{}{}
	}
	if block != nil {
		<-block
	}
	f.mu.Lock()
	f.running--
	f.mu.Unlock()

	if doPanic {
		panic("COM nổ")
	}
	if err != nil {
		return nil, err
	}
	for _, d := range desired {
		if werr := os.WriteFile(filepath.Join(dir, d.File), []byte("x"), 0o644); werr != nil {
			return nil, werr
		}
	}
	return nil, nil
}

func (f *syncFake) snapshot() (calls int, last []string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls, f.lastDone
}

func newTestSyncer(dir string, f *syncFake) *gameShortcutSyncer {
	return &gameShortcutSyncer{
		desktopDir: func() (string, error) { return dir, nil },
		apply:      f.apply,
		isFile:     allFiles,
		logged:     map[string]bool{},
	}
}

func waitIdle(t *testing.T, s *gameShortcutSyncer) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		s.mu.Lock()
		idle := !s.running && s.pending == nil
		s.mu.Unlock()
		if idle {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatal("bộ đối chiếu không dừng")
}

func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	return &buf
}

func noHidden() map[string]bool { return nil }

func TestGameShortcutSyncer_BoQuaKhiKhongDoi(t *testing.T) {
	dir := t.TempDir()
	f := &syncFake{}
	s := newTestSyncer(dir, f)
	m := testMenu(menuReady("a", "Game A"), menuReady("b", "Game B"))

	run := func(m *gameMenu) {
		s.submit(m, noHidden)
		waitIdle(t, s)
	}
	run(m)
	if n, got := f.snapshot(); n != 1 || !reflect.DeepEqual(got, []string{"Game A.lnk", "Game B.lnk"}) {
		t.Fatalf("lần đầu: %d lần, %v", n, got)
	}
	run(m)
	run(testMenu(menuReady("b", "Game B"), menuReady("a", "Game A"))) // đổi thứ tự máy chủ không phải thay đổi
	if n, _ := f.snapshot(); n != 1 {
		t.Errorf("menu không đổi mà đối chiếu lại: %d lần", n)
	}

	// Khách xoá một lối tắt: lần tải sau dựng lại dù menu không đổi.
	if err := os.Remove(filepath.Join(dir, "Game A.lnk")); err != nil {
		t.Fatal(err)
	}
	run(m)
	if n, _ := f.snapshot(); n != 2 {
		t.Errorf("lối tắt bị xoá mà không dựng lại: %d lần", n)
	}

	// Một game sang trạng thái đang cập nhật: tập mong muốn đổi.
	b := menuReady("b", "Game B")
	b.Status = "downloading"
	run(testMenu(menuReady("a", "Game A"), b))
	if n, got := f.snapshot(); n != 3 || !reflect.DeepEqual(got, []string{"Game A.lnk"}) {
		t.Errorf("sau khi game B đang cập nhật: %d lần, %v", n, got)
	}
}

func TestGameShortcutSyncer_LoiKhongDuocNhoVaChiGhiMotLan(t *testing.T) {
	buf := captureLog(t)
	dir := t.TempDir()
	f := &syncFake{err: errors.New("không ghi được Desktop")}
	s := newTestSyncer(dir, f)
	m := testMenu(menuReady("a", "Game A"))

	for i := 0; i < 3; i++ {
		s.submit(m, noHidden)
		waitIdle(t, s)
	}
	if n, _ := f.snapshot(); n != 3 {
		t.Errorf("lỗi thì phải thử lại ở lần sau: %d lần", n)
	}
	if got := strings.Count(buf.String(), "không ghi được Desktop"); got != 1 {
		t.Errorf("lỗi giống nhau ghi %d lần, mong 1:\n%s", got, buf.String())
	}

	// Hết lỗi: làm xong, rồi không làm lại nữa.
	f.mu.Lock()
	f.err = nil
	f.mu.Unlock()
	s.submit(m, noHidden)
	waitIdle(t, s)
	s.submit(m, noHidden)
	waitIdle(t, s)
	if n, _ := f.snapshot(); n != 4 {
		t.Errorf("sau khi hết lỗi: %d lần, mong 4", n)
	}
}

func TestGameShortcutSyncer_BanRonKhongGhiNhatKy(t *testing.T) {
	buf := captureLog(t)
	f := &syncFake{err: errGameLnkBusy}
	s := newTestSyncer(t.TempDir(), f)
	s.submit(testMenu(menuReady("a", "A")), noHidden)
	waitIdle(t, s)
	if buf.Len() != 0 {
		t.Errorf("tiến trình khác đang bận không phải lỗi, nhưng ghi: %s", buf.String())
	}
}

func TestGameShortcutSyncer_KhongChongLenhVaChiGiuBanMoiNhat(t *testing.T) {
	dir := t.TempDir()
	f := &syncFake{block: make(chan struct{}), entered: make(chan struct{}, 8)}
	s := newTestSyncer(dir, f)

	s.submit(testMenu(menuReady("m1", "Một")), noHidden)
	<-f.entered // lần 1 đang chạy, chưa xong
	s.submit(testMenu(menuReady("m2", "Hai")), noHidden)
	s.submit(testMenu(menuReady("m3", "Ba")), noHidden)
	if n, _ := f.snapshot(); n != 1 {
		t.Fatalf("lần 2, 3 chạy chồng lên lần 1: %d lần apply", n)
	}
	close(f.block)
	waitIdle(t, s)

	n, last := f.snapshot()
	if n != 2 || !reflect.DeepEqual(last, []string{"Ba.lnk"}) {
		t.Errorf("%d lần apply, lần cuối %v; mong 2 lần và bản mới nhất (Ba)", n, last)
	}
	if f.maxRun != 1 {
		t.Errorf("có lúc %d lần đối chiếu chạy cùng nhau", f.maxRun)
	}
}

func TestGameShortcutSyncer_PanicKhongLamHong(t *testing.T) {
	buf := captureLog(t)
	dir := t.TempDir()
	f := &syncFake{panicNow: true}
	s := newTestSyncer(dir, f)
	s.submit(testMenu(menuReady("a", "A")), noHidden)
	waitIdle(t, s) // goroutine không được chết kéo theo cả tiến trình

	if !strings.Contains(buf.String(), "COM nổ") {
		t.Errorf("panic phải được ghi nhật ký: %q", buf.String())
	}
	f.mu.Lock()
	f.panicNow = false
	f.mu.Unlock()
	s.submit(testMenu(menuReady("a", "A")), noHidden)
	waitIdle(t, s)
	if n, _ := f.snapshot(); n != 2 {
		t.Errorf("sau panic phải làm lại được: %d lần", n)
	}
}

func TestGameShortcutSyncer_KhongCoDesktop(t *testing.T) {
	f := &syncFake{}
	s := newTestSyncer("", f)
	s.submit(testMenu(menuReady("a", "A")), noHidden)
	waitIdle(t, s)
	if n, _ := f.snapshot(); n != 0 {
		t.Errorf("nền tảng không có desktop mà vẫn đối chiếu %d lần", n)
	}
}

func TestGameShortcutSyncer_ChinhSachAn(t *testing.T) {
	dir := t.TempDir()
	f := &syncFake{}
	s := newTestSyncer(dir, f)
	m := testMenu(menuReady("tft", "Teamfight Tactics"), menuReady("ok", "Game Khác"))

	s.submit(m, func() map[string]bool { return parseHiddenShortcuts("Teamfight Tactics") })
	waitIdle(t, s)
	if _, got := f.snapshot(); !reflect.DeepEqual(got, []string{"Game Khác.lnk"}) {
		t.Errorf("tên trong danh sách ẩn vẫn có lối tắt: %v", got)
	}
	// Quản trị bỏ tên đó khỏi danh sách ẩn: lần tải sau tạo nó.
	s.submit(m, noHidden)
	waitIdle(t, s)
	if _, got := f.snapshot(); !reflect.DeepEqual(got, []string{"Game Khác.lnk", "Teamfight Tactics.lnk"}) {
		t.Errorf("sau khi bỏ khỏi danh sách ẩn: %v", got)
	}
}
