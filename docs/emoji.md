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

Use a personal API key from [jevai.org](https://www.jevai.org/agent/keys).
For a local manager, enter it without putting it in shell history:

```bash
read -rsp 'Jev API key: ' JEV_API_KEY; echo
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
  -p '{"stringData":{"JEV_API_KEY":"<your-Jev-API-key>"}}'
kubectl -n operator-system rollout restart deployment/operator-controller-manager
kubectl -n operator-system rollout status deployment/operator-controller-manager
```

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
The manager needs outbound HTTPS access to `www.jevai.org`.

## Choose the endpoint and model

This integration uses the [JevAI Community REST API](https://www.jevai.org/docs).
Emoji selection needs a custom `choice` question, so the correct endpoint is
`POST https://www.jevai.org/api/v1/decisions`. Authenticate with the personal key
from `/agent/keys` as a Bearer token. Successful responses use the
`{code, message, data}` envelope, with `code: 0` and answers inside `data.answers`.
The preset `/api/v1/decisions/model-route` chooses among application-supplied
model candidates; it does not configure the Jev engine or select emoji.

Leave `JEV_MODEL` unset to use the community service's default. To override it,
use an identifier supported by that service; its documentation gives
`typesafe-ai/jev` as an example. Do not copy a model name from another provider.

[TypeSafe's official API](https://docs.typesafe.ai/api) is a separate integration:
`POST https://api.typesafe.ai/v1/systemone`, a TypeSafe console key, a required
model such as `jev-latest`, and a direct `{model, answers, usage}` response.
A JevAI Community key is not a TypeSafe key. Switching requires matching the
endpoint, credentials, model, and response parser together.

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
combines up to eight independent questions and stays below 30 KiB, leaving room
under the community service's documented 32 KiB body limit. Combining questions
shares the state and reduces request count, following TypeSafe's
[parallel-question pattern](https://docs.typesafe.ai/patterns/fan-out).
Each question's top three advance to a final comparison. This is a shortlist
heuristic, not an exact global ranking: probabilities from different questions
are never compared directly. Every uncached ability still considers the full
catalog before the final comparison. With the bundled catalog, `Flying` requires
nine HTTP requests including the final comparison, down from twenty-one.

The final probabilities determine display order. The winner is always shown;
up to two more are included if each is within **0.05 (five percentage points)**
of the winner. Exact ties use Unicode string order for stable output. Scores
are model estimates, not a guarantee of semantic correctness.

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

HTTP 429 indicates throttling somewhere in the API path. The status alone cannot
identify whether a community limit, an upstream limit, or an account restriction
caused it. It is independent of cluster readiness or kubeconfig. The operator
reports `EmojiRateLimited` with the numeric API error code when available and
the next attempt time. It does not infer an account quota from the status alone. It
honors Jev's `Retry-After` header (seconds or an HTTP date), and prevents early
watch events or other Superpods from making requests during that client's cooldown.
If the header is missing or invalid, consecutive throttled requests back off from
one minute to a maximum of fifteen minutes. A successful request resets the backoff.

Completed batches are cached in memory for one hour (up to 128 decisions), so a
throttled scan can resume without repeating successful requests. This also shares
results for identical abilities within the manager process. Restarting the manager
clears the cooldown and batch cache; completed selections in ConfigMaps survive.
Each uncached ability still needs the full catalog scan described above.

The [community homepage](https://www.jevai.org/) advertises a temporary free
playground promotion. That does not establish that REST, MCP, or upstream limits
have been removed. [TypeSafe documents](https://docs.typesafe.ai/models) separate
token-per-second and request-per-minute limits, as well as per-request context
limits; these do not establish the limits applied to a community key.
For a persistent 429, check the reported diagnostic and the limits for the
service that issued your key. To use the plain page while investigating,
disable emoji:

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
