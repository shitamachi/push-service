package config

import (
	"context"
	"encoding/json"
	"github.com/shitamachi/push-service/config/config_entries"
	"os"
)

type AppConfig struct {
	// worker node id
	WorkerID int `json:"worker_id"`
	// mode; debug 发送的为测试环境的 push; production 为线上环境的 push
	Mode string `json:"mode"`
	// server port
	Port               int                                        `json:"port"`
	LogMode            string                                     `json:"log_mode"`
	LogFilePath        string                                     `json:"log_file_path"`
	ClientConfig       map[string]config_entries.ClientConfigItem `json:"client_config"`
	DBConfig           config_entries.DBConfigItem                `json:"db_config"`
	CacheConfig        config_entries.CacheConfig                 `json:"cache_config"`
	ApplePushConfig    config_entries.ApplePushSecretConfig       `json:"apple_push_config"`
	FirebasePushConfig config_entries.FirebaseConfig              `json:"firebase_push_config"`
	Mq                 config_entries.MqConfig                    `json:"mq"`
}

func InitConfig() *AppConfig {
	bytes, err := os.ReadFile("conf/conf.json")
	if err != nil {
		panic(err)
	}
	var config AppConfig
	err = json.Unmarshal(bytes, &config)
	if err != nil {
		panic(err)
	}
	return &config
}

type SetConfigToContextKey string

var key = SetConfigToContextKey("config")

func SetToContext(ctx context.Context, config *AppConfig) context.Context {
	return context.WithValue(ctx, key, config)
}

func GetFromContext(ctx context.Context) *AppConfig {
	config := ctx.Value(key).(*AppConfig)
	if config == nil {
		config = InitConfig()
	}

	return config
}
