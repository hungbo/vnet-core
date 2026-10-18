package main

import (
	"errors"
	"fmt"
	"math"
	"testing"
	"time"
)

func TestNhipGiamSat(t *testing.T) {
	const chuaBatBaoGio = time.Duration(math.MaxInt64) // now.Sub(time.Time{}) bão hoà về đây
	loiThat := errors.New("CreateProcessAsUser: Access is denied")
	chuaCoPhienBoc := fmt.Errorf("%w (WTSQueryUserToken: 1008)", errChuaCoPhien)

	cases := []struct {
		ten       string
		dangChay  bool
		sauLanBat time.Duration
		loiCuoi   error
		batNgay   bool
		cho       time.Duration
	}{
		{"đang chạy: không bật, kiểm mỗi 2 giây", true, time.Hour, nil, false, checkEvery},
		{"đang chạy dù lần bật trước lỗi: vẫn 2 giây", true, 0, loiThat, false, checkEvery},
		{"lượt đầu, chưa bật lần nào: bật ngay", false, chuaBatBaoGio, nil, true, fastEvery},
		{"chưa ai đăng nhập: thử lại ngay ở nhịp dày", false, fastEvery, errChuaCoPhien, true, fastEvery},
		{"chưa ai đăng nhập (lỗi có bọc): vẫn nhịp dày", false, 0, chuaCoPhienBoc, true, fastEvery},
		{"vừa bật được rồi chết ngay: chưa bật lại", false, time.Second, nil, false, fastEvery},
		{"bật được cách đây 4,8 giây: chưa", false, 4800 * time.Millisecond, nil, false, fastEvery},
		{"bật được cách đây đủ 5 giây: bật lại", false, launchEvery, nil, true, fastEvery},
		{"lỗi thật vừa xảy ra: giữ khoảng cách 5 giây", false, 200 * time.Millisecond, loiThat, false, fastEvery},
		{"lỗi thật đã qua 5 giây: thử lại", false, launchEvery, loiThat, true, fastEvery},
		{"giao diện chết sau nhiều giờ chạy: bật lại ngay", false, 3 * time.Hour, nil, true, fastEvery},
	}
	for _, c := range cases {
		batNgay, cho := nhipGiamSat(c.dangChay, c.sauLanBat, c.loiCuoi)
		if batNgay != c.batNgay || cho != c.cho {
			t.Errorf("%s: nhận (bật=%v, chờ=%v), mong (bật=%v, chờ=%v)", c.ten, batNgay, cho, c.batNgay, c.cho)
		}
	}
}

// moPhongBat chạy vòng giám sát trong một phút giả với giao diện không bao giờ
// sống: loi là kết quả mỗi lần bật. Trả về số lần bật.
func moPhongBat(loi error) int {
	var (
		now        time.Time
		lastLaunch time.Time
		lastErr    error
		lanBat     int
	)
	now = time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	het := now.Add(time.Minute)
	for now.Before(het) {
		batNgay, cho := nhipGiamSat(false, now.Sub(lastLaunch), lastErr)
		if batNgay {
			lastLaunch, lastErr = now, loi
			lanBat++
		}
		now = now.Add(cho)
	}
	return lanBat
}

// Giao diện chết liên tục (bật được rồi chết, hoặc lỗi thật) không được làm vòng
// giám sát quay tít: tối đa một lần bật mỗi launchEvery.
func TestNhipGiamSat_VongChetBatKhongQuayTit(t *testing.T) {
	for ten, loi := range map[string]error{
		"bật được rồi chết": nil,
		"lỗi thật":          errors.New("CreateProcessAsUser: Access is denied"),
	} {
		if n := moPhongBat(loi); n < 11 || n > 13 {
			t.Errorf("%s: %d lần bật trong 60 giây, mong khoảng 12 (mỗi 5 giây)", ten, n)
		}
	}
}

// Chưa ai đăng nhập thì dò ở nhịp dày — để bật giao diện ngay khi có token —
// nhưng nhịp đó vẫn có trần: 5 lần mỗi giây.
func TestNhipGiamSat_ChoPhienDoDay(t *testing.T) {
	n := moPhongBat(errChuaCoPhien)
	if want := int(time.Minute / fastEvery); n != want {
		t.Errorf("%d lần dò trong 60 giây, mong %d (mỗi %v)", n, want, fastEvery)
	}
}
