# Opening a Superpod webpage

For the tested Dev Containers setup, run `make test-superpod` and follow the
[README's browser instructions](../README.md#open-the-demo-from-a-dev-container).
It installs Traefik into a dedicated Kind cluster and leaves a demo at
`http://superpod.localhost` after the checks pass. The test's loopback Ingress
address is configured specifically for its port mapping; use a reachable address
appropriate to your environment for other deployments.

The rest of this guide explains the general setup for other clusters.

The request path is:

```text
Browser -> Ingress controller -> ClusterIP Service -> nginx Pod
                                                       |
                                                 HTML ConfigMap
```

The operator creates the Ingress resource. The cluster needs an Ingress
controller to implement that route, and the browser needs a way to reach it.
The application's Service uses ClusterIP, not NodePort.

## 1. Check the cluster's Ingress support

```sh
kubectl config current-context
kubectl get ingressclasses
kubectl get services -A
```

If using a controller with an IngressClass, set `spec.ingressClassName` to that
class. Omit it when a default class is available or the controller explicitly
handles classless Ingresses.

For Kind, the [official Ingress guide](https://kind.sigs.k8s.io/docs/user/ingress/)
documents native Ingress support in `cloud-provider-kind` v0.9.0 and newer. That
provider must be running; starting Kind alone does not start it. Follow the
[provider installation instructions](https://kind.sigs.k8s.io/docs/user/loadbalancer/)
to enable it. This is shared cluster infrastructure, separate from the Superpod
operator. An existing third-party Ingress controller is also suitable.

## 2. Run the operator and create a Superpod

From the repository root, install the CRD and run the controller locally:

```sh
make install
make run
```

Keep that terminal running. In another terminal, edit the sample's hostname
and optional Ingress class, then apply it:

```sh
kubectl apply -f config/samples/super_v1_superpod.yaml
kubectl get superpods -w
```

The URL column reports `http://<spec.host>`, even while the resources are pending.
It is a configured address, not a promise that DNS already resolves it.

## 3. Read readiness and find the Ingress address

```sh
kubectl describe superpod superpod-sample
kubectl get ingress -o wide
kubectl wait superpod/superpod-sample --for=condition=Ready --timeout=120s
```

The `Ready` condition has these common reasons:

| Reason | Meaning |
| --- | --- |
| `PodNotReady` | nginx is not yet running and passing its HTTP readiness probe. |
| `IngressPending` | nginx is ready, but no Ingress IP or hostname has been published. |
| `ResourcesReady` | nginx is ready and the Ingress has an address. |
| `ResourceTerminating` | A child is being deleted; its name cannot yet be reused. |
| `ReconcileFailed` | A resource could not be created or updated; inspect the message. |
| `ObservationFailed` | The controller could not read the state needed to determine readiness. |

Some Ingress controllers need explicit configuration to publish an address.
Without that, this operator intentionally stays at `IngressPending`, even if
traffic can already flow. `observedGeneration` identifies which version of the
Superpod spec the condition describes. Repeated unchanged reports do not write
status or reset the condition's transition time.

## 4. Connect the hostname to the browser

Configure DNS or a hosts-file entry **on the computer running the browser** so
`spec.host` resolves to a reachable Ingress IP. For example, a hosts entry has the
form `<reachable-ingress-ip> superpod.example.test`. Replace the placeholder
with the actual address; the sample hostname does not resolve automatically.

To check the Ingress route independently of DNS, use the IP from Ingress status:

```sh
curl --resolve superpod.example.test:80:<ingress-ip> http://superpod.example.test/
```

Replace `<ingress-ip>` before running this command. A successful response should
contain the ability text, such as `Flying`. Then open the `status.url` address:

```sh
kubectl get superpod superpod-sample -o jsonpath='{.status.url}{"\n"}'
```

With Docker-in-Docker or a remote dev container, an IP reachable inside the
workspace may not be reachable from your browser. An additional host port mapping
or tunnel to the **Ingress controller** is needed in that case. Preserve the
requested hostname: Ingress matches the HTTP Host header. A random forwarded
hostname can otherwise return a 404. For this example the browser-facing HTTP
port is 80; a tunnel on port 8080 requires `:8080` in the browser URL and is not
represented in `status.url`.

`Ready=True` does not verify browser DNS or connectivity. It also does not prove
that a changed ConfigMap has finished propagating into the running Pod. HTTPS
and TLS certificate management are outside this learning step.

Reference: [Kubernetes Ingress](https://kubernetes.io/docs/concepts/services-networking/ingress/).
