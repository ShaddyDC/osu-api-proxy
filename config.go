package main

import (
	"osu-api-proxy/osuapi"
	"strings"

	"github.com/spf13/viper"
)

type databaseConfig struct {
	Dsn string
}

type endpointConfig struct {
	Handler     string `mapstructure:"handler"`
	CachePolicy string `mapstructure:"cache"`
}

type authServerConfig struct {
	Address string `mapstructure:"address"`
}

type promServerConfig struct {
	Address string `mapstructure:"address"`
}

type apiserverConfig struct {
	Address   string           `mapstructure:"address"`
	Endpoints []endpointConfig `mapstructure:"endpoint"`
}

type config struct {
	Database   databaseConfig   `mapstructure:"database"`
	APIConfig  osuapi.Config    `mapstructure:"api"`
	ApiServer  apiserverConfig  `mapstructure:"apiserver"`
	Auth       authServerConfig `mapstructure:"auth"`
	PromServer promServerConfig `mapstructure:"prom"`
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
