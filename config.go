package main

import (
	"main/database"
	natsLog "main/nats"
	"os"

	"gopkg.in/yaml.v3"
)

// Config структура для конфига
type Config struct {
	Postgres database.PostgresConfig `yaml:"postgres"`
	Redis    database.RedisConfig    `yaml:"redis"`
	Nats     natsLog.NatsConfig      `yaml:"nats"`
}

// loadConfig читаем конфиг
func loadConfig() (config *Config, err error) {
	data, err := os.ReadFile("config.yaml")
	if err != nil {
		return nil, err
	}
	err = yaml.Unmarshal(data, &config)
	return config, err
}
