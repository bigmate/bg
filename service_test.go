package bg

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"
)

type testLogger struct {
	noopLogger
	info chan string
}

func newTestLogger() testLogger { return testLogger{info: make(chan string)} }

func (t testLogger) Infof(msg string, args ...interface{}) {
	t.info <- fmt.Sprintf(msg, args...)
}

func testRunner(Job) {}

func TestSchedule(t *testing.T) {
	t.Run("NoDelay", func(t *testing.T) {
		sch := NewScheduler(context.Background())
		done := make(chan struct{})
		j := NewJob("example", func(ctx context.Context) error {
			defer func() {
				done <- struct{}{}
			}()
			time.Sleep(time.Second)
			return nil
		})

		sch.Schedule(j, NoDelay())

		tm := time.NewTimer(time.Second * 2)

		select {
		case <-tm.C:
			t.Errorf("failed to run on time")
		case <-done:
		}
	})

	t.Run("Delayed", func(t *testing.T) {
		sch := NewScheduler(context.Background())
		done := make(chan struct{})
		j := NewJob("example", func(ctx context.Context) error {
			defer func() {
				done <- struct{}{}
			}()
			time.Sleep(time.Second)
			return nil
		})

		sch.Schedule(j, Delayed(time.Second*2))

		tm := time.NewTimer(time.Second * 4)

		select {
		case <-tm.C:
			t.Errorf("failed to run on time")
		case <-done:
		}
	})

	t.Run("Periodic", func(t *testing.T) {
		sch := NewScheduler(context.Background())
		mu := sync.Mutex{}
		counter := 0
		iterations := make(chan struct{})
		quit := make(chan struct{})

		j := NewJob("example", func(ctx context.Context) error {
			iterations <- struct{}{}
			return nil
		})
		sch.Schedule(j, Periodic(time.Second))

		time.AfterFunc(time.Second*15, func() {
			mu.Lock()
			if counter < 10 {
				t.Errorf("expected at least 10 jobs to executed in 15 seconds")
			}
			mu.Unlock()
			quit <- struct{}{}
		})

		go func() {
			for range iterations {
				mu.Lock()
				counter++
				mu.Unlock()
			}
		}()

		<-quit
	})

	t.Run("ContextCancellationWithDelayedStrategy", func(t *testing.T) {
		logger := newTestLogger()
		sch := NewScheduler(context.Background(), WithLogger(logger))
		j := NewJob("example", func(ctx context.Context) error {
			return nil
		})

		sch.Schedule(j, Delayed(time.Minute))

		time.AfterFunc(time.Second, func() {
			sch.Schedule(j, Delayed(time.Minute))
		})

		msg := <-logger.info
		expected := "example got exit signal"

		if msg != expected {
			t.Errorf("scheduled job has not been removed")
		}
	})

	t.Run("ContextCancellationWithPeriodicStrategy", func(t *testing.T) {
		logger := newTestLogger()
		sch := NewScheduler(context.Background(), WithLogger(logger))
		j := NewJob("example", func(ctx context.Context) error {
			return nil
		})

		sch.Schedule(j, Periodic(time.Millisecond*100))

		time.AfterFunc(time.Second, func() {
			sch.Schedule(j, Periodic(time.Millisecond*100))
		})

		msg := <-logger.info
		expected := "example got exit signal"

		if msg != expected {
			t.Errorf("scheduled job has not been removed")
		}
	})
}

func TestNewScheduler(t *testing.T) {
	t.Run("Options", func(t *testing.T) {
		logger := newTestLogger()
		runner := testRunner

		sch := NewScheduler(
			context.Background(),
			WithLogger(logger),
			WithRunner(runner),
		).(*scheduler)

		if sch.logger != logger {
			t.Errorf("loggers are not identical")
		}

		unEqualRunners := reflect.ValueOf(runner).Pointer() != reflect.ValueOf(sch.run).Pointer()

		if unEqualRunners {
			t.Errorf("runners are not identical")
		}
	})
}
