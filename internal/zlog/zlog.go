package zlog

import (
	"fmt"
	"os"
	"strings"

	"BlahajChatServer/config"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	L *zap.Logger
	S *zap.SugaredLogger
)

func Init() {
	c := config.GetConfig().Log

	level := parseLevel(c.Level)

	encoderCfg := zapcore.EncoderConfig{
		TimeKey:        "ts",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalColorLevelEncoder,
		EncodeTime:     zapcore.TimeEncoderOfLayout("2006-01-02 15:04:05.000"),
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	var encoder zapcore.Encoder
	if strings.ToLower(c.Format) == "json" {
		encoderCfg.EncodeLevel = zapcore.CapitalLevelEncoder
		encoder = zapcore.NewJSONEncoder(encoderCfg)
	} else {
		encoder = zapcore.NewConsoleEncoder(encoderCfg)
	}

	cores := []zapcore.Core{
		zapcore.NewCore(encoder, zapcore.AddSync(os.Stdout), level),
	}

	if c.File != "" {
		f, err := os.OpenFile(c.File, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "zlog: 打开日志文件失败 %s: %v\n", c.File, err)
		} else {
			fileEncCfg := encoderCfg
			fileEncCfg.EncodeLevel = zapcore.CapitalLevelEncoder
			fileEnc := zapcore.NewJSONEncoder(fileEncCfg)
			cores = append(cores, zapcore.NewCore(fileEnc, zapcore.AddSync(f), level))
		}
	}

	core := zapcore.NewTee(cores...)
	L = zap.New(core, zap.AddCaller(), zap.AddCallerSkip(0), zap.AddStacktrace(zapcore.ErrorLevel))
	S = L.Sugar()
}

func parseLevel(s string) zapcore.Level {
	switch strings.ToLower(s) {
	case "debug":
		return zapcore.DebugLevel
	case "warn", "warning":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	case "fatal":
		return zapcore.FatalLevel
	default:
		return zapcore.InfoLevel
	}
}

func Sync() {
	if L != nil {
		_ = L.Sync()
	}
}

// 结构化日志：msg 后跟任意 key-value 对，例如
//
//	zlog.Info("服务启动中", "env", env, "port", port)
func Debug(msg string, kv ...any) { S.Debugw(msg, kv...) }
func Info(msg string, kv ...any)  { S.Infow(msg, kv...) }
func Warn(msg string, kv ...any)  { S.Warnw(msg, kv...) }
func Error(msg string, kv ...any) { S.Errorw(msg, kv...) }
func Fatal(msg string, kv ...any) { S.Fatalw(msg, kv...) }

// printf 风格
func Debugf(format string, args ...any) { S.Debugf(format, args...) }
func Infof(format string, args ...any)  { S.Infof(format, args...) }
func Warnf(format string, args ...any)  { S.Warnf(format, args...) }
func Errorf(format string, args ...any) { S.Errorf(format, args...) }
func Fatalf(format string, args ...any) { S.Fatalf(format, args...) }

// Err 是 err key 的快捷方式：zlog.Error("xxx 失败", zlog.Err(err), "key", v)
func Err(err error) any { return zap.Error(err) }

// WithFields 返回一个绑定字段的 sugared logger，适合一次日志里大量字段复用
func WithFields(kv ...any) *zap.SugaredLogger { return S.With(kv...) }
