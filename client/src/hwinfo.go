package main

import (
	"log"
	"math"
	"runtime"
	"strings"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
)

// MachineSpecs là cấu hình máy — thứ KHÔNG đổi giữa hai lần báo cáo.
//
// Bốn cột cpu_name / gpu_name / ram_gb / storage_gb có sẵn trong bảng machines
// và hiện trên trang Cài đặt của máy trạm, nhưng trước đây chỉ nhập tay ở trang
// quản trị: người vận hành phải đi từng máy, xem cấu hình rồi gõ lại. Máy tự
// đọc thì vừa chính xác hơn vừa không phải làm lại khi thay linh kiện.
type MachineSpecs struct {
	CPUName   string
	GPUName   string
	RAMGB     int
	StorageGB int
}

// readMachineSpecs đo một lần lúc khởi động. Trường nào không đọc được thì để
// trống, và máy chủ giữ nguyên giá trị đang có thay vì ghi đè bằng số rỗng.
func readMachineSpecs() MachineSpecs {
	var s MachineSpecs

	if info, err := cpu.Info(); err == nil && len(info) > 0 {
		// ModelName của Intel thường có hai ba khoảng trắng liền nhau.
		s.CPUName = strings.Join(strings.Fields(info[0].ModelName), " ")
	} else if err != nil {
		log.Printf("cấu hình máy: không đọc được CPU: %v", err)
	}

	if vm, err := mem.VirtualMemory(); err == nil {
		s.RAMGB = bytesToGB(vm.Total)
	} else {
		log.Printf("cấu hình máy: không đọc được RAM: %v", err)
	}

	if du, err := disk.Usage(systemDiskRoot()); err == nil {
		s.StorageGB = bytesToGB(du.Total)
	} else {
		log.Printf("cấu hình máy: không đọc được ổ đĩa: %v", err)
	}

	s.GPUName = readGPUName()
	return s
}

// bytesToGB làm tròn lên theo bội số quen thuộc.
//
// Thanh RAM 16GB báo về khoảng 15,9 GiB vì phần cứng giữ lại một ít; cắt xuống
// sẽ ghi vào database là 15GB, và người vận hành nhìn trang Máy sẽ tưởng máy bị
// thiếu một thanh. Làm tròn số gần nhất cho đúng cái người ta mua.
func bytesToGB(b uint64) int {
	if b == 0 {
		return 0
	}
	return int(math.Round(float64(b) / (1024 * 1024 * 1024)))
}

// systemDiskRoot trả về gốc ổ đĩa hệ thống.
//
// disk.Usage("/") trên Windows không trỏ tới đâu cả — máy trạm chỉ chạy trên
// Windows, nên đường dẫn "/" cứng trong bản cũ nghĩa là % ổ đĩa gửi lên luôn có
// khả năng bằng 0 mà không báo lỗi gì.
func systemDiskRoot() string {
	if runtime.GOOS == "windows" {
		return windowsSystemDrive()
	}
	return "/"
}
