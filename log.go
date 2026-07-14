package migrate

import (
	"context"
)

type Logger interface {
	Info(context.Context, string, ...any)
	Warn(context.Context, string, ...any)
	Error(context.Context, string, ...any)
}
