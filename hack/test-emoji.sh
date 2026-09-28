#!/usr/bin/env bash
# Ask Jev to choose emoji for super abilities in the Kind test cluster and print
# its choices. Run `make test-superpod` first. If the operator has no TypeSafe
# API key yet, this uses JEV_API_KEY from the environment or prompts for it, then
# restarts the manager.
#
#   bash hack/test-emoji.sh                          # sample abilities
#   bash hack/test-emoji.sh "Laser vision" Healing   # your own abilities
#   bash hack/test-emoji.sh --set-key                # replace the stored key
#   bash hack/test-emoji.sh --cleanup                # delete these Superpods
#
# Each new ability costs several Jev requests. Rerunning is cheap: finished
# selections are cached in each Superpod's ConfigMap.
set -euo pipefail
cd "$(dirname "$0")/.."

: "${LOCALBIN:=$PWD/bin}"
: "${KUBECTL:=kubectl}"
: "${EMOJI_TIMEOUT:=1800}"
export PATH="$LOCALBIN:$PATH"
export KUBECONFIG="$LOCALBIN/superpod-e2e/config.kubeconfig"
export KUBECTL_KUBERC=false

label=test.example/emoji-check
secret=(secret operator-jev-api -n operator-system)
manager=(deployment/operator-controller-manager -n operator-system)

command -v jq >/dev/null || { echo "This script requires jq." >&2; exit 1; }
if [[ ! -f "$KUBECONFIG" ]] || ! "$KUBECTL" get "${secret[@]}" >/dev/null 2>&1; then
  echo "The operator is not running in the test cluster; run 'make test-superpod' first." >&2
  exit 1
fi

set_key() {
  local key=${JEV_API_KEY:-}
  if [[ -z "$key" ]]; then
    if [[ ! -t 0 ]]; then
      echo "No TypeSafe API key is configured; run this script in a terminal to enter one." >&2
      exit 1
    fi
    read -rsp 'TypeSafe API key (input hidden): ' key
    echo
  fi
  [[ -n "$key" ]] || { echo "No key entered." >&2; exit 1; }
  # Pass the key through stdin so it never appears in a process listing.
  printf '%s' "$key" | jq -Rs '{stringData: {JEV_API_KEY: .}}' |
    "$KUBECTL" patch "${secret[@]}" --type=merge --patch-file=/dev/stdin
  # The manager reads the key from its environment at startup.
  "$KUBECTL" rollout restart "${manager[@]}"
  "$KUBECTL" rollout status "${manager[@]}" --timeout=180s
}

case "${1:-}" in
  --cleanup)
    "$KUBECTL" delete superpods -n default -l "$label"
    exit
    ;;
  --set-key)
    set_key
    exit
    ;;
esac
if [[ -z "$("$KUBECTL" get "${secret[@]}" -o go-template='{{if .data.JEV_API_KEY}}set{{end}}')" ]]; then
  set_key
fi

if (( $# == 0 )); then
  abilities=(Flying Invisibility "Super strength" "Fire breathing" Telepathy)
else
  abilities=("$@")
fi

names=()
for ability in "${abilities[@]}"; do
  # Superpod names and Ingress hosts must be short lowercase DNS labels.
  slug=$(printf '%s' "$ability" | tr '[:upper:]' '[:lower:]' | tr -cs 'a-z0-9' '-' | cut -c1-40 | sed 's/^-*//; s/-*$//')
  [[ -n "$slug" ]] || slug=$(printf '%s' "$ability" | cksum | cut -d' ' -f1)
  name=emoji-$slug
  names+=("$name")
  jq -n --arg name "$name" --arg ability "$ability" --arg labelName "$label" '{
    apiVersion: "super.elp-max.com/v1", kind: "Superpod",
    metadata: {name: $name, namespace: "default", labels: {($labelName): "true"}},
    spec: {superAbility: $ability, emoji: true, host: "\($name).localhost", ingressClassName: "traefik"}
  }' | "$KUBECTL" apply -f -
done

# Print a Superpod's chosen emoji, or what it is still waiting for.
describe() {
  local superpod configmap
  superpod=$("$KUBECTL" get superpod "$1" -n default -o json) || return 1
  configmap=$("$KUBECTL" get configmap "superpod-$(jq -r .metadata.uid <<<"$superpod")" \
    -n default --ignore-not-found -o json) || return 1
  jq -rn --argjson superpod "$superpod" --arg configmap "$configmap" '
    ($superpod.status.conditions // [] | map(select(.type == "Ready")) | first) as $ready
    | (try ($configmap | fromjson | .data["emoji-cache.json"] | fromjson | .matches) catch null) as $matches
    | if $ready == null or $ready.observedGeneration != $superpod.metadata.generation then
        "waiting for the operator"
      elif ($matches | length) > 0 and ($ready.reason | startswith("Emoji") | not) then
        "chose " + ($matches | map("\(.emoji) \(.name) (\(.score * 100 | round)%)") | join(", "))
      else
        "\($ready.reason): \($ready.message)"
      end'
}

# The operator reconciles one Superpod at a time, so results arrive gradually.
reported=()
deadline=$((SECONDS + EMOJI_TIMEOUT))
while :; do
  waiting=0
  for i in "${!names[@]}"; do
    state=$(describe "${names[$i]}") || state="could not read status"
    if [[ "${reported[$i]:-}" != "$state" ]]; then
      echo "${names[$i]}: $state"
      reported[$i]=$state
    fi
    [[ "$state" == chose* ]] || waiting=$((waiting + 1))
  done
  (( waiting > 0 )) || break
  if (( SECONDS >= deadline )); then
    echo "Timed out with $waiting Superpod(s) still waiting; rerun this script to keep checking." >&2
    exit 1
  fi
  sleep 5
done

echo
for i in "${!names[@]}"; do
  echo "${abilities[$i]} → ${reported[$i]#chose }"
  echo "  http://${names[$i]}.localhost"
done
