# osu-api-proxy

Proxy for the [osu!api v2](https://osu.ppy.sh/docs/index.html) written in go.
Allows applications to communicate with only an api key, while the server takes care of OAuth management.

## Usage

Head to the hosted site, click authenticate and follow the steps to receive an api key.
Include it in requests in the `api-key` header.

Note that this instance on [osuapi.shaddy.dev](https://osuapi.shaddy.dev/) is intended for use with my replay viewer.
Please host your own instance for your own use.

## Self-hosting

Requires:

- Posgresql Database
- Redis cache
- Reverse proxy as with the given Caddyfile configuration

Prometheus metrics are supported and will be documented at a later time.

The included `docker-compose.yml` file may or may not work and be up-to-date.
