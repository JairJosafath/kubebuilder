#!/usr/bin/env bash
# Exercise disposable cluster lifecycle decisions without a real cluster.
set -euo pipefail
cd "$(dirname "$0")/.."
testdir=$(mktemp -d)
trap 'rm -rf "$testdir"' EXIT

cat > "$testdir/mock-tool" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
tool=${0##*/}
[[ "$KIND_EXPERIMENTAL_PROVIDER" == docker && "$KUBECTL_KUBERC" == false ]]
printf '%s %s\n' "$tool" "$*" >> "$TEST_LOG"
if [[ "$KUBECONFIG" == "$TEST_STATE_DIR/user-config" ]]; then
  [[ "$tool:$*" == 'kind:export kubeconfig --name superpod-e2e' || "$tool:$*" == 'kind:delete cluster --name superpod-e2e' ]]
  echo user-config >> "$TEST_LOG"
else
  [[ "$KUBECONFIG" == "$TEST_STATE_DIR/bin/superpod-e2e/config.kubeconfig" ]]
fi
created=false
[[ ! -f "$TEST_STATE_DIR/created" ]] || created=true
case "$tool:$*" in
  'kind:get clusters')
    [[ "$SCENARIO" != discovery_error ]] || exit 1
    echo other-cluster
    case "$SCENARIO" in
      absent|create_error|missing_created_nodes|empty) echo superpod-e2e-old ;;
      *) echo superpod-e2e ;;
    esac
    ;;
  docker:ps*)
    [[ "$*" == 'ps -a --filter label=io.x-k8s.kind.cluster=superpod-e2e --format {{.Names}}' ]]
    [[ "$SCENARIO" != docker_error ]] || exit 1
    if $created; then
      if [[ "$SCENARIO" != missing_created_nodes ]]; then echo superpod-e2e-control-plane; fi
    elif [[ ! -f "$TEST_STATE_DIR/deleted" ]]; then
      case "$SCENARIO" in
        absent|create_error|missing_created_nodes|missing_existing_nodes|empty) ;;
        *) echo superpod-e2e-control-plane ;;
      esac
    fi
    ;;
  'docker:inspect --format {{.Id}} superpod-e2e-control-plane')
    if $created; then echo new-node-id; else echo old-node-id; fi
    ;;
  'docker:inspect --format {{.State.Status}} superpod-e2e-control-plane')
    case "$SCENARIO" in
      stopped|start_error) echo exited ;;
      paused) echo paused ;;
      *) echo running ;;
    esac
    ;;
  'docker:start superpod-e2e-control-plane') [[ "$SCENARIO" != start_error ]] ;;
  'docker:stop superpod-e2e-control-plane'|'docker:unpause superpod-e2e-control-plane') ;;
  kind:create*)
    [[ "$SCENARIO" != create_error ]] || exit 1
    touch "$TEST_STATE_DIR/created"
    ;;
  'kind:delete cluster --name superpod-e2e')
    [[ "$SCENARIO" != delete_error ]] || exit 1
    touch "$TEST_STATE_DIR/deleted"
    ;;
  kind:export*)
    if [[ "$SCENARIO" == export_error ]] && ! $created; then exit 1; fi
    if [[ "${5:-}" == --kubeconfig ]]; then printf 'fresh-config\n' > "$6"; fi
    ;;
  kubectl:*'config current-context')
    if [[ "$SCENARIO" == wrong_context ]] && ! $created; then echo kind-other; else echo kind-superpod-e2e; fi
    ;;
  'kubectl:--request-timeout=5s get --raw=/readyz') ;;
  kubectl:wait*)
    [[ "$*" == 'wait --for=condition=Ready node/superpod-e2e-control-plane --timeout=1s' ]]
    [[ "$SCENARIO" != always_unready ]] || exit 1
    if [[ "$SCENARIO" == unready_nodes ]] && ! $created; then exit 1; fi
    ;;
  *) echo "Unexpected command: $tool $*" >&2; exit 1 ;;
esac
EOF
chmod +x "$testdir/mock-tool"

