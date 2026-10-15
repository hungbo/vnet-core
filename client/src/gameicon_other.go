//go:build !windows

package main

import "fmt"

// Icon game chỉ lấy được trên máy trạm Windows. Bản này để phần còn lại biên
// dịch được khi phát triển trên Mac/Linux; GameIcon khi đó trả "".
func extractIconPixels(path string, size int) (*iconPixels, error) {
	return nil, fmt.Errorf("%w: chỉ lấy được icon trên Windows", errNoGameIcon)
}
