package slogbrick

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"math"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/demeero/bricks/configbrick"
)

type logMsg struct {
	Time   time.Time   `json:"time"`
	Level  string      `json:"level"`
	Source slog.Source `json:"source"`
	Msg    string      `json:"msg"`
	Err    string      `json:"err"`
}

func TestConfigure(t *testing.T) {
	tests := []struct {
		name   string
		config configbrick.Log
		w      *bytes.Buffer
	}{
		{
			name:   "JSON",
			config: configbrick.Log{JSON: true, AddSource: true},
			w:      &bytes.Buffer{},
		},
		{
			name:   "Pretty",
			config: configbrick.Log{Pretty: true, AddSource: true},
			w:      &bytes.Buffer{},
		},
		{
			name:   "Text",
			config: configbrick.Log{AddSource: true},
			w:      &bytes.Buffer{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Configure(tt.config, WithAttrs(slog.String("field1", "value1")), WithWriter(tt.w))
			slog.Error("some error", slog.Any("err", assert.AnError))

			logData := strings.Split(tt.w.String(), "\n")
			require.Len(t, logData, 3) // 2 log lines and an empty line

			today := time.Now().Format(time.DateOnly)
			if !tt.config.JSON {
				assert.Contains(t, logData[0], today)
				assert.Contains(t, logData[0], "INF")
				assert.Contains(t, logData[0], "log configured")
				assert.Contains(t, logData[0], "logger.go")
				assert.Contains(t, logData[0], "field1")
				assert.Contains(t, logData[0], "value1")
				assert.Contains(t, logData[1], today)
				assert.Contains(t, logData[1], "ERR")
				assert.Contains(t, logData[1], "some error")
				assert.Contains(t, logData[1], "logger_test.go")
				assert.Contains(t, logData[1], assert.AnError.Error())
				assert.Contains(t, logData[1], "field1")
				assert.Contains(t, logData[1], "value1")
				return
			}

			var infoMsg logMsg
			assert.NoError(t, json.Unmarshal([]byte(logData[0]), &infoMsg))
			assert.False(t, infoMsg.Time.IsZero())
			assert.Equal(t, slog.LevelInfo.String(), infoMsg.Level)
			assert.NotEmpty(t, infoMsg.Source)
			assert.Equal(t, "log configured", infoMsg.Msg)
			assert.Empty(t, infoMsg.Err)

			var errMsg logMsg
			assert.NoError(t, json.Unmarshal([]byte(logData[1]), &errMsg))
			assert.False(t, errMsg.Time.IsZero())
			assert.Equal(t, slog.LevelError.String(), errMsg.Level)
			assert.NotEmpty(t, errMsg.Source)
			assert.Equal(t, "some error", errMsg.Msg)
			assert.Equal(t, assert.AnError.Error(), errMsg.Err)
		})
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		name     string
		level    string
		fallback slog.Level
		expected slog.Level
	}{
		{
			name:     "ValidInfoLevel1",
			level:    "info",
			fallback: math.MaxInt,
			expected: slog.LevelInfo,
		},
		{
			name:     "ValidDebugLevel",
			level:    "DEBUG",
			fallback: math.MaxInt,
			expected: slog.LevelDebug,
		},
		{
			name:     "ValidErrorLevel3",
			level:    "ErRoR",
			fallback: math.MaxInt,
			expected: slog.LevelError,
		},
		{
			name:     "InvalidLevel",
			level:    "invalid",
			expected: slog.LevelInfo,
			fallback: slog.LevelInfo,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := ParseLevel(tt.level, tt.fallback)
			assert.Equal(t, tt.expected, actual)
		})
	}
}

func TestFromCtx(t *testing.T) {
	lg := slog.New(slog.NewTextHandler(os.Stdout, nil))
	tests := []struct {
		name     string
		ctx      context.Context
		expected *slog.Logger
	}{
		{
			name:     "WithLogger",
			ctx:      ToCtx(context.Background(), lg),
			expected: lg,
		},
		{
			name:     "WithoutLogger",
			ctx:      context.Background(),
			expected: slog.Default(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := FromCtx(tt.ctx)
			assert.Equal(t, tt.expected, actual)
		})
	}
}

func TestToCtx(t *testing.T) {
	lg := slog.New(slog.NewTextHandler(os.Stdout, nil))

	ctx := ToCtx(context.Background(), lg)

	actual, ok := ctx.Value(logKey).(*slog.Logger)

	require.True(t, ok)
	assert.Equal(t, lg, actual)
}
