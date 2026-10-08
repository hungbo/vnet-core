package main

import "testing"

// Đường dẫn tệp chạy game đi thẳng vào ShellExecute, và launcher là chuỗi quản
// trị viên gõ vào trang quản trị. Đây là bài kiểm cho rào chắn duy nhất giữa
// hai thứ đó.
//
// Hàm này thuần và tự xử lý dấu `\`, nên chạy được trên Mac/Linux dù đường dẫn
// là của Windows.
func TestResolveGameLaunch_HopLe(t *testing.T) {
	cases := []struct {
		ten                  string
		root, name, launcher string
		wantExe, wantDir     string
	}{
		{"đường dẫn thường", `D:\Games`, "lol", `Bin\Game.exe`,
			`D:\Games\lol\Bin\Game.exe`, `D:\Games\lol\Bin`},
		{"launcher dùng dấu gạch xuôi", `D:\Games`, "lol", `Bin/Game.exe`,
			`D:\Games\lol\Bin\Game.exe`, `D:\Games\lol\Bin`},
		{"trộn hai kiểu gạch", `D:\Games`, "lol", `Riot Games/Riot Client\RiotClientServices.exe`,
			`D:\Games\lol\Riot Games\Riot Client\RiotClientServices.exe`, `D:\Games\lol\Riot Games\Riot Client`},
		{"tệp ngay trong thư mục game", `D:\Games`, "lol", `Game.exe`,
			`D:\Games\lol\Game.exe`, `D:\Games\lol`},
		{"gốc có gạch ở cuối", `D:\Games\`, "lol", `Bin\Game.exe`,
			`D:\Games\lol\Bin\Game.exe`, `D:\Games\lol\Bin`},
		{"gốc viết bằng gạch xuôi", `D:/Games`, "lol", `Bin\Game.exe`,
			`D:\Games\lol\Bin\Game.exe`, `D:\Games\lol\Bin`},
		{"gốc là cả ổ đĩa", `E:\`, "lol", `Game.exe`,
			`E:\lol\Game.exe`, `E:\lol`},
		// Máy chủ chuẩn hoá `E:\` thành `E:` (xem TestNormalizeGameClientRoot).
		{"gốc là ổ đĩa như máy chủ trả", `E:`, "lol", `Bin\Game.exe`,
			`E:\lol\Bin\Game.exe`, `E:\lol\Bin`},
		{"gốc có khoảng trắng hai đầu", "  D:\\Games  ", "lol", `Game.exe`,
			`D:\Games\lol\Game.exe`, `D:\Games\lol`},
		{"khoảng trắng hai đầu launcher", `D:\Games`, "lol", " Bin/Game.exe  ",
			`D:\Games\lol\Bin\Game.exe`, `D:\Games\lol\Bin`},
		{"tên game có dấu và khoảng trắng", `D:\Games`, "Liên Minh", `Bin\Game.exe`,
			`D:\Games\Liên Minh\Bin\Game.exe`, `D:\Games\Liên Minh\Bin`},

		// Hoa thường: Windows không phân biệt nên chuỗi nhập thế nào thì giữ
		// nguyên thế đó, không được hạ chữ thường làm lệch tên thư mục thật.
		{"chữ hoa trong launcher giữ nguyên", `D:\Games`, "lol", `BIN/GAME.EXE`,
			`D:\Games\lol\BIN\GAME.EXE`, `D:\Games\lol\BIN`},
		{"ổ và gốc viết thường", `d:\games`, "LoL", `bin\game.exe`,
			`d:\games\LoL\bin\game.exe`, `d:\games\LoL\bin`},
	}
	for _, c := range cases {
		t.Run(c.ten, func(t *testing.T) {
			exe, dir, err := resolveGameLaunch(c.root, c.name, c.launcher)
			if err != nil {
				t.Fatalf("lỗi không mong đợi: %v", err)
			}
			if exe != c.wantExe || dir != c.wantDir {
				t.Errorf("exe=%q dir=%q, mong exe=%q dir=%q", exe, dir, c.wantExe, c.wantDir)
			}
		})
	}
}

func TestResolveGameLaunch_BiTuChoi(t *testing.T) {
	const root, name = `D:\Games`, "lol"

	launchers := []struct{ ten, launcher string }{
		{"rỗng", ""},
		{"toàn khoảng trắng", "   "},

		// Thoát khỏi thư mục game.
		{"chỉ có ..", `..`},
		{"lên một cấp", `..\evil.exe`},
		{"lên một cấp, gạch xuôi", `../evil.exe`},
		{"lên giữa đường", `Bin/../../evil.exe`},
		{"lên giữa đường, gạch ngược", `Bin\..\..\evil.exe`},
		{"cuối đường là ..", `Bin/..`},
		{".. vẫn nằm trong thư mục game", `Bin/../Game.exe`},
		{"dấu chấm đơn", `./Game.exe`},
		{"dấu chấm đơn giữa đường", `Bin/./Game.exe`},
		{"ba dấu chấm", `.../evil.exe`},
		{".. kèm khoảng trắng", `Bin/.. /evil.exe`},
		{"thư mục kết thúc bằng khoảng trắng", `Bin /Game.exe`},
		{"tệp kết thúc bằng dấu chấm", `Bin/Game.exe.`},

		// Tuyệt đối, UNC, ổ đĩa.
		{"tuyệt đối có ổ đĩa", `C:\Windows\System32\cmd.exe`},
		{"tuyệt đối có ổ đĩa, gạch xuôi", `C:/Windows/System32/cmd.exe`},
		{"theo ổ đĩa, không gạch", `C:evil.exe`},
		{"tuyệt đối không ổ đĩa", `\Windows\System32\cmd.exe`},
		{"tuyệt đối kiểu Unix", `/etc/passwd`},
		{"UNC gạch ngược", `\\server\share\evil.exe`},
		{"UNC gạch xuôi", `//server/share/evil.exe`},
		{"tiền tố \\?\\", `\\?\C:\evil.exe`},
		{"luồng dữ liệu phụ NTFS", `Bin\Game.exe:evil`},

		// Ký tự không thể có trong tên tệp.
		{"NUL", "Bin/Game.exe\x00.txt"},
		{"ký tự điều khiển", "Bin/Game\n.exe"},
		{"dấu sao", `Bin/Ga*me.exe`},
		{"dấu hỏi", `Bin/Game?.exe`},
		{"dấu |", `Bin/Game|calc.exe`},
		{"hai gạch liền nhau", `Bin//Game.exe`},
		{"kết thúc bằng gạch", `Bin/`},
	}
	for _, c := range launchers {
		t.Run("launcher/"+c.ten, func(t *testing.T) {
			if exe, dir, err := resolveGameLaunch(root, name, c.launcher); err == nil {
				t.Errorf("phải từ chối %q, nhưng ra exe=%q dir=%q", c.launcher, exe, dir)
			}
		})
	}

	names := []struct{ ten, name string }{
		{"rỗng", ""},
		{"..", ".."},
		{".", "."},
		{"có gạch ngược", `a\b`},
		{"có gạch xuôi", `a/b`},
		{"đi lên bằng gạch", `../x`},
		{"kiểu ổ đĩa", "C:"},
		{"kết thúc bằng khoảng trắng", "lol "},
		{"NUL", "lol\x00"},
	}
	for _, c := range names {
		t.Run("tên/"+c.ten, func(t *testing.T) {
			if exe, _, err := resolveGameLaunch(root, c.name, `Game.exe`); err == nil {
				t.Errorf("phải từ chối tên %q, nhưng ra exe=%q", c.name, exe)
			}
		})
	}

	roots := []struct{ ten, root string }{
		{"rỗng", ""},
		{"theo ổ đĩa, không gạch", `D:Games`},
		{"không có ổ đĩa", `Games`},
		{"tuyệt đối không ổ đĩa", `\Games`},
		{"UNC", `\\server\share`},
		{"UNC gạch xuôi", `//server/share`},
		{"tiền tố \\?\\", `\\?\D:\Games`},
		{"có ..", `D:\Games\..\Windows`},
		{"kết thúc bằng ..", `D:\Games\..`},
		{"gạch đôi giữa đường", `D:\\Games`},
		{"NUL", "D:\\Games\x00"},
	}
	for _, c := range roots {
		t.Run("gốc/"+c.ten, func(t *testing.T) {
			if exe, _, err := resolveGameLaunch(c.root, name, `Game.exe`); err == nil {
				t.Errorf("phải từ chối gốc %q, nhưng ra exe=%q", c.root, exe)
			}
		})
	}
}

// insideDir là lần kiểm thứ hai, chạy trên đường dẫn ĐÃ làm sạch. Windows không
// phân biệt hoa thường, nên "d:/games/LOL/x.exe" vẫn là nằm trong "D:/Games/lol".
func TestInsideDir(t *testing.T) {
	cases := []struct {
		ten    string
		dir, p string
		want   bool
	}{
		{"nằm trong", "D:/Games/lol", "D:/Games/lol/Bin/x.exe", true},
		{"khác hoa thường", "D:/Games/lol", "d:/games/LOL/Bin/x.exe", true},
		{"tên chữ Việt khác hoa thường", "D:/Games/Liên Minh", "d:/games/LIÊN MINH/x.exe", true},
		{"chính thư mục không tính", "D:/Games/lol", "D:/Games/lol", false},
		{"thư mục anh em trùng tiền tố", "D:/Games/lol", "D:/Games/lol2/x.exe", false},
		{"thư mục khác", "D:/Games/lol", "D:/Games/valorant/x.exe", false},
		{"ổ khác", "D:/Games/lol", "C:/Games/lol/x.exe", false},
		{"ngắn hơn thư mục", "D:/Games/lol", "D:/Games", false},
	}
	for _, c := range cases {
		t.Run(c.ten, func(t *testing.T) {
			if got := insideDir(c.dir, c.p); got != c.want {
				t.Errorf("insideDir(%q, %q) = %v, mong %v", c.dir, c.p, got, c.want)
			}
		})
	}
}
