package api

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

// The names an owner picks a log level by, as HL_LOG_LEVEL takes them.
const (
	levelDebug = "debug"
	levelInfo  = "info"
	levelWarn  = "warn"
	levelError = "error"
)

// logLevelBody is how much the server logs: now, and when it started.
type logLevelBody struct {
	Level   string `json:"level" enum:"debug,info,warn,error" doc:"What the server logs now: debug for everything, info for what it does, warn for problems, error for failures alone."`
	Started string `json:"started" enum:"debug,info,warn,error" doc:"What it logged when it started, HL_LOG_LEVEL's level, which a restart goes back to."`
}

type logLevelOutput struct {
	Body logLevelBody
}

type setLogLevelInput struct {
	Body struct {
		Level string `json:"level" enum:"debug,info,warn,error" doc:"What the server logs from now until it restarts."`
	}
}

func (h *handlers) registerLogLevel() {
	get := h.op("get-log-level", http.MethodGet, "/log-level", "How much the server logs")
	huma.Register(h.api, get, h.getLogLevel)

	set := h.op("set-log-level", http.MethodPut, "/log-level", "Set how much the server logs")
	set.Description = "From now until the server restarts, which goes back to HL_LOG_LEVEL's level."
	huma.Register(h.api, set, h.setLogLevel)
}

func (h *handlers) getLogLevel(_ context.Context, _ *struct{}) (*logLevelOutput, error) {
	return &logLevelOutput{Body: h.logLevel()}, nil
}

func (h *handlers) setLogLevel(ctx context.Context, in *setLogLevelInput) (*logLevelOutput, error) {
	var level slog.Level
	if err := level.UnmarshalText([]byte(in.Body.Level)); err != nil {
		return nil, huma.Error422UnprocessableEntity("That isn't a log level.")
	}
	was := h.Level.Level()
	h.Level.Set(level)
	if level != was {
		// At a level the new one still logs, so the change is in the log whichever way it went.
		h.Logger.Log(ctx, max(level, slog.LevelInfo), "log level set", "level", levelName(level), "was", levelName(was))
	}
	return &logLevelOutput{Body: h.logLevel()}, nil
}

func (h *handlers) logLevel() logLevelBody {
	return logLevelBody{Level: levelName(h.Level.Level()), Started: levelName(h.StartLevel)}
}

// levelName is a level's name, or the name of the nearest below it for one between them.
func levelName(l slog.Level) string {
	switch {
	case l < slog.LevelInfo:
		return levelDebug
	case l < slog.LevelWarn:
		return levelInfo
	case l < slog.LevelError:
		return levelWarn
	default:
		return levelError
	}
}
