package service

import "testing"

// Khung giờ chồng nhau phải bị chặn lúc nhập: truy vấn ở session.go dùng
// First() không sắp xếp, nên hai khung chồng nhau cho ra giá nào là tuỳ
// database trả về trước — không phân định được.
func TestClockWindowsOverlap(t *testing.T) {
	cases := []struct {
		name                   string
		aS, aE, bS, bE string
		want           bool
	}{
		{"rời hẳn nhau", "08:00", "12:00", "13:00", "17:00", false},
		{"chạm đầu đuôi không tính là chồng", "08:00", "12:00", "12:00", "17:00", false},
		{"chồng một phần", "08:00", "12:00", "11:00", "17:00", true},
		{"lồng hẳn bên trong", "08:00", "18:00", "10:00", "12:00", true},
		{"trùng khít", "08:00", "12:00", "08:00", "12:00", true},

		// Khung vắt qua nửa đêm — quán net chạy xuyên đêm nên đây là khung
		// cao điểm phổ biến nhất.
		{"vắt nửa đêm, chồng phần tối", "22:00", "02:00", "23:00", "23:30", true},
		{"vắt nửa đêm, chồng phần sáng", "22:00", "02:00", "00:30", "01:00", true},
		{"vắt nửa đêm, không chồng", "22:00", "02:00", "08:00", "12:00", false},
		{"hai khung cùng vắt nửa đêm", "22:00", "02:00", "23:00", "03:00", true},
		{"khung thường nằm gọn giữa khung vắt", "23:00", "01:00", "10:00", "20:00", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := clockWindowsOverlap(c.aS, c.aE, c.bS, c.bE); got != c.want {
				t.Errorf("%s–%s vs %s–%s = %v, mong %v", c.aS, c.aE, c.bS, c.bE, got, c.want)
			}
			// Chồng nhau phải đối xứng.
			if got := clockWindowsOverlap(c.bS, c.bE, c.aS, c.aE); got != c.want {
				t.Errorf("đảo chiều: %s–%s vs %s–%s = %v, mong %v", c.bS, c.bE, c.aS, c.aE, got, c.want)
			}
		})
	}
}

func TestClockSegmentsSplitsAtMidnight(t *testing.T) {
	if got := clockSegments("08:00", "12:00"); len(got) != 1 || got[0] != [2]int{480, 720} {
		t.Errorf("khung thường = %v, mong một đoạn 480–720", got)
	}
	got := clockSegments("22:00", "02:00")
	if len(got) != 2 || got[0] != [2]int{1320, 1440} || got[1] != [2]int{0, 120} {
		t.Errorf("khung vắt nửa đêm = %v, mong hai đoạn 1320–1440 và 0–120", got)
	}
}

func TestValidClock(t *testing.T) {
	for _, ok := range []string{"00:00", "08:30", "23:59"} {
		if !validClock(ok) {
			t.Errorf("từ chối giờ hợp lệ %q", ok)
		}
	}
	for _, bad := range []string{"", "8:30", "24:00", "12:60", "12-30", "123:45"} {
		if validClock(bad) {
			t.Errorf("chấp nhận giờ sai %q", bad)
		}
	}
}

// Driver trả cột date thành timestamp đầy đủ; so chuỗi với dạng ngày sẽ luôn
// sai và ô chọn ngày trên giao diện cũng không đọc được.
func TestDateOnly(t *testing.T) {
	cases := map[string]string{
		"2026-08-22T00:00:00Z":       "2026-08-22",
		"2026-08-22T00:00:00+07:00":  "2026-08-22",
		"2026-08-22":                 "2026-08-22",
		"":                           "",
		"2026":                       "2026",
	}
	for in, want := range cases {
		if got := dateOnly(in); got != want {
			t.Errorf("dateOnly(%q) = %q, mong %q", in, got, want)
		}
	}
}

func TestOptionalDate(t *testing.T) {
	if optionalDate("") != nil || optionalDate("   ") != nil {
		t.Error("chuỗi rỗng phải thành NULL — cột date từ chối chuỗi rỗng")
	}
	if v := optionalDate("2026-12-31"); v == nil || *v != "2026-12-31" {
		t.Errorf("ngày hợp lệ = %v", v)
	}
}
