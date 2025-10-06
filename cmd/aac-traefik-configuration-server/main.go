package main

import (
	"context"
	"encoding/json"
	"github.com/Azure/AppConfiguration-GoProvider/azureappconfiguration"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/data/azappconfig"
	"github.com/caarlos0/env/v11"
	"github.com/labdigital-evolve/traefik-config-server/internal"
	"github.com/mitchellh/mapstructure"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/traefik/traefik/v3/pkg/config/dynamic"
	"net/http"
	"os"
	"strings"
	"time"
)

type config struct {
	LogLevel             string        `env:"LOG_LEVEL" envDefault:"INFO"`
	Endpoint             string        `env:"AZURE_APP_CONFIGURATION_ENDPOINT"`
	RefreshInterval      time.Duration `env:"REFRESH_INTERVAL" envDefault:"60s"`
	LabelFilter          string        `env:"LABEL_FILTER"`
	KeyFilter            string        `env:"KEY_FILTER" envDefault:"*"`
	ConfigurationSubPath string        `env:"CONFIGURATION_SUB_PATH" envDefault:""`
	IgnoreSubPath        bool          `env:"IGNORE_SUB_PATH" envDefault:"false"`
}

var cfg config

func init() {
	err := env.Parse(&cfg)
	if err != nil {
		log.Error().Msgf("Failed to parse environment variables: %v", err)
		os.Exit(1)
	}
	logLevel, err := zerolog.ParseLevel(cfg.LogLevel)
	if err != nil {
		log.Error().Msgf("Invalid log level: %s", cfg.LogLevel)
		os.Exit(1)
	}
	zerolog.SetGlobalLevel(logLevel)
}

