# Orbit

A small proxy to turn a Github mono-repo into a Terraform module registry.

## Usage 

This proxy looks for release tags formatted like `<module>/<version>`

For instance if your mono-repo has a `kitchensink` module with a `v0.1.0` 
release, it should have a tag like `kitchensink/v0.1.0`, and we'd expect
the module code in `/kitchensink` on this tag.

## Configuration

You can configure Orbit through the follow ENV variables:

### `CACHE_ENABLED`

Cache GitHub results. Defaults to `false`.

### `CACHE_PATH`

Where to store cache. Defaults to `/tmp`.

### 'CACHE_EXPIRATION'

How long we cache assets. Defaults to `10s`.

### `GITHUB_RESPOSITORES`

Comma separated list of GitHub repositories to proxy. Defaults to allow all 
repositories.

### GITHUB_TOKEN

GitHub authentication token. If unset, you should pass through the token in 
the request.

### Server settings. See `http/server` for more info about these

## HOST

Listen host. Defaults to listen to all interfaces

## PORT

Listen Port. Defaults to `8080`

## TIMEOUT_HANDLER


Timeout for request handling. Defaults to 10s.

###	TIMEOUT_IDLE

Idle timeout. Defaults to value of `TIMEOUT_READ`

### TIMEOUT_READ

Timeout to read request. No default.

###	TIMEOUT_READ_HEADER

Minimum time to receive headers. Defaults to `2s`.

### TIMEOUT_SHUTDOWN

Defaults to `5s`

###	TIMEOUT_WRITE

Write timeout. Defaults to value of `TIMEOUT_READ`

### TLS_ENABLED

Enable TLS. Defaults to false

### TLS_CERT_FILE

Path to certificate, no default.

### TLS_KEY_FILE

Path to certificate key, no default.
