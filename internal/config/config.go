package config

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

var ErrMissingEnvironmentVariables = errors.New("missing required environment variables")

type Config struct {
	Env      string   `mapstructure:"env"`
	Telegram Telegram `mapstructure:"telegram"`
	Redis    Redis    `mapstructure:"redis"`
	Bot      Bot      `mapstructure:"bot"`
}

type Telegram struct {
	Token  string `mapstructure:"token"`
	ChatID int64  `mapstructure:"chat_id"`
	Debug  bool   `mapstructure:"debug"`
}

type Redis struct {
	Addr        string        `mapstructure:"addr"`
	User        string        `mapstructure:"user"`
	Password    string        `mapstructure:"password"`
	DB          int           `mapstructure:"db"`
	MaxRetries  int           `mapstructure:"max_retries"`
	DialTimeout time.Duration `mapstructure:"dial_timeout"`
	Timeout     time.Duration `mapstructure:"timeout"`
}

type Bot struct {
	TimeoutSec   int `mapstructure:"timeout_sec"`
	WarnAfterSec int `mapstructure:"warn_after_sec"`
}

// IsDevelopment returns true if the environment is dev/local.
func (c *Config) IsDevelopment() bool {
	return c.Env == "development" || c.Env == "dev" || c.Env == "local"
}

func Load() (*Config, error) {
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath("./config")

	v.SetDefault("env", "development")

	v.SetDefault("telegram.debug", false)

	// Redis defaults
	v.SetDefault("redis.addr", "127.0.0.1:6379")
	v.SetDefault("redis.db", 0)
	v.SetDefault("redis.max_retries", 3)
	v.SetDefault("redis.dial_timeout", "5s")
	v.SetDefault("redis.timeout", "2s")

	// Bot defaults
	v.SetDefault("bot.timeout_sec", 3600)
	v.SetDefault("bot.warn_after_sec", 1800)

	v.SetEnvPrefix("APP")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		var configFileNotFoundError viper.ConfigFileNotFoundError
		if !errors.As(err, &configFileNotFoundError) {
			return nil, fmt.Errorf("read config: %w", err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	cfg.Telegram.Token = v.GetString("telegram_api_token")
	cfg.Telegram.ChatID = v.GetInt64("telegram_chat_id")

	cfg.Redis.Addr = v.GetString("redis.addr")
	cfg.Redis.Password = v.GetString("redis.password")
	cfg.Redis.DB = v.GetInt("redis.db")

	if cfg.Telegram.Token == "" {
		return nil, fmt.Errorf("%w: APP_TELEGRAM_API_TOKEN", ErrMissingEnvironmentVariables)
	}
	if cfg.Telegram.ChatID == 0 {
		return nil, fmt.Errorf("%w: APP_TELEGRAM_CHAT_ID", ErrMissingEnvironmentVariables)
	}

	return &cfg, nil
}
