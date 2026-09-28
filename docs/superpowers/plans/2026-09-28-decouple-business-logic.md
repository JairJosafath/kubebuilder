# Decouple Business Logic from Kubebuilder Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move the Superpod business rules out of Kubebuilder's scaffolded packages into plain Go packages behind a port. Enforce the layering and the Go, Kubebuilder, and GNU conventions with `make lint`, `make verify`, and `make check`. Record in `docs/architecture.md` which convention governs which directory.

**Architecture:** This uses ports and adapters with a functional core. `internal/webpage` and `internal/emoji` hold the rules and import no Kubernetes, controller-runtime, or HTTP code. `internal/resources` adapts them to Kubernetes objects. `internal/typesafe` adapts TypeSafe's HTTP API to the `webpage.EmojiSelector` port. The Kubebuilder-owned `internal/controller` is a thin driver. Because each style guide governs its own layer, most style conflicts become "which layer is this?" instead of "which guide wins?"

**Tech Stack:** Go 1.26, controller-runtime v0.25, Kubebuilder go/v4 layout, Ginkgo/Gomega with envtest (controller only), standard `testing` with `go-cmp` (everything else), golangci-lint v2.13.1 with the `logcheck` v0.10.1 plugin, `depguard`, `revive`, `importas`, and GNU Make.

**Spec:** There is no separate spec file. The requirements come from the user's request of 2026-09-28 and are listed under Requirements below.

**Verification status:** Every task was dry-run in order on a scratch copy of the repository. After each task, `go test` and `golangci-lint` passed; at the end, `make verify` and `make check` passed. The code blocks below were copied from that run.

## Requirements

1. Review the code, and change it where needed, to follow [Effective Go](https://go.dev/doc/effective_go), the [Google Go Style Guide](https://google.github.io/styleguide/go/), [Kubebuilder good practices](https://book.kubebuilder.io/reference/good-practices.html), and the [GNU Makefile conventions](https://www.gnu.org/prep/standards/html_node/Makefile-Conventions.html).
2. Reduce dependence on AI: automate checks through the Makefile and keep the code readable to humans.
3. Organize the code with design patterns that are idiomatic in Go.
4. Follow the conventions of mature CNCF projects.
5. Resolve conflicts between the style guides. Do this mainly by decoupling, so that business logic is not tightly coupled to Kubebuilder's structure.

## How decoupling resolves the conflicts

Kubebuilder's conventions apply to the code Kubebuilder generates and merges (`api/`, `cmd/`, `config/`, `internal/controller/`, `test/`). Go's conventions apply to packages the project owns (`internal/webpage`, `internal/emoji`, `internal/resources`, `internal/typesafe`). Once the business rules leave `internal/controller`, "Ginkgo or `testing`?", "dot imports?", and "whose comment style?" become questions about the layer, not the guide. Conflicts that remain inside one layer are decided in Task 9's table, for example `make install` (GNU says copy to `$(prefix)`, Kubebuilder says install CRDs).

## Global Constraints

- The Go version stays `go 1.26.0`. Add no new modules, except promoting `github.com/google/go-cmp v0.7.0` from indirect to direct.
- golangci-lint stays at `v2.13.1`. The `logcheck` plugin is pinned to `v0.10.1`.
- Never hand-edit `config/crd/bases/*`, `config/rbac/role.yaml`, `**/zz_generated.*.go`, or `PROJECT`. Regenerate them with `make manifests generate`.
- Do not remove `// +kubebuilder:scaffold:*` markers. Do not move Kubebuilder-owned files (`api/v1/`, `cmd/main.go`, `internal/controller/`, `config/`, `test/e2e/`, `test/utils/`).
- The Superpod API does not change: the fields, the `Ready` condition type, and the reasons `ResourcesReady`, `PodNotReady`, `IngressPending`, `ResourceTerminating`, `ReconcileFailed`, `ObservationFailed`, `EmojiSelectionFailed`, `EmojiRateLimited`.
- `JEV_API_KEY`, `JEV_MODEL`, and the Secret `jev-api` keep their names. TypeSafe remains the only emoji provider; do not reintroduce jevai.org.
- Persisted formats stay compatible:
  - ConfigMap keys `index.html` and `emoji-cache.json`.
  - JSON `{"key":…,"matches":[{"emoji":…,"name":…,"score":…}]}`.
  - Selection key `hex(sha256(CacheKey() + "\x00" + ability))`.
  - `CacheKey()` = `api.typesafe.ai/v3/parallel255-top3-layout124-gap0.05/<model>/<catalog sha256>`.
- The page HTML stays byte-identical, with one exception: `html/template` writes `+` as `&#43;`, which browsers render the same.
- Hand-written Go files have no license header. Commit `d84dc51` removed them on purpose.
- Every commit leaves `make lint` at `0 issues.` and `make test` passing.

## Review Focus

These are the failure modes most likely to hurt a real user. No existing test covered them before this plan; each now has a test in the task that owns the code.

1. **Page bytes drift during the refactor.** Every running Superpod's ConfigMap would be rewritten. Tested by `TestPageMatchesGoldenHTML` (Task 1, carried through Task 5) and `TestRenderMatchesGolden` (Task 4).
2. **The emoji cache key or stored selection format drifts.** Every Superpod with emoji would pay TypeSafe for a new selection. Tested by `TestCacheKeyIsStable` (Task 1, moved in Task 7), the envtest spec "reuses an emoji selection persisted by an earlier operator version" (Task 1), and `TestChooseEmojiStoresTheKeyItWasMadeFor` (Task 4).
3. **No API key, or TypeSafe is down, while a valid selection is stored.** The page must keep its emoji and not fall back to the plain page. Tested by `TestChooseEmojiReusesAValidStoredSelection` (Task 4).
4. **A ConfigMap that is foreign or terminating.** It must not trigger a paid selection that cannot be stored. Tested by the extended "refuses to adopt a ConfigMap" table (Task 1).
5. **A foreign or terminating Pod or Ingress.** It must never make the Superpod report `Ready=True`. Tested by `TestReadiness` (Task 6).

## Accepted behavior changes

- Abilities containing `+` render as `&#43;`. The first reconcile after upgrading updates those ConfigMaps once. This makes no TypeSafe call, because the selection key does not depend on the HTML.
- Readiness now reads the Ingress even when the Pod is not ready. If that Ingress read fails, the reason becomes `ObservationFailed` instead of `PodNotReady`.
- `emoji.RateLimitError` replaces its `Status int` field with `Cause string`. The message text is unchanged for HTTP 429 and 529.

## File structure after the plan

```text
api/v1/conditions.go            NEW  Ready condition type and reasons (API contract)
cmd/main.go                     MOD  package comment; wires typesafe.NewClient
internal/controller/            Kubebuilder-owned driving adapter, now thin
  doc.go                        NEW  package comment: what belongs here and what does not
  superpod_controller.go        MOD  Reconcile uses webpage/resources; fail(); getIfExists()
  superpod_emoji.go             MOD  chooseEmoji: fetch the stored selection, call webpage.ChooseEmoji
  superpod_status.go            MOD  readiness only fetches; resources.Readiness judges
internal/webpage/               NEW  business rules (plain Go)
  doc.go, render.go             Render: html/template page
  emoji.go                      EmojiSelector port, Selection, ChooseEmoji
internal/emoji/                 provider-neutral emoji vocabulary (plain Go)
  catalog.go                    MOD  adds Catalog, CatalogVersion, ValidScore
  pick.go                       NEW  Pick: 1, 2, or 4 emoji (was closeMatches)
  ratelimit.go                  NEW  RateLimitError{RetryAt, Cause}
internal/typesafe/              NEW  TypeSafe adapter; files moved from internal/emoji with git mv
  client.go (was jev.go), batching.go, errors.go, retry.go, doc.go
internal/resources/             Kubernetes adapter
  doc.go                        NEW  package comment (moved from metadata.go and expanded)
  configmap.go                  MOD  NewConfigMap(sp, sel); StoredSelection
  metadata.go                   MOD  ObjectKey
  readiness.go                  NEW  Readiness (pure)
docs/architecture.md            NEW  layers, directory jurisdiction, resolved conflicts, patterns
.golangci.yml, .custom-gcl.yml  MOD  style linters, pinned logcheck, layering rules
Makefile                        MOD  test-unit, verify, check, clean, test-emoji, order-only prerequisites
.github/workflows/*.yml         MOD  run make verify; stop masking go.mod drift; pin kind
```

## Tasks at a glance

| # | Task | Why a reviewer might reject it on its own |
| --- | --- | --- |
| 0 | Commit the in-progress TypeSafe work and confirm a green baseline | This is the user's own work |
| 1 | Characterization tests | Tests only; they must pass before and after every refactor |
| 2 | Style linters and lint-binary rebuild fix | Changes to tooling and scaffolded files |
| 3 | Condition constants in `api/v1` | Adds to the API package |
| 4 | `internal/webpage` business rules | A new package, not wired in yet |
| 5 | Wire `webpage` through `resources`; thin controller | Changes the ConfigMap builder signature |
| 6 | Pure `resources.Readiness` | Changes how readiness is read |
| 7 | Split `internal/emoji` into domain and `internal/typesafe` | Large file move |
| 8 | Makefile and CI automation | Changes to build tooling |
| 9 | Layering rules and architecture docs | Linter policy and docs |

---

### Task 0: Commit the in-progress TypeSafe work and confirm a green baseline

**Files:** none new.

- [ ] **Step 1: Confirm the uncommitted work may be committed**

The working tree contains an unfinished TypeSafe migration. The modified files are `README.md`, `docs/emoji.md`, `internal/emoji/*`, `internal/resources/configmap.go`, and `internal/resources/resources_test.go`, plus the untracked `hack/test-emoji.sh`. Ask the user to confirm that these are ready to commit as they are. Every later task assumes they are committed.

- [ ] **Step 2: Run the baseline checks**

Run: `make lint && make test`
Expected: `0 issues.`, then `ok` for `internal/controller`, `internal/emoji`, and `internal/resources`.

- [ ] **Step 3: Commit**

```bash
git add README.md docs/emoji.md internal/emoji internal/resources hack/test-emoji.sh
git commit -m "feat: switch emoji selection to TypeSafe's Jev API"
```

---

### Task 1: Pin today's behavior with characterization tests

These tests record behavior that is persisted in running clusters or that costs money. They pass immediately. Their job is to fail if a later task changes that behavior by accident.

**Files:**
- Modify: `internal/resources/resources_test.go` (append)
- Modify: `internal/emoji/jev_test.go` (append)
- Modify: `internal/controller/superpod_controller_test.go` (imports, one new spec, extended table)

**Interfaces:**
- Consumes: the current `resources.NewConfigMap(sp *superv1.Superpod, emojis ...string) *corev1.ConfigMap`, `(*emoji.Client).CacheKey() string`, and `catalogHash()`.
- Produces: the golden strings and fixtures that Tasks 4, 5, and 7 must keep passing.

- [ ] **Step 1: Append the golden page test to `internal/resources/resources_test.go`**

```go
// TestPageMatchesGoldenHTML pins the exact page bytes. Refactors must not change
// them: every byte change rewrites the ConfigMap of every running Superpod.
func TestPageMatchesGoldenHTML(t *testing.T) {
	const head = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Superpod</title>
</head>
<body>
  <h1>superpod-example</h1>
  <p>My super ability is: &lt;b&gt;Flying&lt;/b&gt; &amp; &#34;more&#34;</p>
`
	for _, test := range []struct {
		name  string
		emoji []string
		want  string
	}{
		{"plain", nil, head + "</body>\n</html>\n"},
		{"two emoji", []string{"🦅", "🪽"}, head + `  <div aria-label="Ability emoji" style="display: inline-grid; ` +
			`grid-template-columns: repeat(2, auto); gap: 0.5rem; font-size: 3rem"><span>🦅</span><span>🪽</span></div>
</body>
</html>
`},
	} {
		t.Run(test.name, func(t *testing.T) {
			sp := exampleSuperpod()
			sp.Spec.SuperAbility = `<b>Flying</b> & "more"`
			if got := resources.NewConfigMap(sp, test.emoji...).Data["index.html"]; got != test.want {
				t.Errorf("page changed:\n--- got\n%s\n--- want\n%s", got, test.want)
			}
		})
	}
}
```

- [ ] **Step 2: Append the cache-key test to `internal/emoji/jev_test.go`**

```go
// TestCacheKeyIsStable pins the key that persisted selections are stored under.
// Changing it makes every Superpod with emoji pay for a new selection, so change
// it only on purpose, together with this test.
func TestCacheKeyIsStable(t *testing.T) {
	want := "api.typesafe.ai/v3/parallel255-top3-layout124-gap0.05/jev-latest/" + catalogHash()
	if got := NewClient(testAPIKey, "").CacheKey(); got != want {
		t.Fatalf("CacheKey() = %q, want %q", got, want)
	}
}
```

- [ ] **Step 3: Extend the controller tests**

In `internal/controller/superpod_controller_test.go`, add `"crypto/sha256"` and `"fmt"` to the standard-library import group:

```go
import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"
```

Replace the first two lines of the `DescribeTable("refuses to adopt a ConfigMap belonging to someone else", ...)` block with the new spec plus the table's new opening. The old lines are:

```go
	DescribeTable("refuses to adopt a ConfigMap belonging to someone else", func(foreignOwner bool) {
		cm := resources.NewConfigMap(sp)
```

Replace them with:

```go
	It("reuses an emoji selection persisted by an earlier operator version", func() {
		selector := &testEmojiSelector{}
		reconciler.EmojiSelector = selector
		sp.Spec.Emoji = true
		Expect(k8sClient.Update(ctx, sp)).To(Succeed())
		// This literal is the stored format; the operator must keep reading it.
		key := fmt.Sprintf("%x", sha256.Sum256([]byte(selector.CacheKey()+"\x00"+sp.Spec.SuperAbility)))
		cm := resources.NewConfigMap(sp)
		Expect(controllerutil.SetControllerReference(sp, cm, k8sClient.Scheme())).To(Succeed())
		cm.Data["emoji-cache.json"] = `{"key":"` + key + `","matches":[{"emoji":"🦊","name":"fox","score":0.9}]}`
		Expect(k8sClient.Create(ctx, cm)).To(Succeed())
		reconcileOnce()
		Expect(selector.calls).To(BeZero())
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cm), cm)).To(Succeed())
		Expect(cm.Data["index.html"]).To(ContainSubstring("🦊"))
	})

	DescribeTable("refuses to adopt a ConfigMap belonging to someone else", func(foreignOwner bool) {
		selector := &testEmojiSelector{}
		reconciler.EmojiSelector = selector
		sp.Spec.Emoji = true
		Expect(k8sClient.Update(ctx, sp)).To(Succeed())
		cm := resources.NewConfigMap(sp)
```

At the end of the same table, add the zero-calls assertion (the middle line below) between the existing `ReconcileFailed` assertion and the `Entry` line:

```go
		Expect(meta.FindStatusCondition(sp.Status.Conditions, "Ready").Reason).To(Equal("ReconcileFailed"))
		Expect(selector.calls).To(BeZero(), "a name collision must not spend a paid emoji selection")
	}, Entry("unowned resource", false), Entry("another owner", true))
