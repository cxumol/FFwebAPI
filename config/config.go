// ffwebapi/config/config.go
package config

import (
	"reflect"
	"strings"
	"time"

	"github.com/c2h5oh/datasize"
	"github.com/mitchellh/mapstructure"
	"github.com/spf13/viper"
)

type Config struct {
	FFBin               string        `mapstructure:"FF_BIN"`
	FFTimeout           time.Duration `mapstructure:"FF_TIMEOUT"`
	OutputLocalLifetime time.Duration `mapstructure:"OUTPUT_LOCAL_LIFETIME"`
	MaxInputSize        int64         `mapstructure:"MAX_INPUT_SIZE"`
	MaxConcurrency      int           `mapstructure:"MAX_CONCURRENCY"`
	ThrottleCPU         float64       `mapstructure:"THROTTLE_CPU"`
	ThrottleFreeMem     int64         `mapstructure:"THROTTLE_FREEMEM"`
	ThrottleFreeDisk    int64         `mapstructure:"THROTTLE_FREEDISK"`
	AuthEnable          bool          `mapstructure:"AUTH_ENABLE"`
	AuthKey             string        `mapstructure:"AUTH_KEY"`
	Port                string        `mapstructure:"PORT"`
	BaseURL             string        `mapstructure:"BASE"`
	TempDir             string
}

// stringToDurationHookFunc is a custom Viper hook for parsing Go's duration strings.
// <-- NEW
func stringToDurationHookFunc() mapstructure.DecodeHookFunc {
	return func(
		f reflect.Type,
		t reflect.Type,
		data interface{},
	) (interface{}, error) {
		// We only care about converting strings to time.Duration.
		if f.Kind() != reflect.String || t != reflect.TypeOf(time.Duration(0)) {
			return data, nil
		}

		// It is a string -> time.Duration. Parse it.
		return time.ParseDuration(data.(string))
	}
}

// stringToByteSizeHookFunc is a custom Viper hook for parsing human-readable size strings.
func stringToByteSizeHookFunc() mapstructure.DecodeHookFunc {
	return func(
		f reflect.Type,
		t reflect.Type,
		data interface{},
	) (interface{}, error) {
		// We only care about converting strings to int64s for byte sizes.
		if f.Kind() != reflect.String || t.Kind() != reflect.Int64 {
			return data, nil
		}

		var size datasize.ByteSize
		err := size.UnmarshalText([]byte(data.(string)))
		if err != nil {
			// Not a valid size string, let other parsers handle it.
			return data, nil
		}

		return int64(size.Bytes()), nil
	}
}

func Load() (*Config, error) {
	vp := viper.New()

	// Set default values as strings, the hooks will handle them.
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
	// Unmarshal the config, providing our custom composed hooks.
	// The order matters: the first hook that succeeds is used.
	// <-- CHANGED
	err := vp.Unmarshal(&cfg, viper.DecodeHook(
		mapstructure.ComposeDecodeHookFunc(
			stringToDurationHookFunc(),
			stringToByteSizeHookFunc(),
		),
	))
	if err != nil {
		return nil, err
	}

	return &cfg, nil
}
