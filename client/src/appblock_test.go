package main

import "testing"

// Danh sách chặn là cài đặt chung, đến nguyên khối trong chính sách: mỗi lần
// nhận là THAY toàn bộ, nên bỏ một tên khỏi cài đặt là máy trạm thôi chặn nó.
func TestAppBlocker_SetAllThayToanBo(t *testing.T) {
	b := newAppBlocker()

	b.setAll("Chrome.EXE,steam")
	if got := b.list(); len(got) != 2 || got[0] != "chrome" || got[1] != "steam" {
		t.Fatalf("danh sách = %v, mong [chrome steam]", got)
	}

	b.setAll("steam")
	if got := b.list(); len(got) != 1 || got[0] != "steam" {
		t.Fatalf("sau khi bỏ chrome = %v, mong [steam]", got)
	}

	b.setAll("")
	if got := b.list(); len(got) != 0 {
		t.Fatalf("chính sách rỗng mà còn chặn %v", got)
	}
}

// Chặn nhầm tiến trình hệ thống là treo máy của khách; chặn chính máy trạm là
// tự tắt mình. Máy chủ đã lọc, nhưng máy trạm vẫn phải tự bỏ qua chúng.
func TestAppBlocker_BoQuaTienTrinhHeThong(t *testing.T) {
	b := newAppBlocker()
	b.setAll(`explorer.exe,vnet-client,csrss,svchost.exe,winlogon,..\evil,zalo`)
	if got := b.list(); len(got) != 1 || got[0] != "zalo" {
		t.Errorf("danh sách = %v, mong chỉ [zalo]", got)
	}
}
