package config

import (
	"github.com/ilyakaznacheev/cleanenv"
)

type AppConfig struct {
	App struct {
		Port           string `yaml:"port" env:"PORT" env-default:"7910"`
		MetricsEnabled bool   `yaml:"metrics_enabled" env:"METRICS_ENABLED"`
		IsProd         bool   `yaml:"is_prod" env:"IS_PROD"`
	} `yaml:"app"`

	Core struct {
		StrictMode bool `yaml:"strict_mode"`
		// В YAML написано modulse, тег считывает именно его
		Modules []string `yaml:"modules"`
	} `yaml:"core"`

	Cors struct {
		URL  string   `yaml:"url" env:"CORS_URL"`
		URLs []string `yaml:"urls" env:"CORS_URLS"`
	} `yaml:"cors"`
}

func LoadAppConfig(path string) (*AppConfig, error) {
	var cfg AppConfig
	err := cleanenv.ReadConfig(path, &cfg)
	if err != nil {
		return nil, err
	}

	return &cfg, err
}
