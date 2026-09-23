# Superpod resource builders

Each `New...` function returns a fresh Kubernetes object in memory. It does not
create anything in the cluster. Call these functions with a Superpod fetched from
the API server, which has a namespace and UID.

Read the files in this order:

1. `metadata.go`: stable names and labels shared by the resources.
2. `configmap.go`: generates `index.html`, escaping the ability and optional emoji as plain text.
   When `spec.emoji` is enabled, the controller selects emoji through Jev and
   persists the selection alongside the HTML. See [emoji setup](../../docs/emoji.md).
3. `serviceaccount.go`: the Pod's identity, with automatic API token mounting disabled.
4. `pod.go`: nginx serves the ConfigMap mounted at `/usr/share/nginx/html`.
5. `service.go`: a ClusterIP Service selects that Superpod's Pod by its UID label.
6. `ingress.go`: the configured hostname routes to the Service's HTTP port.

All children use `superpod-<UID>` as their name. Different kinds can share a name;
using the server-assigned UID avoids long-name problems and collisions. Names and
selectors stay the same when `superAbility` changes.

The HTML directory is mounted without `subPath`, allowing Kubernetes to project
ConfigMap changes into the running Pod. Propagation is eventual, not immediate.
The Pod uses the official `nginx:stable-alpine` image with `Always` pull policy;
this tag can receive updates when a new Pod starts.

`reconcile.go` contains `Ensure`, which performs the API operations:

1. Fetch the child using its stable name.
2. If missing, create it with a Superpod controller owner reference.
3. If present, refuse to change it unless that Superpod owns it.
4. Wait if deletion is still in progress.
5. Compare and update managed fields only when something changed.

The controller calls `Ensure` for the ServiceAccount, ConfigMap, Pod, Service,
and Ingress, in that order. It watches all five kinds. Deleting a child triggers
reconciliation and recreation; deleting the Superpod lets Kubernetes garbage
collection remove its children. No custom finalizer is needed for these resources.

Updates preserve Kubernetes-assigned fields such as the Service's ClusterIP.
When `ingressClassName` is omitted, an existing Ingress retains its assigned class;
set an explicit class to change it. Most Pod fields are immutable, so existing
Pods only have managed labels and container images reconciled. HTML lives in the
ConfigMap and can change without replacing the Pod.

The controller reports `status.podName`, `status.url` (HTTP), and a `Ready`
condition with a reason and the observed spec generation. `Ready=True` requires
a running, ready nginx Pod and an IP or hostname published in Ingress status.
Controllers that do not publish addresses will leave it at `IngressPending`.
Readiness does not verify browser DNS, external connectivity, or whether a
ConfigMap update has reached the Pod yet. See [the access guide](../../docs/webpage-access.md).

ConfigMap API
access restrictions are skipped; the generated controller RBAC only supplies
the operator's own required permissions.

Run the helper tests without a cluster using `go test ./internal/resources`.
Run `make test` for reconciliation tests against an isolated envtest API server.
Envtest has no kubelet or garbage collector: it validates API behavior, ownership,
and watches, but cannot prove that nginx serves traffic or that GC removes children.

References: [official nginx image](https://hub.docker.com/_/nginx) and
[ConfigMap volume updates](https://kubernetes.io/docs/concepts/configuration/configmap/#mounted-configmaps-are-updated-automatically).
