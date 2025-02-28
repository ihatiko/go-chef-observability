package tech

import (
	"fmt"
	tC "github.com/ihatiko/go-chef-configuration/config"
	"github.com/ihatiko/go-chef-configuration/toml"
)

func Use(arg string) error {
	c := new(Config)
	err := toml.Unmarshal(defaultTechConfig, c)
	if err != nil {
		e := fmt.Errorf("error unmarshalling tech-config: %v command %s", err, arg)
		return e
	}
	err = tC.ToConfig(c)
	if err != nil {
		e := fmt.Errorf("error unmarshalling tech-config (applyed main config): %v command %s", err, arg)
		return e
	}
	c.ToEnv()
	c.Tech.Http.New().Run()
	c.Tech.Logger.New()
	c.Tech.Tracer.New()
	return err
}
