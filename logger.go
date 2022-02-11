package background

import (
	"context"
)

type Logger interface {
	Errorf(ctx context.Context, msg string, args ...interface{})
	Infof(ctx context.Context, msg string, args ...interface{})
}