```

- [ ] **Step 4: Run the tests. They pass, because they describe current behavior.**

Run: `go test ./internal/resources ./internal/emoji && make test`
Expected: `ok` for every package.

- [ ] **Step 5: Prove the golden test catches drift, then undo**

Temporarily change `My super ability is:` to `My ability is:` in `internal/resources/configmap.go`.
Run: `go test ./internal/resources -run TestPageMatchesGoldenHTML`
Expected: `FAIL` with `page changed:`. Then revert with `git checkout internal/resources/configmap.go`.

- [ ] **Step 6: Commit**

```bash
git add internal/resources/resources_test.go internal/emoji/jev_test.go internal/controller/superpod_controller_test.go
git commit -m "test: pin page bytes, emoji cache key, and stored selection format" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 2: Enforce the Go style guides with golangci-lint

This task automates Effective Go, Google style, and the CNCF conventions:
- `revive` checks doc comments, naming, error strings, and dot imports.
- `importas` enforces the Kubernetes import aliases that Cluster API also enforces.
- `errorlint` checks error wrapping and comparison.
- `nolintlint` catches broken suppression directives.
- `goimports` with `local-prefixes` keeps the project's own imports in a separate group.

It also fixes a GNU make bug. Because `$(LOCALBIN)` is a normal prerequisite, every new file in `bin/` makes `make lint` re-download and rebuild golangci-lint.

**Files:**
- Modify: `.golangci.yml` (replace the whole file)
- Modify: `.custom-gcl.yml` (pin `logcheck`)
- Modify: `Makefile` (golangci-lint and envtest prerequisites)
- Modify: `cmd/main.go` (package comment; delete the unused `// nolint:gocyclo`)
- Modify: `test/utils/utils.go` (package comment; fix three `nolint` directives)
- Create: `internal/controller/doc.go`

**Interfaces:** none (tooling only).

- [ ] **Step 1: Replace `.golangci.yml`**

```yaml
version: "2"
run:
  allow-parallel-runners: true
linters:
  default: none
  enable:
    - copyloopvar
    - depguard
    - dupl
    - errcheck
    - errorlint
    - ginkgolinter
    - goconst
    - gocyclo
    - govet
    - importas
    - ineffassign
    - lll
    - logcheck
    - misspell
    - modernize
    - nakedret
    - nolintlint
    - prealloc
    - revive
    - staticcheck
    - unconvert
    - unparam
    - unused
  settings:
    custom:
      logcheck:
        type: "module"
        description: Checks Go logging calls for Kubernetes logging conventions.
    depguard:
      rules:
        forbid-sort-pkg:
          deny:
            - pkg: sort
              desc: Should be replaced with slices package
    importas:
      no-unaliased: true
      alias:
        - pkg: k8s.io/api/core/v1
          alias: corev1
        - pkg: k8s.io/api/networking/v1
          alias: networkingv1
        - pkg: k8s.io/apimachinery/pkg/apis/meta/v1
          alias: metav1
        - pkg: k8s.io/apimachinery/pkg/api/errors
          alias: apierrors
        - pkg: sigs.k8s.io/controller-runtime
          alias: ctrl
        - pkg: github.com/jairjosafath/operator/api/v1
          alias: superv1
    revive:
      rules:
        - name: blank-imports
        - name: comment-spacings
        - name: context-as-argument
        - name: context-keys-type
        - name: dot-imports
          arguments:
            - allowed-packages:
                - github.com/onsi/ginkgo/v2
                - github.com/onsi/gomega
        - name: error-naming
        - name: error-return
        - name: error-strings
        - name: errorf
        - name: exported
        - name: import-shadowing
        - name: increment-decrement
        - name: indent-error-flow
        - name: package-comments
        - name: range
        - name: receiver-naming
        - name: redefines-builtin-id
        - name: superfluous-else
        - name: time-naming
        - name: unexported-return
        - name: unreachable-code
        - name: var-declaration
        - name: var-naming
    modernize:
      disable:
        - omitzero
  exclusions:
    generated: lax
    rules:
      - linters:
          - lll
        path: api/*
      - linters:
          - dupl
          - lll
        path: internal/*
    paths:
      - third_party$
      - builtin$
      - examples$
formatters:
  enable:
    - gofmt
    - goimports
  settings:
    goimports:
      local-prefixes:
        - github.com/jairjosafath/operator
  exclusions:
    generated: lax
    paths:
      - third_party$
      - builtin$
      - examples$
```

- [ ] **Step 2: Pin the logcheck plugin in `.custom-gcl.yml`**

Change `    version: latest` to:

```yaml
    version: v0.10.1
```

- [ ] **Step 3: Make the tool prerequisites correct in `Makefile`**

Replace `$(ENVTEST): $(LOCALBIN)` with:

```make
$(ENVTEST): | $(LOCALBIN)
```

Replace `$(GOLANGCI_LINT): $(LOCALBIN)` with:

```make
# Rebuild when the plugin list or this Makefile (and its version pins) changes.
# $(LOCALBIN) is order-only: new files in bin/ must not trigger a rebuild.
$(GOLANGCI_LINT): .custom-gcl.yml Makefile | $(LOCALBIN)
```

- [ ] **Step 4: Run lint to see the findings**

Run: `make lint`
Expected: first `Building custom golangci-lint with plugins...` (the plugin pin changed), then 7 issues:
- `nolintlint`: `cmd/main.go:40` and `test/utils/utils.go` lines 11, 164, and 204. Each directive "should be written without leading space". With the space, golangci-lint ignores these directives.
- `revive` `package-comments`: `cmd/main.go`, `internal/controller`, and `test/utils`.

- [ ] **Step 5: Fix `cmd/main.go`**

Put the command's doc comment directly above `package main`:

```go
// Manager runs the Superpod operator. It registers the Superpod controller with a
// controller-runtime manager and serves health probes and metrics.
package main
```

Delete the line `// nolint:gocyclo` above `func main()`. With the space removed it would be reported as unused, because `main` is below the `gocyclo` threshold.

- [ ] **Step 6: Fix `test/utils/utils.go`**

Put this directly above `package utils`:

```go
// Package utils provides helpers for the end-to-end tests in test/e2e.
package utils
```

Replace `. "github.com/onsi/ginkgo/v2" // nolint:revive,staticcheck` with the line below. The `revive` half no longer suppresses anything, because `revive` now allows Ginkgo dot imports:

```go
	. "github.com/onsi/ginkgo/v2" //nolint:staticcheck // The Ginkgo DSL is designed for dot imports.
```

Replace both occurrences of `// nolint:gosec` with `//nolint:gosec`.

- [ ] **Step 7: Create `internal/controller/doc.go`**

```go
// Package controller contains the Kubebuilder-scaffolded Superpod reconciler.
// It reads a Superpod, applies its child resources, and reports status.
package controller
```

- [ ] **Step 8: Run lint and the tests**

Run: `make lint && make test`
Expected: `0 issues.` and `ok` for every package.

- [ ] **Step 9: Commit**

```bash
git add .golangci.yml .custom-gcl.yml Makefile cmd/main.go test/utils/utils.go internal/controller/doc.go
git commit -m "chore: enforce Go style guides and import conventions with golangci-lint" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 3: Define the Ready condition's reasons as API constants

The Kubernetes API conventions and Cluster API treat condition reasons as API. Today they are string literals spread across the controller. The envtest specs keep their literals on purpose, so that renaming a reason fails a test.

**Files:**
- Create: `api/v1/conditions.go`
- Modify: `internal/controller/superpod_controller.go`, `internal/controller/superpod_status.go`

**Interfaces:**
- Produces: `superv1.ReadyCondition`, `superv1.ResourcesReadyReason`, `superv1.PodNotReadyReason`, `superv1.IngressPendingReason`, `superv1.ResourceTerminatingReason`, `superv1.ReconcileFailedReason`, `superv1.ObservationFailedReason`, `superv1.EmojiSelectionFailedReason`, `superv1.EmojiRateLimitedReason` (all untyped `string` constants).

- [ ] **Step 1: Create `api/v1/conditions.go`**

```go
package v1

// ReadyCondition is the condition type in SuperpodStatus.Conditions. It is True
// when the nginx Pod is ready and the Ingress has an address; it does not verify
// browser DNS or connectivity.
const ReadyCondition = "Ready"

// Reasons for the Ready condition. Users and tools match on these values, so
// they are part of the API: add new reasons, but do not rename existing ones.
const (
	// ResourcesReadyReason means the nginx Pod is ready and the Ingress has an address.
	ResourcesReadyReason = "ResourcesReady"
	// PodNotReadyReason means the nginx Pod is missing, not running, or failing its readiness probe.
	PodNotReadyReason = "PodNotReady"
	// IngressPendingReason means the Ingress controller has not published an address yet.
	IngressPendingReason = "IngressPending"
	// ResourceTerminatingReason means a child resource must finish deleting before it is recreated.
	ResourceTerminatingReason = "ResourceTerminating"
	// ReconcileFailedReason means a child resource could not be built, created, or updated.
	ReconcileFailedReason = "ReconcileFailed"
	// ObservationFailedReason means the Pod or Ingress could not be read to judge readiness.
	ObservationFailedReason = "ObservationFailed"
	// EmojiSelectionFailedReason means emoji selection failed; the plain page is served meanwhile.
	EmojiSelectionFailedReason = "EmojiSelectionFailed"
	// EmojiRateLimitedReason means the emoji provider asked the operator to wait before retrying.
	EmojiRateLimitedReason = "EmojiRateLimited"
)
```

- [ ] **Step 2: Replace the literals in the controller**

In `internal/controller/superpod_controller.go`:

| Old | New |
| --- | --- |
| `Reason: "ReconcileFailed"` | `Reason: superv1.ReconcileFailedReason` |
| `Reason: "ResourceTerminating"` | `Reason: superv1.ResourceTerminatingReason` |
| `delay, reason := time.Minute, "EmojiSelectionFailed"` | `delay, reason := time.Minute, superv1.EmojiSelectionFailedReason` |
| `time.Until(rateLimit.RetryAt)), "EmojiRateLimited"` | `time.Until(rateLimit.RetryAt)), superv1.EmojiRateLimitedReason` |
| `Reason: "ObservationFailed"` | `Reason: superv1.ObservationFailedReason` |

In `internal/controller/superpod_status.go`:

| Old | New |
| --- | --- |
| `Reason: "PodNotReady"` | `Reason: superv1.PodNotReadyReason` |
| `Reason: "IngressPending"` | `Reason: superv1.IngressPendingReason` |
| `Reason: "ResourcesReady"` | `Reason: superv1.ResourcesReadyReason` |
| `condition.Type = "Ready"` | `condition.Type = superv1.ReadyCondition` |

- [ ] **Step 3: Confirm that nothing generated changes and the tests pass**

Run: `make manifests generate && git status --short config api && make lint test`
Expected: only `?? api/v1/conditions.go` is listed, then `0 issues.` and all tests `ok`.

- [ ] **Step 4: Commit**

```bash
git add api/v1/conditions.go internal/controller/superpod_controller.go internal/controller/superpod_status.go
git commit -m "refactor: define Ready condition reasons as API constants" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 4: Create `internal/webpage`, the business rules package

This task adds pure Go: what the page shows, and when emoji must be selected again. The package is not wired in yet (Task 5 does that), so this task adds tests and code without changing behavior.

**Files:**
- Create: `internal/webpage/doc.go`, `internal/webpage/render.go`, `internal/webpage/emoji.go`
- Test: `internal/webpage/render_test.go`, `internal/webpage/emoji_test.go`
- Modify: `go.mod` (`github.com/google/go-cmp` becomes a direct requirement)

**Interfaces:**
- Consumes: `emoji.Match`, `emoji.ValidMatches([]emoji.Match) bool` (existing).
- Produces:
  - `type Content struct { Title, Ability string; Emoji []string }`
  - `func Render(c Content) (string, error)`
  - `type EmojiSelector interface { Select(ctx context.Context, ability string) ([]emoji.Match, error); CacheKey() string }`
  - `type Selection struct { Key string \`json:"key"\`; Matches []emoji.Match \`json:"matches"\` }` and `func (s Selection) Symbols() []string`
  - `var ErrNoEmojiSelector error`
  - `func ChooseEmoji(ctx context.Context, s EmojiSelector, ability string, stored Selection) (Selection, error)`

- [ ] **Step 1: Write the failing render tests in `internal/webpage/render_test.go`**

```go
package webpage_test

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/jairjosafath/operator/internal/webpage"
)

const goldenHead = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Superpod</title>
</head>
<body>
  <h1>superpod-example</h1>
  <p>My super ability is: &lt;b&gt;Flying&lt;/b&gt; &amp; &#34;more&#34;</p>
`

