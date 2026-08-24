package scheduler

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScheduler_RunsJobUntilContextCancelled(t *testing.T) {
	var runs int32
	s := New()
	s.Add(Job{
		Name:     "counter",
		Interval: 5 * time.Millisecond,
		Run: func(ctx context.Context) error {
			atomic.AddInt32(&runs, 1)
			return nil
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	s.Start(ctx)
	time.Sleep(40 * time.Millisecond)
	cancel()
	s.Wait()

	assert.Greater(t, atomic.LoadInt32(&runs), int32(1))
}

// One bad job must not stop its own schedule or take the process down.
func TestScheduler_PanicDoesNotStopSchedule(t *testing.T) {
	var runs int32
	s := New()
	s.Add(Job{
		Name:     "panicky",
		Interval: 5 * time.Millisecond,
		Run: func(ctx context.Context) error {
			atomic.AddInt32(&runs, 1)
			panic("boom")
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	require.NotPanics(t, func() {
		s.Start(ctx)
		time.Sleep(40 * time.Millisecond)
		cancel()
		s.Wait()
	})

	assert.Greater(t, atomic.LoadInt32(&runs), int32(1), "schedule should survive a panic")
}

func TestScheduler_ErrorDoesNotStopSchedule(t *testing.T) {
	var runs int32
	s := New()
	s.Add(Job{
		Name:     "failing",
		Interval: 5 * time.Millisecond,
		Run: func(ctx context.Context) error {
			atomic.AddInt32(&runs, 1)
			return errors.New("nope")
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	s.Start(ctx)
	time.Sleep(40 * time.Millisecond)
	cancel()
	s.Wait()

	assert.Greater(t, atomic.LoadInt32(&runs), int32(1))
}

func TestScheduler_RejectsNonPositiveInterval(t *testing.T) {
	s := New()
	assert.Panics(t, func() {
		s.Add(Job{Name: "bad", Interval: 0, Run: func(context.Context) error { return nil }})
	})
}

func TestScheduler_WaitReturnsWithoutJobs(t *testing.T) {
	s := New()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s.Start(ctx)
	s.Wait()
}

// Tác vụ chạy mỗi giờ mà máy chủ khởi động lại thường xuyên hơn thế thì không
// bao giờ tới lượt: đồng hồ đếm lại từ đầu sau mỗi lần khởi động. RunAtStart
// chạy một lượt ngay, trước lần tick đầu tiên.
func TestScheduler_RunAtStartChayTruocLanTickDauTien(t *testing.T) {
	var runs int32
	s := New()
	s.Add(Job{
		Name:       "dọn-dẹp",
		Interval:   time.Hour, // dài hơn hẳn thời gian sống của bài kiểm
		RunAtStart: true,
		Run: func(context.Context) error {
			atomic.AddInt32(&runs, 1)
			return nil
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	s.Start(ctx)

	assert.Eventually(t, func() bool { return atomic.LoadInt32(&runs) == 1 },
		2*time.Second, 10*time.Millisecond,
		"tác vụ RunAtStart phải chạy ngay, không đợi hết một giờ")

	cancel()
	s.Wait()
	assert.Equal(t, int32(1), atomic.LoadInt32(&runs), "chỉ được chạy đúng một lượt")
}

// Mặc định thì KHÔNG chạy lúc khởi động — mọi tác vụ đang có đều dựa vào điều đó.
func TestScheduler_MacDinhKhongChayLucKhoiDong(t *testing.T) {
	var runs int32
	s := New()
	s.Add(Job{
		Name:     "khong-chay-ngay",
		Interval: time.Hour,
		Run: func(context.Context) error {
			atomic.AddInt32(&runs, 1)
			return nil
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	s.Start(ctx)
	time.Sleep(150 * time.Millisecond)
	cancel()
	s.Wait()

	assert.Equal(t, int32(0), atomic.LoadInt32(&runs))
}

// days <= 0 nghĩa là giữ lại tất cả. Phải thoát trước khi chạm tới database,
// nếu không thì "tắt dọn dẹp" lại thành một câu DELETE với mốc thời gian tương lai.
func TestPruneHardwareHistory_KhongNgayThiKhongDungToiDatabase(t *testing.T) {
	for _, days := range []int{0, -1} {
		// db nil: chạm tới nó là panic, nên bài kiểm này chứng minh được hàm
		// thoát ra trước đó.
		assert.NoError(t, pruneHardwareHistory(nil, days))
	}
}
