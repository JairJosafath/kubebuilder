# Kind cluster test and browser demo

`make test-superpod` deploys the operator into a disposable Kind cluster, checks
the whole Superpod lifecycle with real traffic, and leaves a demo webpage
running. Run all commands from the repository root.

| File | Purpose |
| --- | --- |
| [`kind.yaml`](kind.yaml) | Cluster definition, including the port 80 mapping Traefik uses |
| [`traefik-values.yaml`](traefik-values.yaml) | Test-only Traefik Ingress controller settings |
| [`Dockerfile`](Dockerfile) | Wraps the locally built manager binary in a distroless image |
| [`deploy/`](deploy/) | Kustomize overlay that deploys `config/default` with that test image |
| [`superpod.yaml`](superpod.yaml) | The `superpod-demo` Superpod the test creates |

The scripts that drive the test live in `hack/` (`test-superpod.sh`,
`setup-superpod-cluster.sh`, `setup-superpod-deps.sh`).

## Prerequisites

Start with Bash and Make. `make test-superpod` checks for Go, Kind, kubectl, Helm,
curl, ripgrep (`rg`), and Docker before running the test. Existing commands are
reused; missing Go, Kind, kubectl, Helm, and ripgrep binaries are downloaded with
checksum verification into `bin/` on Linux/macOS (amd64/arm64). Missing base
utilities use apt, dnf, or Homebrew; missing Docker uses apt on Debian/Ubuntu or
Docker Desktop via Homebrew on macOS. System package installation may require
sudo. On other systems, install Docker first.

Docker must be running and accessible to your user. The setup checks this before
creating a cluster. Port 80 inside the dev container must be free on the first
run. The test also downloads the Traefik Helm chart and container images.

## Run the test

```sh
make setup-test-superpod    # Dependencies, cluster, and working kubectl context
kubectl get nodes
make test-superpod          # Deploy and test the operator; leave the demo running

make stop-test-superpod     # Stop nodes; retain workloads for later
make setup-test-superpod    # Resume, or recreate if the cluster is broken
make destroy-test-superpod  # Delete the cluster and its local state
```

To prepare dependencies without running the cluster test, use
`make test-superpod-deps`. Running `bash hack/test-superpod.sh` also goes through
the Makefile dependency setup. Download versions can be overridden with
`GO_VERSION`, `KIND_VERSION`, `KUBECTL_VERSION`, `HELM_VERSION`, and `RG_VERSION`;
these apply only when the corresponding command is missing. `LOCALBIN` changes
the download and test state directory. `KIND`, `KUBECTL`, `HELM`, and `RG` can
select existing executable paths.

Make adds `bin/` to its command search path. If a command such as `kubectl` was
downloaded there, run `export PATH="$PWD/bin:$PATH"` from the repository root
to use it directly in your terminal too.

## What the test checks

1. The controller creates all five children and reports the expected URL.
2. HTTP through Ingress returns the ability text.
3. Repeated reconciliation keeps the same five child identities.
4. An ability change reaches the webpage without replacing the Pod.
5. Deleting the Pod creates a replacement that serves the updated page.
6. Deleting the Superpod garbage-collects all five children.

ConfigMap projection is eventual, so the update check can take a minute or two.
On failure, the script prints diagnostics and retains the cluster for inspection.
On success, it leaves a fresh `superpod-demo` with the ability `Flying` running.

The existing `make test-e2e` target is the separate Kubebuilder manager/metrics
test suite. Use `make test-superpod` for the application lifecycle checks above.

## How the test cluster is managed

The `superpod-e2e` cluster is disposable. Setup checks its node containers,
configuration fingerprint, API access, and node readiness. Healthy clusters with
the recorded configuration are reused; stopped nodes are restarted. An existing
cluster with unknown/changed configuration, replaced nodes, or failed health
checks is deleted and recreated once from [`kind.yaml`](kind.yaml), including
the port mapping used by Traefik. Recreating it deletes its workloads and Secrets.
Other clusters are outside this workflow.

Setup writes fresh credentials and the current API endpoint to
`bin/superpod-e2e/config.kubeconfig`, then updates your normal kubeconfig and
selects `kind-superpod-e2e`, so `kubectl get nodes` works immediately. If you set
`KUBECONFIG`, that configuration is updated instead. Other kubeconfig entries
are preserved. This also repairs stale API ports after cluster recreation.
The tests always use the dedicated file.

`make test-superpod` includes setup, then deploys the operator with its generated
RBAC and installs Traefik. `make stop-test-superpod` retains the cluster and its
state for resuming; `make destroy-test-superpod` removes the test cluster, its
kubeconfig entries, and `bin/superpod-e2e/`, while keeping downloaded tools.
`make clean-test-superpod` is an alias for destroy. Use `make test-superpod-setup`
to test the lifecycle decisions without a real cluster.

## Open the demo from a dev container

After `make test-superpod` succeeds:

1. In VS Code's **Ports** panel, forward container port **80**.
2. Set its **local port to 80** if VS Code chose another port.
3. Open **http://superpod.localhost**.

The route is browser → forwarded port 80 → Kind port mapping → Traefik Ingress →
ClusterIP Service → nginx. Neither the application nor Traefik uses a NodePort
Service in this setup.

If `superpod.localhost` does not resolve on your computer, add
`127.0.0.1 superpod.localhost` to that computer's hosts file. If local port 80 is
already occupied, forward to 8080 and open `http://superpod.localhost:8080`;
`status.url` still shows the standard HTTP address without that local tunnel port.

You can also check from the dev-container terminal:

```sh
curl --noproxy '*' --resolve superpod.localhost:80:127.0.0.1 http://superpod.localhost/
```

## Inspect or change the running demo

Setup already selects the test cluster in your kubeconfig. Inspect it with:

```sh
kubectl get superpods,pods,ingresses
```

If the API endpoint becomes stale, rerun `make setup-test-superpod` to repair it.
Use `kubectl config get-contexts` to list contexts and `kubectl config use-context <name>`
to switch back to another cluster.

Use the separate kubeconfig explicitly:

```sh
kubectl --kubeconfig bin/superpod-e2e/config.kubeconfig get superpods,pods,configmaps,services,ingresses
kubectl --kubeconfig bin/superpod-e2e/config.kubeconfig describe superpod superpod-demo

kubectl --kubeconfig bin/superpod-e2e/config.kubeconfig patch superpod superpod-demo \
  --type=merge -p '{"spec":{"superAbility":"Invisibility"}}'
```

Refresh the webpage after Kubernetes projects the updated ConfigMap. To rerun all
checks, run `make test-superpod` again; it resets only its demo in the test cluster.

To try emoji matching in this cluster, run `make test-emoji`. It makes paid Jev API
calls; see [emoji setup](../../docs/emoji.md) for the API key.

To stop the demo while retaining its workloads, or destroy it completely:

```sh
make stop-test-superpod
# Or:
make destroy-test-superpod
```
