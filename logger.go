package bg

type Logger interface {
	Errorf(format string, args ...interface{})
	Infof(format string, args ...interface{})
}

type noopLogger struct{}

func (noopLogger) Errorf(format string, args ...interface{}) {}

func (noopLogger) Infof(format string, args ...interface{}) {}
