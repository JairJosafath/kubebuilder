# Optional Jev emoji matching

The operator already generates `index.html`. Set `spec.emoji: true` to add
matching emoji beside the ability. The default remains the plain ability page,
with no Jev requests or credentials required.

```yaml
apiVersion: super.elp-max.com/v1
kind: Superpod
metadata:
  name: flying-demo
spec:
  superAbility: Flying
  emoji: true
  host: superpod.example.test
```

## Configure the key

Create an API key in the [TypeSafe console](https://console.typesafe.ai/keys).
For a local manager, enter it without putting it in shell history:

```bash
read -rsp 'TypeSafe API key: ' JEV_API_KEY; echo
export JEV_API_KEY
make install
make run
```

For a deployed manager, the normal deployment creates an empty
`operator-jev-api` Secret in `operator-system` alongside the controller. From
the repository root, deploy your image and then patch the existing Secret:

```bash
make deploy IMG=<your-registry>/operator:<tag>
kubectl -n operator-system patch secret operator-jev-api --type=merge \
  -p '{"stringData":{"JEV_API_KEY":"<your-TypeSafe-API-key>"}}'
kubectl -n operator-system rollout restart deployment/operator-controller-manager
kubectl -n operator-system rollout status deployment/operator-controller-manager
```

In the Kind test cluster from the README, `bash hack/test-emoji.sh --set-key`
prompts for the key without echoing it, stores it, and restarts the manager.

Replace the key placeholder with your key. The manager starts without a key; plain Superpods
continue working, while emoji-enabled Superpods report `EmojiSelectionFailed`
until a key is configured. Restart after the initial patch and after each key
rotation because the manager reads the key from its environment at startup.
Reapplying the deployment preserves the patched key; the Secret manifest does
not manage its data. `config/emoji` remains an alias for the default deployment.
If you used the previous manually created `jev-api` Secret, patch
`operator-jev-api` with your key when upgrading; the manager now uses that Secret.

Only the manager receives the key; nginx, Superpod specs, generated HTML, and
cached results never contain it.
The manager needs outbound HTTPS access to `api.typesafe.ai`.

## Endpoint and model

The operator calls [TypeSafe's Jev API](https://docs.typesafe.ai/api) at
`POST https://api.typesafe.ai/v1/systemone`, sending the key as a Bearer token.
Each request names a model, passes the ability as `state`, and asks `choice`
questions. Responses contain `{model, answers, usage}`; each answer maps every
option to a probability.

The model defaults to `jev-latest`, TypeSafe's alias for its current Jev release.
To pin a release, set `JEV_MODEL` in the manager's environment to an identifier
from [TypeSafe's model list](https://docs.typesafe.ai/models), such as `jev-1.13.0`.

## How selection works

The request sends the ability as structured state and asks a `choice` question:
“Find the most fitting emoji for the super ability in super_ability.”
Every option includes its Unicode name.

The bundled Unicode Emoji 18.0 catalog contains all fully qualified emoji from
[Unicode's emoji test data](https://unicode.org/Public/emoji/latest/emoji-test.txt),
including flags, skin tones, and joined sequences. Alternate encodings of the
same emoji and standalone modifier components are not separate choices.
The original data and Unicode license are in `internal/emoji`. Updating the
bundled catalog requires rebuilding the operator. Device and font support varies;
newer emoji may not render everywhere. Proprietary stickers are not Unicode emoji.

All entries are considered in Choice questions of up to 255 options. A request
combines up to eight independent questions and stays below 30 KiB, well inside
the model's 64k-token context window. Combining questions
shares the state and reduces request count, following TypeSafe's
[parallel-question pattern](https://docs.typesafe.ai/patterns/fan-out).
Each question's top three advance to a final comparison. This is a shortlist
heuristic, not an exact global ranking: probabilities from different questions
are never compared directly. Every uncached ability still considers the full
catalog before the final comparison. With the bundled catalog, `Flying` requires
nine HTTP requests including the final comparison, down from twenty-one.

The final scores decide the layout. An emoji is close when it is within
**0.05 (five percentage points)** of the winner:

- A clear winner is shown alone.
- Two close emoji are shown side by side.
- When three or four are close, the top four are shown in a 2×2 square; with
  three close, the fourth-highest completes the square.

Scores are used as TypeSafe returns them; they are not required to add up to
one. Exact ties use Unicode string order for stable output. Scores are model
estimates, not a guarantee of semantic correctness.

Successful results are stored in the owned ConfigMap as `emoji-cache.json`,
alongside `index.html`. This file contains only public emoji names, scores, and
a cache identifier, and is also accessible through nginx. Reconciles and manager
restarts reuse it. Changing the ability, configured model, catalog, or selection
algorithm triggers fresh selection; deleting the ConfigMap also triggers it.
Turning `emoji` off removes the emoji and cache without replacing the Pod.

## Failure behavior and testing

Missing credentials, provider failures, or malformed responses leave the plain
ability page available. `Ready=False` with reason `EmojiSelectionFailed` describes
these failures, and the operator schedules another attempt after one minute.
Individual requests have a 20-second timeout and each selection attempt has a
three-minute deadline. Raw provider error bodies are not copied into logs or status.

TypeSafe answers HTTP 429 when a rate limit is exceeded and HTTP 529 when the
service is overloaded; neither depends on cluster readiness or kubeconfig. The
operator reports `EmojiRateLimited` with the status and the next attempt time.
It honors a `Retry-After` header (seconds or an HTTP date) when present, and
prevents early watch events or other Superpods from making requests during that
client's cooldown. If the header is missing or invalid, consecutive throttled
requests back off from one minute to a maximum of fifteen minutes. A successful
request resets the backoff.

Completed batches are cached in memory for one hour (up to 128 decisions), so a
throttled scan can resume without repeating successful requests. This also shares
results for identical abilities within the manager process. Restarting the manager
clears the cooldown and batch cache; completed selections in ConfigMaps survive.
Each uncached ability still needs the full catalog scan described above.

[TypeSafe documents its limits](https://docs.typesafe.ai/models) in tokens per
second, requests per minute, and tokens per request. For a persistent 429 or 529,
check the reported status and your TypeSafe account. To use the plain page while
investigating, disable emoji:

```bash
kubectl patch superpod superpod-sample --type=merge -p '{"spec":{"emoji":false}}'
```

```bash
kubectl describe superpod flying-demo
make lint-fix
make test
```

Tests use an HTTP stub and Kubernetes envtest, so they do not require a key or
make paid Jev requests. A live check is to enable `emoji`, wait for the ConfigMap
to contain a selection, and open the usual Superpod URL. ConfigMap projection
into nginx may take a short time, as with ordinary ability updates.