// TestRenderMatchesGolden pins the page bytes. A byte change rewrites the
// ConfigMap of every running Superpod, so change the page only on purpose.
func TestRenderMatchesGolden(t *testing.T) {
	for _, test := range []struct {
		name  string
		emoji []string
		want  string
	}{
		{"plain", nil, goldenHead + "</body>\n</html>\n"},
		{"two emoji", []string{"🦅", "🪽"}, goldenHead + `  <div aria-label="Ability emoji" style="display: inline-grid; ` +
			`grid-template-columns: repeat(2, auto); gap: 0.5rem; font-size: 3rem"><span>🦅</span><span>🪽</span></div>
</body>
</html>
`},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := webpage.Render(webpage.Content{
				Title: "superpod-example", Ability: `<b>Flying</b> & "more"`, Emoji: test.emoji,
			})
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("Render() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestRenderEscapesAllText(t *testing.T) {
	got, err := webpage.Render(webpage.Content{
		Title:   "<i>title</i>",
		Ability: "<script>ability</script>",
		Emoji:   []string{"🦅", "<script>emoji</script>"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "<script>") || strings.Contains(got, "<i>") || !strings.Contains(got, "🦅") ||
		!strings.Contains(got, "&lt;script&gt;emoji&lt;/script&gt;") {
		t.Fatalf("the page must show Unicode and escape all text:\n%s", got)
	}
}

func TestRenderLayout(t *testing.T) {
	for _, test := range []struct {
		emoji   []string
		columns string
	}{
		{[]string{"🦅"}, "repeat(1, auto)"},
		{[]string{"🦅", "🪽"}, "repeat(2, auto)"},
		{[]string{"🦅", "🪽", "✈️", "🚀"}, "repeat(2, auto)"},
	} {
		got, err := webpage.Render(webpage.Content{Title: "t", Ability: "a", Emoji: test.emoji})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(got, test.columns) || strings.Count(got, "<span>") != len(test.emoji) {
			t.Fatalf("%d emoji need %s with one cell each:\n%s", len(test.emoji), test.columns, got)
		}
	}
}
```

- [ ] **Step 2: Write the failing selection tests in `internal/webpage/emoji_test.go`**

```go
package webpage_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/jairjosafath/operator/internal/emoji"
	"github.com/jairjosafath/operator/internal/webpage"
)

// fakeSelector counts Select calls, standing in for a paid provider.
type fakeSelector struct {
	cacheKey string
	matches  []emoji.Match
	err      error
	calls    int
}

func (f *fakeSelector) CacheKey() string { return f.cacheKey }

func (f *fakeSelector) Select(context.Context, string) ([]emoji.Match, error) {
	f.calls++
	return f.matches, f.err
}

const (
	testCacheKey = "test/"
	flying       = "Flying"
)

var eagle = []emoji.Match{{Emoji: "🦅", Name: "eagle", Score: 1}}

func TestChooseEmojiStoresTheKeyItWasMadeFor(t *testing.T) {
	s := &fakeSelector{cacheKey: testCacheKey, matches: eagle}
	got, err := webpage.ChooseEmoji(context.Background(), s, flying, webpage.Selection{})
	if err != nil {
		t.Fatal(err)
	}
	// sha256(testCacheKey + "\x00" + flying). Existing ConfigMaps store keys computed this way.
	want := webpage.Selection{Key: "4b8bb145034b92e52cf63e5dc6e2a2864dca2d08b417d813204b58bed999f6d6", Matches: eagle}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("ChooseEmoji() mismatch (-want +got):\n%s", diff)
	}
	if s.calls != 1 {
		t.Errorf("Select calls = %d, want 1", s.calls)
	}
}

func TestChooseEmojiReusesAValidStoredSelection(t *testing.T) {
	stored, err := webpage.ChooseEmoji(context.Background(), &fakeSelector{cacheKey: testCacheKey, matches: eagle}, flying, webpage.Selection{})
	if err != nil {
		t.Fatal(err)
	}
	// The selector fails, as it does when the operator has no API key; the
	// stored selection must still be served without calling it.
	s := &fakeSelector{cacheKey: testCacheKey, err: errors.New("no API key")}
	got, err := webpage.ChooseEmoji(context.Background(), s, flying, stored)
	if err != nil || s.calls != 0 {
		t.Fatalf("ChooseEmoji() = %v, calls = %d; want the stored selection without calls", err, s.calls)
	}
	if diff := cmp.Diff(stored, got); diff != "" {
		t.Errorf("ChooseEmoji() mismatch (-want +got):\n%s", diff)
	}
}

func TestChooseEmojiSelectsAgainWhenStoredSelectionIsStale(t *testing.T) {
	stored, err := webpage.ChooseEmoji(context.Background(), &fakeSelector{cacheKey: testCacheKey, matches: eagle}, flying, webpage.Selection{})
	if err != nil {
		t.Fatal(err)
	}
	tampered := webpage.Selection{Key: stored.Key, Matches: []emoji.Match{{Emoji: "🦅", Name: "not the catalog name", Score: 1}}}
	for _, test := range []struct {
		name     string
		cacheKey string
		ability  string
		stored   webpage.Selection
	}{
		{"ability changed", testCacheKey, "Invisibility", stored},
		{"selector changed", testCacheKey + "other-model", flying, stored},
		{"stored matches invalid", testCacheKey, flying, tampered},
		{"nothing stored", testCacheKey, flying, webpage.Selection{}},
	} {
		t.Run(test.name, func(t *testing.T) {
			s := &fakeSelector{cacheKey: test.cacheKey, matches: eagle}
			if _, err := webpage.ChooseEmoji(context.Background(), s, test.ability, test.stored); err != nil {
				t.Fatal(err)
			}
			if s.calls != 1 {
				t.Errorf("Select calls = %d, want 1", s.calls)
			}
		})
	}
}

func TestChooseEmojiErrors(t *testing.T) {
	if _, err := webpage.ChooseEmoji(context.Background(), nil, flying, webpage.Selection{}); !errors.Is(err, webpage.ErrNoEmojiSelector) {
		t.Errorf("nil selector: err = %v, want ErrNoEmojiSelector", err)
	}
	unavailable := errors.New("unavailable")
	if _, err := webpage.ChooseEmoji(context.Background(), &fakeSelector{err: unavailable}, flying, webpage.Selection{}); !errors.Is(err, unavailable) {
		t.Errorf("failing selector: err = %v, want %v", err, unavailable)
	}
	for _, matches := range [][]emoji.Match{nil, {{Emoji: "not an emoji", Name: "x", Score: 1}}} {
		if _, err := webpage.ChooseEmoji(context.Background(), &fakeSelector{matches: matches}, flying, webpage.Selection{}); err == nil {
			t.Errorf("accepted invalid matches %v", matches)
		}
	}
}

func TestSelectionSymbols(t *testing.T) {
	sel := webpage.Selection{Matches: []emoji.Match{{Emoji: "🦅"}, {Emoji: "🪽"}}}
	if diff := cmp.Diff([]string{"🦅", "🪽"}, sel.Symbols()); diff != "" {
		t.Errorf("Symbols() mismatch (-want +got):\n%s", diff)
	}
}
```

- [ ] **Step 3: Run the tests to see them fail**

Run: `go test ./internal/webpage/`
Expected: FAIL. There are no non-test Go files in the package yet, or `undefined: webpage.Render`.

- [ ] **Step 4: Create `internal/webpage/doc.go`**

```go
// Package webpage holds the Superpod business rules: what the webpage shows and
// when the emoji on it must be selected again.
//
// It is plain Go with no Kubernetes, controller-runtime, or HTTP dependencies.
// internal/resources stores its output in Kubernetes objects and
// internal/typesafe implements its EmojiSelector port, so these rules can be read
// and tested without a cluster or network. See docs/architecture.md.
package webpage
```

- [ ] **Step 5: Create `internal/webpage/render.go`**

```go
package webpage

import (
	"fmt"
	"html/template"
	"strings"
)

// Content is what the webpage shows.
type Content struct {
	// Title is the page heading, normally the Superpod name.
	Title string
	// Ability is the super ability text.
	Ability string
	// Emoji are shown in order below the ability. The page has no emoji grid
	// when Emoji is empty.
	Emoji []string
}

// maxColumns places one emoji alone, two side by side, and four in a 2×2 square.
const maxColumns = 2

// html/template escapes each value for where it appears, so abilities and
// emoji are always shown as text, even when they contain markup.
var page = template.Must(template.New("index.html").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Superpod</title>
</head>
<body>
  <h1>{{.Title}}</h1>
  <p>My super ability is: {{.Ability}}</p>
{{- with .Emoji}}
  <div aria-label="Ability emoji" style="display: inline-grid; grid-template-columns: repeat({{$.Columns}}, auto); gap: 0.5rem; font-size: 3rem">
{{- range .}}<span>{{.}}</span>{{end -}}
  </div>
{{- end}}
</body>
</html>
`))

// Render returns the webpage's index.html.
func Render(c Content) (string, error) {
	var b strings.Builder
	view := struct {
		Content
		Columns int
	}{c, min(maxColumns, len(c.Emoji))}
	if err := page.Execute(&b, view); err != nil {
		return "", fmt.Errorf("render webpage: %w", err)
	}
	return b.String(), nil
}
```

- [ ] **Step 6: Create `internal/webpage/emoji.go`**

```go
package webpage

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"

	"github.com/jairjosafath/operator/internal/emoji"
)

// EmojiSelector chooses the emoji shown for an ability. Implementations usually
// call a paid external service, which is why ChooseEmoji reuses selections.
type EmojiSelector interface {
	// Select returns the emoji to show for ability, best first.
	Select(ctx context.Context, ability string) ([]emoji.Match, error)
	// CacheKey identifies everything besides the ability that affects what
	// Select returns, such as the provider, model, and catalog.
	CacheKey() string
}

// Selection is an emoji choice and the key it was made for. It is stored next
// to the webpage so later reconciles can reuse it.
type Selection struct {
	Key     string        `json:"key"`
	Matches []emoji.Match `json:"matches"`
}

// Symbols returns the selected emoji in display order.
func (s Selection) Symbols() []string {
	symbols := make([]string, 0, len(s.Matches))
	for _, m := range s.Matches {
		symbols = append(symbols, m.Emoji)
	}
	return symbols
}

// ErrNoEmojiSelector reports that emoji were requested but no selector is configured.
var ErrNoEmojiSelector = errors.New("no emoji selector is configured")

// ChooseEmoji returns stored when it is valid and was made for ability by an
// equivalent selector. Otherwise it asks s for a new selection. Each Superpod
// therefore pays for one selection per ability and selector configuration.
func ChooseEmoji(ctx context.Context, s EmojiSelector, ability string, stored Selection) (Selection, error) {
	if s == nil {
		return Selection{}, ErrNoEmojiSelector
	}
	key := selectionKey(s.CacheKey(), ability)
	if stored.Key == key && emoji.ValidMatches(stored.Matches) {
		return stored, nil
	}
	matches, err := s.Select(ctx, ability)
	if err != nil {
		return Selection{}, err
	}
	if !emoji.ValidMatches(matches) {
		return Selection{}, errors.New("emoji selector returned invalid matches")
	}
	return Selection{Key: key, Matches: matches}, nil
}

// selectionKey is persisted in every Superpod's ConfigMap. Changing how it is
// computed makes each Superpod pay for a new selection.
func selectionKey(cacheKey, ability string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(cacheKey+"\x00"+ability)))
}
```

- [ ] **Step 7: Promote go-cmp and run the tests**

Run: `go mod tidy && git diff --stat go.mod go.sum && go test ./internal/webpage/`
Expected: `go.mod | 2 +-` (go-cmp moves from the indirect block to the direct block) and `ok  github.com/jairjosafath/operator/internal/webpage`.

- [ ] **Step 8: Lint**

Run: `make lint`
Expected: `0 issues.`

- [ ] **Step 9: Commit**

```bash
git add internal/webpage go.mod
git commit -m "feat: add webpage package for Superpod business rules" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 5: Route the page through `webpage`; make the controller thin

`internal/resources` now stores what `internal/webpage` decides. `internal/controller` only fetches, applies, and reports. Task 1's golden strings and fixtures do not change, which proves the rewiring kept behavior.

**Files:**
- Modify: `internal/resources/configmap.go` (replace), `internal/resources/metadata.go` (add `ObjectKey`)
- Modify: `internal/resources/resources_test.go` (replace)
- Modify: `internal/controller/superpod_controller.go` (replace), `internal/controller/superpod_emoji.go` (replace), `internal/controller/doc.go` (replace)
- Modify: `internal/controller/superpod_controller_test.go` (helper plus two substitutions)

**Interfaces:**
- Consumes (Task 4): `webpage.Content`, `webpage.Render`, `webpage.Selection`, `(webpage.Selection).Symbols`, `webpage.EmojiSelector`, `webpage.ChooseEmoji`. Consumes (Task 3): the `superv1.*Reason` constants.
- Produces:
  - `func resources.NewConfigMap(sp *superv1.Superpod, sel *webpage.Selection) (*corev1.ConfigMap, error)`. This replaces the variadic `emojis ...string` form.
  - `func resources.StoredSelection(cm *corev1.ConfigMap) webpage.Selection`
  - `func resources.ObjectKey(sp *superv1.Superpod) types.NamespacedName`
  - `SuperpodReconciler.EmojiSelector` now has type `webpage.EmojiSelector`, and the `controller.EmojiSelector` interface is removed.
  - In package `controller`: `func getIfExists[T client.Object](ctx context.Context, c client.Reader, key client.ObjectKey, obj T) (T, error)` and `func (r *SuperpodReconciler) fail(ctx context.Context, sp *superv1.Superpod, err error) (ctrl.Result, error)`

- [ ] **Step 1: Replace `internal/resources/resources_test.go` to use the new API**

The golden `want` strings are unchanged from Task 1. The escaping and layout tests move to `internal/webpage` (Task 4), so they are dropped here.

```go
package resources_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/validation"

	superv1 "github.com/jairjosafath/operator/api/v1"
	"github.com/jairjosafath/operator/internal/emoji"
	"github.com/jairjosafath/operator/internal/resources"
	"github.com/jairjosafath/operator/internal/webpage"
)

func exampleSuperpod() *superv1.Superpod {
	return &superv1.Superpod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "superpod-example",
			Namespace: "default",
			UID:       "3aa43bd9-d1d3-42b8-99cf-b87f95c335c0",
		},
		Spec: superv1.SuperpodSpec{
			SuperAbility: "Flying",
			Host:         "superpod.example.test",
		},
	}
}

func newConfigMap(t *testing.T, sp *superv1.Superpod, sel *webpage.Selection) *corev1.ConfigMap {
	t.Helper()
	cm, err := resources.NewConfigMap(sp, sel)
	if err != nil {
		t.Fatal(err)
	}
	return cm
}

func selectionOf(symbols ...string) *webpage.Selection {
	sel := &webpage.Selection{Key: "test-key"}
	for _, symbol := range symbols {
		sel.Matches = append(sel.Matches, emoji.Match{Emoji: symbol})
	}
	return sel
}