func main() {
	var ctx = context.Background()

	log.Debug().Msgf("Using Azure App Configuration endpoint: %s", cfg.Endpoint)

	var credentials, err = azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		log.Error().Msgf("Failed to create Azure credentials: %v", err)
		os.Exit(1)
	}
	authOptions := azureappconfiguration.AuthenticationOptions{
		Endpoint:   cfg.Endpoint,
		Credential: credentials,
	}

	appConfig, err := azureappconfiguration.Load(ctx, authOptions, &azureappconfiguration.Options{
		Selectors: []azureappconfiguration.Selector{
			{
				KeyFilter:   cfg.KeyFilter,
				LabelFilter: cfg.LabelFilter,
			},
		},
		RefreshOptions: azureappconfiguration.KeyValueRefreshOptions{
			Interval: cfg.RefreshInterval,
			Enabled:  true,
		},
		ClientOptions: &azappconfig.ClientOptions{
			ClientOptions: policy.ClientOptions{
				Logging: policy.LogOptions{
					IncludeBody: true,
				},
			},
		},
	})
	if err != nil {
		log.Error().Msgf("Failed to load Azure App Configuration: %v", err)
		os.Exit(1)
	}

	var combinedConfig, refreshErr = loadConfigurations(appConfig)
	if refreshErr != nil {
		log.Error().Msgf("Failed to load configurations: %v", refreshErr)
		os.Exit(1)
	}

	appConfig.OnRefreshSuccess(func() {
		combinedConfig, refreshErr = loadConfigurations(appConfig)
	})

	http.HandleFunc("/configuration", func(w http.ResponseWriter, r *http.Request) {
		if err := appConfig.Refresh(ctx); err != nil {
			log.Error().Msgf("Failed to refresh Azure App Configuration: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if refreshErr != nil {
			log.Error().Msgf("Failed to load Azure App Configuration: %v", refreshErr)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		out, err := json.Marshal(combinedConfig)
		if err != nil {
			log.Error().Msgf("Error marshaling configuration: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write(out); err != nil {
			log.Error().Msgf("Error writing response: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		log.Debug().Msgf("Configuration served successfully: %s", r.URL.Path)
	})

	http.HandleFunc("/healthcheck", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	log.Info().Msgf("Starting server on port %s", ":4000")
	if err := http.ListenAndServe(":4000", nil); err != nil {
		log.Error().Msgf("Failed to start server: %s", err)
		os.Exit(1)
	}
}

// flattenConfigs recursively flattens nested maps using dot notation for keys.
func flattenConfigs(prefix string, in map[string]any, out map[string]any) {
	for k, v := range in {
		fullKey := k
		if prefix != "" {
			fullKey = prefix + "." + k
		}
		if subMap, ok := v.(map[string]any); ok {
			flattenConfigs(fullKey, subMap, out)
		} else {
			out[fullKey] = v
		}
	}
}

func loadConfigurations(appConfig *azureappconfiguration.AzureAppConfiguration) (*dynamic.Configuration, error) {
	var configurations = make(dynamic.Configurations)

	bytes, err := appConfig.GetBytes(nil)
	if err != nil {
		return nil, err
	}

	var configs map[string]any
	if err := json.Unmarshal(bytes, &configs); err != nil {
		return nil, err
	}

	log.Debug().Msgf("Raw configs from Azure: %#v", configs)

	// Helper to decode a value (string or map) into a dynamic.Configuration
	decodeConfig := func(key string, val any) *dynamic.Configuration {
		var newConfig dynamic.Configuration
		switch v := val.(type) {
		case string:
			if err := json.Unmarshal([]byte(v), &newConfig); err != nil {
				log.Error().Msgf("Failed to unmarshal configuration for key %s: %s", key, err)
				return nil
			}
		case map[string]any:
			decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
				ErrorUnused: true,
				Result:      &newConfig,
			})
			if err != nil {
				log.Error().Msgf("Failed to create decoder for key %s: %s", key, err)
				return nil
			}
			if err := decoder.Decode(v); err != nil {
				log.Error().Msgf("Failed to decode configuration for key %s: %s", key, err)
				return nil
			}
		default:
			log.Debug().Msgf("Skipping key %s: unsupported type %T", key, val)
			return nil
		}
		return &newConfig
	}

	// Helper to navigate nested maps using dot notation path
	navigateToSubPath := func(data map[string]any, path string) (map[string]any, bool) {
		if path == "" {
			return data, true
		}
		
		parts := strings.Split(path, ".")
		current := data
		
		for _, part := range parts {
			if next, ok := current[part].(map[string]any); ok {
				current = next
			} else {
				log.Debug().Msgf("Sub-path not found: %s (stopped at %s)", path, part)
				return nil, false
			}
		}
		
		return current, true
	}

	// Process configurations based on sub-path settings
	if cfg.IgnoreSubPath {
		// Process all top-level keys, ignoring sub-path
		for key, val := range configs {
			if config := decodeConfig(key, val); config != nil {
				configurations[key] = config
				log.Debug().Msgf("Loaded configuration for key: %s", key)
			}
		}
	} else if cfg.ConfigurationSubPath != "" {
		// Navigate to the specified sub-path
		if subConfigs, found := navigateToSubPath(configs, cfg.ConfigurationSubPath); found {
			for key, val := range subConfigs {
				if config := decodeConfig(key, val); config != nil {
					configurations[key] = config
					log.Debug().Msgf("Loaded configuration for sub-path key: %s", key)
				}
			}
		}
		
		// Also process top-level keys that are not part of the sub-path
		subPathParts := strings.Split(cfg.ConfigurationSubPath, ".")
		rootKey := subPathParts[0]
		for key, val := range configs {
			if key != rootKey {
				if config := decodeConfig(key, val); config != nil {
					configurations[key] = config
					log.Debug().Msgf("Loaded configuration for top-level key: %s", key)
				}
			}
		}
	} else {
		// Default behavior: process all top-level keys
		for key, val := range configs {
			if config := decodeConfig(key, val); config != nil {
				configurations[key] = config
				log.Debug().Msgf("Loaded configuration for key: %s", key)
			}
		}
	}

	log.Debug().Msgf("Loaded configurations: %v", configurations)
	return internal.MergeConfigurations(configurations), nil
}
