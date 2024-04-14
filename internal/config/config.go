package config

import (
	"github.com/num30/config"
)

type Config struct {
	RunAddress  string   `default:":8080" envvar:"RUN_ADDR"`
	LogLevel    string   `default:"info" flag:"loglevel" envvar:"LOGLEVEL"`
	DB          Database `default:"{}"`
	RedisURL    string   `envvar:"REDIS_URL"`
	CacheExpiry int      `default:"3600" envvar:"CACHE_EXPIRY"`
}

type Database struct {
	Host     string `default:"localhost" validate:"required" envvar:"DB_HOST"`
	Port     int    `default:"5434" envvar:"DB_PORT"`
	Password string `default:"banner_db" validate:"required" envvar:"DB_PASS"`
	DbName   string `default:"banner_db" envvar:"DB_NAME"`
	Username string `default:"banner_db" envvar:"DB_USERNAME"`
}

func MustBuild(cfgFile string) *Config {
	var conf Config
	err := config.NewConfReader(cfgFile).Read(&conf)
	if err != nil {
		panic(err)
	}

	return &conf
}