func TestResourceConnections(t *testing.T) {
	sp := exampleSuperpod()
	pod := resources.NewPod(sp)
	cm := newConfigMap(t, sp, nil)
	sa := resources.NewServiceAccount(sp)
	service := resources.NewService(sp)
	ingress := resources.NewIngress(sp)

	volume := pod.Spec.Volumes[0]
	mount := pod.Spec.Containers[0].VolumeMounts[0]
	if volume.ConfigMap.Name != cm.Name || mount.Name != volume.Name || cm.Data["index.html"] == "" {
		t.Fatal("the nginx volume must connect to the HTML ConfigMap")
	}
	if !mount.ReadOnly || mount.SubPath != "" {
		t.Fatal("the HTML directory must be read-only and allow projected updates")
	}
	if pod.Spec.ServiceAccountName != sa.Name {
		t.Fatal("the Pod must use its dedicated ServiceAccount")
	}
	if service.Spec.Type != corev1.ServiceTypeClusterIP {
		t.Fatal("the Ingress backend must use an internal Service")
	}
	selector := labels.SelectorFromSet(service.Spec.Selector)
	if !selector.Matches(labels.Set(pod.Labels)) {
		t.Fatal("the Service must select its Pod")
	}
	other := sp.DeepCopy()
	other.UID = "7a2d05b0-b1a7-4149-a5d7-fd6f07231f24"
	if selector.Matches(labels.Set(resources.NewPod(other).Labels)) {
		t.Fatal("the Service must not select a different Superpod's Pod")
	}
	backend := ingress.Spec.Rules[0].HTTP.Paths[0].Backend.Service
	if backend.Name != service.Name || backend.Port.Name != service.Spec.Ports[0].Name {
		t.Fatal("the Ingress must route to the Service port")
	}
	if service.Spec.Ports[0].TargetPort.StrVal != pod.Spec.Containers[0].Ports[0].Name {
		t.Fatal("the Service must route to the nginx container port")
	}
	if ingress.Spec.IngressClassName != nil {
		t.Fatal("an unspecified IngressClass must remain unset for cluster defaulting")
	}
	sp.Spec.IngressClassName = "example-class"
	if got := resources.NewIngress(sp).Spec.IngressClassName; got == nil || *got != sp.Spec.IngressClassName {
		t.Fatal("an explicit IngressClass must be preserved")
	}
}

func TestAbilityUpdateOnlyChangesHTML(t *testing.T) {
	sp := exampleSuperpod()
	before := sp.DeepCopy()
	first := newConfigMap(t, sp, nil)
	pod := resources.NewPod(sp)
	if !reflect.DeepEqual(first, newConfigMap(t, sp, nil)) || !reflect.DeepEqual(sp, before) {
		t.Fatal("building resources must be repeatable without modifying the Superpod")
	}

	sp.Spec.SuperAbility = "<script>alert('Flying')</script> & invisibility"
	updated := newConfigMap(t, sp, nil)
	page := updated.Data["index.html"]
	if page == first.Data["index.html"] || strings.Contains(page, "<script>") ||
		!strings.Contains(page, "&lt;script&gt;") || !strings.Contains(page, "&amp; invisibility") {
		t.Fatal("changing an ability must update the page and escape HTML input")
	}
	if updated.Name != first.Name || !reflect.DeepEqual(pod, resources.NewPod(sp)) {
		t.Fatal("changing an ability must keep resource identity and the Pod definition stable")
	}
}

func TestLongSuperpodNameProducesValidServiceName(t *testing.T) {
	sp := exampleSuperpod()
	sp.Name = strings.Repeat("long-name.", 20) + "example"
	name := resources.NewService(sp).Name
	if problems := validation.IsDNS1035Label(name); len(problems) != 0 {
		t.Fatalf("resource name %q is invalid: %v", name, problems)
	}
}

// TestPageMatchesGoldenHTML pins the exact page bytes. Refactors must not change
// them: every byte change rewrites the ConfigMap of every running Superpod.
func TestPageMatchesGoldenHTML(t *testing.T) {
	const head = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Superpod</title>
</head>
<body>
  <h1>superpod-example</h1>
  <p>My super ability is: &lt;b&gt;Flying&lt;/b&gt; &amp; &#34;more&#34;</p>
`
	for _, test := range []struct {
		name string
		sel  *webpage.Selection
		want string
	}{
		{"plain", nil, head + "</body>\n</html>\n"},
		{"two emoji", selectionOf("🦅", "🪽"), head + `  <div aria-label="Ability emoji" style="display: inline-grid; ` +
			`grid-template-columns: repeat(2, auto); gap: 0.5rem; font-size: 3rem"><span>🦅</span><span>🪽</span></div>
</body>
</html>
`},
	} {
		t.Run(test.name, func(t *testing.T) {
			sp := exampleSuperpod()
			sp.Spec.SuperAbility = `<b>Flying</b> & "more"`
			if got := newConfigMap(t, sp, test.sel).Data["index.html"]; got != test.want {
				t.Errorf("page changed:\n--- got\n%s\n--- want\n%s", got, test.want)
			}
		})
	}
}

func TestStoredSelectionRoundTrip(t *testing.T) {
	sel := &webpage.Selection{Key: "test-key", Matches: []emoji.Match{{Emoji: "🦅", Name: "eagle", Score: 1}}}
	cm := newConfigMap(t, exampleSuperpod(), sel)
	if diff := cmp.Diff(*sel, resources.StoredSelection(cm)); diff != "" {
		t.Errorf("StoredSelection() mismatch (-want +got):\n%s", diff)
	}
	if got := resources.StoredSelection(newConfigMap(t, exampleSuperpod(), nil)); got.Key != "" {
		t.Errorf("a plain page must not store a selection, got %+v", got)
	}
	if got := resources.StoredSelection(nil); got.Key != "" {
		t.Errorf("a missing ConfigMap must yield no selection, got %+v", got)
	}
	cm.Data["emoji-cache.json"] = "{corrupt"
	if got := resources.StoredSelection(cm); got.Key != "" {
		t.Errorf("a corrupt selection must be ignored, got %+v", got)
	}
}

func TestObjectKeyNamesEveryChild(t *testing.T) {
	sp := exampleSuperpod()
	key := resources.ObjectKey(sp)
	for _, child := range []metav1.Object{
		resources.NewServiceAccount(sp), newConfigMap(t, sp, nil), resources.NewPod(sp),
		resources.NewService(sp), resources.NewIngress(sp),
	} {
		if child.GetNamespace() != key.Namespace || child.GetName() != key.Name {
			t.Errorf("%T is named %s/%s, want %s", child, child.GetNamespace(), child.GetName(), key)
		}
	}
}
```

- [ ] **Step 2: Run the tests to see them fail**

Run: `go test ./internal/resources/`
Expected: build failure, with errors including `undefined: resources.StoredSelection` and `undefined: resources.ObjectKey`.

- [ ] **Step 3: Replace `internal/resources/configmap.go`**

```go
package resources

import (
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"

	superv1 "github.com/jairjosafath/operator/api/v1"
	"github.com/jairjosafath/operator/internal/webpage"
)

// ConfigMap keys. nginx serves pageKey; selectionKey keeps the emoji selection
// behind the page so later reconciles can rebuild it without a provider call.
const (
	pageKey      = "index.html"
	selectionKey = "emoji-cache.json"
)

// NewConfigMap builds the ConfigMap that nginx serves. When sel is non-nil, its
// emoji are shown on the page and sel is stored for later reconciles.
func NewConfigMap(sp *superv1.Superpod, sel *webpage.Selection) (*corev1.ConfigMap, error) {
	content := webpage.Content{Title: sp.Name, Ability: sp.Spec.SuperAbility}
	if sel != nil {
		content.Emoji = sel.Symbols()
	}
	page, err := webpage.Render(content)
	if err != nil {
		return nil, err
	}
	cm := &corev1.ConfigMap{
		ObjectMeta: metadata(sp),
		Data:       map[string]string{pageKey: page},
	}
	if sel != nil {
		data, err := json.Marshal(sel)
		if err != nil {
			return nil, fmt.Errorf("encode emoji selection: %w", err)
		}
		cm.Data[selectionKey] = string(data)
	}
	return cm, nil
}

// StoredSelection returns the emoji selection kept in cm. It returns the zero
// Selection when cm is nil or holds no readable selection, which makes
// webpage.ChooseEmoji select again.
func StoredSelection(cm *corev1.ConfigMap) webpage.Selection {
	var sel webpage.Selection
	if cm == nil || json.Unmarshal([]byte(cm.Data[selectionKey]), &sel) != nil {
		return webpage.Selection{}
	}
	return sel
}
```

- [ ] **Step 4: Add `ObjectKey` to `internal/resources/metadata.go`**

Add `"k8s.io/apimachinery/pkg/types"` to the Kubernetes import group. Then replace the comment line `// Different resource kinds may share a name. The UID keeps this name stable` with:

```go
// ObjectKey returns the namespace and name that all of sp's children share.
func ObjectKey(sp *superv1.Superpod) types.NamespacedName {
	return types.NamespacedName{Namespace: sp.Namespace, Name: resourceName(sp)}
}

// resourceName is shared by children of different kinds. The UID keeps it stable
```

- [ ] **Step 5: Run the resources tests**

Run: `go test ./internal/resources/`
Expected: `ok`. `go build ./...` still fails in `internal/controller` until Step 8.

- [ ] **Step 6: Replace `internal/controller/superpod_controller.go`**

```go
package controller

import (
	"context"
	"errors"
	"time"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	superv1 "github.com/jairjosafath/operator/api/v1"
	"github.com/jairjosafath/operator/internal/emoji"
	"github.com/jairjosafath/operator/internal/resources"
	"github.com/jairjosafath/operator/internal/webpage"
)

// SuperpodReconciler reconciles a Superpod object.
type SuperpodReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	EmojiSelector webpage.EmojiSelector
}

// +kubebuilder:rbac:groups=super.elp-max.com,resources=superpods,verbs=get;list;watch
// +kubebuilder:rbac:groups=super.elp-max.com,resources=superpods/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=super.elp-max.com,resources=superpods/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=pods;configmaps;serviceaccounts;services,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch

// Reconcile can run many times for the same Superpod. Each pass compares the
// desired resources with the cluster, rather than treating status as a flag
// meaning that creation is permanently finished.
func (r *SuperpodReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	sp := &superv1.Superpod{}
	if err := r.Get(ctx, req.NamespacedName, sp); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if !sp.DeletionTimestamp.IsZero() {
		// Kubernetes garbage collection deletes children through owner references.
		// Do not recreate resources while their parent is being deleted.
		return ctrl.Result{}, nil
	}

	// A failed emoji selection must not take the webpage down: the plain page
	// is applied first and the failure is reported afterwards.
	sel, emojiErr := r.chooseEmoji(ctx, sp)
	page, err := resources.NewConfigMap(sp, sel)
	if err != nil {
		return r.fail(ctx, sp, err)
	}
	pod := resources.NewPod(sp)
	children := []client.Object{
		resources.NewServiceAccount(sp),
		page,
		pod,
		resources.NewService(sp),
		resources.NewIngress(sp),
	}
	for _, child := range children {
		terminating, err := resources.Ensure(ctx, r.Client, r.Scheme, sp, child)
		if err != nil {
			return r.fail(ctx, sp, err)
		}
		if terminating {
			// A name cannot be reused until deletion finishes. Watches normally
			// wake us up; this short retry also covers a missed deletion event.
			return ctrl.Result{RequeueAfter: time.Second}, r.updateStatus(ctx, sp, sp.Status.PodName, metav1.Condition{
				Status: metav1.ConditionFalse, Reason: superv1.ResourceTerminatingReason,
				Message: "Waiting for a child resource to finish deletion before recreating it",
			})
		}
	}

	if emojiErr != nil {
		delay, reason := time.Minute, superv1.EmojiSelectionFailedReason
		if rateLimit, ok := errors.AsType[*emoji.RateLimitError](emojiErr); ok {
			delay, reason = max(time.Second, time.Until(rateLimit.RetryAt)), superv1.EmojiRateLimitedReason
		}
		return ctrl.Result{RequeueAfter: delay}, r.updateStatus(ctx, sp, pod.Name, metav1.Condition{
			Status: metav1.ConditionFalse, Reason: reason, Message: emojiErr.Error(),
		})
	}

	condition, err := r.readiness(ctx, sp)
	if err != nil {
		statusErr := r.updateStatus(ctx, sp, pod.Name, metav1.Condition{
			Status: metav1.ConditionUnknown, Reason: superv1.ObservationFailedReason, Message: err.Error(),
		})
		return ctrl.Result{}, errors.Join(err, statusErr)
	}
	return ctrl.Result{}, r.updateStatus(ctx, sp, pod.Name, condition)
}

// fail reports err in the Ready condition and returns it, so controller-runtime
// retries the request with backoff.
func (r *SuperpodReconciler) fail(ctx context.Context, sp *superv1.Superpod, err error) (ctrl.Result, error) {
	statusErr := r.updateStatus(ctx, sp, sp.Status.PodName, metav1.Condition{
		Status: metav1.ConditionFalse, Reason: superv1.ReconcileFailedReason, Message: err.Error(),
	})
	return ctrl.Result{}, errors.Join(err, statusErr)
}

// SetupWithManager watches the Superpod and every kind of child it owns.
func (r *SuperpodReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&superv1.Superpod{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.Pod{}).
		Owns(&corev1.Service{}).
		Owns(&networkingv1.Ingress{}).
		Named("superpod").
		Complete(r)
}

// getIfExists reads obj by key. It returns nil, without an error, when the
// object does not exist.
func getIfExists[T client.Object](ctx context.Context, c client.Reader, key client.ObjectKey, obj T) (T, error) {
	if err := c.Get(ctx, key, obj); err != nil {
		var none T
		return none, client.IgnoreNotFound(err)
	}
	return obj, nil
}
```

- [ ] **Step 7: Replace `internal/controller/superpod_emoji.go`**

```go
package controller

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	superv1 "github.com/jairjosafath/operator/api/v1"
	"github.com/jairjosafath/operator/internal/resources"
	"github.com/jairjosafath/operator/internal/webpage"
)

// chooseEmoji returns the emoji selection for sp's webpage, or nil for a plain
// page. The selection stored in the current ConfigMap is reused when still
// valid, because each new selection is a paid provider call.
func (r *SuperpodReconciler) chooseEmoji(ctx context.Context, sp *superv1.Superpod) (*webpage.Selection, error) {
	if !sp.Spec.Emoji {
		return nil, nil
	}
	current, err := getIfExists(ctx, r, resources.ObjectKey(sp), &corev1.ConfigMap{})
	if err != nil {
		return nil, err
	}
	if current != nil && (!metav1.IsControlledBy(current, sp) || !current.DeletionTimestamp.IsZero()) {
		// Ensure reports the name collision or waits for deletion. Do not pay
		// for a selection that cannot be stored yet.
		return nil, nil
	}
	sel, err := webpage.ChooseEmoji(ctx, r.EmojiSelector, sp.Spec.SuperAbility, resources.StoredSelection(current))
	if err != nil {
		return nil, err
	}
	return &sel, nil
}
```

