package main

import (
	"strings"
	"testing"
	"time"
)

func TestExeBaseName(t *testing.T) {
	cases := map[string]string{
		`C:\Riot Games\VALORANT\live\ShooterGame\Binaries\Win64\VALORANT-Win64-Shipping.exe`: "VALORANT-Win64-Shipping.exe",
		`C:\Windows\explorer.exe`: "explorer.exe",
		`chrome.exe`:              "chrome.exe",
		``:                        "",
		`/usr/bin/foo`:            "foo",
	}
	for in, want := range cases {
		if got := exeBaseName(in); got != want {
			t.Errorf("exeBaseName(%q) = %q, want %q", in, got, want)
		}
	}
	if got := exeBaseName(strings.Repeat("a", 200)); len([]rune(got)) != foregroundMaxLen {
		t.Errorf("tên dài %d, mong %d", len([]rune(got)), foregroundMaxLen)
	}
}

func TestRateBetween(t *testing.T) {
	down, up := rateBetween(netCounters{recv: 1000, sent: 500}, netCounters{recv: 4000, sent: 1500}, 3*time.Second)
	if down != 1000 || up != 333 {
		t.Errorf("down=%d up=%d, mong 1000 và 333", down, up)
	}
	// Bộ đếm đặt lại: 0, không phải số khổng lồ.
	down, up = rateBetween(netCounters{recv: 5000, sent: 5000}, netCounters{recv: 100, sent: 100}, time.Second)
	if down != 0 || up != 0 {
		t.Errorf("bộ đếm đặt lại: down=%d up=%d, mong 0", down, up)
	}
	if d, u := rateBetween(netCounters{}, netCounters{recv: 10, sent: 10}, 0); d != 0 || u != 0 {
		t.Errorf("khoảng thời gian 0: %d %d", d, u)
	}
}

func TestNetRateFirstSampleHasNoRate(t *testing.T) {
	var r netRate
	t0 := time.Unix(1000, 0)
	if _, _, ok := r.sample(t0, netCounters{recv: 100, sent: 100}); ok {
		t.Fatal("lần đầu không có gì để so, phải ok=false")
	}
	down, up, ok := r.sample(t0.Add(10*time.Second), netCounters{recv: 10100, sent: 2100})
	if !ok || down != 1000 || up != 200 {
		t.Errorf("down=%d up=%d ok=%v, mong 1000, 200, true", down, up, ok)
	}
}
