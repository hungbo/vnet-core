package main

import "testing"

func TestNICWatcher(t *testing.T) {
	w := &nicWatcher{}
	chinh := []nicInfo{{"Ethernet", "192.168.1.20"}}

	if w.extra(chinh) != nil {
		t.Fatal("chưa chốt danh sách thì không được khai gì")
	}
	w.setBaselineOnce(chinh)
	if got := w.extra(chinh); got == nil || *got != "" {
		t.Fatalf("chỉ có card chính: nhận %v, mong chuỗi rỗng", got)
	}

	them := append(chinh, nicInfo{"Wi-Fi", "10.0.0.5"}, nicInfo{"Ethernet 2", "172.20.10.3"})
	if got := w.extra(them); got == nil || *got != "Ethernet 2 (172.20.10.3), Wi-Fi (10.0.0.5)" {
		t.Fatalf("hai card lạ: nhận %v", *got)
	}

	// Chốt lần hai không được ghi đè: card lạ đang cắm sẽ thành "của máy".
	w.setBaselineOnce(them)
	if got := w.extra(them); *got == "" {
		t.Fatal("chốt lại lần hai đã nuốt mất card lạ")
	}

	// Rút card lạ ra thì hết báo.
	if got := w.extra(chinh); *got != "" {
		t.Fatalf("đã rút mà vẫn báo %q", *got)
	}
}
