# Selective Router

Native dynamic-library plugin for [CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI).

This plugin is independent from Moon Bridge. It implements the CLIProxyAPI C ABI and is loaded as a `.so`/`.dylib`/`.dll` from CLIProxyAPI's plugin discovery directory.

## Capabilities

The plugin declares:

- `model_router`
- `request_normalizer`

It performs route conversion for matching requests. It can also inject a native web search tool into matching Responses-shaped requests before routing; it does not execute model calls itself.

## What It Does

- Routes image-capable response requests when `route_vision` is enabled.
- Injects an `image_generation` tool using `image_tool_model` for explicit image generation requests when `route_image_generation` is enabled.
- Optionally routes image generation requests to `route_provider`/`route_model` when `image_route_override` is enabled.
- Routes web-search-capable response requests when `route_web_search` is enabled.
- Injects a native web search tool for explicit search intent when `route_web_search` is enabled.
- Routes compact response requests when `route_compact` is enabled.
- Leaves ordinary response requests on the original route.

## Build

```bash
cd cpa-selective-router
GOPATH=/tmp/go HOME=/tmp GOMODCACHE=/tmp/gomodcache GOCACHE=/tmp/gocache make build
```

Output:

```text
bin/selective-router.so
```

## Test

```bash
GOPATH=/tmp/go HOME=/tmp GOMODCACHE=/tmp/gomodcache GOCACHE=/tmp/gocache go test ./...
```

## CLIProxyAPI Config

Install the built shared library into CLIProxyAPI's plugin discovery directory, then configure `plugins.configs.selective-router`.

For Linux ARM64 with CLIProxyAPI running from `$CLIPROXYAPI_HOME`:

```bash
mkdir -p "$CLIPROXYAPI_HOME/plugins/linux/arm64"
cp bin/selective-router.so "$CLIPROXYAPI_HOME/plugins/linux/arm64/selective-router.so"
chmod 755 "$CLIPROXYAPI_HOME/plugins/linux/arm64/selective-router.so"
```

CLIProxyAPI scans `plugins/<goos>/<goarch>/` before the root plugin directory.

```yaml
plugins:
  enabled: true
  dir: "plugins"
  configs:
    selective-router:
      enabled: true
      priority: 50
      route_provider: "<provider>"
      route_model: "<target-model>"
      image_tool_model: "gpt-image-2"
      image_route_override: false
      models: []          # requested model allowlist; empty = all models
      excluded_models: [] # requested model denylist; supports '*' wildcards, e.g. model-*
      route_compact: true
      route_web_search: true
      route_vision: true
      route_image_generation: true
```

## Config Fields

| Field | Description |
| --- | --- |
| `enabled` | Disable routing when false. |
| `route_provider` | Provider used for direct `model_router` route conversion. |
| `route_model` | Target model used for direct `model_router` route conversion. |
| `image_tool_model` | Model used by the injected `image_generation` tool. Default: `gpt-image-2`. |
| `image_route_override` | Route image generation requests to `route_provider`/`route_model` instead of preserving the host's normal model routing. Default: `false`. |
| `models` | Requested model allowlist. Empty means all models. Supports `*` wildcards, e.g. `model-*`. |
| `excluded_models` | Requested model denylist. Takes precedence over `models`. Supports `*` wildcards, e.g. `model-*`. |
| `route_compact` | Route matching compact response requests directly to `route_provider`/`route_model`. Default: `true`. |
| `route_web_search` | Route matching web search requests directly to `route_provider`/`route_model`; also injects a native web search tool for matching Responses requests with explicit search intent. |
| `route_vision` | Route matching response requests containing image input. |
| `route_image_generation` | Inject an `image_generation` tool for explicit image generation requests. This does not register a Codex built-in `image_gen` tool; it only rewrites Responses-shaped requests passing through CLIProxyAPI. |

## Files

- `cmd/selective-model-router/`: CLIProxyAPI dynamic plugin entrypoint.
- `core/`, `plugin/`, `openai/`: reusable protocol-neutral library code kept for unit testing and future non-CLIProxyAPI embedding.

## Notes

- The actual CLIProxyAPI plugin ABI lives in `cmd/selective-model-router`.
- Go plugins are built with `-buildmode=c-shared`, not Go's `plugin` package.
- The shared library is trusted in-process code, consistent with CLIProxyAPI's plugin model.
