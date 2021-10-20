package config_entries

type MqConfig struct {
	// 重新恢复消息的时间间隔, 单位 ms
	RecoverMessageDuration int `json:"recover_message_duration"`
	MaxRetryCount          int `json:"max_retry_count"`
}
