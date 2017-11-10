package main

import (
	"errors"
	"fmt"
	"net/url"
	"os"
)

type Config struct {
	listenAddress  string
	metricPath     string
	apiLogin       string
	apiPassword    string
	apiEndpoint    string
	apiEndpointUrl *url.URL
}

func NewConfig() *Config {
	return &Config{
		listenAddress: ":9190",
		metricPath:    "/metrics",
	}
}

func (c *Config) Load() error {
	if err := c.LoadFromEnv(); err != nil {
		return err
	}
	if len(c.listenAddress) == 0 {
		return errors.New("Listen address must be defined")
	}
	if len(c.metricPath) == 0 {
		return errors.New("Metric path must be defined")
	}
	if len(c.apiLogin) == 0 {
		return errors.New("API login must be defined")
	}
	if len(c.apiPassword) == 0 {
		return errors.New("API password must be defined")
	}
	if len(c.apiEndpoint) == 0 {
		return errors.New("API URL must be defined")
	}
	u, err := url.Parse(c.apiEndpoint)
	if err != nil {
		return fmt.Errorf("Can't parse API URL: %v", err)
	}
	c.apiEndpointUrl = u
	return nil
}

func (c *Config) LoadFromEnv() error {
	listenAddressRaw := os.Getenv("TE_LISTEN_ADDRESS")
	if len(listenAddressRaw) != 0 {
		c.listenAddress = listenAddressRaw
	}
	metricPathRaw := os.Getenv("TE_METRIC_PATH")
	if len(metricPathRaw) != 0 {
		c.metricPath = metricPathRaw
	}
	apiLoginRaw := os.Getenv("TE_API_LOGIN")
	if len(apiLoginRaw) != 0 {
		c.apiLogin = apiLoginRaw
	}
	apiPasswordRaw := os.Getenv("TE_API_PASSWORD")
	if len(apiPasswordRaw) != 0 {
		c.apiPassword = apiPasswordRaw
	}
	apiEndpointRaw := os.Getenv("TE_API_URL")
	if len(apiEndpointRaw) != 0 {
		c.apiEndpoint = apiEndpointRaw
	}
	return nil
}
