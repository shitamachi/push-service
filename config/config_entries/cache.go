package config_entries

type CacheConfig struct {
	RedisAddr     string `json:"redis_addr"`
	RedisDB       int    `json:"redis_db"`
	RedisPassword string `json:"redis_password"`
	ReadTimeout   int    `json:"read_timeout"`
	WriteTimeout  int    `json:"write_timeout"`
}