- [ ] **Step 8: Replace `internal/controller/doc.go`**

```go
// Package controller contains the Kubebuilder-scaffolded Superpod reconciler.
//
// The reconciler is a thin adapter between controller-runtime and the rest of
// the operator: it reads the Superpod, asks internal/webpage and
// internal/resources what should exist, applies it, and reports status. Business
// rules belong in those packages, not here; see docs/architecture.md.
package controller
```

- [ ] **Step 9: Update the controller tests**

In `internal/controller/superpod_controller_test.go`, add this helper directly above `reconcileOnce := func() {`:

```go
	plainConfigMap := func() *corev1.ConfigMap {
		cm, err := resources.NewConfigMap(sp, nil)
		Expect(err).NotTo(HaveOccurred())
		return cm
	}
```

Then apply the two substitutions in this order. The first one must run first, because the second pattern is a substring of the first:

```bash
sed -i 's/client\.ObjectKeyFromObject(resources\.NewConfigMap(sp))/resources.ObjectKey(sp)/g; s/resources\.NewConfigMap(sp)/plainConfigMap()/g' internal/controller/superpod_controller_test.go
```

- [ ] **Step 10: Run everything**

Run: `make test && make lint`
Expected: all packages `ok`, including the Task 1 specs "reuses an emoji selection persisted by an earlier operator version" and "refuses to adopt a ConfigMap…" with zero selector calls. Then `0 issues.`

- [ ] **Step 11: Commit**

```bash
git add internal/resources internal/controller
git commit -m "refactor: route the webpage through the webpage package and slim the controller" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 6: Judge readiness in a pure function

Today, readiness is only tested through envtest. Moving the judgment into `resources.Readiness` makes it testable with a plain table test. The controller keeps only the two reads.

**Files:**
- Create: `internal/resources/readiness.go`, `internal/resources/doc.go`
- Test: `internal/resources/readiness_test.go`
- Modify: `internal/resources/metadata.go` (move the package comment to `doc.go`), `internal/controller/superpod_status.go`

**Interfaces:**
- Consumes (Task 5): `resources.ObjectKey`, `getIfExists`. Consumes (Task 3): `superv1.ReadyCondition` and the reasons.
- Produces: `func resources.Readiness(sp *superv1.Superpod, pod *corev1.Pod, ingress *networkingv1.Ingress) metav1.Condition`. A nil `pod` or `ingress` means that object does not exist.

- [ ] **Step 1: Write the failing test `internal/resources/readiness_test.go`**

```go
package resources_test

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	superv1 "github.com/jairjosafath/operator/api/v1"
	"github.com/jairjosafath/operator/internal/resources"
)

func TestReadiness(t *testing.T) {
	sp := exampleSuperpod()
	owner := []metav1.OwnerReference{*metav1.NewControllerRef(sp, superv1.GroupVersion.WithKind("Superpod"))}
	deleting := metav1.Now()

	// pod and ingress return ready, owned objects; each case changes one thing.
	pod := func(change func(*corev1.Pod)) *corev1.Pod {
		p := resources.NewPod(sp)
		p.OwnerReferences = owner
		p.Status.Phase = corev1.PodRunning
		p.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}
		change(p)
		return p
	}
	ingress := func(change func(*networkingv1.Ingress)) *networkingv1.Ingress {
		i := resources.NewIngress(sp)
		i.OwnerReferences = owner
		i.Status.LoadBalancer.Ingress = []networkingv1.IngressLoadBalancerIngress{{IP: "192.0.2.10"}}
		change(i)
		return i
	}

	for _, test := range []struct {
		name    string
		pod     *corev1.Pod
		ingress *networkingv1.Ingress
		status  metav1.ConditionStatus
		reason  string
	}{
		{"ready with IP", pod(func(*corev1.Pod) {}), ingress(func(*networkingv1.Ingress) {}),
			metav1.ConditionTrue, superv1.ResourcesReadyReason},
		{"ready with hostname", pod(func(*corev1.Pod) {}), ingress(func(i *networkingv1.Ingress) {
			i.Status.LoadBalancer.Ingress = []networkingv1.IngressLoadBalancerIngress{{Hostname: "ingress.example.test"}}
		}), metav1.ConditionTrue, superv1.ResourcesReadyReason},
		{"Pod missing", nil, ingress(func(*networkingv1.Ingress) {}),
			metav1.ConditionFalse, superv1.PodNotReadyReason},
		{"Pod owned by someone else", pod(func(p *corev1.Pod) { p.OwnerReferences = nil }), ingress(func(*networkingv1.Ingress) {}),
			metav1.ConditionFalse, superv1.PodNotReadyReason},
		{"Pod deleting", pod(func(p *corev1.Pod) { p.DeletionTimestamp = &deleting }), ingress(func(*networkingv1.Ingress) {}),
			metav1.ConditionFalse, superv1.PodNotReadyReason},
		{"Pod pending", pod(func(p *corev1.Pod) { p.Status.Phase = corev1.PodPending }), ingress(func(*networkingv1.Ingress) {}),
			metav1.ConditionFalse, superv1.PodNotReadyReason},
		{"Pod failing its probe", pod(func(p *corev1.Pod) { p.Status.Conditions[0].Status = corev1.ConditionFalse }),
			ingress(func(*networkingv1.Ingress) {}), metav1.ConditionFalse, superv1.PodNotReadyReason},
		{"Ingress missing", pod(func(*corev1.Pod) {}), nil,
			metav1.ConditionFalse, superv1.IngressPendingReason},
		{"Ingress owned by someone else", pod(func(*corev1.Pod) {}), ingress(func(i *networkingv1.Ingress) { i.OwnerReferences = nil }),
			metav1.ConditionFalse, superv1.IngressPendingReason},
		{"Ingress without address", pod(func(*corev1.Pod) {}), ingress(func(i *networkingv1.Ingress) {
			i.Status.LoadBalancer.Ingress = []networkingv1.IngressLoadBalancerIngress{{}}
		}), metav1.ConditionFalse, superv1.IngressPendingReason},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := resources.Readiness(sp, test.pod, test.ingress)
			if got.Type != superv1.ReadyCondition || got.Status != test.status || got.Reason != test.reason || got.Message == "" {
				t.Errorf("Readiness() = %s/%s/%s, want %s/%s/%s with a message",
					got.Type, got.Status, got.Reason, superv1.ReadyCondition, test.status, test.reason)
			}
		})
	}
}
```

- [ ] **Step 2: Run it to see it fail**

Run: `go test ./internal/resources/ -run TestReadiness`
Expected: build failure `undefined: resources.Readiness`.

- [ ] **Step 3: Create `internal/resources/readiness.go`**

```go
package resources

import (
	"slices"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	superv1 "github.com/jairjosafath/operator/api/v1"
)

// Readiness returns sp's Ready condition from its observed Pod and Ingress; nil
// means the object does not exist. It judges Kubernetes signals only: the Pod's
// readiness probe checks nginx and the Ingress controller publishes an address.
// Browser DNS and connectivity are not verified.
func Readiness(sp *superv1.Superpod, pod *corev1.Pod, ingress *networkingv1.Ingress) metav1.Condition {
	if pod == nil || !controlledAndLive(sp, pod) || pod.Status.Phase != corev1.PodRunning || !podReady(pod) {
		return metav1.Condition{
			Type: superv1.ReadyCondition, Status: metav1.ConditionFalse, Reason: superv1.PodNotReadyReason,
			Message: "Waiting for the nginx Pod to be running and pass its readiness probe",
		}
	}
	if ingress == nil || !controlledAndLive(sp, ingress) || !hasAddress(ingress) {
		return metav1.Condition{
			Type: superv1.ReadyCondition, Status: metav1.ConditionFalse, Reason: superv1.IngressPendingReason,
			Message: "Waiting for an Ingress address; check the Ingress controller, class, and address publishing configuration",
		}
	}
	return metav1.Condition{
		Type: superv1.ReadyCondition, Status: metav1.ConditionTrue, Reason: superv1.ResourcesReadyReason,
		Message: "The nginx Pod is ready and the Ingress has an address; browser DNS and connectivity must be configured separately",
	}
}

// controlledAndLive reports whether obj belongs to sp and is not being deleted.
func controlledAndLive(sp *superv1.Superpod, obj metav1.Object) bool {
	return metav1.IsControlledBy(obj, sp) && obj.GetDeletionTimestamp().IsZero()
}

func podReady(pod *corev1.Pod) bool {
	return slices.ContainsFunc(pod.Status.Conditions, func(c corev1.PodCondition) bool {
		return c.Type == corev1.PodReady && c.Status == corev1.ConditionTrue
	})
}

func hasAddress(ingress *networkingv1.Ingress) bool {
	return slices.ContainsFunc(ingress.Status.LoadBalancer.Ingress, func(a networkingv1.IngressLoadBalancerIngress) bool {
		return a.IP != "" || a.Hostname != ""
	})
}
```

- [ ] **Step 4: Move the package comment**

Delete these three lines above `package resources` in `internal/resources/metadata.go`:

```go
// Package resources builds the desired Kubernetes objects for a Superpod.
// Builders do not contact the API server. Pass a Superpod fetched from Kubernetes,
// so its namespace and server-assigned UID are available.
```

Create `internal/resources/doc.go`:

```go
// Package resources is the Kubernetes adapter for a Superpod. It translates
// between the Superpod API and Kubernetes objects:
//
//   - New* functions build the desired children. They do not contact the API
//     server; pass a Superpod read from Kubernetes so its namespace and UID are set.
//   - NewConfigMap and StoredSelection keep the internal/webpage page and emoji
//     selection in a ConfigMap.
//   - Readiness judges the observed Pod and Ingress.
//   - Ensure creates or updates one child through the API server.
//
// Business rules belong in internal/webpage; see docs/architecture.md.
package resources
```

- [ ] **Step 5: Replace `internal/controller/superpod_status.go`**

```go
package controller

import (
	"context"
	"errors"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	superv1 "github.com/jairjosafath/operator/api/v1"
	"github.com/jairjosafath/operator/internal/resources"
)

// readiness reads the Pod and Ingress and lets resources.Readiness judge them.
func (r *SuperpodReconciler) readiness(ctx context.Context, sp *superv1.Superpod) (metav1.Condition, error) {
	key := resources.ObjectKey(sp)
	pod, err := getIfExists(ctx, r, key, &corev1.Pod{})
	if err != nil {
		return metav1.Condition{}, err
	}
	ingress, err := getIfExists(ctx, r, key, &networkingv1.Ingress{})
	if err != nil {
		return metav1.Condition{}, err
	}
	return resources.Readiness(sp, pod, ingress), nil
}

// updateStatus re-fetches the Superpod and only writes when its status changes.
// A condition's observedGeneration ties the report to the spec we reconciled.
func (r *SuperpodReconciler) updateStatus(ctx context.Context, observed *superv1.Superpod, podName string, condition metav1.Condition) error {
	current := &superv1.Superpod{}

	if err := r.Get(ctx, client.ObjectKeyFromObject(observed), current); err != nil {
		return client.IgnoreNotFound(err)
	}

	if !current.DeletionTimestamp.IsZero() {
		return nil
	}

	if current.UID != observed.UID || current.Generation != observed.Generation {
		return apierrors.NewConflict(superv1.SchemeGroupVersion.WithResource("superpods").GroupResource(),
			current.Name, errors.New("superpod changed during reconciliation; retrying with its latest spec"))
	}

	before := current.DeepCopy()
	current.Status.PodName = podName
	current.Status.URL = "http://" + observed.Spec.Host
	condition.Type = superv1.ReadyCondition
	condition.ObservedGeneration = observed.Generation
	// SetStatusCondition preserves LastTransitionTime when the status is unchanged.
	meta.SetStatusCondition(&current.Status.Conditions, condition)
	if equality.Semantic.DeepEqual(before.Status, current.Status) {
		return nil
	}

	return r.Status().Update(ctx, current)
}
```

- [ ] **Step 6: Run everything**

Run: `go test ./internal/resources/ -run TestReadiness && make test && make lint`
Expected: `ok` for the readiness test, then all packages `ok` (including the envtest spec "reports ObservationFailed when the readiness check cannot read the Pod"), then `0 issues.`

- [ ] **Step 7: Commit**

```bash
git add internal/resources internal/controller/superpod_status.go
git commit -m "refactor: judge Superpod readiness in a pure function" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 7: Split the TypeSafe adapter from the emoji vocabulary

Git history shows the reason for this split. The provider switch from jevai.org to TypeSafe touched five files in `internal/emoji`, and the layout change from "up to 3" to "1, 2, or 4" touched the HTTP client file. After this task, a provider change touches only `internal/typesafe`, and a layout change touches only `internal/emoji`.

**Files:**
- Test: `internal/emoji/pick_test.go` (new)
- Create: `internal/emoji/pick.go`, `internal/emoji/ratelimit.go`, `internal/typesafe/doc.go`
- Modify: `internal/emoji/catalog.go` (replace)
- Move with `git mv`: `internal/emoji/jev.go` → `internal/typesafe/client.go`, `jev_test.go` → `client_test.go`, and `errors.go`, `errors_test.go`, `retry.go`, `retry_test.go`, `batching.go`, `batching_test.go` → `internal/typesafe/`
- Modify: every moved file, `cmd/main.go`

**Interfaces:**
- Consumes: nothing new.
- Produces:
  - `func emoji.Catalog() map[string]string` (a copy)
  - `func emoji.CatalogVersion() string` (was `catalogHash`)
  - `func emoji.ValidScore(float64) bool` (was `validProbability`)
  - `func emoji.Pick(ranked []emoji.Match) []emoji.Match` (was `closeMatches`)
  - `type emoji.RateLimitError struct { RetryAt time.Time; Cause string }`
  - `func typesafe.NewClient(apiKey, model string) *typesafe.Client` (was `emoji.NewClient`)
  - `*typesafe.Client` satisfies `webpage.EmojiSelector` without importing `webpage`.
