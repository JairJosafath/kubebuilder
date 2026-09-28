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
