package logger

import (
    "os"
    "path/filepath"

    "go.uber.org/zap"
    "go.uber.org/zap/zapcore"
)

var log *zap.SugaredLogger

func Init(logPath string) error {
    if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
        return err
    }

    consoleEncoder := zapcore.NewConsoleEncoder(zapcore.EncoderConfig{
        TimeKey:        "T",
        LevelKey:       "L",
        NameKey:        "N",
        CallerKey:      "C",
        MessageKey:     "M",
        StacktraceKey:  "S",
        LineEnding:     zapcore.DefaultLineEnding,
        EncodeLevel:    zapcore.CapitalColorLevelEncoder,
        EncodeTime:     zapcore.TimeEncoderOfLayout("15:04:05"),
        EncodeDuration: zapcore.StringDurationEncoder,
        EncodeCaller:   zapcore.ShortCallerEncoder,
    })

    fileEncoder := zapcore.NewJSONEncoder(zapcore.EncoderConfig{
        TimeKey:        "timestamp",
        LevelKey:       "level",
        NameKey:        "logger",
        CallerKey:      "caller",
        MessageKey:     "msg",
        StacktraceKey:  "stacktrace",
        LineEnding:     zapcore.DefaultLineEnding,
        EncodeLevel:    zapcore.LowercaseLevelEncoder,
        EncodeTime:     zapcore.ISO8601TimeEncoder,
        EncodeDuration: zapcore.SecondsDurationEncoder,
        EncodeCaller:   zapcore.ShortCallerEncoder,
    })

    consoleOutput := zapcore.Lock(os.Stdout)
    file, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
    if err != nil {
        return err
    }
    fileOutput := zapcore.Lock(file)

    level := zap.NewAtomicLevelAt(zap.InfoLevel)

    core := zapcore.NewTee(
        zapcore.NewCore(consoleEncoder, consoleOutput, level),
        zapcore.NewCore(fileEncoder, fileOutput, level),
    )

    logger := zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))
    log = logger.Sugar()
    
    return nil
}

func Info(args ...interface{})  { log.Info(args...) }
func Error(args ...interface{}) { log.Error(args...) }
func Debug(args ...interface{}) { log.Debug(args...) }
func Warn(args ...interface{})  { log.Warn(args...) }