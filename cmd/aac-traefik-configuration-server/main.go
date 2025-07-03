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
	"time"
)

type config struct {
	LogLevel        string        `env:"LOG_LEVEL" envDefault:"INFO"`
	Endpoint        string        `env:"AZURE_APP_CONFIGURATION_ENDPOINT"`
	RefreshInterval time.Duration `env:"REFRESH_INTERVAL" envDefault:"60s"`
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
		log.Error().Msgf("Failed to load configurations: %v", err)
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
			log.Error().Msgf("Failed to load Azure App Configuration: %v", err)
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

	for key, c := range configs {
		var newConfig dynamic.Configuration

		// Test if c is a string, if so, try to unmarshal it as JSON
		if str, ok := c.(string); ok {
			if err := json.Unmarshal([]byte(str), &newConfig); err != nil {
				log.Error().Msgf("Failed to unmarshal configuration for key %s: %s", key, err)
				continue
			}
		} else {
			decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
				ErrorUnused: true,
				Result:      &newConfig,
			})
			if err != nil {
				return nil, err
			}

			err = decoder.Decode(c)
			if err != nil {
				log.Error().Msgf("Failed to decode configuration for key %s: %s", key, err)
				continue
			}
		}

		configurations[key] = &newConfig
	}

	log.Debug().Msgf("Loaded configurations: %v", configurations)

	return internal.MergeConfigurations(configurations), nil
}
