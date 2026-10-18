package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestChuanHoaNguonRiot(t *testing.T) {
	cases := map[string]string{
		`D:\Games\Riot Games\_meta`:       `D:\Games\Riot Games\_meta`,
		`  d:/Games/meta  `:               `d:/Games/meta`,
		``:                                ``,
		`Games\meta`:                      ``,
		`\\server\share\meta`:             ``,
		`D:Games`:                         ``,
		`D:\Games\..\Windows`:             ``,
		`D:\Games\*`:                      ``,
		"D:\\Games\\a\x00b":               ``,
		`C:\ProgramData\Riot Games`:       `C:\ProgramData\Riot Games`,
		`D:\` + string(make([]byte, 300)): ``,
	}
	for in, want := range cases {
		if got := chuanHoaNguonRiot(in); got != want {
			t.Errorf("chuanHoaNguonRiot(%q) = %q, want %q", in, got, want)
		}
	}
}

func write(t *testing.T, path, body string, mod time.Time) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, mod, mod); err != nil {
		t.Fatal(err)
	}
}

func TestSyncRiotMetadata(t *testing.T) {
	done := map[string]riotStamp{}
	src, dst := t.TempDir(), t.TempDir()
	m1 := time.Date(2026, 10, 8, 11, 22, 19, 0, time.UTC)
	m2 := m1.Add(time.Hour)
	write(t, filepath.Join(src, "RiotClientInstalls.json"), `{"a":1}`, m1)
	write(t, filepath.Join(src, "Metadata", "league_of_legends.live", "x.manifest"), "v1", m1)
	write(t, filepath.Join(src, "vanguard", "v.cfg"), "no", m1)
	write(t, filepath.Join(src, "machine.cfg"), "no", m1)
	write(t, filepath.Join(dst, "Extra", "keep.txt"), "keep", m1)

	n, err := syncRiotMetadata(src, dst, done)
	if err != nil || n != 2 {
		t.Fatalf("lần 1: n=%d err=%v, muốn 2 tệp", n, err)
	}
	for _, p := range []string{"vanguard", "machine.cfg"} {
		if _, err := os.Stat(filepath.Join(dst, p)); err == nil {
			t.Errorf("%s không được chép", p)
		}
	}
	if b, _ := os.ReadFile(filepath.Join(dst, "Extra", "keep.txt")); string(b) != "keep" {
		t.Error("tệp có sẵn trong đích bị đụng tới")
	}
	if _, err := os.Stat(filepath.Join(dst, "RiotClientInstalls.json.vnet-tmp")); err == nil {
		t.Error("để lại tệp tạm")
	}

	// Không đổi gì thì không chép lại.
	if n, err := syncRiotMetadata(src, dst, done); err != nil || n != 0 {
		t.Fatalf("lần 2: n=%d err=%v, muốn 0", n, err)
	}

	// Bản vá: manifest đổi nội dung và giờ sửa.
	write(t, filepath.Join(src, "Metadata", "league_of_legends.live", "x.manifest"), "v2!", m2)
	if n, err := syncRiotMetadata(src, dst, done); err != nil || n != 1 {
		t.Fatalf("lần 3: n=%d err=%v, muốn 1", n, err)
	}
	if b, _ := os.ReadFile(filepath.Join(dst, "Metadata", "league_of_legends.live", "x.manifest")); string(b) != "v2!" {
		t.Errorf("manifest = %q, muốn v2!", b)
	}
}

func TestSyncRiotMetadataSourceMissing(t *testing.T) {
	if _, err := syncRiotMetadata(filepath.Join(t.TempDir(), "none"), t.TempDir(), map[string]riotStamp{}); err == nil {
		t.Error("nguồn không có mà không báo lỗi")
	}
}

func TestNormalizePolicyRiotSource(t *testing.T) {
	p := normalizePolicy(clientPolicy{RiotMetadataSource: `..\evil`})
	if p.RiotMetadataSource != "" {
		t.Errorf("nguồn không hợp lệ lọt qua: %q", p.RiotMetadataSource)
	}
	p = normalizePolicy(clientPolicy{RiotMetadataSource: ` D:\Games\meta `})
	if p.RiotMetadataSource != `D:\Games\meta` {
		t.Errorf("nguồn = %q", p.RiotMetadataSource)
	}
}

// Khách thay thư mục con trong ProgramData bằng liên kết: không được ghi xuyên qua.
func TestSyncRiotMetadataRefusesLinkedDest(t *testing.T) {
	done := map[string]riotStamp{}
	src, dst, outside := t.TempDir(), t.TempDir(), t.TempDir()
	m := time.Date(2026, 10, 8, 11, 22, 19, 0, time.UTC)
	write(t, filepath.Join(src, "Metadata", "x.manifest"), "v1", m)
	if err := os.Symlink(outside, filepath.Join(dst, "Metadata")); err != nil {
		t.Skip("không tạo được symlink:", err)
	}
	n, err := syncRiotMetadata(src, dst, done)
	if err == nil || n != 0 {
		t.Fatalf("n=%d err=%v, muốn lỗi và 0 tệp", n, err)
	}
	if entries, _ := os.ReadDir(outside); len(entries) != 0 {
		t.Errorf("đã ghi xuyên liên kết: %v", entries)
	}
}

func TestSyncRiotMetadataRefusesLinkedRoot(t *testing.T) {
	src, outside := t.TempDir(), t.TempDir()
	link := filepath.Join(t.TempDir(), "Riot Games")
	m := time.Date(2026, 10, 8, 11, 22, 19, 0, time.UTC)
	write(t, filepath.Join(src, "a.json"), "{}", m)
	if err := os.Symlink(outside, link); err != nil {
		t.Skip("không tạo được symlink:", err)
	}
	if n, err := syncRiotMetadata(src, link, map[string]riotStamp{}); err == nil || n != 0 {
		t.Fatalf("n=%d err=%v, muốn lỗi và 0 tệp", n, err)
	}
	if entries, _ := os.ReadDir(outside); len(entries) != 0 {
		t.Errorf("đã ghi xuyên liên kết: %v", entries)
	}
}

// Tên tệp tạm đoán trước được mà là liên kết: không được mở và cắt đích của nó.
func TestSyncRiotMetadataTmpLinkNotFollowed(t *testing.T) {
	done := map[string]riotStamp{}
	src, dst, outside := t.TempDir(), t.TempDir(), t.TempDir()
	m := time.Date(2026, 10, 8, 11, 22, 19, 0, time.UTC)
	write(t, filepath.Join(src, "a.json"), "{}", m)
	victim := filepath.Join(outside, "victim.txt")
	write(t, victim, "keep", m)
	if err := os.Symlink(victim, filepath.Join(dst, "a.json.vnet-tmp")); err != nil {
		t.Skip("không tạo được symlink:", err)
	}
	if n, err := syncRiotMetadata(src, dst, done); err != nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if b, _ := os.ReadFile(victim); string(b) != "keep" {
		t.Errorf("tệp đích của liên kết bị ghi đè: %q", b)
	}
}

// Riot tự ghi lại tệp trong ProgramData (vá ngay trên máy, đổi ngôn ngữ): nguồn không
// đổi thì không được chép đè, kẻo VNET và Riot giành nhau mỗi phút.
func TestSyncRiotMetadataLeavesRiotWritesAlone(t *testing.T) {
	src, dst := t.TempDir(), t.TempDir()
	done := map[string]riotStamp{}
	m := time.Date(2026, 10, 8, 11, 22, 19, 0, time.UTC)
	manifest := filepath.Join("Metadata", "league_of_legends.live", "x.manifest")
	write(t, filepath.Join(src, manifest), "v1", m)
	if n, err := syncRiotMetadata(src, dst, done); err != nil || n != 1 {
		t.Fatalf("lần 1: n=%d err=%v", n, err)
	}
	write(t, filepath.Join(dst, manifest), "riot-patched", m.Add(time.Hour))
	if n, err := syncRiotMetadata(src, dst, done); err != nil || n != 0 {
		t.Fatalf("lần 2: n=%d err=%v, muốn 0", n, err)
	}
	if b, _ := os.ReadFile(filepath.Join(dst, manifest)); string(b) != "riot-patched" {
		t.Errorf("chép đè lên tệp Riot vừa ghi: %q", b)
	}
	// Nguồn đổi (quán cập nhật metadata) thì vẫn chép.
	write(t, filepath.Join(src, manifest), "v2", m.Add(2*time.Hour))
	if n, err := syncRiotMetadata(src, dst, done); err != nil || n != 1 {
		t.Fatalf("lần 3: n=%d err=%v, muốn 1", n, err)
	}
	if b, _ := os.ReadFile(filepath.Join(dst, manifest)); string(b) != "v2" {
		t.Errorf("manifest = %q, muốn v2", b)
	}
}

// Đích đã khớp nguồn từ đầu (image có sẵn bản đúng): ghi nhận luôn, để sau đó Riot
// ghi lại cũng không bị chép đè.
func TestSyncRiotMetadataRecordsMatchingDest(t *testing.T) {
	src, dst := t.TempDir(), t.TempDir()
	done := map[string]riotStamp{}
	m := time.Date(2026, 10, 8, 11, 22, 19, 0, time.UTC)
	write(t, filepath.Join(src, "a.json"), "{}", m)
	write(t, filepath.Join(dst, "a.json"), "{}", m)
	if n, err := syncRiotMetadata(src, dst, done); err != nil || n != 0 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	write(t, filepath.Join(dst, "a.json"), `{"riot":1}`, m.Add(time.Hour))
	if n, _ := syncRiotMetadata(src, dst, done); n != 0 {
		t.Errorf("chép đè %d tệp Riot vừa ghi", n)
	}
}

func TestNguonRiotAnToan(t *testing.T) {
	base := t.TempDir()
	dst := filepath.Join(base, "ProgramData", "Riot Games")
	bad := []string{
		string(filepath.Separator),
		dst,
		filepath.Join(base, "ProgramData"),
		filepath.Join(dst, "Metadata"),
		strings.ToUpper(dst),
	}
	for _, src := range bad {
		if err := nguonRiotAnToan(src, dst); err == nil {
			t.Errorf("nguồn %q lẽ ra bị từ chối", src)
		}
		if n, err := syncRiotMetadata(src, dst, map[string]riotStamp{}); err == nil || n != 0 {
			t.Errorf("sync với nguồn %q: n=%d err=%v", src, n, err)
		}
	}
	for _, src := range []string{filepath.Join(base, "Games", "_riot_metadata", "Riot Games"), dst + "2"} {
		if err := nguonRiotAnToan(src, dst); err != nil {
			t.Errorf("nguồn %q bị từ chối nhầm: %v", src, err)
		}
	}
}

// Trỏ nhầm vào thư mục game: quá nhiều tệp thì không chép tệp nào.
func TestSyncRiotMetadataTooManyFiles(t *testing.T) {
	src, dst := t.TempDir(), t.TempDir()
	m := time.Date(2026, 10, 8, 11, 22, 19, 0, time.UTC)
	for i := 0; i <= maxRiotMetaFiles; i++ {
		write(t, filepath.Join(src, "d", fmt.Sprintf("%05d.bin", i)), "x", m)
	}
	n, err := syncRiotMetadata(src, dst, map[string]riotStamp{})
	if err == nil || n != 0 {
		t.Fatalf("n=%d err=%v, muốn lỗi và 0 tệp", n, err)
	}
	if entries, _ := os.ReadDir(dst); len(entries) != 0 {
		t.Errorf("đã chép một phần: %v", entries)
	}
}

// Thư mục con mới cần tạo nằm dưới một liên kết: không được tạo thư mục ở đích của liên kết.
func TestSyncRiotMetadataNoMkdirThroughLink(t *testing.T) {
	src, dst, outside := t.TempDir(), t.TempDir(), t.TempDir()
	m := time.Date(2026, 10, 8, 11, 22, 19, 0, time.UTC)
	write(t, filepath.Join(src, "Metadata", "new.live", "deep", "x.manifest"), "v1", m)
	if err := os.Symlink(outside, filepath.Join(dst, "Metadata")); err != nil {
		t.Skip("không tạo được symlink:", err)
	}
	if n, err := syncRiotMetadata(src, dst, map[string]riotStamp{}); err == nil || n != 0 {
		t.Fatalf("n=%d err=%v, muốn lỗi và 0 tệp", n, err)
	}
	if entries, _ := os.ReadDir(outside); len(entries) != 0 {
		t.Errorf("đã tạo thư mục xuyên liên kết: %v", entries)
	}
}

// riotFilesIn: các tệp trong dir (đường dẫn "/" tính từ dir), theo thứ tự tên.
func riotFilesIn(t *testing.T, dir string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(dir, p)
		out = append(out, filepath.ToSlash(rel))
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestRiotMetaSkipped(t *testing.T) {
	cases := []struct {
		rel   string
		isDir bool
		want  bool
	}{
		{"vanguard", true, true},
		{"Vanguard", true, true},
		{"logs", true, true},
		{"Metadata/RiotClientServices/Update", true, true},
		{"machine.cfg", false, true},
		{"MACHINE.CFG", false, true},
		{"lockfile", false, true},
		{"vanguard.lockfile", false, true},
		{"a.db-journal", false, true},
		{"a.db-wal", false, true},
		{"a.db-shm", false, true},
		{"x.yaml.vnet-tmp", false, true},
		{"Metadata", true, false},
		{"RiotClientServices", true, false},
		{"a.yaml", false, false},
		{"lockfile.txt", false, false},
		{"vanguard", false, false},
	}
	for _, c := range cases {
		if got := riotMetaSkipped(filepath.FromSlash(c.rel), c.isDir); got != c.want {
			t.Errorf("riotMetaSkipped(%q, dir=%v) = %v, muốn %v", c.rel, c.isDir, got, c.want)
		}
	}
}

// Mục bỏ qua nằm sâu bên trong (bản chụp tay cũ, Riot đang chạy lúc chụp): vẫn không chép.
func TestSyncRiotMetadataSkipsAtAnyDepth(t *testing.T) {
	src, dst := t.TempDir(), t.TempDir()
	m := time.Date(2026, 10, 8, 11, 22, 19, 0, time.UTC)
	for _, rel := range []string{
		"Metadata/vanguard/setup.exe", "Metadata/vanguard.lockfile", "Metadata/lol/x.db-journal",
		"Metadata/x/lockfile", "Logs/a.log", "Metadata/Logs/b.log", "Metadata/sub/machine.cfg",
		"Metadata/RiotClientServices/Update/u.bin", "Metadata/lol/x.manifest",
	} {
		write(t, filepath.Join(src, filepath.FromSlash(rel)), "x", m)
	}
	n, err := syncRiotMetadata(src, dst, map[string]riotStamp{})
	if err != nil || n != 1 {
		t.Fatalf("n=%d err=%v, muốn 1 tệp", n, err)
	}
	if got := riotFilesIn(t, dst); len(got) != 1 || got[0] != "Metadata/lol/x.manifest" {
		t.Errorf("đích có %v, muốn chỉ Metadata/lol/x.manifest", got)
	}
}

// Bảng vàng: bản y hệt nằm ở backend (gameupdate TestRewriteRiotPaths), hai bên phải đổi giống nhau.
func TestRewriteRiotPaths(t *testing.T) {
	const from, to = `D:\GameMaster\Riot Games`, `E:\Games\Riot Games`
	cases := []struct{ from, to, in, want string }{
		{from, to,
			`"rc_default": "D:/GameMaster/Riot Games/Riot Client/RiotClientServices.exe"`,
			`"rc_default": "E:/Games/Riot Games/Riot Client/RiotClientServices.exe"`},
		{from, to,
			`"associated_client": {"D:/GameMaster/Riot Games/League of Legends/": "D:/GameMaster/Riot Games/Riot Client/RiotClientServices.exe"}`,
			`"associated_client": {"E:/Games/Riot Games/League of Legends/": "E:/Games/Riot Games/Riot Client/RiotClientServices.exe"}`},
		{from, to, `product_install_root: "d:/gamemaster/riot games/"`, `product_install_root: "E:/Games/Riot Games/"`},
		{from, to, "product_install_full_path: D:/GameMaster/Riot Games\nx: 1\n", "product_install_full_path: E:/Games/Riot Games\nx: 1\n"},
		{from, to, `product_install_root: 'D:\GameMaster\Riot Games\Riot Client'`, `product_install_root: 'E:\Games\Riot Games\Riot Client'`},
		{from, to, `{"p": "D:\\GameMaster\\Riot Games\\League of Legends\\"}`, `{"p": "E:\\Games\\Riot Games\\League of Legends\\"}`},
		{from, to, `x: D:/GameMaster/Riot Games`, `x: E:/Games/Riot Games`},
		{from, to, `[D:/GameMaster/Riot Games, 1]`, `[E:/Games/Riot Games, 1]`},
		{from, to, `{a: D:/GameMaster/Riot Games}`, `{a: E:/Games/Riot Games}`},
		{from, to, `D:/GameMaster/Riot Games 2/x`, `E:/Games/Riot Games 2/x`},
		{from, to, `XD:/GameMaster/Riot Games`, `XD:/GameMaster/Riot Games`},
		{from, to, `C:/GameMaster/Riot Games`, `C:/GameMaster/Riot Games`},
		{from, to, `D:/GameMaster/Other`, `D:/GameMaster/Other`},
		{from, to, `D:/GameMaster\Riot Games`, `D:/GameMaster\Riot Games`},
		{from, to, `D:\/GameMaster\/Riot Games`, `D:\/GameMaster\/Riot Games`},
		{from, to, "\xef\xbb\xbf{\"a\": \"D:/GameMaster/Riot Games\"}", "\xef\xbb\xbf{\"a\": \"E:/Games/Riot Games\"}"},
		{from, to, `{"a": 1}`, `{"a": 1}`},
		{from, `E:\GameDisk01\Riot Games`, `"D:/GameMaster/Riot Games/x"`, `"E:/GameDisk01/Riot Games/x"`},
	}
	for _, c := range cases {
		if got := string(rewriteRiotPaths([]byte(c.in), c.from, c.to)); got != c.want {
			t.Errorf("rewriteRiotPaths(%q, %q, %q) = %q, muốn %q", c.in, c.from, c.to, got, c.want)
		}
	}
}

func TestRiotRewriteFor(t *testing.T) {
	base := t.TempDir()
	m := time.Date(2026, 10, 8, 11, 22, 19, 0, time.UTC)
	x := filepath.Join(base, "Riot Games")
	src := filepath.Join(x, "_vnet", "ProgramData", "Riot Games")
	rec := filepath.Join(x, "_vnet", riotMetaRecord)
	wantTo := strings.ReplaceAll(x, "/", `\`)

	// Bố cục cũ: chép nguyên văn như trước.
	if vnet, from, to, err := riotRewriteFor(filepath.Join(base, "Games", "_riot_metadata", "Riot Games")); vnet || from != "" || to != "" || err != nil {
		t.Errorf("bố cục cũ: vnet=%v from=%q to=%q err=%v", vnet, from, to, err)
	}
	// Bố cục _vnet chưa có bản ghi: giữ nguyên chữ, vẫn so nội dung. Hoa thường khác hay
	// dấu phân cách ở cuối vẫn nhận ra.
	for _, s := range []string{src, filepath.Join(x, "_VNET", "programdata", "riot games") + string(filepath.Separator)} {
		if vnet, from, to, err := riotRewriteFor(s); !vnet || from != "" || to != "" || err != nil {
			t.Errorf("%s không bản ghi: vnet=%v from=%q to=%q err=%v", s, vnet, from, to, err)
		}
	}
	for _, body := range []string{`D:\GameMaster\Riot Games`, "  D:/GameMaster/Riot Games/ \r\n"} {
		write(t, rec, body, m)
		if vnet, from, to, err := riotRewriteFor(src); !vnet || from != `D:\GameMaster\Riot Games` || to != wantTo || err != nil {
			t.Errorf("bản ghi %q: vnet=%v from=%q to=%q err=%v", body, vnet, from, to, err)
		}
	}
	// Bản ghi cuối cùng hợp lệ sau TrimSpace: chỉ giới hạn 1024 byte mới chặn được nó.
	for _, body := range []string{`D:\`, `..\x`, `D:\` + strings.Repeat("a", 2048), `D:\GameMaster\Riot Games` + strings.Repeat(" ", 2048)} {
		write(t, rec, body, m)
		if _, _, _, err := riotRewriteFor(src); err == nil {
			t.Errorf("bản ghi %.20q lẽ ra bị từ chối", body)
		}
	}
	if err := os.Remove(rec); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(rec, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := riotRewriteFor(src); err == nil {
		t.Error("bản ghi là thư mục lẽ ra bị từ chối")
	}

	// X là gốc ổ đĩa: không đổi được đường dẫn master, cũng không được chép nguyên văn.
	root := filepath.VolumeName(base) + string(filepath.Separator)
	if _, _, _, err := riotRewriteFor(filepath.Join(root, "_vnet", "ProgramData", "Riot Games")); err == nil {
		t.Error("X là gốc ổ lẽ ra bị từ chối")
	}

	// Bản ghi trùng X (khác hoa thường): không cần đổi. Bản ghi phải là đường dẫn ổ đĩa,
	// nên ngoài Windows X là thư mục tương đối "D:/Riot Games" dưới base.
	same := x
	if runtime.GOOS != "windows" {
		t.Chdir(base)
		same = filepath.Join("D:", "Riot Games")
	}
	write(t, filepath.Join(same, "_vnet", riotMetaRecord), strings.ToUpper(strings.ReplaceAll(same, "/", `\`)), m)
	if vnet, from, _, err := riotRewriteFor(filepath.Join(same, "_vnet", "ProgramData", "Riot Games")); !vnet || from != "" || err != nil {
		t.Errorf("bản ghi trùng X: vnet=%v from=%q err=%v", vnet, from, err)
	}
}

// Bản chụp của master trên ổ khác: tệp chữ được đổi sang thư mục game máy trạm thấy,
// mỗi chỗ giữ kiểu dấu của nó; tệp khác và bản ghi thì không.
func TestSyncRiotMetadataRewrites(t *testing.T) {
	x := filepath.Join(t.TempDir(), "Riot Games")
	src := filepath.Join(x, "_vnet", "ProgramData", "Riot Games")
	dst := t.TempDir()
	m := time.Date(2026, 10, 8, 11, 22, 19, 0, time.UTC)
	w := strings.ReplaceAll(x, "/", `\`)
	slash, esc := strings.ReplaceAll(w, `\`, "/"), strings.ReplaceAll(w, `\`, `\\`)
	write(t, filepath.Join(x, "_vnet", riotMetaRecord), `D:\GameMaster\Riot Games`, m)
	yaml := filepath.Join("Metadata", "lol", "x.product_settings.yaml")
	files := map[string][2]string{ // rel → {nguồn, đích mong muốn}
		"RiotClientInstalls.json": {
			`{"rc_default": "D:/GameMaster/Riot Games/Riot Client/RiotClientServices.exe"}`,
			`{"rc_default": "` + slash + `/Riot Client/RiotClientServices.exe"}`},
		yaml: {
			"product_install_full_path: D:\\GameMaster\\Riot Games\\League of Legends\nproduct_install_root: 'D:\\GameMaster\\Riot Games'\n",
			"product_install_full_path: " + w + "\\League of Legends\nproduct_install_root: '" + w + "'\n"},
		"p.json": {
			`{"p": "D:\\GameMaster\\Riot Games\\League of Legends"}`,
			`{"p": "` + esc + `\\League of Legends"}`},
		"x.manifest": {"bin D:/GameMaster/Riot Games bin", "bin D:/GameMaster/Riot Games bin"},
	}
	for rel, c := range files {
		write(t, filepath.Join(src, rel), c[0], m)
	}
	done := map[string]riotStamp{}
	if n, err := syncRiotMetadata(src, dst, done); err != nil || n != 4 {
		t.Fatalf("lần 1: n=%d err=%v, muốn 4", n, err)
	}
	for rel, c := range files {
		p := filepath.Join(dst, rel)
		if b, _ := os.ReadFile(p); string(b) != c[1] {
			t.Errorf("%s = %q, muốn %q", rel, b, c[1])
		}
		if st, err := os.Stat(p); err != nil || !st.ModTime().Equal(m) {
			t.Errorf("%s: giờ sửa khác nguồn (err=%v)", rel, err)
		}
	}
	if got := riotFilesIn(t, dst); len(got) != 4 {
		t.Errorf("đích có %v, muốn đúng 4 tệp (không có %s)", got, riotMetaRecord)
	}

	if n, err := syncRiotMetadata(src, dst, done); err != nil || n != 0 {
		t.Fatalf("lần 2: n=%d err=%v, muốn 0", n, err)
	}
	// Riot ghi lại tệp trên máy: nguồn không đổi thì để nguyên.
	write(t, filepath.Join(dst, yaml), "riot", m.Add(time.Hour))
	if n, err := syncRiotMetadata(src, dst, done); err != nil || n != 0 {
		t.Fatalf("lần 3: n=%d err=%v, muốn 0", n, err)
	}
	if b, _ := os.ReadFile(filepath.Join(dst, yaml)); string(b) != "riot" {
		t.Errorf("chép đè lên tệp Riot vừa ghi: %q", b)
	}
	// Master chụp bản mới.
	write(t, filepath.Join(src, yaml), "product_install_root: D:/GameMaster/Riot Games\n", m.Add(2*time.Hour))
	if n, err := syncRiotMetadata(src, dst, done); err != nil || n != 1 {
		t.Fatalf("lần 4: n=%d err=%v, muốn 1", n, err)
	}
	if b, _ := os.ReadFile(filepath.Join(dst, yaml)); string(b) != "product_install_root: "+slash+"\n" {
		t.Errorf("yaml = %q", b)
	}
}

// Cỡ + giờ sửa trùng không đủ: bản chép nguyên văn cũ có thể dài đúng bằng bản đã đổi.
func TestSyncRiotMetadataRewriteFixesSameSizeStale(t *testing.T) {
	m := time.Date(2026, 10, 8, 11, 22, 19, 0, time.UTC)
	x := filepath.Join(t.TempDir(), "Riot Games")
	src := filepath.Join(x, "_vnet", "ProgramData", "Riot Games")
	to := strings.ReplaceAll(x, "/", `\`)
	from := `D:\` + strings.Repeat("G", len(to)-len(`D:\`)-len(`\Riot Games`)) + `\Riot Games`
	write(t, filepath.Join(x, "_vnet", riotMetaRecord), from, m)
	body := "product_install_root: '" + from + "'\n"
	fixed := "product_install_root: '" + to + "'\n"
	write(t, filepath.Join(src, "a.yaml"), body, m)

	// Đích còn bản chép nguyên văn, cùng cỡ cùng giờ sửa: vẫn phải đổi.
	dst := t.TempDir()
	write(t, filepath.Join(dst, "a.yaml"), body, m)
	if n, err := syncRiotMetadata(src, dst, map[string]riotStamp{}); err != nil || n != 1 {
		t.Fatalf("đích nguyên văn: n=%d err=%v, muốn 1", n, err)
	}
	if b, _ := os.ReadFile(filepath.Join(dst, "a.yaml")); string(b) != fixed {
		t.Errorf("a.yaml = %q, muốn %q", b, fixed)
	}

	// Đích đã đúng: không chép, nhưng ghi nhận.
	dst = t.TempDir()
	write(t, filepath.Join(dst, "a.yaml"), fixed, m)
	done := map[string]riotStamp{}
	if n, err := syncRiotMetadata(src, dst, done); err != nil || n != 0 {
		t.Fatalf("đích đúng: n=%d err=%v, muốn 0", n, err)
	}
	if _, ok := done["a.yaml"]; !ok {
		t.Error("đích đúng mà không ghi nhận vào done")
	}

	// Không có bản ghi (from==""): tệp chữ vẫn so theo nội dung.
	x = filepath.Join(t.TempDir(), "Riot Games")
	src = filepath.Join(x, "_vnet", "ProgramData", "Riot Games")
	write(t, filepath.Join(src, "a.yaml"), "patch: 2\n", m)
	dst = t.TempDir()
	write(t, filepath.Join(dst, "a.yaml"), "patch: 1\n", m)
	if n, err := syncRiotMetadata(src, dst, map[string]riotStamp{}); err != nil || n != 1 {
		t.Fatalf("không bản ghi: n=%d err=%v, muốn 1", n, err)
	}
	if b, _ := os.ReadFile(filepath.Join(dst, "a.yaml")); string(b) != "patch: 2\n" {
		t.Errorf("a.yaml = %q, muốn bản nguồn", b)
	}
}

// Bản ghi hỏng: không chép gì, kẻo đường dẫn master lọt vào ProgramData.
func TestSyncRiotMetadataBadRecord(t *testing.T) {
	m := time.Date(2026, 10, 8, 11, 22, 19, 0, time.UTC)
	x := filepath.Join(t.TempDir(), "Riot Games")
	src := filepath.Join(x, "_vnet", "ProgramData", "Riot Games")
	dst := t.TempDir()
	write(t, filepath.Join(src, "RiotClientInstalls.json"), `{"rc_default": "D:/GameMaster/Riot Games"}`, m)
	write(t, filepath.Join(x, "_vnet", riotMetaRecord), `D:\`, m)
	if n, err := syncRiotMetadata(src, dst, map[string]riotStamp{}); err == nil || n != 0 {
		t.Fatalf("n=%d err=%v, muốn lỗi và 0 tệp", n, err)
	}
	if entries, _ := os.ReadDir(dst); len(entries) != 0 {
		t.Errorf("đã chép: %v", entries)
	}
}

// Nguồn dài hay ngắn hơn cỡ đã liệt kê (khách sửa tệp giữa hai lượt): không ghi gì.
func TestRiotWriteKeepTimeBounded(t *testing.T) {
	dst := t.TempDir()
	m := time.Date(2026, 10, 8, 11, 22, 19, 0, time.UTC)
	target := filepath.Join(dst, "a", "x.json")
	for _, body := range []string{"12345", "123"} {
		if err := writeKeepTime(dst, target, strings.NewReader(body), 4, m); err == nil {
			t.Errorf("nguồn %q với cỡ 4 lẽ ra lỗi", body)
		}
		for _, p := range []string{target, target + ".vnet-tmp"} {
			if _, err := os.Lstat(p); err == nil {
				t.Errorf("nguồn %q: còn %s", body, p)
			}
		}
	}
	if err := writeKeepTime(dst, target, strings.NewReader("1234"), 4, m); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(target); string(b) != "1234" {
		t.Errorf("đích = %q", b)
	}
	if st, err := os.Stat(target); err != nil || !st.ModTime().Equal(m) {
		t.Errorf("giờ sửa không đặt được (err=%v)", err)
	}
}
