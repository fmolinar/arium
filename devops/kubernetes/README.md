# Kubernetes (`devops/kubernetes/`)

Kustomize manifests for running Arium on Kubernetes, next to the Docker Compose stack in [`../docker`](../docker),
which stays the way to run it locally. Only the news collector is here so far.

## Layout

Planned entries are marked; everything else exists.

```
devops/kubernetes/
├── base/                        # cluster-agnostic manifests, namespace "arium"
│   ├── kustomization.yaml
│   ├── namespace.yaml           # Pod Security "restricted"
│   ├── collector/               # CronJob + ConfigMap + PVC
│   ├── backend/                 # planned: Deployment (probes on /health), Service, ConfigMap
│   ├── app/                     # planned: Deployment, Service
│   ├── mongo/                   # planned: StatefulSet for local dev only; prod uses Atlas
│   └── network-policies/        # planned: default deny, app → backend → mongo, prometheus → :9464
├── monitoring/                  # planned: kube-prometheus-stack values, ServiceMonitors,
│                                #   PrometheusRules, the existing Grafana dashboards as ConfigMaps
└── overlays/
    ├── local/                   # single-node cluster on the Docker host, local images
    └── aws/                     # planned, if the AWS side ever runs Kubernetes
```

## Collector

[`base/collector`](base/collector) replaces the Compose service's `-schedule` loop with a `CronJob` that runs
`collector -once` at 00:00, 08:00 and 16:00 UTC. Kubernetes owns the schedule, so the heartbeat file and
`-healthcheck` aren't used.

- `concurrencyPolicy: Forbid`: runs share the `collector-data` volume and its lock.
- A run that can't start within 10 minutes of its time is skipped, and a hung one is killed after 15.
- A failed run is retried twice. The CLI only fails when every source failed or the Mongo sync failed.
- The Mongo sync is on when a `collector-mongo` Secret with `MONGO_URI` exists, and off without it.
- No `/metrics`: a one-shot run exits before Prometheus can scrape it. Alert on
  `time() - kube_cronjob_status_last_successful_time{cronjob="collector"}` from kube-state-metrics instead
  (> 10h means a whole slot was missed).

### Run it

On Docker Desktop's Kubernetes, which uses the same image store as `docker compose build`:

```sh
cd devops/docker && docker compose build collector && cd ../kubernetes

# Optional: sync into Mongo. host.docker.internal reaches the Compose mongo from the cluster.
kubectl create namespace arium
kubectl -n arium create secret generic collector-mongo \
  --from-literal=MONGO_URI=mongodb://host.docker.internal:27017/arium

kubectl apply -k overlays/local
kubectl -n arium create job --from=cronjob/collector collector-manual-$(date +%s)   # run once now
kubectl -n arium logs -f job/<job name>
```

On k3d or kind, import the image first (`k3d image import arium-collector:latest`, `kind load docker-image ...`).

Don't run this and the Compose `collector` against the same Mongo at the same time. Both would work, since
`Import` is an idempotent upsert, but each would run on its own file store and its own schedule.
