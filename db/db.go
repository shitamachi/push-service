package db

import (
	"entgo.io/ent/dialect/sql"
	"fmt"
	"github.com/shitamachi/push-service/config"
	"github.com/shitamachi/push-service/config/config_entries"
	"github.com/shitamachi/push-service/ent"
	"time"
)

func InitDB(config *config.AppConfig) *ent.Client {
	driver, err := sql.Open(
		"mysql",
		//username:password.@tcp(127.0.0.1:3306)/db_name?checkConnLiveness=false&loc=Local&parseTime=true&readTimeout=1s&timeout=3s&writeTimeout=1s
		getDSN(&config.DBConfig),
	)
	if err != nil {
		panic(err)
	}

	db := driver.DB()
	db.SetMaxIdleConns(10)
	db.SetMaxOpenConns(100)
	db.SetConnMaxLifetime(time.Hour)

	client := ent.NewClient(ent.Driver(driver), ent.Debug())
	if client == nil {
		panic("got client but is nil")
	}

	return client
}

func getDSN(conf *config_entries.DBConfigItem) string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?checkConnLiveness=false&loc=Local&parseTime=true&readTimeout=3s&timeout=3s&writeTimeout=1s",
		conf.User, conf.Password, conf.Addr, conf.Port, conf.DB,
	)
}
