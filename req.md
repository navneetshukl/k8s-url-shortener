
## Capstone: a production-ready URL shortener

Build a Go API (Gin) with PostgreSQL and Redis, then harden it in phases. The app stays simple so the work is all Kubernetes.

**Phase 1: Config and health**
- Move config to ConfigMaps and Secrets, and mount them as env vars and as files.
- Add liveness, readiness and startup probes (`/healthz`, `/readyz`). Make `/readyz` check the DB and Redis connections.
- Handle SIGTERM in Go for graceful shutdown, then verify that a rolling update causes zero dropped requests.

**Phase 2: Stateful and batch workloads**
- Run PostgreSQL as a StatefulSet with a headless Service and volumeClaimTemplates.
- Add a CronJob for expired-link cleanup and a Job for DB migrations (run as a pre-deploy step).

**Phase 3: Traffic**
- Expose the app through an Ingress (NGINX Ingress Controller or Gateway API) with TLS from cert-manager.
- Add NetworkPolicies so that only the API can reach Postgres and Redis, with default-deny for everything else.

**Phase 4: Scaling and resilience**
- Set resource requests and limits, then watch what happens under CPU throttling and OOMKills.
- Add an HPA on CPU, then on a custom metric. Try KEDA if you want event-driven scaling.
- Add a PodDisruptionBudget, pod anti-affinity or topology spread constraints, and PriorityClasses.
- Load-test with `k6` or `hey` and watch the autoscaling happen.

**Phase 5: Security**
- Create a dedicated ServiceAccount per app with minimal RBAC.
- Set `securityContext` (non-root, read-only root filesystem, dropped capabilities) and enforce Pod Security Admission.
- Build a distroless Go image and scan it with Trivy.

**Phase 6: Observability**
- Expose Prometheus metrics from Go (`promhttp`) and install kube-prometheus-stack.
- Add a ServiceMonitor, a Grafana dashboard and an alert rule (for example error rate or p99 latency).
- Add structured logging with Loki and tracing with OpenTelemetry and Jaeger.

**Phase 7: Packaging and delivery**
- Package the app as a Helm chart (values per environment), or use Kustomize overlays for dev/staging/prod.
- Set up GitOps with ArgoCD or Flux so a git push deploys the change.
- Try canary or blue-green releases with Argo Rollouts.

## Go-specific projects

**1. A custom controller with `client-go`**
Write a controller that watches Deployments and does something useful, such as auto-annotating them, or restarting pods when a ConfigMap changes. This teaches informers, work queues, the reconcile loop and leader election, which is how Kubernetes works internally.

**2. A Kubernetes Operator with Kubebuilder (CRD)**
Define a custom resource like `URLShortener` or `Database`, and have the operator create and manage the Deployment, Service and PVC for you. You'll learn CRDs, controller-runtime, status conditions, finalizers and owner references. This is the most valuable project for a Go backend developer.

**3. An admission webhook (optional)**
A validating or mutating webhook that rejects pods without resource limits, or injects a sidecar automatically. It teaches API server extension points and webhook TLS.

## Suggested order

1. Capstone phases 1 to 4. Do these first, since they matter most for production.
2. Phases 5 and 6 (security and observability).
3. Phase 7 (Helm, GitOps).
4. Controller, then Operator.

