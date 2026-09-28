# Dev containers

Two configurations are available. Both provide Go, Docker, kubectl, and Kind; the
Makefile downloads any other tools it needs into `bin/`.

| Configuration | Base image | What it adds |
| --- | --- | --- |
| [`devcontainer.json`](devcontainer.json) (default) | Debian with Docker-in-Docker | Go, kubectl, Helm, minikube, Kind, GitHub CLI, and Claude Code, as dev container features |
| [`operator/devcontainer.json`](operator/devcontainer.json) | `golang:1.26` | Kubebuilder's scaffolded setup: [`operator/post-install.sh`](operator/post-install.sh) installs Kind, Kubebuilder, and kubectl with bash completion |
