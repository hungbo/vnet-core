package main

import (
	"sync/atomic"
	"testing"
	"time"
)

// Nhịp tim không bao giờ chờ việc đo cấu hình: chưa được phép đo thì không đo, đang
// đo thì gửi rỗng, đo xong thì có số, và cả quá trình chỉ đo đúng một lần.
func TestLazySpecs_NhipTimKhongChoDo(t *testing.T) {
	var soLanDo atomic.Int32
	mo := make(chan struct{})
	l := &lazySpecs{read: func() MachineSpecs {
		soLanDo.Add(1)
		<-mo
		return MachineSpecs{CPUName: "CPU thử"}
	}}

	if s := l.get(false); s.CPUName != "" {
		t.Fatalf("get(false) = %+v, muốn rỗng", s)
	}
	if n := soLanDo.Load(); n != 0 {
		t.Fatalf("get(false) đã đo %d lần, muốn 0", n)
	}

	kq := make(chan MachineSpecs, 1)
	go func() { kq <- l.get(true) }()
	select {
	case s := <-kq:
		if s.CPUName != "" {
			t.Fatalf("get(true) lúc đang đo = %+v, muốn rỗng", s)
		}
	case <-time.After(time.Second):
		t.Fatal("get(true) chờ việc đo")
	}

	l.get(true)
	close(mo)
	hetHan := time.Now().Add(2 * time.Second)
	for l.get(true).CPUName != "CPU thử" {
		if time.Now().After(hetHan) {
			t.Fatal("đo xong rồi mà get vẫn trả rỗng")
		}
		time.Sleep(5 * time.Millisecond)
	}

	if n := soLanDo.Load(); n != 1 {
		t.Fatalf("đo %d lần, muốn đúng 1", n)
	}
}
