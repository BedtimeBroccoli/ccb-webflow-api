package middleware

import (
	stdContext "context"
	"net/http"
	"time"

	"github.com/kataras/iris/v12/context"
	"github.com/mruVOUS/ccb-webflow-api/lib/vouslog"
	"github.com/pborman/uuid"
	"github.com/sirupsen/logrus"
)

type cotextKey int

const (
	correlationIDKey cotextKey = 1
)

const correlationIdHeader = "Correlation-Id"

type loggingMiddleware struct{}

// NewLogging creates and returns a new request logger middleware.
func NewLogging() context.Handler {
	l := &loggingMiddleware{}
	return l.ServeHTTP
}

func getCorrelationID(ctx stdContext.Context) string {
	if val, ok := ctx.Value(correlationIDKey).(string); ok {
		return val
	}
	return ""
}

// initCorrelationID looks for the correlation id, or generate one
// then populates the context.
// Returns the id and the new context.
func initCorrelationID(ctx stdContext.Context, h http.Header) (stdContext.Context, string) {
	// If the correlation is already in the context, use it.
	if id := getCorrelationID(ctx); id != "" {
		return ctx, id
	}

	// Otherwise, look in the headers.
	id := h.Get(correlationIdHeader)
	if id == "" {
		// If not set, generate one.
		id = uuid.New()
	}

	return stdContext.WithValue(ctx, correlationIDKey, id), id
}

// Serve serves the middleware
func (l *loggingMiddleware) ServeHTTP(ctx context.Context) {
	start := time.Now()

	req := ctx.Request()
	stdCtx := req.Context()

	stdCtx, correlationID := initCorrelationID(stdCtx, req.Header)
	req.Header.Set(correlationIdHeader, correlationID) // Request.
	ctx.Header(correlationIdHeader, correlationID)     // Response.

	logger := logrus.NewEntry(logrus.StandardLogger()).
		WithFields(logrus.Fields{
			"correlation_id": correlationID,
			"method":      ctx.Method(),
			"path":        ctx.Path(),
			"ip":          ctx.RemoteAddr(),
		})

	// Replace the context in iris with the updated context containing
	// the logger.
	stdCtx = vouslog.WithLogger(stdCtx, logger)
	req = req.WithContext(stdCtx)
	ctx.ResetRequest(req)
	// TODO: Does this persist the header change

	logger.Info("Starting request.")
	context.DefaultNext(ctx)

	// no time.Since in order to format it well after
	duration := time.Since(start)
	logger = logger.WithFields(logrus.Fields{
		"duration_ms": float64(duration) / 1e6, // ms.
		"status_code": ctx.GetStatusCode(),
	})
	logger.Info("Request finished.") // Print first to avoid issues with exotic errors.
}
