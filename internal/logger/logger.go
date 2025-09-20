package logger

import (
	"go.uber.org/zap"
)

// New creates a new zap logger based on environment.
func New(env string) *zap.Logger {
	if env == "development" {
		l, _ := zap.NewDevelopment()
		return l
	}
	l, _ := zap.NewProduction()
	return l
}
