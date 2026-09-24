#!/usr/bin/env bash
# Called by make test-superpod-deps. Install only commands that are missing.
set -euo pipefail
cd "$(dirname "$0")/.."

: "${LOCALBIN:=$PWD/bin}"
mkdir -p "$LOCALBIN"
LOCALBIN=$(cd "$LOCALBIN" && pwd)
export PATH="$LOCALBIN:$PATH"

fail() { echo "$*" >&2; exit 1; }

# Base utilities and the Docker runtime need the host package manager. All
# standalone development tools below are downloaded into the project bin dir.
install_package() {
  local package=$1
  local elevate=()
  if command -v brew >/dev/null; then
    brew install "$package"
    return
  fi
  if (( EUID != 0 )); then
    command -v sudo >/dev/null || fail "Installing $package requires root or sudo."
    elevate=(sudo)
  fi
  if command -v apt-get >/dev/null; then
    "${elevate[@]}" apt-get update
    "${elevate[@]}" apt-get install -y "$package"
  elif command -v dnf >/dev/null; then
    "${elevate[@]}" dnf install -y "$package"
  else
    fail "No supported package manager found. Install $package and rerun make test-superpod-deps."
  fi
}

command -v curl >/dev/null || install_package curl
command -v tar >/dev/null || install_package tar
command -v gzip >/dev/null || install_package gzip
if ! command -v sha256sum >/dev/null && ! command -v shasum >/dev/null; then
  install_package coreutils
fi

case "$(uname -s)" in
  Linux) os=linux ;;
  Darwin) os=darwin ;;
  *) fail "Automatic tool downloads support Linux and macOS only." ;;
esac
case "$(uname -m)" in
  x86_64|amd64) arch=amd64; rg_arch=x86_64 ;;
  aarch64|arm64) arch=arm64; rg_arch=aarch64 ;;
  *) fail "Unsupported architecture: $(uname -m)" ;;
esac

tempdir=$(mktemp -d "$LOCALBIN/.superpod-deps.XXXXXX")
trap 'rm -rf "$tempdir"' EXIT

download() {
  local url=$1 checksum_url=$2 expected actual
  curl --fail --silent --show-error --location --retry 3 "$url" -o "$tempdir/download"
  curl --fail --silent --show-error --location --retry 3 "$checksum_url" -o "$tempdir/checksum"
  expected=$(awk '{print $1; exit}' "$tempdir/checksum")
  if command -v sha256sum >/dev/null; then
    actual=$(sha256sum "$tempdir/download")
  else
    actual=$(shasum -a 256 "$tempdir/download")
  fi
  [[ "$expected" =~ ^[a-fA-F0-9]{64}$ && "${actual%% *}" == "$expected" ]] || fail "Checksum mismatch for $url"
}

needs_tool() {
  local command_name=$1 default_name=$2
  if command -v "$command_name" >/dev/null; then
    echo "Using $command_name: $(command -v "$command_name")"
    return 1
  fi
  # A typo in an explicit override should not silently select a different tool.
  [[ "$command_name" == "$default_name" ]] || fail "Configured command not found: $command_name"
  echo "Installing $default_name in $LOCALBIN"
}

if needs_tool go go; then
  url="https://dl.google.com/go/go${GO_VERSION:?}.$os-$arch.tar.gz"
  download "$url" "$url.sha256"
  tar -xzf "$tempdir/download" -C "$tempdir"
  mv "$tempdir/go" "$LOCALBIN/go-$GO_VERSION"
  ln -sf "$LOCALBIN/go-$GO_VERSION/bin/go" "$LOCALBIN/go"
  ln -sf "$LOCALBIN/go-$GO_VERSION/bin/gofmt" "$LOCALBIN/gofmt"
fi
if needs_tool "${KIND:-kind}" kind; then
  url="https://kind.sigs.k8s.io/dl/${KIND_VERSION:?}/kind-$os-$arch"
  download "$url" "$url.sha256sum"
  install -m 0755 "$tempdir/download" "$LOCALBIN/kind"
fi
if needs_tool "${KUBECTL:-kubectl}" kubectl; then
  url="https://dl.k8s.io/release/${KUBECTL_VERSION:?}/bin/$os/$arch/kubectl"
  download "$url" "$url.sha256"
  install -m 0755 "$tempdir/download" "$LOCALBIN/kubectl"
fi
if needs_tool "${HELM:-helm}" helm; then
  url="https://get.helm.sh/helm-${HELM_VERSION:?}-$os-$arch.tar.gz"
  download "$url" "$url.sha256"
  tar -xzf "$tempdir/download" -C "$tempdir"
  install -m 0755 "$tempdir/$os-$arch/helm" "$LOCALBIN/helm"
fi
if needs_tool "${RG:-rg}" rg; then
  case "$os/$arch" in
    linux/amd64) rg_target=x86_64-unknown-linux-musl ;;
    linux/arm64) rg_target=aarch64-unknown-linux-gnu ;;
    darwin/*) rg_target=$rg_arch-apple-darwin ;;
  esac
  archive="ripgrep-${RG_VERSION:?}-$rg_target"
  url="https://github.com/BurntSushi/ripgrep/releases/download/$RG_VERSION/$archive.tar.gz"
  download "$url" "$url.sha256"
  tar -xzf "$tempdir/download" -C "$tempdir"
  install -m 0755 "$tempdir/$archive/rg" "$LOCALBIN/rg"
fi

if ! command -v docker >/dev/null; then
  if [[ "$os" == darwin ]] && command -v brew >/dev/null; then
    brew install --cask docker
    open -a Docker
  elif command -v apt-get >/dev/null; then
    install_package docker.io
  else
    fail "Install Docker Engine or Docker Desktop, then rerun make test-superpod-deps."
  fi
fi
docker info >/dev/null 2>&1 || fail "Docker is installed but its daemon is not accessible. Start Docker and ensure your user can run 'docker info', then rerun make test-superpod."
echo "Superpod test dependencies are ready"
