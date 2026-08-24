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

// Thu nhỏ phải lấy TRUNG BÌNH từng khối, không lấy mẫu điểm: chữ trên màn hình
// bị lấy mẫu điểm sẽ đứt nét, mà đọc được chữ chính là lý do khách gửi ảnh.
func TestDownscaleAveragesBlocks(t *testing.T) {
	// 4×4 chia đôi: nửa trái đen, nửa phải trắng. Thu nhỏ hệ số 2 phải cho ra
	// 2×2 vẫn đen/trắng, không lẫn.
	src := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			c := color.RGBA{0, 0, 0, 255}
			if x >= 2 {
				c = color.RGBA{255, 255, 255, 255}
			}
			src.Set(x, y, c)
		}
	}

	out := downscale(src, 2)
	b := out.Bounds()
	if b.Dx() != 2 || b.Dy() != 2 {
		t.Fatalf("kích thước sau thu nhỏ %dx%d, mong 2x2", b.Dx(), b.Dy())
	}
	r, _, _, _ := out.At(0, 0).RGBA()
	if r>>8 != 0 {
		t.Errorf("ô trái = %d, mong 0 (đen)", r>>8)
	}
	r, _, _, _ = out.At(1, 0).RGBA()
	if r>>8 != 255 {
		t.Errorf("ô phải = %d, mong 255 (trắng)", r>>8)
	}
}

// Trung bình thật sự: khối nửa đen nửa trắng phải ra xám, không phải một trong hai.
func TestDownscaleBlendsMixedBlock(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 2, 2))
	src.Set(0, 0, color.RGBA{0, 0, 0, 255})
	src.Set(1, 0, color.RGBA{255, 255, 255, 255})
	src.Set(0, 1, color.RGBA{0, 0, 0, 255})
	src.Set(1, 1, color.RGBA{255, 255, 255, 255})

	out := downscale(src, 1)
	r, _, _, _ := out.At(0, 0).RGBA()
	got := int(r >> 8)
	if got < 120 || got > 135 {
		t.Errorf("khối trộn = %d, mong khoảng 127 (xám)", got)
	}
}

// Ảnh đã nhỏ hơn ngưỡng thì giữ nguyên, không phóng to.
func TestDownscaleLeavesSmallImages(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 100, 80))
	out := downscale(src, 1280)
	if out.Bounds().Dx() != 100 || out.Bounds().Dy() != 80 {
		t.Errorf("ảnh nhỏ bị đổi kích thước: %v", out.Bounds())
	}
}

// Ảnh rất lớn phải xuống dưới ngưỡng, không được vượt.
func TestDownscaleRespectsMaxEdge(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 3840, 2160))
	out := downscale(src, 1280)
	b := out.Bounds()
	if b.Dx() > 1280 || b.Dy() > 1280 {
		t.Errorf("sau thu nhỏ vẫn %dx%d, vượt ngưỡng 1280", b.Dx(), b.Dy())
	}
	if b.Dx() < 640 {
		t.Errorf("thu nhỏ quá tay: %dx%d", b.Dx(), b.Dy())
	}
}

// Chuỗi trả về phải là data URI mà thẻ <img> hiển thị được ngay — giao diện
// chat nhét thẳng nó vào src, không xử lý gì thêm.
func TestEncodeScreenshotProducesUsableDataURI(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 200, 120))
	for y := 0; y < 120; y++ {
		for x := 0; x < 200; x++ {
			src.Set(x, y, color.RGBA{uint8(x), uint8(y), 128, 255})
		}
	}

	uri, err := encodeScreenshot(src)
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
	uri, err := encodeScreenshot(src)
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
