package main

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"
)

func equal(t *testing.T, want, got interface{}, msg string) {
	t.Helper()
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("%s: muốn %#v, được %#v", msg, want, got)
	}
}

func TestRootOf(t *testing.T) {
	equal(t, `D:\Games`, (&gameMenuItem{}).rootOf(`D:\Games`), "không có gốc riêng")
	equal(t, `D:\Games`, (&gameMenuItem{Root: "  "}).rootOf(`D:\Games`), "gốc riêng toàn khoảng trắng là không có")
	equal(t, `E:\Games2`, (&gameMenuItem{Root: `E:\Games2`}).rootOf(`D:\Games`), "gốc riêng")
}

func TestGameMenu_JSONCuVaMoi(t *testing.T) {
	var old gameMenu
	if err := json.Unmarshal([]byte(`{"root":"D:","items":[{"key":"A","name":"A","launcher":"a.exe","status":"ready"}]}`), &old); err != nil {
		t.Fatal(err)
	}
	equal(t, "", old.Items[0].Root, "máy chủ cũ không gửi root")
	equal(t, 0, len(old.Items[0].RunFirst), "máy chủ cũ không gửi run_first")

	var cur gameMenu
	if err := json.Unmarshal([]byte(`{"root":"D:","items":[{"key":"A","name":"A","launcher":"a.exe","root":"E:\\G2",
		"run_first":[{"launcher":"pre/x.bat","launch_args":"/s"}]}]}`), &cur); err != nil {
		t.Fatal(err)
	}
	equal(t, `E:\G2`, cur.Items[0].Root, "root")
	equal(t, []gameRunStep{{Launcher: "pre/x.bat", LaunchArgs: "/s"}}, cur.Items[0].RunFirst, "run_first")
}

func TestPlanRunFirst(t *testing.T) {
	plan, problems := planRunFirst(`D:\Games`, "Game", []gameRunStep{
		{Launcher: "pre/setup.bat", LaunchArgs: " /s "},
		{Launcher: `..\evil.exe`},
		{Launcher: `C:\Windows\notepad.exe`},
		{Launcher: ""},
		{Launcher: "svc.exe"},
	})
	equal(t, []resolvedStep{
		{Exe: `D:\Games\Game\pre\setup.bat`, Dir: `D:\Games\Game\pre`, Args: "/s"},
		{Exe: `D:\Games\Game\svc.exe`, Dir: `D:\Games\Game`},
	}, plan, "bước hợp lệ vẫn chạy khi có bước hỏng")
	equal(t, 3, len(problems), "ba bước hỏng")

	plan, problems = planRunFirst(`E:\Other`, "G", []gameRunStep{{Launcher: "a.exe"}})
	equal(t, `E:\Other\G\a.exe`, plan[0].Exe, "dùng gốc riêng của game")
	equal(t, 0, len(problems), "không có lỗi")

	plan, problems = planRunFirst(`D:\Games`, "Game", []gameRunStep{{Launcher: "1.exe"}, {Launcher: "2.exe"}, {Launcher: "3.exe"},
		{Launcher: "4.exe"}, {Launcher: "5.exe"}, {Launcher: "6.exe"}, {Launcher: "7.exe"}})
	equal(t, maxRunFirstSteps, len(plan), "chỉ nhận tối đa")
	equal(t, 1, len(problems), "ghi nhận phần dư bị bỏ")

	_, problems = planRunFirst("Games", "Game", []gameRunStep{{Launcher: "a.exe"}})
	equal(t, 1, len(problems), "gốc hỏng thì mọi bước đều bị từ chối")
}

func TestRunPlan(t *testing.T) {
	plan := []resolvedStep{{Exe: "a", Dir: "d", Args: "1"}, {Exe: "missing"}, {Exe: "b", Args: "2"}, {Exe: "c"}}
	var ran []string
	var waits []time.Duration
	exists := func(p string) bool { return p != "missing" }
	run := func(exe, dir, args string, wait time.Duration) error {
		ran = append(ran, exe+":"+args)
		waits = append(waits, wait)
		if exe == "b" {
			return errors.New("lỗi giả")
		}
		return nil
	}
	clock := time.Unix(0, 0)
	problems := runPlan(plan, run, exists, 30*time.Second, 60*time.Second, func() time.Time { return clock })
	equal(t, []string{"a:1", "b:2", "c:"}, ran, "đúng thứ tự, bỏ tệp không có, bước lỗi không chặn bước sau")
	equal(t, 2, len(problems), "một tệp thiếu và một bước lỗi")
	equal(t, 30*time.Second, waits[0], "hạn mỗi bước")

	// hết hạn chung: mỗi lần hỏi giờ trôi thêm 25 giây
	ran, waits = nil, nil
	now := time.Unix(0, 0)
	tick := func() time.Time { n := now; now = now.Add(25 * time.Second); return n }
	problems = runPlan([]resolvedStep{{Exe: "a"}, {Exe: "b"}, {Exe: "c"}, {Exe: "d"}}, run, exists, 30*time.Second, 60*time.Second, tick)
	if len(problems) == 0 || len(ran) >= 4 {
		t.Fatalf("hết hạn chung phải bỏ các bước còn lại: ran=%v problems=%v", ran, problems)
	}
	for _, w := range waits {
		if w > 30*time.Second {
			t.Fatalf("hạn một bước vượt 30 giây: %v", w)
		}
	}
}

func TestGameIconTarget_GocRieng(t *testing.T) {
	m := &gameMenu{Root: `D:\Games`, Items: []gameMenuItem{
		{Key: "A", Name: "A", Launcher: "a.exe"},
		{Key: "B", Name: "B", Launcher: "b.exe", Root: `E:\Games2`},
	}}
	exe, dir, ok := gameIconTarget(m, "A")
	equal(t, true, ok, "A có icon")
	equal(t, `D:\Games\A\a.exe`, exe, "A exe")
	equal(t, `D:\Games\A`, dir, "A dir")
	exe, dir, ok = gameIconTarget(m, "B")
	equal(t, true, ok, "B có icon")
	equal(t, `E:\Games2\B\b.exe`, exe, "B exe")
	equal(t, `E:\Games2\B`, dir, "B dir")
}