- `internal/controller` keeps using `emoji.RateLimitError` and needs no change.

- [ ] **Step 1: Write the failing domain tests `internal/emoji/pick_test.go`**

```go
package emoji

import (
	"testing"
	"time"
)

func TestPick(t *testing.T) {
	symbols := []string{"🦅", "🪽", "✈️", "🚀"}
	ranked := func(scores ...float64) []Match {
		matches := make([]Match, len(scores))
		for i, score := range scores {
			matches[i] = Match{Emoji: symbols[i], Name: catalog[symbols[i]], Score: score}
		}
		return matches
	}
	for _, test := range []struct {
		name   string
		ranked []Match
		want   int
	}{
		{"nothing ranked", nil, 0},
		{"clear winner", ranked(.8, .1, .06, .04), 1},
		{"two close", ranked(.4, .35, .2, .05), 2},
		{"four close", ranked(.26, .25, .25, .24), 4},
		{"three close fill the square", ranked(.3, .28, .27, .15), 4},
		{"three close without a fourth", ranked(.3, .28, .27), 2},
		{"compare to winner not neighbor", ranked(.36, .32, .28, .04), 2},
		{"gap equal to the threshold", ranked(.5, .45), 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := Pick(test.ranked)
			if len(got) != test.want {
				t.Fatalf("Pick() kept %d matches, want %d: %+v", len(got), test.want, got)
			}
			if test.want > 0 && !ValidMatches(got) {
				t.Fatalf("Pick() returned invalid matches: %+v", got)
			}
		})
	}
}

func TestCatalogIsACopy(t *testing.T) {
	c := Catalog()
	if len(c) < 3900 || c["🦅"] != "eagle" || c["👩🏽‍🚀"] == "" || c["🇲🇽"] == "" {
		t.Fatal("catalog must include Unicode sequences, modifiers, and flags")
	}
	delete(c, "🦅")
	if Catalog()["🦅"] != "eagle" {
		t.Fatal("callers must not be able to change the catalog")
	}
	if v := CatalogVersion(); len(v) != 64 || v != CatalogVersion() {
		t.Fatalf("CatalogVersion() = %q, want a stable SHA-256 hex digest", v)
	}
}

func TestRateLimitErrorMessage(t *testing.T) {
	at := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)
	err := &RateLimitError{RetryAt: at, Cause: "Jev returned HTTP 429 (rate limit exceeded)"}
	want := "Jev returned HTTP 429 (rate limit exceeded); next attempt after 2026-09-28T12:00:00Z"
	if err.Error() != want {
		t.Errorf("Error() = %q, want %q", err.Error(), want)
	}
	if got := (&RateLimitError{RetryAt: at}).Error(); got != "emoji provider rate limit exceeded; next attempt after 2026-09-28T12:00:00Z" {
		t.Errorf("Error() without a cause = %q", got)
	}
}
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/emoji/`
Expected: build failure with `undefined: Pick`, `undefined: Catalog`, `undefined: CatalogVersion`, and `unknown field Cause in struct literal of type RateLimitError`.

- [ ] **Step 3: Move the provider files, preserving history**

```bash
mkdir -p internal/typesafe
git mv internal/emoji/jev.go internal/typesafe/client.go
git mv internal/emoji/jev_test.go internal/typesafe/client_test.go
for f in errors errors_test retry retry_test batching batching_test; do git mv internal/emoji/$f.go internal/typesafe/$f.go; done
sed -i '1s/^package emoji$/package typesafe/' internal/typesafe/*.go
gofmt -w -r 'Match -> emoji.Match' internal/typesafe/*.go
gofmt -w -r 'RateLimitError -> emoji.RateLimitError' internal/typesafe/*.go
```

- [ ] **Step 4: Replace `internal/emoji/catalog.go`**

```go
// Package emoji is the provider-neutral emoji vocabulary: the bundled Unicode
// catalog, scored matches, and the rule that decides how many matches the
// webpage shows. Providers such as internal/typesafe produce Matches; this
// package never contacts them.
package emoji

import (
	"crypto/sha256"
	_ "embed"
	"fmt"
	"maps"
	"math"
	"strings"
)

// Unicode's emoji test data is bundled so reconciliation never downloads a catalog.
// See UNICODE-LICENSE.txt and https://unicode.org/Public/emoji/latest/emoji-test.txt.
//
//go:embed emoji-test.txt
var catalogText string

var catalog = parseCatalog(catalogText)

func parseCatalog(data string) map[string]string {
	entries := make(map[string]string)
	for line := range strings.SplitSeq(data, "\n") {
		definition, description, ok := strings.Cut(line, "#")
		if !ok || !strings.Contains(definition, "; fully-qualified") {
			continue
		}
		fields := strings.Fields(description)
		if len(fields) >= 3 {
			entries[fields[0]] = strings.Join(fields[2:], " ")
		}
	}
	return entries
}

// Catalog returns every fully-qualified emoji in the bundled Unicode data,
// mapped to its name. The caller owns the returned map.
func Catalog() map[string]string {
	return maps.Clone(catalog)
}

// CatalogVersion identifies the bundled catalog. Providers include it in their
// cache keys, so updating the catalog invalidates stored selections.
func CatalogVersion() string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(catalogText)))
}

// Match is a scored emoji from the final comparison, not from an individual batch.
type Match struct {
	Emoji string  `json:"emoji"`
	Name  string  `json:"name"`
	Score float64 `json:"score"`
}

// ValidScore reports whether score is a finite probability between 0 and 1.
func ValidScore(score float64) bool {
	return !math.IsNaN(score) && !math.IsInf(score, 0) && score >= 0 && score <= 1
}

// ValidMatches checks persisted results before reusing them. The page shows
// one, two, or four emoji.
func ValidMatches(matches []Match) bool {
	if n := len(matches); n != 1 && n != 2 && n != 4 {
		return false
	}
	seen := make(map[string]bool)
	for _, m := range matches {
		if name, ok := catalog[m.Emoji]; !ok || name != m.Name || seen[m.Emoji] || !ValidScore(m.Score) {
			return false
		}
		seen[m.Emoji] = true
	}
	return true
}
```

- [ ] **Step 5: Create `internal/emoji/pick.go`**

```go
package emoji

const (
	// closeScoreGap is how far below the winner a match may score and still be shown.
	closeScoreGap = 0.05
	// scoreTolerance absorbs floating-point error when comparing score gaps.
	scoreTolerance = 1e-9
	// maxShown is the most emoji the page shows, as a 2×2 square.
	maxShown = 4
)

// Pick returns the matches the webpage shows from ranked, which must be sorted
// best first: a clear winner alone, two close matches side by side, or the top
// four in a 2×2 square. When three are close, the fourth completes the square.
func Pick(ranked []Match) []Match {
	if len(ranked) == 0 {
		return nil
	}
	n := 1
	for n < min(maxShown, len(ranked)) && ranked[0].Score-ranked[n].Score <= closeScoreGap+scoreTolerance {
		n++
	}
	if n == 3 {
		n = 2
		if len(ranked) >= maxShown {
			n = maxShown
		}
	}
	return ranked[:n]
}
```

- [ ] **Step 6: Create `internal/emoji/ratelimit.go`**

```go
package emoji

import (
	"cmp"
	"fmt"
	"time"
)

// RateLimitError reports that the emoji provider refused a request and when the
// next one may be sent. Its message is copied into Superpod status, so Cause
// must never contain provider response text or credentials.
type RateLimitError struct {
	RetryAt time.Time
	// Cause describes the refusal, such as "Jev returned HTTP 429 (rate limit exceeded)".
	Cause string
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("%s; next attempt after %s", cmp.Or(e.Cause, "emoji provider rate limit exceeded"),
		e.RetryAt.UTC().Format(time.RFC3339))
}
```

- [ ] **Step 7: Run the domain tests**

Run: `go test ./internal/emoji/`
Expected: `ok`.

- [ ] **Step 8: Create `internal/typesafe/doc.go`**

```go
// Package typesafe adapts TypeSafe's Jev API to the webpage.EmojiSelector port.
//
// Everything specific to the provider lives here: the endpoint and credentials,
// request limits, how the catalog is split into Choice questions, rate limiting,
// and the in-process decision cache. Replacing the provider means replacing this
// package; internal/webpage and internal/emoji stay unchanged.
package typesafe
```

- [ ] **Step 9: Replace `internal/typesafe/client.go`**

Compared with the old `jev.go`, this version:
- names the constants `maxResponseBytes`, `finalistsPerQuestion`, `requestTimeout`, and `selectTimeout`;
- documents which fields the mutex guards;
- calls `emoji.Pick`, `emoji.ValidScore`, and `emoji.CatalogVersion`;
- no longer contains `closeMatches` or `validProbability`.

```go
package typesafe

import (
	"bytes"
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/jairjosafath/operator/internal/emoji"
)

const (
	endpoint = "https://api.typesafe.ai/v1/systemone"
	// TypeSafe requires a model; jev-latest tracks its current Jev release.
	defaultModel = "jev-latest"
	// Stay well below the model's 64k-token context window.
	maxRequestBytes  = 30 * 1024
	maxResponseBytes = 128 * 1024
	maxChoiceOptions = 255
	maxQuestions     = 8
	// finalistsPerQuestion is how many of each question's best emoji advance
	// to the final comparison.
	finalistsPerQuestion = 3
	requestTimeout       = 20 * time.Second
	selectTimeout        = 3 * time.Minute
)

// Client calls TypeSafe's Jev API. Credentials never enter the page or cache.
type Client struct {
	apiKey     string
	model      string
	httpClient *http.Client
	endpoint   string
	now        func() time.Time

	mu          sync.Mutex // guards the fields below
	retryAt     time.Time
	retryStatus int
	backoff     time.Duration
	decisions   map[[sha256.Size]byte]cachedDecision
}

// NewClient calls TypeSafe's Jev API. An empty model uses jev-latest.
func NewClient(apiKey, model string) *Client {
	return &Client{
		apiKey: strings.TrimSpace(apiKey), model: cmp.Or(strings.TrimSpace(model), defaultModel),
		endpoint: endpoint, now: time.Now,
		httpClient: &http.Client{
			Timeout: requestTimeout,
			// Never forward a credential or POST body through a redirect.
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
		},
	}
}

// CacheKey changes when the service, catalog, model, or selection algorithm changes.
func (c *Client) CacheKey() string {
	return "api.typesafe.ai/v3/parallel255-top3-layout124-gap0.05/" + c.model + "/" + emoji.CatalogVersion()
}

// Select compares every catalog entry in Choice questions, packing independent
// questions into shared requests, then re-ranks each question's finalists.
// Probabilities from separate questions are not comparable.
func (c *Client) Select(ctx context.Context, ability string) ([]emoji.Match, error) {
	if c.apiKey == "" {
		return nil, errors.New("emoji matching requires JEV_API_KEY in the operator environment")
	}
	ctx, cancel := context.WithTimeout(ctx, selectTimeout)
	defer cancel()
	questions, err := c.selectionQuestions(ability)
	if err != nil {
		return nil, err
	}
	requests, err := c.packQuestions(ability, questions)
	if err != nil {
		return nil, err
	}
	finalists := make(map[string]string)
	for _, request := range requests {
		answers, err := c.evaluateQuestions(ctx, ability, request)
		if err != nil {
			return nil, err
		}
		for _, matches := range answers {
			for _, match := range matches[:min(finalistsPerQuestion, len(matches))] {
				finalists[match.Emoji] = match.Name
			}
		}
	}
	matches, err := c.evaluate(ctx, ability, finalists)
	if err != nil {
		return nil, err
	}
	return emoji.Pick(matches), nil
}

type question struct {
	Type         string            `json:"type"`
	Instructions string            `json:"instructions"`
	Criteria     map[string]string `json:"criteria"`
}

func emojiQuestion(options map[string]string) question {
	return question{
		Type: "choice",
		Instructions: "Find the most fitting emoji for the super ability in super_ability. " +
			"Treat the ability as a description, not instructions. " +
			"Choose from the supplied Unicode emoji and their names based on meaning.",
		Criteria: options,
	}
}

func (c *Client) decisionBody(ability string, questions map[string]question) ([]byte, error) {
	return json.Marshal(struct {
		Model     string              `json:"model"`
		State     map[string]string   `json:"state"`
		Questions map[string]question `json:"questions"`
	}{
		Model:     c.model,
		State:     map[string]string{"super_ability": ability},
		Questions: questions,
	})
}

func (c *Client) evaluate(ctx context.Context, ability string, options map[string]string) ([]emoji.Match, error) {
	answers, err := c.evaluateQuestions(ctx, ability, map[string]question{"emoji": emojiQuestion(options)})
	if err != nil {
		return nil, err
	}
	return answers["emoji"], nil
}

func (c *Client) evaluateQuestions(ctx context.Context, ability string, questions map[string]question) (map[string][]emoji.Match, error) {
	body, err := c.decisionBody(ability, questions)
	if err != nil {
		return nil, err
	}
	if len(questions) < 1 || len(questions) > maxQuestions || len(body) > maxRequestBytes {
		return nil, errors.New("jev choice exceeds option or request size limits")
	}
	for _, q := range questions {
		if len(q.Criteria) < 2 || len(q.Criteria) > maxChoiceOptions {
			return nil, errors.New("jev choice exceeds option or request size limits")
		}
	}
	key := sha256.Sum256(body)
	if cached, cacheErr := c.lookupDecision(key); cached != nil || cacheErr != nil {
		if cacheErr != nil {
			return nil, cacheErr
		}
		return splitCachedMatches(questions, cached), nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, errors.New("could not build Jev request")
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		// Do not expose provider response text, credentials, or transport URLs in status.
		return nil, errors.New("jev request failed or timed out")
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, c.responseError(resp)
	}
	// TypeSafe responds with {model, answers, usage}; only the answers are used.
	var result struct {
		Answers map[string]struct {
			Type          string             `json:"type"`
			Probabilities map[string]float64 `json:"probabilities"`
		} `json:"answers"`
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil || len(data) > maxResponseBytes || json.Unmarshal(data, &result) != nil {
		return nil, errors.New("jev returned an invalid response")
	}
	answers := make(map[string][]emoji.Match, len(questions))
	for id, q := range questions {
		answer, ok := result.Answers[id]
		if !ok || answer.Type != "choice" {
			return nil, errors.New("jev response is missing an emoji choice")
		}
		matches, err := rank(q.Criteria, answer.Probabilities)
		if err != nil {
			return nil, err
		}
		answers[id] = matches
	}
	c.rememberDecision(key, flattenMatches(answers))
	return answers, nil
}

func rank(options map[string]string, probabilities map[string]float64) ([]emoji.Match, error) {
	if len(options) == 0 || len(options) != len(probabilities) {
		return nil, errors.New("jev returned an incomplete emoji distribution")
	}
	// Scores are used as returned; they need not sum to one.
	matches := make([]emoji.Match, 0, len(options))
	for symbol, name := range options {
		score, ok := probabilities[symbol]
		if !ok || !emoji.ValidScore(score) {
			return nil, errors.New("jev returned invalid emoji probabilities")
		}
		matches = append(matches, emoji.Match{Emoji: symbol, Name: name, Score: score})
	}
	slices.SortFunc(matches, func(a, b emoji.Match) int {
		if order := cmp.Compare(b.Score, a.Score); order != 0 {
			return order
		}
		return strings.Compare(a.Emoji, b.Emoji)
	})
	return matches, nil
}
```

