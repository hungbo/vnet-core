package main

// Windows khoá phiên mỗi lần máy thức dậy từ chế độ ngủ: cài đặt "Require a password
// on wakeup" (CONSOLELOCK) mặc định BẬT. Với Lightning Boot của Cloud Update (máy boot
// sẵn rồi ngủ S3) thì khách bấm nút nguồn sẽ gặp màn hình khoá của Windows, phải bấm
// Enter mới vào được, và màn khoá VNET — giao diện bật lại ngay lúc thức — nằm sau nó
// nên khách không thấy chỗ đăng nhập; đồng hồ "không ai đăng nhập" (supervise.go,
// luật 4) vẫn chạy trong lúc đó.
//
// Màn khoá VNET đã là hàng rào của quán; khoá của Windows ở đây chỉ chắn đường vào nó.
const (
	consoleLockGUID = "0e796bdb-100d-47d6-a2d5-f7d2daa51f51"
	// Chính sách máy (Group Policy: "Require a password when a computer wakes"),
	// ACSettingIndex/DCSettingIndex = 0 là Disabled. Thắng cài đặt của lược đồ nguồn.
	consoleLockPolicyKey = `SOFTWARE\Policies\Microsoft\Power\PowerSettings\` + consoleLockGUID
)

// wakeLockPowercfgArgs là các lệnh powercfg đặt "yêu cầu đăng nhập khi thức" = 0 cho
// lược đồ nguồn đang dùng (cắm điện và pin) rồi áp lại. Đúng các lệnh có thể gõ tay
// trên máy để thử.
func wakeLockPowercfgArgs() [][]string {
	return [][]string{
		{"/setacvalueindex", "SCHEME_CURRENT", "SUB_NONE", "CONSOLELOCK", "0"},
		{"/setdcvalueindex", "SCHEME_CURRENT", "SUB_NONE", "CONSOLELOCK", "0"},
		{"/setactive", "SCHEME_CURRENT"},
	}
}
