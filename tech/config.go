package tech

import (
	_ "embed"
	"os"

	"github.com/ihatiko/go-chef-observability/http"
	"github.com/ihatiko/go-chef-observability/logger"
	"github.com/ihatiko/go-chef-observability/tracer"
)

//go:embed config/tech.config.toml
var defaultTechConfig []byte

type Service struct {
	Name  string `toml:"name"`
	Debug bool   `toml:"debug"`
}

type Config struct {
	Tech struct {
		Service Service       `toml:"service"`
		Logger  logger.Config `toml:"logger"`
		Tracer  tracer.Config `toml:"tracer"`
		Http    http.Config   `toml:"http"`
	} `toml:"tech"`
}

func (cfg *Config) ToEnv() {
	if os.Getenv("TECH.SERVICE.NAME") != "" {
		err := os.Setenv("TECH.SERVICE.NAME", cfg.Tech.Service.Name)
		if err != nil {
			return
		}
	}
	if os.Getenv("TECH.SERVICE.NAME") != "" {
		r := "false"
		if cfg.Tech.Service.Debug {
			r = "true"
		}
		err := os.Setenv("TECH.SERVICE.DEBUG", r)
		if err != nil {
			return
		}
	}
}
