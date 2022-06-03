package config_entries

type MqConfig struct {
	// 重新恢复消息的时间间隔, 单位 ms
	RecoverMessageDuration   int `json:"recover_message_duration"`
	MaxRetryCount            int `json:"max_retry_count"`
	OnceReadMessageCount     int `json:"once_read_message_count"`
	InitCreatedConsumerCount int `json:"init_created_consumer_count"`
	MaxPendingTime           int `json:"max_pending_time"`
}
