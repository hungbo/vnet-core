package main

import (
	"errors"
	"testing"
	"time"
)

// fakeMenuSource đếm số lần máy chủ bị hỏi.
type fakeMenuSource struct {
	version      string
	versionErr   error
	menu         *gameMenu
	menuErr      error
	versionCalls int
	menuCalls    int
	clock        time.Time
}

func (f *fakeMenuSource) src() menuSource {
	return menuSource{
		version: func() (string, error) { f.versionCalls++; return f.version, f.versionErr },
		menu:    func() (*gameMenu, error) { f.menuCalls++; return f.menu, f.menuErr },
		now:     func() time.Time { return f.clock },
	}
}

func newFake() *fakeMenuSource {
	return &fakeMenuSource{version: "v1", menu: &gameMenu{Root: "D:", Items: []gameMenuItem{{Key: "A", Name: "A", Status: "ready"}}}, clock: time.Unix(1000, 0)}
}

func TestMenuCache_VersionKhongDoiThiDungLai(t *testing.T) {
	var c menuCache
	f := newFake()
	m1, err := c.get(f.src())
	if err != nil || m1 == nil {
		t.Fatal(err)
	}
	equal(t, 1, f.menuCalls, "lần đầu kéo danh sách")
	for i := 0; i < 5; i++ {
		f.clock = f.clock.Add(10 * time.Second)
		m, err := c.get(f.src())
		if err != nil || m != m1 {
			t.Fatalf("phải dùng lại đúng danh sách cũ: %v", err)
		}
	}
	equal(t, 1, f.menuCalls, "version không đổi thì không kéo lại danh sách")
	equal(t, 6, f.versionCalls, "một lần ở lượt đầu và năm lần ở các lượt sau")
}

func TestMenuCache_VersionDoiThiKeoLai(t *testing.T) {
	var c menuCache
	f := newFake()
	_, _ = c.get(f.src())
	f.version = "v2"
	f.menu = &gameMenu{Root: "D:", Items: []gameMenuItem{{Key: "B", Name: "B", Status: "ready"}}}
	f.clock = f.clock.Add(10 * time.Second)
	m, _ := c.get(f.src())
	equal(t, "B", m.Items[0].Key, "danh sách mới")
	equal(t, 2, f.menuCalls, "version đổi thì kéo lại")
	f.clock = f.clock.Add(10 * time.Second)
	_, _ = c.get(f.src())
	equal(t, 2, f.menuCalls, "rồi lại dùng bản nhớ")
}

func TestMenuCache_DangTaiThiLuonKeoLai(t *testing.T) {
	var c menuCache
	f := newFake()
	f.menu = &gameMenu{Items: []gameMenuItem{{Key: "A", Name: "A", Status: "downloading", Progress: 10}}}
	_, _ = c.get(f.src())
	f.clock = f.clock.Add(10 * time.Second)
	f.menu = &gameMenu{Items: []gameMenuItem{{Key: "A", Name: "A", Status: "downloading", Progress: 40}}}
	m, _ := c.get(f.src())
	equal(t, 40, m.Items[0].Progress, "tiến độ không nằm trong version nên phải kéo lại")
	equal(t, 2, f.menuCalls, "kéo lại mỗi lượt khi còn game đang tải")
	f.menu = &gameMenu{Items: []gameMenuItem{{Key: "A", Name: "A", Status: "ready"}}}
	f.clock = f.clock.Add(10 * time.Second)
	_, _ = c.get(f.src())
	f.clock = f.clock.Add(10 * time.Second)
	_, _ = c.get(f.src())
	equal(t, 3, f.menuCalls, "xong tải thì dùng lại bản nhớ")
}

func TestMenuCache_QuaHanThiKeoLai(t *testing.T) {
	var c menuCache
	f := newFake()
	_, _ = c.get(f.src())
	f.clock = f.clock.Add(menuMaxAge + time.Second)
	_, _ = c.get(f.src())
	equal(t, 2, f.menuCalls, "quá menuMaxAge kéo lại dù version không đổi (lượt mở, nhãn Hot/New)")
}

func TestMenuCache_MayChuCuKhongCoVersion(t *testing.T) {
	var c menuCache
	f := newFake()
	f.versionErr = errors.New("404")
	_, _ = c.get(f.src())
	for i := 0; i < 3; i++ {
		f.clock = f.clock.Add(10 * time.Second)
		_, _ = c.get(f.src())
	}
	equal(t, 4, f.menuCalls, "máy chủ cũ: kéo mỗi lượt như trước")
	equal(t, 1, f.versionCalls, "không hỏi lại version liên tục")
	f.versionErr = nil
	f.clock = f.clock.Add(menuVersionRetry + time.Second)
	_, _ = c.get(f.src())
	equal(t, 2, f.versionCalls, "sau menuVersionRetry thử hỏi version lại")
	f.clock = f.clock.Add(10 * time.Second)
	_, _ = c.get(f.src())
	equal(t, 5, f.menuCalls, "đã có version thì dùng lại bản nhớ")
}

func TestMenuCache_LoiVaReset(t *testing.T) {
	var c menuCache
	f := newFake()
	f.menuErr = errors.New("mất mạng")
	if _, err := c.get(f.src()); err == nil {
		t.Fatal("lỗi kéo danh sách phải trả ra")
	}
	f.menuErr = nil
	_, _ = c.get(f.src())
	c.reset()
	_, _ = c.get(f.src())
	equal(t, 3, f.menuCalls, "reset bắt kéo lại")
}
