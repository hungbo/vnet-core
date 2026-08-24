package main

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/jpeg"
)

const (
	// Cạnh dài nhất sau khi thu nhỏ. Ảnh 1920×1080 mã hoá base64 rồi nhét vào
	// một tin nhắn chat là vài trăm KB; thu nhỏ trước khi mã hoá giữ tin nhắn
	// ở mức gửi được mà vẫn đọc được chữ trên màn hình.
	maxScreenshotEdge = 1280
	jpegQuality       = 72
)

// encodeScreenshot thu nhỏ rồi mã hoá thành data URI để gửi kèm tin nhắn chat.
//
// Tách khỏi phần chụp bằng GDI để kiểm được: bước gọi Windows không chạy thử
// trên máy phát triển, nhưng toàn bộ phần xử lý ảnh thì có.
func encodeScreenshot(img *image.RGBA) (string, error) {
	var out bytes.Buffer
	if err := jpeg.Encode(&out, downscale(img, maxScreenshotEdge),
		&jpeg.Options{Quality: jpegQuality}); err != nil {
		return "", err
	}
	return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(out.Bytes()), nil
}

// downscale thu nhỏ ảnh theo hệ số nguyên bằng cách lấy trung bình từng khối.
//
// Lấy trung bình chứ không lấy mẫu điểm: chữ trên màn hình bị lấy mẫu điểm sẽ
// đứt nét và không đọc được, mà đọc được chữ chính là lý do khách gửi ảnh.
//
// Hệ số nguyên đủ dùng ở đây và tránh hẳn phép nội suy: mục tiêu là tin nhắn
// gửi được, không phải ảnh đẹp.
func downscale(src *image.RGBA, maxEdge int) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if maxEdge <= 0 || (w <= maxEdge && h <= maxEdge) {
		return src
	}

	factor := 1
	for (w/(factor+1)) > maxEdge || (h/(factor+1)) > maxEdge {
		factor++
	}
	factor++
	if factor < 2 {
		return src
	}

	nw, nh := w/factor, h/factor
	if nw < 1 || nh < 1 {
		return src
	}

	dst := image.NewRGBA(image.Rect(0, 0, nw, nh))
	area := factor * factor
	for y := 0; y < nh; y++ {
		for x := 0; x < nw; x++ {
			var r, g, bl int
			for dy := 0; dy < factor; dy++ {
				row := (y*factor + dy) * w
				for dx := 0; dx < factor; dx++ {
					i := (row + x*factor + dx) * 4
					r += int(src.Pix[i])
					g += int(src.Pix[i+1])
					bl += int(src.Pix[i+2])
				}
			}
			o := (y*nw + x) * 4
			dst.Pix[o] = uint8(r / area)
			dst.Pix[o+1] = uint8(g / area)
			dst.Pix[o+2] = uint8(bl / area)
			dst.Pix[o+3] = 0xFF
		}
	}
	return dst
}
