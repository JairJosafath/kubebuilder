# Superpod operator

A small Kubernetes operator, built with [Kubebuilder](https://book.kubebuilder.io),
for learning how operators work. You create a `Superpod` with a line of text, its
"super ability". The operator runs an nginx Pod that serves a webpage showing that
text, and exposes it through a Service and an Ingress. Optionally, it asks
TypeSafe's Jev API to pick matching emoji for the page.

The controller repairs missing resources, reports the page's URL and a `Ready`
condition, and leaves cleanup to Kubernetes when the Superpod is deleted.

## Quick start

Requires Go 1.26 or newer and Make; the Makefile downloads everything else into `bin/`.

```sh
make test-unit   # business rules, in seconds
make check       # everything CI runs: generated files, lint, and all tests
make help        # every other target
```

## Documentation

| To | Read |
| --- | --- |
| Understand how the code is organized | [docs/architecture.md](docs/architecture.md) |
| Run the full demo in a local Kind cluster | [test/superpod/README.md](test/superpod/README.md) |
| Deploy to your own cluster and open the webpage | [docs/webpage-access.md](docs/webpage-access.md) |
| Turn on emoji matching | [docs/emoji.md](docs/emoji.md) |
| See what each Kubernetes resource does | [internal/resources/README.md](internal/resources/README.md) |
| Set up the dev container | [.devcontainer/README.md](.devcontainer/README.md) |

## License

Copyright 2026. Licensed under the
[Apache License, Version 2.0](http://www.apache.org/licenses/LICENSE-2.0).