called() { grep -Fq -- "$1" "$TEST_LOG" || { cat "$TEST_LOG"; echo "Missing call: $1" >&2; exit 1; }; }
absent() { if grep -Fq -- "$1" "$TEST_LOG"; then cat "$TEST_LOG"; echo "Unexpected call: $1" >&2; exit 1; fi; }
count=0
run_case() {
  export SCENARIO=$1
  local expected=$2 action=${3:-up}
  count=$((count + 1))
  export TEST_STATE_DIR="$testdir/$count-$SCENARIO"
  export TEST_LOG="$TEST_STATE_DIR/commands"
  local state_dir="$TEST_STATE_DIR/bin/superpod-e2e"
  mkdir -p "$state_dir"
  : > "$TEST_LOG"
  printf 'stale-config\n' > "$state_dir/config.kubeconfig"
  for tool in kind docker kubectl; do cp "$testdir/mock-tool" "$TEST_STATE_DIR/bin/$tool"; done
  case "$SCENARIO" in
    healthy|stopped|start_error|wrong_context|unready_nodes|always_unready|export_error|paused)
      if command -v sha256sum >/dev/null; then
        sha256sum < test/superpod/kind.yaml > "$state_dir/cluster-state"
      else
        shasum -a 256 < test/superpod/kind.yaml > "$state_dir/cluster-state"
      fi
      echo old-node-id >> "$state_dir/cluster-state"
      ;;
    config_changed) echo outdated-config > "$state_dir/cluster-state" ;;
  esac
  local result=success
  LOCALBIN="$TEST_STATE_DIR/bin" KIND="$TEST_STATE_DIR/bin/kind" \
    KUBECTL="$TEST_STATE_DIR/bin/kubectl" RG="$(command -v grep)" \
    KUBECONFIG="$TEST_STATE_DIR/user-config" SUPERPOD_READY_TIMEOUT=1 \
    bash hack/setup-superpod-cluster.sh "$action" > "$TEST_STATE_DIR/output" 2>&1 || result=failure
  if [[ "$result" != "$expected" ]]; then
    cat "$TEST_STATE_DIR/output"
    echo "$SCENARIO/$action: expected $expected, got $result" >&2
    exit 1
  fi
  absent 'kind delete cluster --name other-cluster'
  if [[ "$expected/$action" == success/up ]]; then
    called user-config
    grep -q fresh-config "$state_dir/config.kubeconfig"
    test -s "$state_dir/cluster-state"
  fi
}

run_case absent success
called 'kind create cluster --name superpod-e2e --config test/superpod/kind.yaml'
absent 'kind delete'
run_case healthy success
absent 'kind create'
absent 'docker start'
run_case stopped success
called 'docker start superpod-e2e-control-plane'
absent 'kind create'
run_case paused success
called 'docker unpause superpod-e2e-control-plane'
absent 'kind create'

for scenario in unknown_cluster config_changed missing_existing_nodes start_error wrong_context unready_nodes export_error; do
  run_case "$scenario" success
  called 'kind delete cluster --name superpod-e2e'
  called 'kind create cluster --name superpod-e2e'
done
for scenario in discovery_error docker_error delete_error; do
  run_case "$scenario" failure
  absent 'kind create'
done
for scenario in create_error missing_created_nodes always_unready; do
  run_case "$scenario" failure
  called 'kind create'
  test "$(grep -c 'kind create' "$TEST_LOG")" -eq 1
done

run_case healthy success stop
called 'docker stop superpod-e2e-control-plane'
absent 'kind delete'
test -f "$TEST_STATE_DIR/bin/superpod-e2e/cluster-state"
run_case empty success stop
absent 'docker stop'
run_case healthy success destroy
called 'kind delete cluster --name superpod-e2e'
called user-config
test ! -e "$TEST_STATE_DIR/bin/superpod-e2e"
run_case empty success destroy
test ! -e "$TEST_STATE_DIR/bin/superpod-e2e"
run_case delete_error failure destroy
test -e "$TEST_STATE_DIR/bin/superpod-e2e/config.kubeconfig"

echo "All $count cluster lifecycle scenarios passed"
