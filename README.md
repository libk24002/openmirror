# OpenMirror

OpenMirror is a small Go service that proxies and caches common CI dependency traffic behind a single internal endpoint. It provides path-based mirrors for container images, npm packages, and PyPI artifacts to reduce repeated upstream downloads.

## Features

- Path-based mirror endpoints for Docker, npm, and PyPI
- Filesystem-backed cache with configurable cache directory and metadata TTL
- Internal observability endpoints (`/healthz`, `/metrics`)
- Kubernetes deployment manifests with persistent cache volume and ingress routing

## Quick Start

### Run locally

```bash
go run ./cmd/openmirror
```

Service starts on `:8080` by default and uses `/var/cache/openmirror` for cache storage.

### Build and run with Docker

```bash
docker build -t openmirror:latest .
docker run --rm -p 8080:8080 -v $(pwd)/.cache:/var/cache/openmirror openmirror:latest
```

### Deploy to Kubernetes

Build and push the image before applying manifests. The base deployment references `ghcr.io/libk24002/openmirror:latest`.

```bash
docker build -t ghcr.io/libk24002/openmirror:latest .
docker push ghcr.io/libk24002/openmirror:latest
```

If you want to deploy a different registry or tag, update the `image` field in `deploy/k8s/base/deployment.yaml` before applying.

```bash
kubectl apply -f deploy/k8s/base
```

## Endpoints

- `GET /healthz` - health check
- `GET /metrics` - Prometheus metrics
- `/docker/*` - Docker/OCI mirror path
- `/npm/*` - npm mirror path
- `/pypi/*` - PyPI mirror path

For Kubernetes ingress, the default host in this repo is `mirror.internal`, with `/docker`, `/npm`, and `/pypi` routed to the OpenMirror service.
