//go:build !windows

package main

import (
	"errors"
	"time"
)

// awakeTime: chỉ Windows có máy ngủ cần phát hiện (Lightning Boot của Cloud
// Update); nơi khác trả 0 = không biết, phần phát hiện bỏ qua.
func awakeTime() time.Duration { return 0 }

// sleepMachine: chỉ máy trạm Windows mới cho ngủ chờ.
func sleepMachine() error { return errors.New("chỉ hỗ trợ trên Windows") }
