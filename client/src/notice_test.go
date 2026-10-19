package main

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"
)

// Thông báo đi từ thanh chính sang cửa sổ phụ qua dòng lệnh: chữ có dấu, xuống dòng và
// dấu ngoặc kép phải ra lại y nguyên.
func TestNoticeArg_RoundTrip(t *testing.T) {
	in := Notice{Title: "Thông báo", Body: "Còn 5 phút nữa hết giờ.\nBấm \"Nạp tiền\" để chơi tiếp — cảm ơn bạn!", At: 1760770000000}

	arg := encodeNoticeArg(in)
	if !strings.HasPrefix(arg, "--notice=") {
		t.Fatalf("thiếu tiền tố --notice=: %q", arg)
	}
	// Đi qua dòng lệnh: không được có khoảng trắng hay ký tự cần bọc ngoặc.
	if strings.ContainsAny(arg, " \t\r\n\"'&|<>^%") {
		t.Errorf("tham số chứa ký tự dễ vỡ trên dòng lệnh Windows: %q", arg)
	}

	got, ok := decodeNotice(noticeArgOf([]string{`C:\vnet\vnet-client.exe`, "--window", "notice", arg}))
	if !ok {
		t.Fatal("không đọc lại được thông báo")
	}
	if got != in {
		t.Errorf("đọc lại = %+v, mong %+v", got, in)
	}
}

// Quầy gõ dài tới đâu thì dòng lệnh vẫn dưới giới hạn 32767 ký tự của Windows, và chữ bị
// cắt theo ký tự chứ không chẻ đôi một chữ có dấu.
func TestNoticeArg_CatChuQuaDai(t *testing.T) {
	in := Notice{Title: strings.Repeat("Đ", 500), Body: strings.Repeat("ế", 10000)}

	arg := encodeNoticeArg(in)
	if len(arg) > 20000 {
		t.Errorf("tham số dài %d ký tự, gần giới hạn dòng lệnh Windows", len(arg))
	}
	got, ok := decodeNotice(strings.TrimPrefix(arg, "--notice="))
	if !ok {
		t.Fatal("không đọc lại được")
	}
	if n := utf8.RuneCountInString(got.Body); n != noticeBodyMax {
		t.Errorf("nội dung còn %d ký tự, mong %d", n, noticeBodyMax)
	}
	if n := utf8.RuneCountInString(got.Title); n != noticeTitleMax {
		t.Errorf("tiêu đề còn %d ký tự, mong %d", n, noticeTitleMax)
	}
	if !utf8.ValidString(got.Body) || !strings.HasSuffix(got.Body, "…") {
		t.Errorf("chữ bị cắt không gọn: kết thúc %q", got.Body[len(got.Body)-6:])
	}
}

func TestDecodeNotice_TuChoiRac(t *testing.T) {
	cases := map[string]string{
		"rỗng":                         "",
		"không phải base64":            "!!!không-hợp-lệ!!!",
		"base64 nhưng không phải JSON": "aGVsbG8",
	}
	for ten, v := range cases {
		if _, ok := decodeNotice(v); ok {
			t.Errorf("%s: đáng lẽ bị từ chối", ten)
		}
	}
	// Thông báo không có nội dung thì không có gì để hiện.
	if _, ok := decodeNotice(strings.TrimPrefix(encodeNoticeArg(Notice{Body: "   "}), "--notice=")); ok {
		t.Error("nội dung toàn khoảng trắng đáng lẽ bị từ chối")
	}
}

// Dạng "--notice VALUE" (hai phần tử) cũng đọc được, giống windowArg.
func TestNoticeArgOf_DangTach(t *testing.T) {
	if got := noticeArgOf([]string{"--window", "notice", "--notice", "abc"}); got != "abc" {
		t.Errorf("dạng tách = %q", got)
	}
	if got := noticeArgOf([]string{"--window", "support"}); got != "" {
		t.Errorf("không có --notice mà trả %q", got)
	}
}

