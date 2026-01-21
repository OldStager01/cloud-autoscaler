package config

import (
	"github.com/OldStager01/cloud-autoscaler/pkg/database"
)

func (d DatabaseConfig) ToDBConfig() database.Config {
	return database.Config{
		Host:           d.Host,
		Port:           d.Port,
		Name:           d.Name,
		User:           d.User,
		Password:       d.Password,
		MaxConnections:  d.MaxConnections,
		SSLMode:        d.SSLMode,
	}
}