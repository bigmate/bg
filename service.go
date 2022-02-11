package background

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
	rw            sync.RWMutex
	ctx           context.Context
	cancelCtx     func()
	logger        Logger
	delayedJobs   map[string]*timerChanPair
	scheduledJobs map[string]*tickerChanPair
}

//NewScheduler returns Scheduler
func NewScheduler(ctx context.Context, log Logger) Scheduler {
	ctx, cancel := context.WithCancel(ctx)
	sch := &scheduler{
		ctx:           ctx,
		cancelCtx:     cancel,
		logger:        log,
		delayedJobs:   make(map[string]*timerChanPair),
		scheduledJobs: make(map[string]*tickerChanPair),
	}

	closer.Add(func() error {
		sch.stop()
		return nil
	})

	return sch
}

func (s *scheduler) stop() {
	s.cancelCtx()
	s.rw.RLock()
	defer s.rw.RUnlock()

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

func (s *scheduler) runAfter(job Job, delay time.Duration) {
	s.rw.Lock()
	defer s.rw.Unlock()

	if st, ok := s.delayedJobs[job.Name()]; ok {
		st.timer.Stop()
		st.stopChan <- struct{}{}
	}

	tm := time.NewTimer(delay)
	ch := make(chan struct{})

	s.delayedJobs[job.Name()] = &timerChanPair{
		timer:    tm,
		stopChan: ch,
	}

	go func() {
		select {
		case <-tm.C:
			s.run(job)
		case <-ch:
			s.logger.Infof(s.ctx, "%s got exit signal", job.Name())
			return
		}
	}()
}

func (s *scheduler) schedule(job Job, interval time.Duration) {
	s.rw.Lock()
	defer s.rw.Unlock()

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

func (s *scheduler) run(job Job) {
	execute := job.Executable()
	if err := execute(s.ctx); err != nil {
		s.logger.Errorf(s.ctx, "failed to finish job: %s, %v", job.Name(), err)
	}
}