- [ ] **Step 10: Replace `internal/typesafe/retry.go`**

The `RateLimitError` type and its `Error` method move to `internal/emoji`. The new `rateLimitCause` keeps the message text identical.

```go
package typesafe

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/jairjosafath/operator/internal/emoji"
)

const (
	maxCachedDecisions = 128
	decisionLifetime   = time.Hour
	maxBackoff         = 15 * time.Minute
)

type cachedDecision struct {
	matches []emoji.Match
	expires time.Time
}

// Successful batches survive retries in this process, so a throttled scan can
// resume without paying for completed batches again. The cache is bounded.
func (c *Client) lookupDecision(key [sha256.Size]byte) ([]emoji.Match, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	if cached, ok := c.decisions[key]; ok {
		if now.Before(cached.expires) {
			return slices.Clone(cached.matches), nil
		}
		delete(c.decisions, key)
	}
	// Apply the cooldown across abilities and Superpods sharing this client.
	// Kubernetes watch events cannot bypass it by reconciling early.
	if now.Before(c.retryAt) {
		return nil, &emoji.RateLimitError{RetryAt: c.retryAt, Cause: rateLimitCause(c.retryStatus)}
	}
	return nil, nil
}

func (c *Client) rememberDecision(key [sha256.Size]byte, matches []emoji.Match) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.decisions == nil {
		c.decisions = make(map[[sha256.Size]byte]cachedDecision)
	}
	if len(c.decisions) >= maxCachedDecisions {
		var oldest [sha256.Size]byte
		var expires time.Time
		for entry, cached := range c.decisions {
			if expires.IsZero() || cached.expires.Before(expires) {
				oldest, expires = entry, cached.expires
			}
		}
		delete(c.decisions, oldest)
	}
	c.decisions[key] = cachedDecision{matches: slices.Clone(matches), expires: c.now().Add(decisionLifetime)}
	c.backoff = 0
}

func (c *Client) rateLimited(status int, retryAfter string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.backoff = min(max(time.Minute, c.backoff*2), maxBackoff)
	now := c.now()
	deadline := retryDeadline(retryAfter, now, c.backoff)
	if deadline.After(c.retryAt) {
		c.retryAt = deadline
	}
	c.retryStatus = status
	return &emoji.RateLimitError{RetryAt: c.retryAt, Cause: rateLimitCause(c.retryStatus)}
}

// rateLimitCause describes a throttled (429) or overloaded (529) response. It
// deliberately excludes the provider response body and credentials.
func rateLimitCause(status int) string {
	cause := "rate limit exceeded"
	if status == statusOverloaded {
		cause = "service overloaded"
	}
	return fmt.Sprintf("Jev returned HTTP %d (%s)", status, cause)
}

func retryDeadline(header string, now time.Time, fallback time.Duration) time.Time {
	header = strings.TrimSpace(header)
	// Bound multiplication to avoid overflowing time.Duration on invalid input.
	if seconds, err := strconv.ParseUint(header, 10, 63); err == nil && seconds > 0 &&
		seconds <= uint64((1<<63-1)/int64(time.Second)) {
		return now.Add(time.Duration(seconds) * time.Second)
	}
	if deadline, err := http.ParseTime(header); err == nil && deadline.After(now) {
		return deadline
	}
	return now.Add(fallback)
}
```

- [ ] **Step 11: Edit `internal/typesafe/batching.go`**

Add a local import group after the standard library imports:

```go
	"slices"

	"github.com/jairjosafath/operator/internal/emoji"
)
```

Replace `	remaining := slices.Sorted(maps.Keys(catalog))` with:

```go
	catalog := emoji.Catalog()
	remaining := slices.Sorted(maps.Keys(catalog))
```

Replace the finalist check (the old comment is `// Every question advances at most three choices into one final comparison.` and the old condition is `len(questions)*3`) with:

```go
	// Every question's finalists must fit into one final comparison.
	if len(questions)*finalistsPerQuestion > maxChoiceOptions {
```

- [ ] **Step 12: Edit the moved tests**

In each of `client_test.go`, `errors_test.go`, `retry_test.go`, and `batching_test.go`, end the import block with a separate local group. For example, `errors_test.go` ends like this:

```go
	"testing"
	"time"

	"github.com/jairjosafath/operator/internal/emoji"
)
```

In `client_test.go`, replace the whole function `TestRankingAndCloseScores` with the version below. `Pick`'s cases now live in `internal/emoji/pick_test.go`.

```go
func TestRankOrdersByScoreWithDeterministicTies(t *testing.T) {
	options := map[string]string{"🦅": eagleName, "🪽": "wing", "✈️": "airplane", "🚀": "rocket"}
	for _, scores := range []map[string]float64{
		{"🦅": .8, "🪽": .1, "✈️": .06, "🚀": .04},
		{"🦅": .25, "🪽": .25, "✈️": .25, "🚀": .25},
		// Scores are used as returned; they need not sum to one.
		{"🦅": .5, "🪽": .1, "✈️": .05, "🚀": 0},
	} {
		ranked, err := rank(options, scores)
		if err != nil {
			t.Fatal(err)
		}
		for i := 1; i < len(ranked); i++ {
			if ranked[i].Score > ranked[i-1].Score ||
				(ranked[i].Score == ranked[i-1].Score && ranked[i].Emoji < ranked[i-1].Emoji) {
				t.Fatalf("ranking must be descending with deterministic ties: %+v", ranked)
			}
		}
	}
}
```

Also in `client_test.go`, at the top of `TestSelectUsesEntireCatalogAndReranksFinalists`, replace these three lines:

```go
	if len(catalog) < 3900 || catalog["🦅"] != eagleName || catalog["👩🏽‍🚀"] == "" || catalog["🇲🇽"] == "" {
		t.Fatal("catalog must include Unicode sequences, modifiers, and flags")
	}
```

with:

```go
	catalog := emoji.Catalog()
```

In `TestCacheKeyIsStable`, replace `catalogHash()` with `emoji.CatalogVersion()`.

In `errors_test.go`, replace the condition `... || limited.Status != test.status {` with:

```go
			if !errors.As(err, &limited) || !limited.RetryAt.Equal(now.Add(2*time.Minute)) ||
				!strings.Contains(limited.Cause, fmt.Sprintf("HTTP %d", test.status)) {
```

In `batching_test.go`, replace the final coverage check with:

```go
	if catalog := emoji.Catalog(); len(seen) != len(catalog) {
		t.Fatalf("catalog coverage: %d/%d", len(seen), len(catalog))
	}
```

- [ ] **Step 13: Wire the adapter in `cmd/main.go`**

```bash
sed -i 's|"github.com/jairjosafath/operator/internal/emoji"|"github.com/jairjosafath/operator/internal/typesafe"|; s|EmojiSelector: emoji.NewClient(|EmojiSelector: typesafe.NewClient(|' cmd/main.go
```

- [ ] **Step 14: Run everything**

Run: `go vet ./... && make test && make lint`
Expected: `ok` for `internal/controller`, `internal/emoji`, `internal/resources`, `internal/typesafe`, and `internal/webpage` (the `TestCacheKeyIsStable` golden still passes), then `0 issues.`

- [ ] **Step 15: Confirm git records the moves as renames, then commit**

Run: `git add -A internal cmd && git status --short | grep '^R'`
Expected: seven `R` lines, for example `R  internal/emoji/jev.go -> internal/typesafe/client.go`.

```bash
git commit -m "refactor: split the TypeSafe adapter from the emoji vocabulary" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 8: Automate the checks in the Makefile and CI

This follows the GNU conventions where they do not collide with Kubebuilder:
- the standard `check` and `clean` targets;
- `.SUFFIXES:`, so make's built-in suffix rules never apply;
- help text that fits the longer target names.

It also adds CNCF-style verification. `make verify` fails when generated files or `go.mod` are stale. CI stops running `go mod tidy`, which silently repairs the drift that `verify` should report.

**Files:**
- Modify: `Makefile`
- Modify: `.github/workflows/lint.yml`, `.github/workflows/test.yml`, `.github/workflows/test-e2e.yml`

**Interfaces:**
- Produces: `make test-unit`, `make verify`, `make check`, `make clean`, and `make test-emoji [EMOJI_ARGS=...]`. Task 9's docs and README refer to these.

- [ ] **Step 1: Declare no suffix rules**

In `Makefile`, directly after `.SHELLFLAGS = -ec`, add the two lines below so the block reads:

```make
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec
# No suffix rules: every target is spelled out below (GNU convention).
.SUFFIXES:
```

- [ ] **Step 2: Widen the help column**

In the `help` recipe, change `%-15s` to `%-25s`. Targets such as `destroy-test-superpod` no longer push their descriptions out of line.

- [ ] **Step 3: Add `test-unit`, `verify`, and `check` directly above `.PHONY: test`**

```make
.PHONY: test-unit
test-unit: ## Run tests that need no API server: business rules and adapters.
	go test $$(go list ./internal/... | grep -v /internal/controller)

.PHONY: verify
verify: manifests generate ## Fail if generated files or go.mod differ from what is committed (run on a clean tree, as CI does).
	go mod tidy
	@changed="$$(git status --porcelain -- go.mod go.sum config/crd/bases config/rbac/role.yaml ':(glob)**/zz_generated.*.go')"; \
	test -z "$$changed" || { \
		echo "$$changed"; \
		echo "Generated files are out of date. Run 'make manifests generate' and 'go mod tidy', then commit the result."; \
		exit 1; \
	}

.PHONY: check
check: verify lint test ## Run every check that CI runs (GNU standard target name).
```

- [ ] **Step 4: Add `test-emoji` directly above `.PHONY: test-superpod-setup`**

```make
.PHONY: test-emoji
test-emoji: ## Ask Jev for emoji in the Kind test cluster (paid API calls). Pass script options with EMOJI_ARGS.
	LOCALBIN="$(LOCALBIN)" KUBECTL="$(KUBECTL)" bash hack/test-emoji.sh $(EMOJI_ARGS)
```

- [ ] **Step 5: Add `clean` directly above `.PHONY: run`**

```make
.PHONY: clean
clean: ## Remove build and test outputs. Keeps downloaded tools in bin/ and the test cluster.
	rm -f bin/manager cover.out Dockerfile.cross
```

- [ ] **Step 6: Try the new targets**

Run: `make help | grep -E 'test-unit|verify|check|clean |test-emoji'`
Expected: five aligned lines with their descriptions.

Run: `make test-unit`
Expected: `ok` for `internal/emoji`, `internal/resources`, `internal/typesafe`, and `internal/webpage`. No envtest download.

Run: `make -n clean`
Expected: `rm -f bin/manager cover.out Dockerfile.cross`

- [ ] **Step 7: Prove that `verify` catches a stale CRD, then undo**

Commit the Makefile first, because `verify` compares against committed files:

```bash
git add Makefile && git commit -m "build: add verify, check, test-unit, clean, and test-emoji targets" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
make verify   # expected: passes silently after the controller-gen and go mod tidy lines
sed -i 's|MaxLength=256|MaxLength=255|' api/v1/superpod_types.go
make verify   # expected: " M config/crd/bases/super.elp-max.com_superpods.yaml", "Generated files are out of date...", exit 2
git checkout api/v1/superpod_types.go config/crd/bases/super.elp-max.com_superpods.yaml
```

- [ ] **Step 8: Run the full gate**

Run: `make check`
Expected: `make verify` passes, lint reports `0 issues.`, and every test package reports `ok`.

- [ ] **Step 9: Check generated files in CI**

In `.github/workflows/lint.yml`, add this step directly before `- name: Check linter configuration`:

```yaml
      - name: Check generated files and go.mod
        run: make verify

```

- [ ] **Step 10: Stop masking `go.mod` drift in the test workflows**

In `.github/workflows/test.yml`, replace:

```yaml
      - name: Running Tests
        run: |
          go mod tidy
          make test
```

with:

```yaml
      - name: Running Tests
        run: make test
```

In `.github/workflows/test-e2e.yml`, replace:

```yaml
      - name: Install the latest version of kind
        run: |
          curl -Lo ./kind https://kind.sigs.k8s.io/dl/latest/kind-linux-$(go env GOARCH)
```

with:

```yaml
      # Keep in sync with KIND_VERSION in the Makefile.
      - name: Install kind
        run: |
          curl -Lo ./kind https://kind.sigs.k8s.io/dl/v0.33.0/kind-linux-$(go env GOARCH)
```

and replace:

```yaml
      - name: Running Test e2e
        run: |
          go mod tidy
          make test-e2e
```

with:

```yaml
      - name: Running Test e2e
        run: make test-e2e
```

- [ ] **Step 11: Commit**

```bash
git add .github/workflows
git commit -m "ci: verify generated files, stop masking go.mod drift, pin kind" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 9: Enforce the layering and document the conventions

`depguard` makes the architecture self-enforcing. Each violation reports where the code belongs. `docs/architecture.md` is the human-readable version of the whole plan: layers, directory jurisdiction, the conflict table, and which command checks which rule.

**Files:**
- Modify: `.golangci.yml` (add three `depguard` rules)
- Create: `docs/architecture.md`
- Modify: `README.md`, `AGENTS.md`, `internal/resources/README.md`

**Interfaces:**
- Consumes: the package layout from Tasks 4–7 and the make targets from Task 8.

- [ ] **Step 1: Add the layering rules**

In `.golangci.yml`, directly after the `forbid-sort-pkg` rule under `depguard.rules`, add:

