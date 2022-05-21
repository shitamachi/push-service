package log

import (
	"github.com/shitamachi/push-service/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
	"os"
	"path/filepath"
)

var (
	Logger *zap.Logger
	Sugar  *zap.SugaredLogger
)

func InitLogger() {

	switch config.GlobalConfig.Mode {
	case "debug":
		logger, err := zap.NewDevelopment()
		if err != nil {
			panic(err)
		}
		Logger = logger
		Sugar = logger.Sugar()
	case "test":
		devEncoder := newDevEncoder()
		testCore := zapcore.NewCore(devEncoder, getLogWriter(), zap.LevelEnablerFunc(func(level zapcore.Level) bool {
			//所有 debug 以上级别（不包含 debug ）的日志将被输出到本地
			return level > zap.DebugLevel
		}))
		logger := zap.New(zapcore.NewTee(testCore), zap.AddCaller(), zap.AddCallerSkip(1))

		Logger = logger
		Sugar = logger.Sugar()
	case "release":
		releaseEncoder := newReleaseEncoder()
		testCore := zapcore.NewCore(releaseEncoder, getLogWriter(), zap.LevelEnablerFunc(func(level zapcore.Level) bool {
			//所有 debug 以上级别（不包含 debug ）的日志将被输出到本地
			return level > zap.DebugLevel
		}))
		logger := zap.New(zapcore.NewTee(testCore), zap.AddCaller(), zap.AddCallerSkip(1))

		Logger = logger
		Sugar = logger.Sugar()
	default:
		logger, err := zap.NewDevelopment()
		if err != nil {
			panic(err)
		}

		Logger = logger
		Sugar = logger.Sugar()
	}
}

func newDevEncoder() zapcore.Encoder {
	encoderConfig := zap.NewDevelopmentEncoderConfig()
	encoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout("2006-01-02 15:04:05")
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	return zapcore.NewConsoleEncoder(encoderConfig)
}

func newReleaseEncoder() zapcore.Encoder {
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout("2006-01-02 15:04:05")
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	return zapcore.NewJSONEncoder(encoderConfig)
}

func getLogWriter() zapcore.WriteSyncer {
	logPath := config.GlobalConfig.LogFilePath
	EnsureDirExisted(logPath)
	writer := &lumberjack.Logger{
		Filename:   logPath,
		MaxSize:    10,
		MaxAge:     7,
		MaxBackups: 10,
		LocalTime:  true,
		Compress:   false,
	}
	return zapcore.AddSync(writer)
}

func EnsureDirExisted(paths ...string) {
	for _, fileOrDirPath := range paths {
		path := filepath.Dir(fileOrDirPath)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			// log dir does not exist
			err := os.Mkdir(path, os.ModePerm)
			if err != nil {
				panic(err)
			}
		}
	}
}
