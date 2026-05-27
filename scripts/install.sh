#!/usr/bin/env bash
# Install rmesh from GitHub Releases.
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/relaymonkey/rmesh-cli/main/scripts/install.sh | bash
#   RMESH_VERSION=v1.0.1 bash install.sh
#   RMESH_INSTALL_DIR=~/.local/bin bash install.sh
set -euo pipefail

REPO="${RMESH_REPO:-relaymonkey/rmesh-cli}"
GITHUB="${GITHUB:-https://github.com}"
API="https://api.github.com/repos/${REPO}"

err() {
  printf 'rmesh install: %s\n' "$*" >&2
  exit 1
}

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || err "missing required command: $1"
}

latest_tag() {
  curl -fsSL -H "Accept: application/vnd.github+json" "${API}/releases/latest" \
    | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' \
    | head -n1
}

detect_platform() {
  local raw_os raw_arch
  raw_os="$(uname -s)"
  raw_arch="$(uname -m)"

  case "${raw_os}" in
    Darwin) os=darwin ;;
    Linux) os=linux ;;
    *)
      err "unsupported OS: ${raw_os} (use scripts/install.ps1 on Windows)"
      ;;
  esac

  case "${raw_arch}" in
    x86_64 | amd64)
      arch=amd64
      suffix="${os}_${arch}"
      ;;
    aarch64 | arm64)
      arch=arm64
      suffix="${os}_${arch}"
      ;;
    armv7l | armv6l)
      if [ "${os}" != linux ]; then
        err "unsupported architecture on ${os}: ${raw_arch}"
      fi
      arch=arm
      suffix="linux_armv7"
      ;;
    *)
      err "unsupported architecture: ${raw_arch}"
      ;;
  esac

  ext=tar.gz
  bin_name=rmesh
}

resolve_install_dir() {
  if [ -n "${RMESH_INSTALL_DIR:-}" ]; then
    install_dir="${RMESH_INSTALL_DIR}"
    return
  fi

  install_dir="/usr/local/bin"
  if [ -w "${install_dir}" ] || [ "$(id -u)" -eq 0 ]; then
    return
  fi

  if command -v sudo >/dev/null 2>&1; then
    use_sudo=1
    return
  fi

  install_dir="${HOME}/.local/bin"
  use_sudo=0
}

install_binary() {
  local tag version artifact url tmpdir archive dest
  tag="${RMESH_VERSION:-}"
  if [ -z "${tag}" ]; then
    tag="$(latest_tag)" || err "could not resolve latest release (set RMESH_VERSION?)"
  fi
  version="${tag#v}"
  artifact="rmesh_${version}_${suffix}.${ext}"
  url="${GITHUB}/${REPO}/releases/download/${tag}/${artifact}"

  tmpdir="$(mktemp -d)"
  trap 'rm -rf "${tmpdir}"' EXIT
  archive="${tmpdir}/${artifact}"

  printf 'Downloading %s\n' "${url}"
  curl -fsSL -o "${archive}" "${url}" || err "download failed: ${url}"

  if [ "${ext}" = tar.gz ]; then
    tar -xzf "${archive}" -C "${tmpdir}" "${bin_name}"
  fi

  dest="${install_dir}/${bin_name}"
  if [ "${use_sudo:-0}" -eq 1 ] && [ ! -w "${install_dir}" ]; then
    sudo mkdir -p "${install_dir}"
    sudo install -m 0755 "${tmpdir}/${bin_name}" "${dest}"
  else
    mkdir -p "${install_dir}"
    install -m 0755 "${tmpdir}/${bin_name}" "${dest}"
  fi

  printf 'Installed rmesh %s to %s\n' "${version}" "${dest}"
  if ! command -v rmesh >/dev/null 2>&1; then
    printf 'Add %s to your PATH, then run: rmesh --version\n' "${install_dir}"
  fi
}

main() {
  need_cmd curl
  need_cmd uname
  detect_platform
  resolve_install_dir
  need_cmd tar
  install_binary
}

main "$@"
