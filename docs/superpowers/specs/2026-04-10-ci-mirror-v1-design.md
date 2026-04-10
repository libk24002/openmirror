# CI Mirror Service V1 Design

## 1. Background

The goal is to speed up CI builds by introducing a mirror layer that proxies and caches three dependency sources:

- Container images
- npm packages
- PyPI packages

V1 priorities are:

1. Reduce CI total duration
2. Keep build success high through automatic fallback
3. Improve cache hit ratio over time

## 2. Scope

### In Scope (V1)

- Single region, single Kubernetes cluster deployment
- Small scale load profile
- Public upstream sources only
- Local persistent cache with PVC
- No user authentication in V1
- Company-network-only access (not open internet)
- Automatic fallback to upstream from CI pipelines

### Out of Scope (V1)

- Multi-region deployment
- Unified self-developed protocol gateway
- Fine-grained tenant permissions or billing
- Public internet access

## 3. Upstream Targets

V1 supports these public sources:

- OCI/Docker: docker.io, registry-1.docker.io, registry.k8s.io
- npm: registry.npmjs.org
- PyPI: pypi.org and files.pythonhosted.org

## 4. Architecture

### 4.1 High-Level Pattern

Use a split-proxy architecture with one entry point and three protocol-specific backend proxies.

- Unified ingress endpoint (internal company domain)
- `image-mirror` backend for OCI image pulls
- `npm-mirror` backend for npm metadata and tarballs
- `pypi-mirror` backend for PyPI index and artifacts

This keeps V1 implementation low risk while preserving clear service boundaries.

### 4.2 Routing Model

One domain is exposed to CI clients, with path-based routing:

- `/docker/*` -> `image-mirror`
- `/npm/*` -> `npm-mirror`
- `/pypi/*` -> `pypi-mirror`

CI systems only need one base host, reducing rollout complexity.

### 4.3 Storage Model

- Each mirror service uses its own PVC
- Cache storage is isolated per protocol
- LRU and TTL policies are applied independently

Isolation prevents one protocol from consuming all cache capacity.

## 5. Request and Cache Behavior

### 5.1 Basic Flow

1. Client request enters ingress
2. Route to protocol mirror backend
3. Backend checks local PVC cache
4. On hit: return immediately
5. On miss: fetch from upstream, store in cache, return response

### 5.2 Freshness Policy

- Immutable artifacts (image blobs by digest, npm tarballs, wheel/sdist files): long retention
- Mutable metadata (tags, npm metadata documents, PyPI simple index): short TTL

Default mutable metadata TTL is 10 minutes (within accepted 5-15 minute window).

### 5.3 Upstream Protection

- Per-service upstream timeout and concurrency limits
- Single-flight request coalescing for identical misses
- Error code clarity for CI fallback logic

## 6. Availability and Failure Handling

### 6.1 Kubernetes Deployment

- Each backend service starts with 2 replicas
- Separate Deployments/Services/PVCs for image/npm/pypi
- Health probes for readiness and liveness

### 6.2 Failure Isolation

- Protocol paths are independent operational units
- npm upstream issues do not impact image or pypi mirror paths
- Ingress timeout/circuit controls reduce fault propagation

### 6.3 CI Fallback Strategy

When mirror path is unavailable or returns repeated timeout/5xx errors, CI automatically switches to direct upstream sources.

This preserves build continuity and matches the availability-first requirement.

## 7. Network and Security Baseline

V1 does not include application-layer authentication.

Access control is enforced by network boundaries:

- Service is reachable from company network only (office, VPN, enterprise egress)
- Not open to the public internet
- TLS for all client-to-mirror traffic
- Ingress source filtering by corporate egress CIDR allowlist
- Kubernetes NetworkPolicy limits east-west access inside cluster

## 8. Observability, SLO, and Acceptance

### 8.1 Priority Order

1. CI total duration improvement
2. Service availability and fallback effectiveness
3. Cache hit ratio

### 8.2 Metrics

- CI duration before/after (P50/P95)
- Mirror request volume and error rate
- Upstream latency P95 and failure rate
- Cache hit ratio by protocol
- CI fallback rate
- PVC usage ratio

### 8.3 V1 Acceptance Targets

- CI average duration reduced by >= 30% on selected pipelines within 2 weeks
- Mirror entry availability >= 99.5% in the same observation window
- Fallback path proven effective during failure drills
- Initial overall cache hit ratio >= 40%, with roadmap to >= 60%

## 9. Delivery Plan

### 9.1 Phase 0: Baseline (1-2 days)

- Collect current CI duration and failure baseline
- Select 3-5 representative pipelines for rollout validation

### 9.2 Phase 1: Minimum Viable Deployment (3-5 days)

- Deploy ingress + 3 mirror backends on Kubernetes
- Configure PVC cache and baseline monitoring/alerts
- Switch selected CI pipelines to mirror endpoint with fallback enabled

### 9.3 Phase 2: Gradual Rollout and Tuning (5-7 days)

- Expand from pilot pipelines to broader CI workloads
- Tune TTL, upstream concurrency, and per-protocol cache quota
- Track CI improvement and fallback stability

### 9.4 Phase 3: Stable Operations (ongoing)

- Weekly SLO review and cache tuning
- Capacity expansion based on disk and hit-ratio trends
- Reserve extension points for future authenticated package management

## 10. Risks and Mitigations

- Upstream instability: controlled by fallback and timeout policy
- Cache churn from small disk: controlled by protocol-isolated PVC and quota tuning
- Misrouting or malformed client settings: controlled by pilot rollout and standardized CI templates

## 11. Future Evolution (Post-V1)

- Add identity and authorization for package management scenarios
- Add team-level rate limits and audit trails
- Consider unified gateway only after V1 metrics prove stable value
