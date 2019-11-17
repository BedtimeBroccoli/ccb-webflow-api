package vouslog

import (
	"context"

	"github.com/sirupsen/logrus"
)

type loggerKey int

const (
	contextKey loggerKey = 1
)

// WithLogger will instantiate a new context with the received logger on it.
func WithLogger(ctx context.Context, logger *logrus.Entry) context.Context {
	return context.WithValue(ctx, contextKey, logger)
}

// GetLogger retrieves the current logger from context, if nothing is found it
// will return the default logger.
func GetLogger(ctx context.Context) *logrus.Entry {
	logger := ctx.Value(contextKey)

	if logger == nil {
		// Logger should have been set in the context using WithLogger.
		panic("No logger set yet")
	}

	return logger.(*logrus.Entry)
}
