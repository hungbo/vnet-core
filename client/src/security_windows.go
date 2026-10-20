//go:build windows

package main

import (
	"context"
	"os/exec"
	"syscall"
	"time"
)

// securityScript in "khoá|giá trị" cho parseSecurityOutput. Mỗi mục tự nuốt lỗi để mục
// khác vẫn in ra: máy ảo không có TPM, BIOS legacy không có khoá Secure Boot.
//
//   - PEFirmwareType: 1 = BIOS legacy, 2 = UEFI. Máy thật ở phòng test (Windows Server 2019) KHÔNG có giá trị này dù khoá
//     Secure Boot có mặt, nên khi thiếu thì suy từ khoá Secure Boot (chỉ có khi khởi động UEFI), rồi từ bcdedit (winload.efi).
//   - UEFISecureBootEnabled: khoá chỉ có khi máy khởi động bằng UEFI.
//   - Win32_Tpm nằm trong không gian tên riêng và chỉ quản trị đọc được — dịch vụ nền
//     chạy bằng SYSTEM nên đọc được.
//   - HVCI đọc cấu hình đang bật (Enabled=1), không phải trạng thái đang chạy: đủ để
//     người vận hành biết máy đã bật Memory integrity.
//   - VirtualizationFirmwareEnabled: ảo hoá trong BIOS (Intel VT-x / AMD-V).
//   - SoftwareLicensingProduct của Windows (ApplicationId cố định): LicenseStatus và 5 ký tự
//     cuối key. Truy vấn này chậm nhất trong cả bản (vài giây), nên nằm cuối.
const securityScript = `
$ErrorActionPreference = 'SilentlyContinue'
[Console]::OutputEncoding = [Text.Encoding]::UTF8
$fw = (Get-ItemProperty 'HKLM:\SYSTEM\CurrentControlSet\Control' -Name PEFirmwareType).PEFirmwareType
$sb = (Get-ItemProperty 'HKLM:\SYSTEM\CurrentControlSet\Control\SecureBoot\State' -Name UEFISecureBootEnabled).UEFISecureBootEnabled
if ($fw -eq 2) { 'boot|uefi' } elseif ($fw -eq 1) { 'boot|legacy' }
elseif ($sb -ne $null) { 'boot|uefi' }
else { $bc = (bcdedit /enum '{current}' 2>$null | Out-String); if ($bc -match 'winload\.efi') { 'boot|uefi' } elseif ($bc -match 'winload\.exe') { 'boot|legacy' } }
if ($sb -eq 1) { 'secureboot|on' } elseif ($sb -eq 0) { 'secureboot|off' }
$t = Get-CimInstance -Namespace root\cimv2\Security\MicrosoftTpm -ClassName Win32_Tpm
if ($t) { 'tpm|' + (($t.SpecVersion -split ',')[0]).Trim() } else { 'tpm|none' }
$hv = (Get-ItemProperty 'HKLM:\SYSTEM\CurrentControlSet\Control\DeviceGuard\Scenarios\HypervisorEnforcedCodeIntegrity' -Name Enabled).Enabled
if ($hv -eq 1) { 'hvci|on' } else { 'hvci|off' }
$cpu = Get-CimInstance Win32_Processor | Select-Object -First 1
if ($cpu) { if ($cpu.VirtualizationFirmwareEnabled) { 'virt|on' } else { 'virt|off' } }
$b = (Get-CimInstance Win32_BIOS).SMBIOSBIOSVersion
if ($b) { 'bios|' + $b }
$wl = Get-CimInstance SoftwareLicensingProduct -Filter "ApplicationId='55c92734-d682-4d71-983e-d6ec3f16059f' AND PartialProductKey IS NOT NULL" | Select-Object -First 1
if ($wl) { 'winlic|' + [int]$wl.LicenseStatus; 'wintail|' + $wl.PartialProductKey } else { 'winlic|0' }
`

// readSecurity chạy MỘT lần mỗi tiến trình, cùng lúc và cùng lý do với readGPUName.
func readSecurity() MachineSecurity {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-NonInteractive", "-Command", securityScript)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	out, err := cmd.Output()
	if err != nil {
		return MachineSecurity{}
	}
	return parseSecurityOutput(string(out))
}
