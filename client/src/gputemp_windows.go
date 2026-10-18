//go:build windows

package main

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// Nhiệt độ GPU qua D3DKMT — đúng nguồn Task Manager dùng cho cột "GPU
// Temperature" (Windows 10 2004 trở lên, driver WDDM 2.4+). Chạy với NVIDIA,
// AMD và Intel mà không cần driver hay phần mềm riêng của hãng.
//
// gopsutil không có gì cho GPU trên Windows: bản cũ dò tên cảm biến kiểu Linux
// ("nvidia", "amdgpu") nên cột GPU luôn là 0.

var (
	gdi32Kmt                = windows.NewLazySystemDLL("gdi32.dll")
	procD3DKMTEnumAdapters2 = gdi32Kmt.NewProc("D3DKMTEnumAdapters2")
	procD3DKMTQueryAdapter  = gdi32Kmt.NewProc("D3DKMTQueryAdapterInfo")
	procD3DKMTCloseAdapter  = gdi32Kmt.NewProc("D3DKMTCloseAdapter")
)

const kmtqaiAdapterPerfData = 62 // KMTQAITYPE_ADAPTERPERFDATA

type d3dkmtAdapterInfo struct {
	hAdapter   uint32
	luidLow    uint32
	luidHigh   int32
	numSources uint32
	precise    int32
}

type d3dkmtEnumAdapters2 struct {
	numAdapters uint32
	_           uint32
	pAdapters   uintptr
}

type d3dkmtQueryAdapterInfo struct {
	hAdapter uint32
	typ      int32
	pData    uintptr
	size     uint32
	_        uint32
}

// D3DKMT_ADAPTER_PERFDATA. Temperature tính bằng phần mười độ C.
type d3dkmtAdapterPerfData struct {
	physicalAdapterIndex uint32
	_                    uint32
	memoryFrequency      uint64
	maxMemoryFrequency   uint64
	maxMemoryFrequencyOC uint64
	memoryBandwidth      uint64
	pcieBandwidth        uint64
	fanRPM               uint32
	power                uint32
	temperature          uint32
	powerStateOverride   uint8
	_                    [3]byte
}

// readGPUTempD3DKMT trả về nhiệt độ cao nhất trong các card đồ hoạ, hoặc 0 nếu
// không card nào báo (máy ảo, driver cũ, Windows cũ).
func readGPUTempD3DKMT() float64 {
	if procD3DKMTEnumAdapters2.Find() != nil || procD3DKMTQueryAdapter.Find() != nil {
		return 0
	}
	var enum d3dkmtEnumAdapters2
	if r, _, _ := procD3DKMTEnumAdapters2.Call(uintptr(unsafe.Pointer(&enum))); r != 0 || enum.numAdapters == 0 {
		return 0
	}
	adapters := make([]d3dkmtAdapterInfo, enum.numAdapters)
	enum.pAdapters = uintptr(unsafe.Pointer(&adapters[0]))
	if r, _, _ := procD3DKMTEnumAdapters2.Call(uintptr(unsafe.Pointer(&enum))); r != 0 {
		return 0
	}

	var nong float64
	for i := 0; i < int(enum.numAdapters) && i < len(adapters); i++ {
		h := adapters[i].hAdapter
		var perf d3dkmtAdapterPerfData
		q := d3dkmtQueryAdapterInfo{
			hAdapter: h,
			typ:      kmtqaiAdapterPerfData,
			pData:    uintptr(unsafe.Pointer(&perf)),
			size:     uint32(unsafe.Sizeof(perf)),
		}
		if r, _, _ := procD3DKMTQueryAdapter.Call(uintptr(unsafe.Pointer(&q))); r == 0 {
			// Bỏ số vô lý: driver không hỗ trợ có thể trả rác.
			if t := float64(perf.temperature) / 10; t > 0 && t < 150 && t > nong {
				nong = t
			}
		}
		close := struct{ hAdapter uint32 }{h}
		procD3DKMTCloseAdapter.Call(uintptr(unsafe.Pointer(&close)))
	}
	return nong
}
