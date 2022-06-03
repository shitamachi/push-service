package log

import (
	"github.com/shitamachi/push-service/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
	"os"
	"path/filepath"
)

func InitLogger(config *config.AppConfig) (logger *zap.Logger, err error) {

	var mode string
	if len(config.LogMode) > 0 {
		mode = config.LogMode
	} else {
		mode = config.Mode
	}

	switch mode {
	case "debug":
		logger, err = zap.NewDevelopment()
		if err != nil {
			panic(err)
		}
	case "test":
		devEncoder := newDevEncoder()
		testCore := zapcore.NewCore(devEncoder, getLogWriter(config), zap.LevelEnablerFunc(func(level zapcore.Level) bool {
			//所有 debug 以上级别（不包含 debug ）的日志将被输出到本地
			return level > zap.DebugLevel
		}))
		logger = zap.New(zapcore.NewTee(testCore), zap.AddCaller(), zap.AddCallerSkip(1))
	case "release":
		releaseEncoder := newReleaseEncoder()
		testCore := zapcore.NewCore(releaseEncoder, getLogWriter(config), zap.LevelEnablerFunc(func(level zapcore.Level) bool {
			//所有 debug 以上级别（不包含 debug ）的日志将被输出到本地
			return level > zap.DebugLevel
		}))
		logger = zap.New(zapcore.NewTee(testCore), zap.AddCaller(), zap.AddCallerSkip(1))
	default:
		logger, err = zap.NewDevelopment()
		if err != nil {
			panic(err)
		}
	}

	return
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

func getLogWriter(config *config.AppConfig) zapcore.WriteSyncer {
	logPath := config.LogFilePath
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
