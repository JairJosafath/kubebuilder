# Superpod learning operator

A Superpod creates an nginx Pod, an HTML ConfigMap, a ServiceAccount, a ClusterIP
Service, and an Ingress. The webpage displays `spec.superAbility`.

Optionally set `spec.emoji: true` to add up to three matching emoji using Jev.
See [Jev emoji setup](docs/emoji.md) for API key configuration and selection rules.

The controller repairs missing resources and reports a URL and readiness.
Changing the ability updates the HTML without replacing the Pod. Deleting the
Pod recreates it; deleting the Superpod lets Kubernetes clean up all its children.

Start with [the resource helpers](internal/resources/README.md) and then read
[the controller](internal/controller/superpod_controller.go). Resource definitions
and API create/update operations live in `internal/resources/`.

## Run the tests

Run these commands from the `operator` directory.

### Helpers and reconciliation tests

Prerequisites: Go 1.26 or newer and Make. The Makefile downloads its code-generation,
lint, and envtest tools as needed.

```sh
# All helper and controller tests, using an isolated API server and etcd.
# Does not deploy anything into your current Kubernetes cluster.
make test

# Just the resource builders, without starting an API server.
go test ./internal/resources

# Check Go code style.
make lint
```

The controller tests cover repeated reconciliation, ownership collisions, missing
resources, status transitions, and watches. Envtest has no kubelet, Ingress
controller, or garbage collector; use the Kind test below to check real traffic
and cleanup.

### Real Kind cluster test and browser demo

Prerequisites: Docker, Kind, kubectl, Helm, curl, ripgrep (`rg`), Go, and Make.
Docker must be running. Port 80 inside the dev container must be free on the first
run. The setup downloads the Traefik Helm chart and container images.

```sh
make test-superpod
```

This command creates or reuses the dedicated `superpod-e2e` cluster using
`bin/superpod-e2e/config.kubeconfig`. It does not change your normal kubeconfig
or use your existing `kind` cluster. It deploys the operator with its generated
RBAC and installs Traefik as the test Ingress controller.

The test checks:

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

To add the test cluster to your normal kubeconfig and make it the active context:

```sh
kind export kubeconfig --name superpod-e2e
kubectl config use-context kind-superpod-e2e
kubectl get superpods,pods,ingresses
```

This is optional; the test script keeps its kubeconfig separate by default.
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

To remove the dedicated test/demo cluster:

```sh
make clean-test-superpod
```

## Use another cluster

See [Opening a Superpod webpage](docs/webpage-access.md) for the sample manifest,
Ingress class selection, DNS, and readiness troubleshooting.

For local development against that cluster:

```sh
make install
make run
# In another terminal:
kubectl apply -f config/samples/super_v1_superpod.yaml
```

For an in-cluster controller deployment, build and push your operator image, then
deploy it using the existing Makefile targets:

```sh
make docker-build docker-push IMG=<your-registry>/operator:<tag>
make install
make deploy IMG=<your-registry>/operator:<tag>
```

`Ready=True` means nginx is ready and Ingress status has an address. Browser DNS
and connectivity still need to work. This learning project currently uses HTTP;
ConfigMap user-access restrictions and TLS are outside its implemented scope.

## License

Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
