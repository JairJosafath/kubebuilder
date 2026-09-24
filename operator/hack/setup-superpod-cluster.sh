#!/usr/bin/env bash
# Manage the disposable Superpod cluster and its kubeconfig.
set -euo pipefail
cd "$(dirname "$0")/.."

: "${LOCALBIN:=$PWD/bin}"
: "${KIND:=kind}"
: "${KUBECTL:=kubectl}"
: "${RG:=rg}"
: "${SUPERPOD_READY_TIMEOUT:=120}"
case "$LOCALBIN" in /*) ;; *) LOCALBIN="$PWD/$LOCALBIN" ;; esac
export PATH="$LOCALBIN:$PATH"
export KIND_EXPERIMENTAL_PROVIDER=docker
export KUBECTL_KUBERC=false
cluster=superpod-e2e
workdir="$LOCALBIN/$cluster"
user_kubeconfig=${KUBECONFIG:-}
export KUBECONFIG="$workdir/config.kubeconfig"

# Kind merges just this named cluster into the user's configured file(s).
with_user_config() (
  if [[ -n "$user_kubeconfig" ]]; then
    export KUBECONFIG="$user_kubeconfig"
  else
    unset KUBECONFIG
  fi
  "$@"
)

fail() { echo "$*" >&2; exit 1; }
list_nodes() {
  local output node
  output=$(docker ps -a --filter "label=io.x-k8s.kind.cluster=$cluster" --format '{{.Names}}') || return 1
  nodes=()
  while IFS= read -r node; do
    [[ -z "$node" ]] || nodes+=("$node")
  done <<< "$output"
}

case "${1:-up}" in
  stop)
    list_nodes || fail "Could not list test nodes"
    if (( ${#nodes[@]} )); then
      for node in "${nodes[@]}"; do
        state=$(docker inspect --format '{{.State.Status}}' "$node")
        if [[ "$state" == paused ]]; then docker unpause "$node"; fi
        docker stop "$node"
      done
    fi
    echo "Stopped $cluster. Resume with make setup-test-superpod."
    exit 0
    ;;
  destroy)
    with_user_config "$KIND" delete cluster --name "$cluster"
    list_nodes || fail "Could not verify test cluster deletion"
    (( ${#nodes[@]} == 0 )) || fail "Test node containers remain after deletion"
    rm -rf -- "$workdir"
    echo "Destroyed $cluster and removed its local state and kubeconfig."
    exit 0
    ;;
  up) ;;
  *) fail "Usage: $0 [up|stop|destroy]" ;;
esac

[[ "$SUPERPOD_READY_TIMEOUT" =~ ^[1-9][0-9]*$ ]] || fail "SUPERPOD_READY_TIMEOUT must be a positive number of seconds"
mkdir -p "$workdir"
scratch=$(mktemp -d "$LOCALBIN/.superpod-cluster.XXXXXX")
trap 'rm -rf "$scratch"' EXIT

# Record both the source configuration (including port mappings) and container
# identities. An imported/replaced cluster or edited config must be recreated.
cluster_state() {
  (( ${#nodes[@]} )) || return 1
  if command -v sha256sum >/dev/null; then
    sha256sum < test/superpod/kind.yaml || return 1
  else
    shasum -a 256 < test/superpod/kind.yaml || return 1
  fi
  docker inspect --format '{{.Id}}' "${nodes[@]}" | sort
}
config_matches() {
  [[ -f "$workdir/cluster-state" ]] || return 1
  cluster_state > "$scratch/current-state" || return 1
  cmp -s "$workdir/cluster-state" "$scratch/current-state"
}
start_nodes() {
  local node state
  for node in "${nodes[@]}"; do
    state=$(docker inspect --format '{{.State.Status}}' "$node") || return 1
    case "$state" in
      running) ;;
      exited|created) docker start "$node" || return 1 ;;
      paused) docker unpause "$node" || return 1 ;;
      *) return 1 ;;
    esac
  done
}
refresh_and_wait() {
  local context deadline node
  local node_resources=()
  # Export a fresh file so stale endpoints and credentials cannot survive a merge.
  "$KIND" export kubeconfig --name "$cluster" --kubeconfig "$scratch/config" || return 1
  chmod 600 "$scratch/config" || return 1
  context=$("$KUBECTL" --kubeconfig "$scratch/config" config current-context) || return 1
  [[ "$context" == "kind-$cluster" ]] || return 1
  mv "$scratch/config" "$KUBECONFIG" || return 1
  echo "Waiting for the Kubernetes API and nodes in $cluster"
  deadline=$((SECONDS + SUPERPOD_READY_TIMEOUT))
  until "$KUBECTL" --request-timeout=5s get --raw=/readyz >/dev/null 2>&1; do
    (( SECONDS < deadline )) || return 1
    sleep 2
  done
  for node in "${nodes[@]}"; do node_resources+=("node/$node"); done
  (( ${#node_resources[@]} )) || return 1
  "$KUBECTL" wait --for=condition=Ready "${node_resources[@]}" --timeout="${SUPERPOD_READY_TIMEOUT}s" || return 1
}

# Failed discovery is an infrastructure error, not evidence of an absent cluster.
clusters=$("$KIND" get clusters) || fail "Could not list Kind clusters"
list_nodes || fail "Could not list Kind node containers"
exists=false
if printf '%s\n' "$clusters" | "$RG" -Fxq "$cluster" || (( ${#nodes[@]} )); then exists=true; fi

ready=false
if $exists && config_matches; then
  echo "Reusing cluster $cluster with the current configuration"
  if start_nodes && refresh_and_wait; then ready=true; fi
fi
if ! $ready; then
  if $exists; then
    echo "Recreating disposable cluster $cluster: its configuration or health could not be verified"
    "$KIND" delete cluster --name "$cluster" || fail "Could not delete the old test cluster"
    list_nodes || fail "Could not verify old node removal"
    (( ${#nodes[@]} == 0 )) || fail "Old test nodes remain; Docker could not remove them"
  fi
  echo "Creating $cluster from test/superpod/kind.yaml"
  "$KIND" create cluster --name "$cluster" --config test/superpod/kind.yaml \
    --kubeconfig "$KUBECONFIG" --wait "${SUPERPOD_READY_TIMEOUT}s" || fail "Could not create test cluster; check Docker and port 80 availability"
  list_nodes || fail "Could not list new test nodes"
  refresh_and_wait || fail "New test cluster is not ready; check Docker networking and node logs"
fi
cluster_state > "$scratch/cluster-state" || fail "Could not record test cluster configuration"
mv "$scratch/cluster-state" "$workdir/cluster-state"
with_user_config "$KIND" export kubeconfig --name "$cluster" || fail "Could not update your kubeconfig"
echo "Cluster $cluster is ready. kubectl now uses kind-$cluster."
echo "Dedicated test kubeconfig: $KUBECONFIG"
