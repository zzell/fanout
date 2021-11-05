package config

import (
	"fmt"

	"github.com/zzell/fanout/internal/cache"
	"github.com/zzell/fanout/internal/mq"
	"github.com/zzell/fanout/internal/service"
	"gopkg.in/yaml.v3"
)

// Config defines service configuration
type Config struct {
	MqConfig      *mq.Config      `yaml:"mq_config"`
	CacheConfig   *cache.Config   `yaml:"cache_config"`
	ServiceConfig *service.Config `yaml:"service_config"`
}

// NewConfig reads config from file
func NewConfig(file []byte) (*Config, error) {
	var cfg = new(Config)

	err := yaml.Unmarshal(file, cfg)
	if err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	return cfg, nil
}
