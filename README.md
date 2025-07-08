# Traefic config server

This repository provides several implementations of a Traefik configuration server
[operating through HTTP](https://doc.traefik.io/traefik/providers/http/), which can be used to dynamically configure
Traefik instances.

## Example

See the [docker-compose example](./docker-compose.yaml) for a complete example of how to use the Azure App Configuration

## Implementations

Currently, the following implementations are available:

### Azure App Configuration

| Environment Variable             | Type          | Default       | Description                                                        |
|----------------------------------|---------------|---------------|--------------------------------------------------------------------|
| AZURE_APP_CONFIGURATION_ENDPOINT | string        | -             | The endpoint URL for Azure App Configuration                       |
| AZURE_CLIENT_ID                  | string        | -             | The Azure client ID used for authentication (if required)          |
| LOG_LEVEL                        | string        | INFO          | Log level (e.g., DEBUG, INFO, WARN, ERROR)                         |
| REFRESH_INTERVAL                 | time.Duration | 60s           | Interval for refreshing configuration from Azure App Configuration |
| LABEL_FILTER                     | string        | configuration | Label filter for selecting configuration entries                   |

The Azure Client ID needs to have the `App Configuration Data Reader` role assigned to it.

