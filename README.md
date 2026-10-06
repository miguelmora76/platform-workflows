# platform-workflows

A sample **platform layer** for small HTTP services: one shared Helm chart, Terraform that deploys it
to a local minikube cluster with Prometheus and Grafana, and a reusable GitHub Actions workflow that
service repos can call.

> **Honest scope:** this is a learning and portfolio project that runs on one laptop. There is no cloud
> account, no remote Terraform state, no real secrets management and no production traffic. It shows the
> *pattern* (teams supply a values file, the platform supplies the chart and pipeline). It is not a tuned
> production setup. See [What changes at production scale](#what-changes-at-production-scale).

It deploys three sample services I built separately:
[`claims-rag-java`](https://github.com/miguelmora76/claims-rag-java),
[`claims-rag-python`](https://github.com/miguelmora76/claims-rag-python) and
[`mcp-devplatform-guardrails`](https://github.com/miguelmora76/mcp-devplatform-guardrails).
Each owns its own `Dockerfile`; this repo owns how they are deployed.

## Layout

```
charts/service/            One generic chart used by every service
  values.yaml              Defaults
  profiles/dev.yaml        1 replica, small limits
  profiles/prod-like.yaml  2 replicas, PodDisruptionBudget, alert on (still laptop-sized)
services/<name>.yaml       Per-service values: image, port, env, probes
terraform/modules/         monitoring (kube-prometheus-stack), service (one helm_release)
terraform/environments/minikube/   Composes them against the minikube context
.github/workflows/build-deploy.yml Reusable workflow (workflow_call) for service repos
.github/workflows/ci.yml           Lints this repo: helm lint x profiles, terraform fmt/validate
Makefile                   cluster, images, lint, apply, destroy, mcp-token, status
```

Values merge in this order, later wins: chart defaults, then the profile, then the service's own file.

## Run it locally

Needs docker, minikube, kubectl, helm and terraform. The three service repos must sit next to this one
(`../claims-rag-java` and so on) and each have a `Dockerfile`.

```bash
make cluster      # minikube, 4 CPUs / 5 GB
make images       # build each image from its repo and load it into minikube (no registry needed)
make apply        # terraform: namespaces, kube-prometheus-stack, the three services
make status
```

Try the services:

```bash
kubectl -n apps port-forward svc/claims-rag-python 8000:80
curl -s localhost:8000/api/ask -H 'content-type: application/json' \
  -d '{"question":"How many days do I have to file a first-level appeal?"}'
```

Grafana: `kubectl -n monitoring port-forward svc/monitoring-grafana 3000:80` (user `admin`, password is the
local-only default in `terraform/environments/minikube/variables.tf`).

Switch the shape with `make apply PROFILE=prod-like`. Remove everything with `make destroy`.

### The MCP server is port-forward only

`mcp-devplatform-guardrails` binds to `127.0.0.1` and rejects any Host header other than `localhost:8787`.
That is a deliberate security choice in that repo, and I did not change it. So it has no Service and uses
an exec probe, and you reach it from your machine only through a port-forward, with the same local port:

```bash
kubectl -n apps port-forward deploy/mcp-devplatform-guardrails 8787:8787
curl -s localhost:8787/mcp -H "authorization: Bearer $(make -s mcp-token)" \
  -H 'content-type: application/json' -H 'accept: application/json, text/event-stream' \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"k8s","version":"0"}}}'
```

It is also pinned to one replica, because its audit file has a single writer.

## The alert

Each service gets a `PrometheusRule` that fires when no replica of its Deployment is available for one
minute (`kube_deployment_status_replicas_available == 0`). It uses kube-state-metrics, so it works without
the services exposing any metrics of their own.

## Reusing the workflow from a service repo

```yaml
jobs:
  deploy:
    uses: miguelmora76/platform-workflows/.github/workflows/build-deploy.yml@main
    with:
      service: claims-rag-python
      values-file: deploy/values.yaml
```

It lints the chart with the service's values, builds the image, creates a throwaway kind cluster on the
runner, deploys with `helm upgrade --install --wait`, and prints pod state and logs if that fails.

## What was actually verified

On my machine (minikube 1.36, Terraform 1.16, Helm 4.3):

- `terraform apply` created 8 resources; all three services and the monitoring stack reached Running.
- The Java and Python services answered the sample question with the same cited answer inside the cluster.
- The MCP server completed an MCP `initialize` handshake through a port-forward.
- Scaling `claims-rag-python` to zero took its alert from `inactive` to `pending` to `firing` in about 90 s.
- `helm lint` passes for every service and profile; `terraform fmt` and `validate` pass.

Not verified yet: the reusable workflow and this repo's CI have not run on GitHub (they pass `actionlint`).

## Limits and troubleshooting

- The services run with `ASSISTANT_PROVIDER=fake`. To use real Claude, create a Secret holding
  `ANTHROPIC_API_KEY` and set `existingSecret` in the service's values. The chart never creates secrets.
- Terraform state is a local file and contains the generated MCP token and the Grafana password. It is
  git-ignored. Do not reuse these passwords anywhere.
- Images are loaded into minikube with `minikube image load`, so `imagePullPolicy` is `IfNotPresent` and
  the `dev` tag is mutable. Rebuild and reload to pick up changes, then restart the Deployment.
- VPN and TLS-inspection clients (for example Cisco Secure Client) can make Docker Desktop's pulls fail with
  `EOF` while the browser still works. Disconnecting fixed it for me.

## What changes at production scale

| Here | In production |
| --- | --- |
| minikube created by hand | A managed cluster, created and versioned by Terraform |
| Local `terraform.tfstate` | Remote state with locking, per environment |
| One environment, two value profiles | Separate dev, staging and prod environments with promotion between them |
| Images loaded straight into the node | A container registry, immutable tags or digests, image scanning and signing |
| Secret in Terraform state, placeholder passwords | An external secrets manager and no secrets in state |
| Reusable workflow deploys to a throwaway kind cluster | Deploys to real clusters using short-lived OIDC credentials, with approvals |
| One alert, no Alertmanager | Routed alerts, SLOs, dashboards per service, log aggregation |
| No ingress or TLS | Ingress or gateway, TLS, network policies |
