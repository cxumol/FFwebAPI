package config

import (
    "strings"
    "time"

    "github.com/spf13/viper"
)

type Config struct {
    FFBin               string        `mapstructure:"FF_BIN"`
    FFTimeout           time.Duration `mapstructure:"FF_TIMEOUT"`
    OutputLocalLifetime time.Duration `mapstructure:"OUTPUT_LOCAL_LIFETIME"`
    MaxInputSize        int64         `mapstructure:"MAX_INPUT_SIZE"` // in bytes
    MaxConcurrency      int           `mapstructure:"MAX_CONCURRENCY"`
    ThrottleCPU         float64       `mapstructure:"THROTTLE_CPU"` // percentage
    ThrottleFreeMem     int64         `mapstructure:"THROTTLE_FREEMEM"` // in bytes
    ThrottleFreeDisk    int64         `mapstructure:"THROTTLE_FREEDISK"` // in bytes
    AuthEnable          bool          `mapstructure:"AUTH_ENABLE"`
    AuthKey             string        `mapstructure:"AUTH_KEY"`
    Port                string        `mapstructure:"PORT"`
    BaseURL             string        `mapstructure:"BASE"`
    TempDir             string        // Not from config, but useful to have globally
}

// ByteSize is a helper type to unmarshal size strings (e.g., "200MB")
// For simplicity in this implementation, we use Viper's built-in size parsing
// and handle it in the Load function directly.

func Load() (*Config, error) {
    vp := viper.New()
    
    // Set default values
    vp.SetDefault("FF_BIN", "ffmpeg")
    vp.SetDefault("FF_TIMEOUT", "12m3s")
    vp.SetDefault("OUTPUT_LOCAL_LIFETIME", "1h23m")
    vp.SetDefault("MAX_INPUT_SIZE", "200MB")
    vp.SetDefault("MAX_CONCURRENCY", 1)
    vp.SetDefault("THROTTLE_CPU", 50.0)
    vp.SetDefault("THROTTLE_FREEMEM", "200MB")
    vp.SetDefault("THROTTLE_FREEDISK", "200MB")
    vp.SetDefault("AUTH_ENABLE", false)
    vp.SetDefault("AUTH_KEY", "123456")
    vp.SetDefault("PORT", "8080")
    vp.SetDefault("BASE", "")

    // Load from config file
    vp.SetConfigName("ffwebapi_config")
    vp.SetConfigType("yaml")
    vp.AddConfigPath(".")
    vp.AddConfigPath("/etc/ffwebapi/")
    
    if err := vp.ReadInConfig(); err != nil {
        if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
            return nil, err
        }
    }

    // Load from environment variables
    vp.SetEnvPrefix("FFWEBAPI")
    vp.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
    vp.AutomaticEnv()

    var cfg Config
    
    // We need to manually parse duration and size strings when unmarshalling
    // Viper doesn't automatically do this for struct fields from env vars
    // without more complex hooks. This is a simple and effective way.

    d, err := time.ParseDuration(vp.GetString("FF_TIMEOUT"))
    if err != nil { return nil, err }
    cfg.FFTimeout = d

    d, err = time.ParseDuration(vp.GetString("OUTPUT_LOCAL_LIFETIME"))
    if err != nil { return nil, err }
    cfg.OutputLocalLifetime = d

    // The rest of the fields can be unmarshalled directly
    if err := vp.Unmarshal(&cfg); err != nil {
        return nil, err
    }
    
    // Handle size strings from env vars/config using Viper's Get...Size... methods
    cfg.MaxInputSize = vp.GetInt64("MAX_INPUT_SIZE")
    cfg.ThrottleFreeMem = vp.GetInt64("THROTTLE_FREEMEM")
    cfg.ThrottleFreeDisk = vp.GetInt64("THROTTLE_FREEDISK")

    return &cfg, nil
}
