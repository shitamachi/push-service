package db

import (
	"fmt"
	"github.com/shitamachi/push-service/config"
	"github.com/shitamachi/push-service/ent"
)

var Client *ent.Client

func InitDB() (*ent.Client, error) {
	client, err := ent.Open("mysql",
		//username:password.@tcp(127.0.0.1:3306)/db_name?checkConnLiveness=false&loc=Local&parseTime=true&readTimeout=1s&timeout=3s&writeTimeout=1s
		getDSN(),
		ent.Debug(),
	)
	if err != nil {
		return nil, err
	}

	Client = client

	return client, nil
}

func getDSN() string {
	conf := config.GlobalConfig.DBConfig
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?checkConnLiveness=false&loc=Local&parseTime=true&readTimeout=1s&timeout=3s&writeTimeout=1s",
		conf.User, conf.Password, conf.Addr, conf.Port, conf.DB,
	)
}
