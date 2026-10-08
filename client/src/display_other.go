//go:build !windows

package main

// Đổi độ phân giải và chuột chỉ có trên máy trạm Windows. Bản này để phần còn lại
// biên dịch được khi phát triển trên Mac/Linux: không đọc được gì, không đổi được
// gì, và GetDisplayInfo báo supported=false để giao diện ẩn cả mục.

func osDisplayHW() displayHW {
	return displayHW{
		modes: func() ([]rawDisplayMode, rawDisplayMode, error) {
			return nil, rawDisplayMode{}, errDisplayUnsupported
		},
		setMode:        func(DisplayMode) error { return errDisplayUnsupported },
		registryMode:   func() (DisplayMode, error) { return DisplayMode{}, errDisplayUnsupported },
		restoreDefault: func() error { return errDisplayUnsupported },
		getMouse:       func() (int, bool, error) { return 0, false, errDisplayUnsupported },
		setSpeed:       func(int) error { return errDisplayUnsupported },
		setPrecision:   func(bool) error { return errDisplayUnsupported },
	}
}
