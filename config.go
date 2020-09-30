package main

import (
	"osu-api-proxy/osuapi"
	"strings"

	"github.com/spf13/viper"
)

type databaseConfig struct {
	Dsn string
}

type httpConfig struct {
	Address string
}

type config struct {
	Database  databaseConfig `mapstructure:"database"`
	APIConfig osuapi.Config  `mapstructure:"api"`
	HTTP      httpConfig     `mapstructure:"http"`
}

func getConfig() (config, error) {
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	viper.SetConfigName("config")
	viper.SetConfigType("toml")
	viper.AddConfigPath("/etc/osuproxy/")
	viper.AddConfigPath("$HOME/.osuproxy/")
	viper.AddConfigPath(".")

	var cfg config

	if err := viper.ReadInConfig(); err != nil {
		return cfg, err
	}

	if err := viper.Unmarshal(&cfg); err != nil {
		return cfg, err
	}

	return cfg, nil
}
