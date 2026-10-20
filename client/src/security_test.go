package main

import "testing"

func TestParseSecurityOutput(t *testing.T) {
	cases := map[string]struct {
		in   string
		want MachineSecurity
	}{
		"máy UEFI đầy đủ": {
			in: "boot|uefi\r\nsecureboot|on\r\ntpm|2.0\r\nhvci|off\r\nvirt|on\r\nbios|F.42\r\n",
			want: MachineSecurity{BootMode: "uefi", SecureBoot: "on", TPM: "2.0", HVCI: "off",
				Virtualization: "on", BIOSVersion: "F.42"},
		},
		"BIOS legacy không có khoá Secure Boot": {
			in:   "boot|legacy\ntpm|none\nvirt|off\n",
			want: MachineSecurity{BootMode: "legacy", SecureBoot: "unsupported", TPM: "none", Virtualization: "off"},
		},
		"UEFI nhưng Secure Boot tắt": {
			in:   "boot|uefi\nsecureboot|off\ntpm|1.2",
			want: MachineSecurity{BootMode: "uefi", SecureBoot: "off", TPM: "1.2"},
		},
		"giá trị lạ và dòng rác bị bỏ": {
			in:   "WARNING: something\nboot|bios\nsecureboot|maybe\ntpm|<script>\nhvci|ON\nkhac|1\n\n",
			want: MachineSecurity{},
		},
		"Windows đã kích hoạt": {
			in:   "winlic|1\nwintail|abcde\n",
			want: MachineSecurity{WinLicense: "licensed", WinKeyTail: "ABCDE"},
		},
		"Windows chưa có key": {
			in:   "winlic|0\n",
			want: MachineSecurity{WinLicense: "unlicensed"},
		},
		"trạng thái bản quyền lạ và đuôi key sai bị bỏ": {
			in:   "winlic|9\nwintail|ABC\n",
			want: MachineSecurity{},
		},
		"không có gì thì để trống": {
			in:   "",
			want: MachineSecurity{},
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if got := parseSecurityOutput(c.in); got != c.want {
				t.Errorf("got %+v, want %+v", got, c.want)
			}
		})
	}
}

func TestParseSecurityBiosVersionCapped(t *testing.T) {
	long := ""
	for i := 0; i < 100; i++ {
		long += "x"
	}
	got := parseSecurityOutput("bios|" + long).BIOSVersion
	if len([]rune(got)) != 64 {
		t.Errorf("bios_version dài %d, mong 64", len([]rune(got)))
	}
}