func TestAddNotice_ThuTuVaGioiHan(t *testing.T) {
	a := &App{}
	// Hai thông báo cùng mili giây: At vẫn phải tăng dần, vì At là khoá để xoá.
	a.addNotice(Notice{Body: "một", At: 100})
	a.addNotice(Notice{Body: "hai", At: 100})
	if a.notices[0].At >= a.notices[1].At {
		t.Errorf("At không tăng dần: %d, %d", a.notices[0].At, a.notices[1].At)
	}

	for i := 0; i < noticeQueueMax+5; i++ {
		a.addNotice(Notice{Body: "x", At: int64(1000 + i)})
	}
	if len(a.notices) != noticeQueueMax {
		t.Errorf("hàng chờ %d, mong tối đa %d", len(a.notices), noticeQueueMax)
	}
	// Quá giới hạn thì bỏ cái cũ nhất, giữ cái mới nhất.
	if last := a.notices[len(a.notices)-1]; last.At != int64(1000+noticeQueueMax+4) {
		t.Errorf("mất cái mới nhất: At cuối = %d", last.At)
	}
}

func TestNoticesJSON_MoiNhatTruocVaLuonLaMang(t *testing.T) {
	a := &App{}
	if got := a.GetNotices(); got != "[]" {
		t.Errorf("hàng chờ rỗng phải là [], được %q", got)
	}

	a.addNotice(Notice{Body: "cũ", At: 1})
	a.addNotice(Notice{Body: "mới", At: 2})
	var list []Notice
	if err := json.Unmarshal([]byte(a.GetNotices()), &list); err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].Body != "mới" || list[1].Body != "cũ" {
		t.Errorf("thứ tự = %+v, mong mới trước", list)
	}
}

// Khách bấm "Đã hiểu" trong lúc quầy gửi thêm một cái: cái tới sau không được mất.
func TestDismissNotices_GiuLaiCaiToiSau(t *testing.T) {
	a := &App{windowMode: "panel"} // ctx nil nên HidePanel không gọi Wails
	a.addNotice(Notice{Body: "đã đọc", At: 10})
	a.addNotice(Notice{Body: "tới giữa lúc đọc", At: 20})

	left := a.DismissNotices(10)

	var list []Notice
	if err := json.Unmarshal([]byte(left), &list); err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Body != "tới giữa lúc đọc" {
		t.Errorf("còn lại = %+v", list)
	}

	if left := a.DismissNotices(20); left != "[]" {
		t.Errorf("đọc hết rồi mà còn %s", left)
	}
}

func TestFixedGeometry(t *testing.T) {
	for _, m := range manHinhThuongGap {
		t.Run(m.ten, func(t *testing.T) {
			w, h, x, y := fixedGeometry(m.w, m.h, noticeW, noticeH)
			if x < 0 || y < 0 {
				t.Errorf("toạ độ âm: x=%d y=%d", x, y)
			}
			if x+w > m.w || y+h > m.h {
				t.Errorf("tràn màn hình: (%d,%d) %dx%d trong %dx%d", x, y, w, h, m.w, m.h)
			}
			if m.w-dockWidth >= noticeW && x+w > m.w-dockWidth {
				t.Errorf("nằm dưới thanh dọc: mép phải %d, thanh bắt đầu ở %d", x+w, m.w-dockWidth)
			}
		})
	}

	// Màn hình nhỏ hơn cả hộp thông báo: thu hộp lại chứ không để toạ độ âm.
	w, h, x, y := fixedGeometry(400, 300, noticeW, noticeH)
	if w != 400 || h != 300 || x != 0 || y != 0 {
		t.Errorf("màn 400x300: được %dx%d tại (%d,%d)", w, h, x, y)
	}
}

func TestPanelTitles_CoHopThongBao(t *testing.T) {
	if panelTitles[noticeMode] == "" {
		t.Error("chế độ notice thiếu tiêu đề cửa sổ")
	}
}