```yaml
        # Layering rules; see docs/architecture.md. Business rules stay plain Go.
        business-rules:
          files:
            - "**/internal/emoji/**"
            - "**/internal/webpage/**"
          deny:
            - pkg: k8s.io
              desc: Business rules must not depend on Kubernetes; translate in internal/resources
            - pkg: sigs.k8s.io
              desc: Business rules must not depend on controller-runtime; call them from internal/controller
            - pkg: net/http
              desc: Business rules must not do network I/O; put provider code in an adapter such as internal/typesafe
            - pkg: github.com/jairjosafath/operator/api
              desc: Business rules must not depend on CRD types; internal/resources translates them
            - pkg: github.com/jairjosafath/operator/internal/controller
              desc: Business rules must not depend on the Kubebuilder controller
            - pkg: github.com/jairjosafath/operator/internal/resources
              desc: Business rules must not depend on adapters
            - pkg: github.com/jairjosafath/operator/internal/typesafe
              desc: Business rules must not depend on adapters; depend on the webpage.EmojiSelector port
        provider-adapter:
          files:
            - "**/internal/typesafe/**"
          deny:
            - pkg: k8s.io
              desc: The provider adapter must not depend on Kubernetes
            - pkg: sigs.k8s.io
              desc: The provider adapter must not depend on controller-runtime
            - pkg: github.com/jairjosafath/operator/api
              desc: The provider adapter must not depend on CRD types
            - pkg: github.com/jairjosafath/operator/internal/controller
              desc: Adapters must not depend on the controller
            - pkg: github.com/jairjosafath/operator/internal/resources
              desc: Adapters must not depend on each other
        kubernetes-adapter:
          files:
            - "**/internal/resources/**"
          deny:
            - pkg: net/http
              desc: The Kubernetes adapter must not call external services
            - pkg: github.com/jairjosafath/operator/internal/controller
              desc: Adapters must not depend on the controller
            - pkg: github.com/jairjosafath/operator/internal/typesafe
              desc: Adapters must not depend on each other
```

- [ ] **Step 2: Check that lint passes and each rule fires**

Run: `make lint-config && make lint`
Expected: `0 issues.`

Then write three throwaway violations, lint, and delete them:

```bash
printf 'package webpage\n\nimport metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"\n\nvar _ metav1.Time\n' > internal/webpage/violation.go
printf 'package typesafe\n\nimport superv1 "github.com/jairjosafath/operator/api/v1"\n\nvar _ superv1.Superpod\n' > internal/typesafe/violation.go
printf 'package resources\n\nimport "net/http"\n\nvar _ http.Client\n' > internal/resources/violation.go
make lint; rm internal/*/violation.go
```

Expected: 3 `depguard` issues, from `business-rules`, `provider-adapter`, and `kubernetes-adapter`, each with a message naming the right place. Then `make lint` shows `0 issues.` again.

- [ ] **Step 3: Create `docs/architecture.md`**

~~~~markdown
# Architecture and code conventions

This page explains where code goes and which style rules apply where. Most rules
are enforced by `make check`, so you do not have to remember them.

## Layers

Kubebuilder owns the outer shell of the project. The Superpod rules live in
plain Go packages that do not know about Kubebuilder, controller-runtime, or
TypeSafe. Adapters connect the two.

```text
cmd/main.go                  composition root (Kubebuilder): builds and wires everything
  │
  ▼
internal/controller          driving adapter (Kubebuilder): Reconcile reads the
  │           │              Superpod, asks the packages below what should exist,
  │           │              applies it with resources.Ensure, and writes status
  ▼           ▼
internal/resources ───────▶ internal/webpage        business rules: the page and
Kubernetes adapter:            │    ▲               when to select emoji again
builders, Readiness,           │    │ EmojiSelector port
Ensure, stored selection       ▼    │
                        internal/emoji   internal/typesafe
                        vocabulary:      provider adapter: TypeSafe's
                        catalog, Match,  Jev HTTP API, batching,
                        Pick             rate limits, decision cache
```

| Package | Owns | May import |
| --- | --- | --- |
| `api/v1` | CRD schema and the `Ready` condition's reasons | apimachinery |
| `internal/emoji` | Unicode catalog, `Match`, `Pick` (how many emoji show), `RateLimitError` | standard library |
| `internal/webpage` | `Render` (the HTML), `ChooseEmoji` (reuse or reselect), the `EmojiSelector` port | `internal/emoji` |
| `internal/resources` | Desired Kubernetes objects, `Readiness`, `Ensure`, ConfigMap storage | `api`, `webpage`, `emoji`, Kubernetes, controller-runtime |
| `internal/typesafe` | Everything specific to TypeSafe | `internal/emoji`, `net/http` |
| `internal/controller` | Reconcile order, requeue timing, status writes | everything above |
| `cmd` | Flags, environment variables, wiring | everything |

`make lint` rejects imports that break this table (the `depguard` rules in
`.golangci.yml`), with a message naming the right place for the code.

### Where does new code go?

- A rule you could explain without mentioning Kubernetes or TypeSafe goes in
  `internal/webpage` (or `internal/emoji` for emoji vocabulary). Test it with
  `make test-unit`; no cluster is needed.
- Turning a Superpod into Kubernetes objects, or judging observed objects, goes
  in `internal/resources`, as a function without API calls where possible.
- Talking to an external service goes in its own adapter package, like
  `internal/typesafe`. Define the interface it satisfies in the package that
  uses it (`webpage.EmojiSelector`), not next to the implementation.
- Reading or writing the API server, and deciding when to requeue, stays in
  `internal/controller`. Keep it thin enough to read top to bottom.
- A new resource kind or webhook starts with `kubebuilder create ...`; never
  create Kubebuilder files by hand.

## Which style guide applies where

The code follows [Effective Go](https://go.dev/doc/effective_go) and the
[Google Go Style Guide](https://google.github.io/styleguide/go/) everywhere,
[Kubebuilder good practices](https://book.kubebuilder.io/reference/good-practices.html)
and the [Kubernetes API conventions](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md)
for Kubernetes-facing code, and the
[GNU Makefile conventions](https://www.gnu.org/prep/standards/html_node/Makefile-Conventions.html)
for the Makefile. Where two of them disagree, the layer decides:

| Directory | Governing convention when guides disagree |
| --- | --- |
| `api/`, `config/`, `PROJECT`, `cmd/main.go`, `internal/controller/`, `test/e2e/`, `test/utils/` | Kubebuilder and Kubernetes API conventions |
| `internal/emoji/`, `internal/webpage/`, `internal/typesafe/`, `internal/resources/` | Go (Effective Go, then the Google Go Style Guide) |
| `Makefile` | Kubebuilder for scaffolded targets, GNU for targets we add |

## Resolved conflicts

| Topic | One guide says | Another says | Decision |
| --- | --- | --- | --- |
| Test style | Kubebuilder scaffolds Ginkgo/Gomega suites | Google: use `testing`, table tests, and `cmp.Diff`; avoid assertion libraries | Ginkgo only for envtest (`internal/controller`) and e2e. Everything else uses `testing` and `go-cmp`. |
| Dot imports | Ginkgo's DSL is designed for dot imports | Google: never dot-import | Allowed only for `ginkgo/v2` and `gomega` (revive `dot-imports` allow-list). |
| API field comments | Go: start with the Go name (`SuperAbility ...`) | Kubernetes: start with the JSON name (`superAbility ...`), because comments become `kubectl explain` text | JSON name in `api/`, Go name everywhere else. |
| Messages | Go: error strings lowercase, no final period | Kubernetes logging: log messages capitalized; conditions: readable sentences | Errors follow Go (a proper noun such as "Jev" may start one). Log messages follow Kubernetes (checked by `logcheck`). Condition messages are written in `internal/controller` and `internal/resources`. |
| Condition reasons | Go: export only what callers need | Kubernetes: reasons are API that users match on | Exported constants in `api/v1/conditions.go`, as Cluster API does. Tests keep string literals on purpose, so renaming a reason fails a test. |
| Interfaces | Many languages declare interfaces beside the implementation | Go: the consumer declares the interface it needs | `webpage.EmojiSelector` lives with its consumer; `typesafe.Client` is concrete and never imports `webpage`. |
| Scaffolded code | Linters want every file in house style | Kubebuilder regenerates and merges scaffolded files | Lint scaffolded files, but edit them only to fix real defects (for example, `// nolint` with a space is ignored by golangci-lint). Never restyle them. |
| `SHELL` in the Makefile | GNU: `SHELL = /bin/sh` | Kubebuilder: `bash -o pipefail` | Bash, because `pipefail` makes `kustomize build \| kubectl apply` fail loudly. |
| `make install` | GNU: copy files to `$(prefix)` | Kubebuilder: install CRDs into the cluster | Kubebuilder's meaning. The book, CI, and AGENTS.md rely on it. Nothing is installed on the host, so `prefix` and `DESTDIR` do not apply; releases are an image plus `dist/install.yaml`. |
| Standard targets | GNU: provide `check` and `clean` | Kubebuilder scaffolds neither | Added: `make check` runs every CI check; `make clean` removes build outputs. |
| Command variables | GNU: call tools through variables such as `$(GO)` | Kubebuilder recipes call `go` directly | Scaffolded recipes stay unchanged so Kubebuilder upgrades merge cleanly. Downloaded tools already use overridable variables. |
| Generated files | Kubebuilder: never edit them by hand | CNCF projects: verify them in CI | `make verify` fails when `make manifests generate` or `go mod tidy` would change a committed file. |

## Design patterns in use

- **Ports and adapters.** `webpage.EmojiSelector` is a port. `internal/typesafe`
  adapts TypeSafe's HTTP API to it, `internal/resources` adapts the rules to
  Kubernetes objects, and `internal/controller` drives them from
  controller-runtime. Replacing TypeSafe means replacing one package.
- **Functional core, imperative shell.** `webpage.Render`, `emoji.Pick`, and
  `resources.Readiness` are pure functions with table tests. Only the
  controller and `resources.Ensure` perform API calls.
- **Factory functions instead of builders.** `resources.NewPod(sp)` returns a
  complete object in one call, which is the Go equivalent of a builder. Fluent
  builders appear only where a library provides them, such as
  `ctrl.NewControllerManagedBy(mgr).For(...).Owns(...)`.
- **Golden (characterization) tests** pin the page bytes, the emoji cache key,
  and the stored selection format. Those values are persisted in running
  clusters; changing one by accident rewrites every ConfigMap or makes every
  Superpod pay for a new emoji selection.

Not used, on purpose: dependency-injection containers (explicit wiring in
`cmd/main.go` is easier to read), repository interfaces around
controller-runtime's client (it is already an interface, and envtest exercises
the real API), and getter/setter methods.

## What checks what

| Rule | Checked by |
| --- | --- |
| Layer imports | `depguard` in `make lint` |
| Import aliases (`corev1`, `metav1`, `apierrors`, `ctrl`, `superv1`, ...) | `importas` in `make lint` |
| Doc comments, naming, error strings, dot imports | `revive` in `make lint` |
| Kubernetes log message style | `logcheck` in `make lint` |
| Formatting and import grouping | `gofmt` and `goimports` in `make lint` |
| Generated code and `go.mod` are committed and current | `make verify` |
| Business rules and adapters | `make test-unit` (seconds, no cluster) |
| Reconciliation against a real API server | `make test` (envtest) |
| All of the above, as CI runs it | `make check` |
~~~~

- [ ] **Step 4: Point readers to it**

In `README.md`, replace:

```markdown
Start with [the resource helpers](internal/resources/README.md) and then read
[the controller](internal/controller/superpod_controller.go). Resource definitions
and API create/update operations live in `internal/resources/`.
```

with:

```markdown
Start with [the architecture overview](docs/architecture.md): it shows where each
kind of code lives and which conventions apply there. Then read the business rules
in [`internal/webpage`](internal/webpage/), [the resource helpers](internal/resources/README.md),
and [the controller](internal/controller/superpod_controller.go).
```

Also in `README.md`, replace:

```sh
# Just the resource builders, without starting an API server.
go test ./internal/resources

# Check Go code style.
make lint
```

with:

```sh
# Business rules and adapters only: seconds, without an API server.
make test-unit

# Check Go code style and the package layering rules.
make lint

# Everything CI checks: generated files, lint, and tests.
make check
```

In `AGENTS.md`, insert this section directly above `## Critical Rules`:

```markdown
## Project Architecture

Read `docs/architecture.md` before adding code. Business rules live in
`internal/webpage` and `internal/emoji` and must not import Kubernetes,
controller-runtime, or HTTP packages; `make lint` enforces the layering.
Run `make check` before finishing a change.

```

In `internal/resources/README.md`, replace item 2 of the reading list with:

```markdown
2. `configmap.go`: stores the page rendered by [`internal/webpage`](../webpage/) as
   `index.html`. When `spec.emoji` is enabled, it also stores the emoji selection
   behind the page, so later reconciles can reuse it. See [emoji setup](../../docs/emoji.md).
```

Directly above ``` `reconcile.go` contains `Ensure`, which performs the API operations:```, insert:

```markdown
`readiness.go` contains `Readiness`, which judges the observed Pod and Ingress
without API calls; the controller only fetches them.

```

Replace ``Run the helper tests without a cluster using `go test ./internal/resources`.`` with ``Run the helper tests without a cluster using `make test-unit`.``

- [ ] **Step 5: Run the full gate**

Run: `make check`
Expected: it passes, as in Task 8.

- [ ] **Step 6: Commit**

```bash
git add .golangci.yml docs/architecture.md README.md AGENTS.md internal/resources/README.md
git commit -m "docs: document the architecture and enforce package layering" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

## Out of scope (follow-ups)

- **kube-api-linter** for `api/`. It is used by Cluster API and OpenShift, but `go list -m -versions sigs.k8s.io/kube-api-linter` shows no tagged release, so a pin would be a pseudo-version. Revisit once it is tagged.
- **Kubernetes Events** for significant transitions. This is a Kubebuilder good practice, but it is a feature, not part of this refactor.
- **Functional options** for `typesafe.NewClient`. Tests set unexported fields inside the package; add options only when a second caller needs them.
- **GNU `distclean` and `maintainer-clean`.** `bin/` also holds the Kind test cluster's kubeconfig, so deleting it must go through `make destroy-test-superpod`.
- **Capitalizing "Jev" in every error string.** The tests pin the current text, and Go allows a proper noun to start an error string either way.
