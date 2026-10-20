package main

import (
	"regexp"
	"strings"
)

// MachineSecurity là phần "Boot và bảo mật" của máy: những thứ BIOS/UEFI và Windows
// quyết định, và game có chống gian lận (Vanguard của Valorant, Easy Anti-Cheat…)
// đòi hỏi. Trước đây người vận hành phải vào BIOS từng máy để biết máy nào thiếu
// Secure Boot hay TPM 2.0, rồi mới hiểu vì sao Valorant báo lỗi VAN9003 trên máy đó.
//
// Trường nào không đọc được để TRỐNG: máy chủ giữ nguyên giá trị đang có thay vì ghi
// đè bằng "không rõ", cùng quy ước với MachineSpecs.
type MachineSecurity struct {
	BootMode       string // "uefi" | "legacy"
	SecureBoot     string // "on" | "off" | "unsupported" (BIOS legacy không có Secure Boot)
	TPM            string // "2.0", "1.2" hoặc "none" (không có chip)
	HVCI           string // "on" | "off": Memory integrity (bảo vệ nhân bằng ảo hoá)
	Virtualization string // "on" | "off": ảo hoá phần cứng trong BIOS
	BIOSVersion    string
	// Bản quyền Windows (bảng Windows Keys): licensed | unlicensed | grace | notification, và 5
	// ký tự cuối của key đang cài — đủ để so với key quán đã ghi, mà không gửi cả key lên mạng.
	WinLicense string
	WinKeyTail string
}

// Bản đọc về trước khi tin: máy chủ ghi các chuỗi này vào database và trang quản trị
// so khớp từng giá trị, nên giá trị lạ phải bị bỏ ở đây chứ không đẩy đi.
var (
	tpmVersionRe  = regexp.MustCompile(`^\d{1,2}(\.\d{1,2})?$`)
	winKeyTailRe  = regexp.MustCompile(`^[A-Z0-9]{5}$`)
	winLicenseMap = map[string]string{
		"0": "unlicensed", "1": "licensed", "2": "grace", "3": "grace", "4": "notification", "5": "notification", "6": "grace",
	}
)

// parseSecurityOutput đọc đầu ra của securityScript: mỗi dòng "khoá|giá trị". Dòng lạ,
// khoá lạ, giá trị ngoài danh sách đều bị bỏ qua — PowerShell có thể in thêm cảnh báo.
func parseSecurityOutput(out string) MachineSecurity {
	var s MachineSecurity
	pick := func(v string, allowed ...string) string {
		for _, a := range allowed {
			if v == a {
				return v
			}
		}
		return ""
	}
	for _, line := range strings.Split(out, "\n") {
		key, val, ok := strings.Cut(strings.TrimSpace(line), "|")
		if !ok {
			continue
		}
		val = strings.TrimSpace(val)
		switch strings.TrimSpace(key) {
		case "boot":
			s.BootMode = pick(val, "uefi", "legacy")
		case "secureboot":
			s.SecureBoot = pick(val, "on", "off")
		case "tpm":
			if val == "none" || tpmVersionRe.MatchString(val) {
				s.TPM = val
			}
		case "hvci":
			s.HVCI = pick(val, "on", "off")
		case "virt":
			s.Virtualization = pick(val, "on", "off")
		case "winlic":
			// SoftwareLicensingProduct.LicenseStatus là số 0..6.
			s.WinLicense = winLicenseMap[val]
		case "wintail":
			if winKeyTailRe.MatchString(strings.ToUpper(val)) {
				s.WinKeyTail = strings.ToUpper(val)
			}
		case "bios":
			if r := []rune(val); len(r) > 64 {
				val = string(r[:64])
			}
			s.BIOSVersion = val
		}
	}
	// Máy legacy không có khoá trạng thái Secure Boot trong registry: nói rõ là không hỗ
	// trợ thay vì để trống, để trang quản trị phân biệt "chưa đo" với "máy không có".
	if s.BootMode == "legacy" && s.SecureBoot == "" {
		s.SecureBoot = "unsupported"
	}
	return s
}
