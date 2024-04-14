package config

import (
	"time"

	"github.com/num30/config"
)

type Config struct {
	RunAddress            string        `default:":8080" envvar:"RUN_ADDR"`
	LogLevel              string        `default:"info" flag:"loglevel" envvar:"LOGLEVEL"`
	TokenTTL              time.Duration `default:"24h"`
	ProductUpdateInterval time.Duration `default:"1h" envvar:"UPDATE_INTERVAL"`
	DummyJSONAddress      string        `default:"https://dummyjson.com"`
	DB                    Database      `default:"{}"`
	DebugMode             bool          `flag:"debug"`
}

type Database struct {
	Host     string `default:"localhost" validate:"required" envvar:"DB_HOST"`
	Port     int    `default:"5434" envvar:"DB_PORT"`
	Password string `default:"catalog_db" validate:"required" envvar:"DB_PASS"`
	DbName   string `default:"catalog_db" envvar:"DB_NAME"`
	Username string `default:"catalog_db" envvar:"DB_USERNAME"`
}

func MustBuild(cfgFile string) *Config {
	var conf Config
	err := config.NewConfReader(cfgFile).Read(&conf)
	if err != nil {
		panic(err)
	}

	return &conf
}
