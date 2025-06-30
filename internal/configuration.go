package internal

import (
	"fmt"
	"github.com/rs/zerolog/log"
	"github.com/traefik/traefik/v3/pkg/config/dynamic"
)

func MergeConfigurations(configurations dynamic.Configurations) *dynamic.Configuration {
	var merged = &dynamic.Configuration{}

	for key, configuration := range configurations {
		merged.HTTP = mergeHttp(merged.HTTP, configuration.HTTP, key)

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

func mergeHttp(root *dynamic.HTTPConfiguration, upstream *dynamic.HTTPConfiguration, key string) *dynamic.HTTPConfiguration {
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
		httpRouter.Service = fmt.Sprintf("%s-%s", key, httpRouter.Service)

		var middlewareKeys []string
		for _, middleware := range httpRouter.Middlewares {
			middlewareKeys = append(middlewareKeys, fmt.Sprintf("%s-%s", key, middleware))
		}
		httpRouter.Middlewares = middlewareKeys
		root.Routers[fmt.Sprintf("%s-%s", key, rKey)] = httpRouter
	}

	for sKey, httpService := range upstream.Services {
		root.Services[fmt.Sprintf("%s-%s", key, sKey)] = httpService
	}

	for mKey, middleware := range upstream.Middlewares {
		root.Middlewares[fmt.Sprintf("%s-%s", key, mKey)] = middleware
	}

	for mKey, models := range upstream.Models {
		root.Models[fmt.Sprintf("%s-%s", key, mKey)] = models
	}

	for tKey, serversTransport := range upstream.ServersTransports {
		root.ServersTransports[fmt.Sprintf("%s-%s", key, tKey)] = serversTransport
	}

	return root
}
