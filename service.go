package bg

import (
	"context"
	"sync"
	"time"

	"github.com/bigmate/closer"
)

//Scheduler represents background service that executes jobs in the background
type Scheduler interface {
	Schedule(job Job, strategy Strategy)
}

type tickerChanPair struct {
	ticker   *time.Ticker
	stopChan chan struct{}
}

type timerChanPair struct {
	timer    *time.Timer
	stopChan chan struct{}
}

type scheduler struct {
	mx            sync.Mutex
	ctx           context.Context
	cancelCtx     func()
	logger        Logger
	run           func(job Job)
	delayedJobs   map[string]*timerChanPair
	scheduledJobs map[string]*tickerChanPair
}

type Option func(s *scheduler)

func WithLogger(logger Logger) Option {
	return func(s *scheduler) {
		s.logger = logger
	}
}

func WithRunner(runner func(job Job)) Option {
	return func(s *scheduler) {
		s.run = runner
	}
}

//NewScheduler returns Scheduler
func NewScheduler(ctx context.Context, options ...Option) Scheduler {
	ctx, cancel := context.WithCancel(ctx)
	sch := &scheduler{
		ctx:           ctx,
		cancelCtx:     cancel,
		logger:        noopLogger{},
		delayedJobs:   make(map[string]*timerChanPair),
		scheduledJobs: make(map[string]*tickerChanPair),
	}

	sch.run = func(job Job) {
		if err := job.Run(sch.ctx); err != nil {
			sch.logger.Errorf(sch.ctx, "failed to finish job: %s, %v", job.Name(), err)
		}
	}

	for _, apply := range options {
		apply(sch)
	}

	closer.Add(func() error {
		sch.stop()
		return nil
	})

	return sch
}

func (s *scheduler) stop() {
	s.cancelCtx()
	s.mx.Lock()
	defer s.mx.Unlock()

	for _, st := range s.scheduledJobs {
		st.ticker.Stop()
		st.stopChan <- struct{}{}
	}

	for _, st := range s.delayedJobs {
		st.timer.Stop()
		st.stopChan <- struct{}{}
	}

	s.scheduledJobs = make(map[string]*tickerChanPair)
	s.delayedJobs = make(map[string]*timerChanPair)
}

func (s *scheduler) Schedule(job Job, strategy Strategy) {
	p := &params{}

	strategy(p)

	switch {
	case p.strategy(delayed):
		s.runAfter(job, p.delay)
	case p.strategy(periodic):
		s.schedule(job, p.interval)
	}
}

//runAfter runs a job after specified delay.
//If it finds a job already scheduled to run with the same name then
//it removes that job, if it finds a job already running with the same name then it just
//ignores that job, because when job finishes it'll automatically remove itself from delayedJobs map
func (s *scheduler) runAfter(job Job, delay time.Duration) {
	s.mx.Lock()
	defer s.mx.Unlock()

	if st, ok := s.delayedJobs[job.Name()]; ok && st.timer.Stop() {
		st.stopChan <- struct{}{}
		delete(s.delayedJobs, job.Name())
	}

	if delay == 0 {
		go func() {
			s.run(job)
		}()
		return
	}

	tm := time.NewTimer(delay)
	ch := make(chan struct{})
	tp := &timerChanPair{
		timer:    tm,
		stopChan: ch,
	}

	s.delayedJobs[job.Name()] = tp

	go func() {
		select {
		case <-tm.C:
			s.run(job)
		case <-ch:
			s.logger.Infof(s.ctx, "%s got exit signal", job.Name())
		}

		s.mx.Lock()
		defer s.mx.Unlock()

		if s.delayedJobs[job.Name()] == tp {
			delete(s.delayedJobs, job.Name())
		}
	}()
}

//schedule a job to be run periodically. If it finds identical job
//then it removes that job and schedules a new job
func (s *scheduler) schedule(job Job, interval time.Duration) {
	s.mx.Lock()
	defer s.mx.Unlock()

	if st, ok := s.scheduledJobs[job.Name()]; ok {
		st.ticker.Stop()
		st.stopChan <- struct{}{}
	}

	tck := time.NewTicker(interval)
	ch := make(chan struct{})

	s.scheduledJobs[job.Name()] = &tickerChanPair{
		ticker:   tck,
		stopChan: ch,
	}

	go func() {
		for {
			select {
			case <-tck.C:
				s.run(job)
			case <-ch:
				s.logger.Infof(s.ctx, "%s got exit signal", job.Name())
				return
			}
		}
	}()
}
