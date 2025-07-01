package internal

import (
	"github.com/rs/zerolog/log"
	"github.com/traefik/traefik/v3/pkg/config/dynamic"
)

func MergeConfigurations(configurations dynamic.Configurations) *dynamic.Configuration {
	var merged = &dynamic.Configuration{}

	for key, configuration := range configurations {
		merged.HTTP = mergeHttp(merged.HTTP, configuration.HTTP)

		if configuration.TCP != nil {
			log.Warn().Msgf("TCP configuration for %s is not supported yet, skipping", key)
		}

		if configuration.UDP != nil {
			log.Warn().Msgf("UDP configuration for %s is not supported yet, skipping", key)
		}

		if configuration.TLS != nil {
			log.Warn().Msgf("TLS configuration for %s is not supported yet, skipping", key)
		}
	}

	return merged
}

func mergeHttp(root *dynamic.HTTPConfiguration, upstream *dynamic.HTTPConfiguration) *dynamic.HTTPConfiguration {
	if root == nil {
		root = &dynamic.HTTPConfiguration{
			Routers:           make(map[string]*dynamic.Router),
			Services:          make(map[string]*dynamic.Service),
			Middlewares:       make(map[string]*dynamic.Middleware),
			Models:            make(map[string]*dynamic.Model),
			ServersTransports: make(map[string]*dynamic.ServersTransport),
		}
	}

	for rKey, httpRouter := range upstream.Routers {
		root.Routers[rKey] = httpRouter
	}

	for sKey, httpService := range upstream.Services {
		root.Services[sKey] = httpService
	}

	for mKey, middleware := range upstream.Middlewares {
		root.Middlewares[mKey] = middleware
	}

	for mKey, models := range upstream.Models {
		root.Models[mKey] = models
	}

	for tKey, serversTransport := range upstream.ServersTransports {
		root.ServersTransports[tKey] = serversTransport
	}

	return root
}
