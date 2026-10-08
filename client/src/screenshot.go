package main

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/jpeg"

	"golang.org/x/image/draw"
)

// mucChup là kích thước và chất lượng của một lần chụp.
type mucChup struct {
	maxEdge int // cạnh dài nhất sau khi thu nhỏ; ảnh nhỏ hơn giữ nguyên
	quality int // chất lượng JPEG
}

var (
	// Quầy chụp màn hình từ xa để xem khách đang làm gì: giữ nguyên độ phân
	// giải với màn 1080p/1440p. Bản cũ ép cạnh dài về 1280 bằng hệ số NGUYÊN,
	// nên màn 1920×1080 chỉ còn 960×540 (chia đôi) — chữ trên màn hình không
	// đọc nổi khi phóng to.
	chupTuXa = mucChup{maxEdge: 2560, quality: 88}
	// Khách tự chụp gửi vào chat: ảnh nằm thẳng trong tin nhắn (data URI) nên
	// nhỏ hơn một chút, vẫn đủ đọc chữ.
	chupChoChat = mucChup{maxEdge: 1600, quality: 80}
)

// encodeScreenshot thu nhỏ (nếu cần) rồi mã hoá thành data URI.
//
// Tách khỏi phần chụp bằng GDI để kiểm được: bước gọi Windows không chạy thử
// trên máy phát triển, nhưng toàn bộ phần xử lý ảnh thì có.
func encodeScreenshot(img *image.RGBA, m mucChup) (string, error) {
	var out bytes.Buffer
	if err := jpeg.Encode(&out, thuNho(img, m.maxEdge), &jpeg.Options{Quality: m.quality}); err != nil {
		return "", err
	}
	return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(out.Bytes()), nil
}

// thuNho đưa ảnh về đúng kích thước sao cho cạnh dài bằng maxEdge, giữ tỉ lệ.
//
// Dùng bộ lọc Catmull-Rom chứ không chia theo hệ số nguyên: hệ số nguyên nhảy
// thẳng từ 1 lên 2, tức là ảnh hơi lớn hơn ngưỡng một chút cũng bị chia đôi.
// Catmull-Rom giữ nét chữ sắc khi thu nhỏ — đọc được chữ là lý do để chụp.
func thuNho(src *image.RGBA, maxEdge int) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if maxEdge <= 0 || (w <= maxEdge && h <= maxEdge) {
		return src
	}
	nw, nh := maxEdge, h*maxEdge/w
	if h > w {
		nw, nh = w*maxEdge/h, maxEdge
	}
	if nw < 1 || nh < 1 {
		return src
	}
	dst := image.NewRGBA(image.Rect(0, 0, nw, nh))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, b, draw.Src, nil)
	return dst
}
