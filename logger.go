package bg

import (
	"context"
)

type Logger interface {
	Errorf(ctx context.Context, msg string, args ...interface{})
	Infof(ctx context.Context, msg string, args ...interface{})
}

type noopLogger struct {
}

func (n noopLogger) Errorf(ctx context.Context, msg string, args ...interface{}) {}

func (n noopLogger) Infof(ctx context.Context, msg string, args ...interface{}) {}
