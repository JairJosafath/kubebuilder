#!/usr/bin/env bash
# Run the learning project's real-cluster checks, then leave a demo to explore.
set -euo pipefail
cd "$(dirname "$0")/.."

cluster=superpod-e2e
workdir="$PWD/bin/superpod-e2e"
export KUBECONFIG="$workdir/config.kubeconfig"
export KUBECTL_KUBERC=false
url=http://superpod.localhost
mkdir -p "$workdir/image"

for tool in kind docker kubectl helm curl go rg make; do
  command -v "$tool" >/dev/null || { echo "Missing required tool: $tool" >&2; exit 1; }
done

# Only reuse the dedicated cluster with its own kubeconfig. Never use the user's
# current context or silently adopt a different cluster with the same name.
if kind get clusters | rg -qx "$cluster"; then
  test -f "$KUBECONFIG" || { echo "Existing test cluster has no saved kubeconfig" >&2; exit 1; }
else
  kind create cluster --name "$cluster" --config test/superpod/kind.yaml \
    --kubeconfig "$KUBECONFIG" --wait 60s
fi
test "$(kubectl config current-context)" = "kind-$cluster"

diagnostics() {
  echo "Test failed. Cluster retained for inspection; kubeconfig: $KUBECONFIG" >&2
  kubectl get pods,superpods,ingresses -A || true
  kubectl logs -n operator-system deployment/operator-controller-manager --tail=60 || true
  kubectl get events -A --sort-by=.lastTimestamp | tail -30 || true
}
trap diagnostics ERR

echo "Installing the test Ingress controller"
helm upgrade --install traefik traefik --repo https://traefik.github.io/charts \
  --version 41.6.0 --namespace traefik --create-namespace --skip-crds \
  -f test/superpod/traefik-values.yaml --wait --timeout 180s

echo "Building and deploying the operator with its generated RBAC"
make manifests generate kustomize
CGO_ENABLED=0 GOOS=linux go build -o "$workdir/image/manager" ./cmd
docker build -f test/superpod/Dockerfile -t superpod-controller:test "$workdir/image"
kind load docker-image superpod-controller:test --name "$cluster"
bin/kustomize build test/superpod/deploy | kubectl apply -f -
kubectl rollout restart deployment/operator-controller-manager -n operator-system
kubectl rollout status deployment/operator-controller-manager -n operator-system --timeout=180s

# Retry boundedly: ConfigMap volume projection and Ingress routing are eventual.
wait_for() {
  local description=$1
  shift
  local deadline=$((SECONDS + 180))
  until "$@"; do
    if (( SECONDS >= deadline )); then
      echo "Timed out: $description" >&2
      return 1
    fi
    sleep 2
  done
}
page_contains() {
  local page
  page=$(curl --noproxy '*' --silent --show-error --fail --max-time 5 \
    --resolve superpod.localhost:80:127.0.0.1 "$url/") || return 1
  [[ "$page" == *"My super ability is: $1"* ]]
}
children_gone() {
  local remaining
  remaining=$(kubectl get pods,configmaps,serviceaccounts,services,ingresses \
    -n default -l "$selector" -o name) || return 1
  test -z "$remaining"
}
pod_recreated() {
  local current_uid
  current_uid=$(kubectl get pod "$pod_name" -n default --ignore-not-found \
    -o jsonpath='{.metadata.uid}') || return 1
  test -n "$current_uid" && test "$current_uid" != "$old_pod_uid"
}

# Reset just this script's demo on repeat runs.
if kubectl get superpod superpod-demo -n default >/dev/null 2>&1; then
  old_uid=$(kubectl get superpod superpod-demo -n default -o jsonpath='{.metadata.uid}')
  selector="super.elp-max.com/superpod-uid=$old_uid"
  kubectl delete superpod superpod-demo -n default --timeout=60s
  wait_for "previous demo cleanup" children_gone
fi

echo "Checking creation and HTTP through Ingress"
kubectl apply -f test/superpod/superpod.yaml
kubectl wait superpod/superpod-demo -n default --for=condition=Ready --timeout=180s
wait_for "initial webpage" page_contains Flying
uid=$(kubectl get superpod superpod-demo -n default -o jsonpath='{.metadata.uid}')
selector="super.elp-max.com/superpod-uid=$uid"
pod_name=$(kubectl get superpod superpod-demo -n default -o jsonpath='{.status.podName}')
old_pod_uid=$(kubectl get pod "$pod_name" -n default -o jsonpath='{.metadata.uid}')
test "$(kubectl get superpod superpod-demo -n default -o jsonpath='{.status.url}')" = "$url"
test "$(kubectl get service "$pod_name" -n default -o jsonpath='{.spec.type}')" = ClusterIP

echo "Checking repeated reconciliation keeps the same five children"
before=$(kubectl get pods,configmaps,serviceaccounts,services,ingresses -n default \
  -l "$selector" -o jsonpath='{range .items[*]}{.metadata.uid}{"\n"}{end}')
test "$(printf '%s\n' "$before" | wc -l)" -eq 5
for iteration in 1 2 3; do
  kubectl annotate superpod superpod-demo -n default test.example/reconcile="$iteration" --overwrite
done
sleep 2
after=$(kubectl get pods,configmaps,serviceaccounts,services,ingresses -n default \
  -l "$selector" -o jsonpath='{range .items[*]}{.metadata.uid}{"\n"}{end}')
test "$before" = "$after"

echo "Checking ability updates reach nginx without replacing its Pod"
kubectl patch superpod superpod-demo -n default --type=merge -p '{"spec":{"superAbility":"Invisibility"}}'
wait_for "updated HTML projection" page_contains Invisibility
test "$(kubectl get pod "$pod_name" -n default -o jsonpath='{.metadata.uid}')" = "$old_pod_uid"

echo "Checking deleted Pod recreation"
kubectl delete pod "$pod_name" -n default --timeout=60s
wait_for "replacement Pod" pod_recreated
kubectl wait pod/"$pod_name" -n default --for=condition=Ready --timeout=180s
wait_for "webpage after Pod recreation" page_contains Invisibility

echo "Checking garbage collection of all five children"
kubectl delete superpod superpod-demo -n default --timeout=60s
wait_for "child garbage collection" children_gone

echo "Checks passed. Creating a fresh demo for browser inspection"
kubectl apply -f test/superpod/superpod.yaml
kubectl wait superpod/superpod-demo -n default --for=condition=Ready --timeout=180s
wait_for "fresh demo webpage" page_contains Flying
kubectl get superpod superpod-demo -n default
echo "Forward dev-container port 80 to local port 80 in VS Code, then open $url"
echo "Test kubeconfig: $KUBECONFIG"
