package config

import (
	"encoding/json"
	"github.com/shitamachi/push-service/config/config_entries"
	"os"
)

type AppConfig struct {
	WorkerID           int                                        `json:"worker_id"` // 工作机器的 id
	Mode               string                                     `json:"mode"`      // mode; debug 发送的为测试环境的 push; production 为线上环境的 push
	Port               int                                        `json:"port"`      // server port
	LogFilePath        string                                     `json:"log_file_path"`
	ClientConfig       map[string]config_entries.ClientConfigItem `json:"client_config"`
	DBConfig           config_entries.DBConfigItem                `json:"db_config"`
	CacheConfig        config_entries.CacheConfig                 `json:"cache_config"`
	ApplePushConfig    config_entries.ApplePushSecretConfig       `json:"apple_push_config"`
	FirebasePushConfig config_entries.FirebaseConfig              `json:"firebase_push_config"`
}

var GlobalConfig *AppConfig

func init() {
	bytes, err := os.ReadFile("conf/conf.json")
	if err != nil {
		panic(err)
	}
	var tmp AppConfig
	err = json.Unmarshal(bytes, &tmp)
	if err != nil {
		panic(err)
	}
	GlobalConfig = &tmp
}
