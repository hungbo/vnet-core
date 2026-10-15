//go:build !windows

package main

import "time"

// awakeTime: chỉ Windows có máy ngủ cần phát hiện (Lightning Boot của Cloud
// Update); nơi khác trả 0 = không biết, phần phát hiện bỏ qua.
func awakeTime() time.Duration { return 0 }
