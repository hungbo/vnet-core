package main

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/jpeg"
	"strings"
	"testing"
)

// Màn 1920×1080 khi quầy chụp từ xa phải GIỮ NGUYÊN độ phân giải. Bản cũ chia
// đôi còn 960×540 — chữ trên màn hình không đọc nổi.
func TestThuNho_ChupTuXaGiuNguyen1080p(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
	out := thuNho(src, chupTuXa.maxEdge)
	if b := out.Bounds(); b.Dx() != 1920 || b.Dy() != 1080 {
		t.Fatalf("ảnh 1080p bị đổi thành %v", b)
	}
}

// Thu nhỏ về ĐÚNG ngưỡng, không chia theo hệ số nguyên: 1703 rộng với ngưỡng
// 1600 phải ra 1600, không phải 851 như bản cũ.
func TestThuNho_DungKichThuocDich(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 1703, 1043))
	b := thuNho(src, 1600).Bounds()
	if b.Dx() != 1600 {
		t.Fatalf("rộng %d, mong 1600", b.Dx())
	}
	if want := 1043 * 1600 / 1703; b.Dy() != want {
		t.Fatalf("cao %d, mong %d (giữ tỉ lệ)", b.Dy(), want)
	}
	// Ảnh dọc: cạnh dài là chiều cao.
	b = thuNho(image.NewRGBA(image.Rect(0, 0, 1080, 1920)), 1600).Bounds()
	if b.Dy() != 1600 || b.Dx() != 1080*1600/1920 {
		t.Fatalf("ảnh dọc ra %v", b)
	}
}

// Thu nhỏ vẫn giữ màu từng vùng: nửa trái đen, nửa phải trắng thì sau khi thu
// nhỏ hai mép vẫn đen và trắng, không thành một mảng xám.
func TestThuNho_GiuTuongPhan(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 400, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 400; x++ {
			c := color.RGBA{0, 0, 0, 255}
			if x >= 200 {
				c = color.RGBA{255, 255, 255, 255}
			}
			src.Set(x, y, c)
		}
	}
	out := thuNho(src, 200)
	l := color.RGBAModel.Convert(out.At(10, 25)).(color.RGBA)
	r := color.RGBAModel.Convert(out.At(190, 25)).(color.RGBA)
	if l.R > 20 || r.R < 235 {
		t.Fatalf("mất tương phản: trái %v, phải %v", l, r)
	}
}

func TestThuNho_AnhNhoGiuNguyen(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 800, 600))
	if thuNho(src, 1600) != image.Image(src) {
		t.Fatal("ảnh nhỏ hơn ngưỡng vẫn bị vẽ lại")
	}
}

func TestEncodeScreenshotProducesUsableDataURI(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 200, 120))
	for y := 0; y < 120; y++ {
		for x := 0; x < 200; x++ {
			src.Set(x, y, color.RGBA{uint8(x), uint8(y), 128, 255})
		}
	}

	uri, err := encodeScreenshot(src, chupTuXa)
	if err != nil {
		t.Fatalf("mã hoá lỗi: %v", err)
	}
	const prefix = "data:image/jpeg;base64,"
	if !strings.HasPrefix(uri, prefix) {
		t.Fatalf("thiếu tiền tố data URI: %.40s", uri)
	}

	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(uri, prefix))
	if err != nil {
		t.Fatalf("phần base64 không giải mã được: %v", err)
	}
	// JPEG bắt đầu bằng FF D8 FF.
	if len(raw) < 3 || raw[0] != 0xFF || raw[1] != 0xD8 || raw[2] != 0xFF {
		t.Errorf("dữ liệu không phải JPEG: % x", raw[:min(3, len(raw))])
	}

	back, err := jpeg.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("ảnh không giải mã lại được: %v", err)
	}
	if back.Bounds().Dx() != 200 {
		t.Errorf("kích thước sau vòng mã hoá = %v", back.Bounds())
	}
}

// Ảnh full HD phải ra tin nhắn đủ nhỏ để gửi qua chat.
func TestEncodeScreenshotStaysSmallEnoughToSend(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
	for y := 0; y < 1080; y++ {
		for x := 0; x < 1920; x++ {
			src.Set(x, y, color.RGBA{uint8(x % 256), uint8(y % 256), 100, 255})
		}
	}
	uri, err := encodeScreenshot(src, chupChoChat)
	if err != nil {
		t.Fatal(err)
	}
	// 1 MB là ngưỡng thoáng; ảnh không thu nhỏ dễ vượt xa mức này.
	if len(uri) > 1_000_000 {
		t.Errorf("data URI dài %d byte — quá nặng cho một tin nhắn chat", len(uri))
	}
	if len(uri) < 1000 {
		t.Errorf("data URI chỉ %d byte — nhiều khả năng ảnh rỗng", len(uri))
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
