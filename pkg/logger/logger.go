package logger

import (
    "os"
    "path/filepath"

    "go.uber.org/zap"
    "go.uber.org/zap/zapcore"
)

var log *zap.SugaredLogger

func Init(logPath string) error {
    // 确保日志目录存在
    if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
        return err
    }

    // 创建两个encoder
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

    // 创建输出
    consoleOutput := zapcore.Lock(os.Stdout)
    file, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
    if err != nil {
        return err
    }
    fileOutput := zapcore.Lock(file)

    // 设置日志级别
    level := zap.NewAtomicLevelAt(zap.InfoLevel)

    // 创建core
    core := zapcore.NewTee(
        zapcore.NewCore(consoleEncoder, consoleOutput, level),
        zapcore.NewCore(fileEncoder, fileOutput, level),
    )

    // 创建logger
    logger := zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))
    log = logger.Sugar()
    
    return nil
}

// 导出日志方法
func Info(args ...interface{})  { log.Info(args...) }
func Error(args ...interface{}) { log.Error(args...) }
func Debug(args ...interface{}) { log.Debug(args...) }
func Warn(args ...interface{})  { log.Warn(args...) }