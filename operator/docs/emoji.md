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
the `operator` directory, deploy your image and then patch the existing Secret:

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
`JEV_MODEL` optionally selects a Jev model identifier supported by the service;
when absent, the service selects its default. The manager needs outbound HTTPS
access to `www.jevai.org`.

## How selection works

The integration uses [Jev's native Decisions endpoint](https://www.jevai.org/docs),
`POST https://www.jevai.org/api/v1/decisions`, with Bearer authentication.
It sends the ability as structured state and asks a `choice` question:
“Find the most fitting emoji for the super ability in super_ability.”
Every option includes its Unicode name.

The bundled Unicode Emoji 18.0 catalog contains all fully qualified emoji from
[Unicode's emoji test data](https://unicode.org/Public/emoji/latest/emoji-test.txt),
including flags, skin tones, and joined sequences. Alternate encodings of the
same emoji and standalone modifier components are not separate choices.
The original data and Unicode license are in `internal/emoji`. Updating the
bundled catalog requires rebuilding the operator. Device and font support varies;
newer emoji may not render everywhere. Proprietary stickers are not Unicode emoji.

All entries are considered in batches of up to 200, reduced as needed to stay
below the service's 32 KiB request limit. Each batch's top three advance to a
final comparison. This is a shortlist heuristic, not an exact global ranking:
probabilities from different batches are never compared directly. Expect roughly
one request per 200 emoji plus one final request per uncached ability.

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
the problem, and the operator retries after one minute. Individual requests have
a 20-second timeout and the complete selection has a three-minute deadline.
Provider error bodies are not copied into logs or resource status.

```bash
kubectl describe superpod flying-demo
make lint-fix
make test
```

Tests use an HTTP stub and Kubernetes envtest, so they do not require a key or
make paid Jev requests. A live check is to enable `emoji`, wait for the ConfigMap
to contain a selection, and open the usual Superpod URL. ConfigMap projection
into nginx may take a short time, as with ordinary ability updates.
