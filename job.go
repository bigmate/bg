package background

import (
	"context"
)

//Job represents single unit of job that should
//be executed in the background
type Job interface {
	Name() string
	Executable() func(ctx context.Context) error
}

type job struct {
	name string
	exe  func(ctx context.Context) error
}

func (j *job) Name() string {
	return j.name
}

func (j *job) Executable() func(ctx context.Context) error {
	return j.exe
}

//NewJob creates new job
func NewJob(name string, exe func(ctx context.Context) error) Job {
	return &job{
		name: name,
		exe:  exe,
	}
}
