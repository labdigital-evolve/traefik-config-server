package main

import (
	"context"
	"encoding/json"
	"github.com/Azure/AppConfiguration-GoProvider/azureappconfiguration"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/caarlos0/env/v11"
	"github.com/labdigital-evolve/traefik-config-server/internal"
	"github.com/rs/zerolog/log"
	"github.com/traefik/traefik/v3/pkg/config/dynamic"
	"net/http"
	"os"
)

type config struct {
	Endpoint string `env:"AZURE_APP_CONFIGURATION_ENDPOINT"`
	Port     string `env:"Port" envDefault:":9000"`
}

var cfg config

func init() {
	err := env.Parse(&cfg)
	if err != nil {
		log.Error().Msgf("Failed to parse environment variables: %v", err)
		os.Exit(1)
	}
}

func main() {
	var ctx = context.Background()
	var configurations dynamic.Configurations

	var credentials, err = azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		log.Error().Msgf("Failed to create Azure credentials: %v", err)
		os.Exit(1)
	}
	authOptions := azureappconfiguration.AuthenticationOptions{
		Endpoint:   cfg.Endpoint,
		Credential: credentials,
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Load configuration from Azure App Configuration
		appConfig, err := azureappconfiguration.Load(ctx, authOptions, &azureappconfiguration.Options{})
		if err != nil {
			log.Error().Msgf("Failed to load Azure App Configuration: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if err = appConfig.Unmarshal(&configurations, nil); err != nil {
			log.Error().Msgf("Failed to unmarshal configurations: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		var combinedConfig = internal.MergeConfigurations(configurations)

		out, err := json.Marshal(combinedConfig)
		if err != nil {
			log.Error().Msgf("Error marshaling configuration: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write(out); err != nil {
			log.Error().Msgf("Error writing response: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	})

	log.Info().Msgf("Starting server on port %s", cfg.Port)
	if err := http.ListenAndServe(cfg.Port, nil); err != nil {
		log.Error().Msgf("Failed to start server: %s", err)
		os.Exit(1)
	}
}
