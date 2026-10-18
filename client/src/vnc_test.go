package main

import (
	"crypto/des"
	"math/bits"
	"testing"
)

// Khoá DES phải là khoá d3des của VNC đảo bit từng byte; sai một bit là
// TightVNC từ chối mật khẩu noVNC gửi lên mà không báo gì rõ ràng.
func TestVNCDESKeyLaKhoaD3desDaoBit(t *testing.T) {
	d3des := []byte{23, 82, 107, 6, 35, 78, 88, 7}
	for i, b := range d3des {
		if got := bits.Reverse8(b); got != vncDESKey[i] {
			t.Fatalf("byte %d: muốn %#x, có %#x", i, got, vncDESKey[i])
		}
	}
}

func TestVNCObfuscateGiaiNguocDuoc(t *testing.T) {
	enc := vncObfuscate("abcd1234")
	if len(enc) != 8 {
		t.Fatalf("muốn 8 byte, có %d", len(enc))
	}
	block, _ := des.NewCipher(vncDESKey)
	plain := make([]byte, 8)
	block.Decrypt(plain, enc)
	if string(plain) != "abcd1234" {
		t.Fatalf("giải ra %q", plain)
	}
}

func TestVNCFirewallArgsChiMoChoIPMayChu(t *testing.T) {
	args := vncFirewallArgs("192.168.1.2")
	last := args[len(args)-1]
	if last != "remoteip=192.168.1.2" {
		t.Fatalf("remoteip sai: %q", last)
	}
}

func TestVNCRemoteIPLuiVeLocalSubnet(t *testing.T) {
	for _, u := range []string{"http://localhost:20800", "http://127.0.0.1:8080", "::bad"} {
		if got := vncRemoteIP(u); got != "LocalSubnet" {
			t.Errorf("%s: muốn LocalSubnet, có %q", u, got)
		}
	}
	if got := vncRemoteIP("http://192.168.1.2:8080"); got != "192.168.1.2" {
		t.Errorf("muốn 192.168.1.2, có %q", got)
	}
}
