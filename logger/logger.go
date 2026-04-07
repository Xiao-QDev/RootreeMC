// Package logger 自定义日志处理器
package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
)

// MCLogHandler 自定义 Minecraft 风格日志处理器
type MCLogHandler struct {
	handler slog.Handler
	writer  io.Writer
}

// NewMCLogHandler 创建自定义日志处理器
func NewMCLogHandler(w io.Writer) *MCLogHandler {
	return &MCLogHandler{
		handler: slog.NewTextHandler(w, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}),
		writer: w,
	}
}

// Enabled 实现 slog.Handler 接口
func (h *MCLogHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.handler.Enabled(ctx, level)
}

// Handle 实现 slog.Handler 接口
func (h *MCLogHandler) Handle(ctx context.Context, record slog.Record) error {
	level := record.Level.String()
	msg := record.Message

	// 格式化时间：[月 日 时:分:秒]
	timestamp := record.Time.Format("01/02 15:04:05")

	// 构建结构化字段
	attrs := ""
	record.Attrs(func(a slog.Attr) bool {
		attrs += " " + a.Key + "=" + a.Value.String()
		return true
	})

	// 输出格式：[时间戳] [级别] 消息 字段
	line := fmt.Sprintf("[%s] [%s] %s%s\n", timestamp, level, msg, attrs)

	_, err := h.writer.Write([]byte(line))
	return err
}

// WithAttrs 实现 slog.Handler 接口
func (h *MCLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &MCLogHandler{
		handler: h.handler.WithAttrs(attrs),
		writer:  h.writer,
	}
}

// WithGroup 实现 slog.Handler 接口
func (h *MCLogHandler) WithGroup(name string) slog.Handler {
	return &MCLogHandler{
		handler: h.handler.WithGroup(name),
		writer:  h.writer,
	}
}
