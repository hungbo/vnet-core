// Package scheduler runs the recurring background work the business rules
// depend on. Before it existed nothing in the backend was time-driven: curfew
// could not be enforced, bookings never expired, sessions never closed
// themselves and machines never went offline.
package scheduler

import (
	"context"
	"log"
	"sync"
	"time"
)

// Job is one unit of recurring work. Run is called on every tick and should
// return quickly; long work belongs in its own job with a longer interval.
type Job struct {
	Name     string
	Interval time.Duration
	// RunAtStart chạy một lượt ngay khi khởi động, trước lần tick đầu tiên.
	// Cần cho những tác vụ thưa hơn chu kỳ khởi động lại của máy chủ.
	RunAtStart bool
	Run        func(ctx context.Context) error
}

type Scheduler struct {
	jobs []Job
	wg   sync.WaitGroup
}

func New() *Scheduler {
	return &Scheduler{}
}

func (s *Scheduler) Add(job Job) {
	if job.Interval <= 0 {
		panic("scheduler: job " + job.Name + " has a non-positive interval")
	}
	s.jobs = append(s.jobs, job)
}

// Start launches every job and returns immediately. Jobs stop when ctx is
// cancelled; Wait blocks until they have all returned.
func (s *Scheduler) Start(ctx context.Context) {
	for _, job := range s.jobs {
		s.wg.Add(1)
		go s.run(ctx, job)
	}
	log.Printf("[scheduler] started %d jobs", len(s.jobs))
}

func (s *Scheduler) Wait() {
	s.wg.Wait()
}

func (s *Scheduler) run(ctx context.Context, job Job) {
	defer s.wg.Done()

	ticker := time.NewTicker(job.Interval)
	defer ticker.Stop()

	// Tác vụ chạy thưa (mỗi giờ) mà máy chủ khởi động lại thường xuyên hơn thế
	// thì sẽ KHÔNG BAO GIỜ chạy: đồng hồ đếm lại từ đầu sau mỗi lần khởi động.
	if job.RunAtStart {
		s.invoke(ctx, job)
	}

	for {
		select {
		case <-ctx.Done():
			log.Printf("[scheduler] %s stopped", job.Name)
			return
		case <-ticker.C:
			s.invoke(ctx, job)
		}
	}
}

// invoke isolates one execution: a panicking job must not take the process
// down, and a failing job must not stop its own schedule.
func (s *Scheduler) invoke(ctx context.Context, job Job) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[scheduler] %s panicked: %v", job.Name, r)
		}
	}()

	if err := job.Run(ctx); err != nil {
		log.Printf("[scheduler] %s: %v", job.Name, err)
	}
}
