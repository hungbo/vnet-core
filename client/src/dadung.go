package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// Dấu "lần bật máy này đã có người đăng nhập", để chế độ ngủ chờ (supervise.go,
// decide, luật 4) không cho ngủ một máy còn dấu vết của khách trước.
//
// Bộ giám sát nhớ điều đó trong bộ nhớ, nhưng dịch vụ nền tự khởi động lại giữa một
// lần bật máy (tự cập nhật, chế độ khôi phục của SCM) thì mất. Nên lần đầu thấy có
// người đăng nhập, dịch vụ ghi boot_at của lần bật máy này ra tệp cạnh .exe. Máy
// không ổ cứng thì tệp mất ở lần bật sau; máy có ổ cứng thì boot_at lệch xa nên bỏ
// qua — cùng cách nhận ra một lần bật máy với báo cáo khởi động (bootreport.go).

type dauDaDung struct {
	BootAt string `json:"boot_at"`
}

func daDungFilePath() string {
	exe, err := os.Executable()
	if err != nil {
		return "used.json"
	}
	return filepath.Join(filepath.Dir(exe), "used.json")
}

// vuaTuCapNhat: tệp .exe vừa được thay giữa một lần bật máy — tự cập nhật để lại
// vnet-client.exe.old*, donTepCu xoá chúng khi runAutoUpdate chạy. Phải gọi trước đó.
func vuaTuCapNhat() bool {
	exe, err := os.Executable()
	if err != nil {
		return false
	}
	cu, _ := filepath.Glob(exe + ".old*")
	return len(cu) > 0
}

// ghiDaDung ghi dấu cho lần bật máy này. Lỗi ghi bỏ qua: mất dấu thì lần dịch vụ
// khởi động lại giữa chừng vẫn bị mayDaDung chặn bằng thời gian máy đã chạy.
func ghiDaDung(tep string, now time.Time, up time.Duration) {
	data, _ := json.Marshal(dauDaDung{BootAt: tinhBootAt(now, up).Format(time.RFC3339)})
	_ = os.WriteFile(tep, data, 0o644)
}

// mayDaDung: lúc dịch vụ bắt đầu, lần bật máy này đã có người dùng chưa.
//
// Có dấu của chính lần bật máy này thì có. Không có dấu thì vẫn coi là có khi dịch
// vụ không thấy được những gì xảy ra trước nó:
//   - vuaCapNhat: bản cũ (trước 1.0.16, chưa biết ghi dấu) vừa tự cập nhật lên. Tự cập
//     nhật chỉ chờ không có HỘI VIÊN; nhân viên hay bảo trì đang đăng nhập vẫn bị
//     cập nhật đè, có khi ngay giây thứ 90 sau khi bật máy.
//   - dịch vụ bắt đầu quá curtainMaxUptime sau khi bật máy (tệp hỏng, đồng hồ bị
//     chỉnh quá bootAtLech...).
//
// Đoán nhầm phía này chỉ tốn một lần khởi động lại; nhầm phía kia là lộ tài khoản
// của khách. Ngoài Windows up luôn bằng 0: không đoán theo thời gian.
func mayDaDung(tep string, now time.Time, up time.Duration, vuaCapNhat bool) bool {
	var d dauDaDung
	if data, err := os.ReadFile(tep); err == nil && json.Unmarshal(data, &d) == nil &&
		cungLanBat(d.BootAt, tinhBootAt(now, up)) {
		return true
	}
	return vuaCapNhat || up >= curtainMaxUptime
}
