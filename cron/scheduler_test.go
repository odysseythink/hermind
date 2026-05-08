package cron

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestSchedulerFiresJob(t *testing.T) {
	var hits int32
	s := NewScheduler()
	s.Add(Job{
		Name:     "test",
		Schedule: Schedule{Every: 20 * time.Millisecond},
		Run: func(context.Context) error {
			atomic.AddInt32(&hits, 1)
			return nil
		},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Millisecond)
	defer cancel()
	_ = s.Start(ctx)
	if atomic.LoadInt32(&hits) < 2 {
		t.Errorf("expected at least 2 hits, got %d", hits)
	}
}

func TestSchedulerNoJobs(t *testing.T) {
	s := NewScheduler()
	if err := s.Start(context.Background()); err == nil {
		t.Error("expected error")
	}
}
