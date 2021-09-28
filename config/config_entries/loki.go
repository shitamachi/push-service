package config_entries

type LokiConfig struct {
	// the loki api url
	URL string `json:"url"`
	//(optional, default: hot-novel) 用于区分不同应用的日志来源,且会依据不同的 LoggerType 添加后缀用于区分不同的 logger
	Source string `json:"source"`
	//(optional, default: severity) the label's key to distinguish log's level, it will be added to Labels map
	LevelName string `json:"level_name"`
	// (optional, default: zapcore.InfoLevel) logs beyond this level will be sent
	SendLevel int8 `json:"send_level"`
	// the labels which will be sent to loki, contains the {levelname: level}
	Labels map[string]string `json:"labels"`
}
