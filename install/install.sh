#!/usr/bin/env sh
set -eu

REPO="${SPEC_KITTY_ANALYZER_REPO:-Priivacy-ai/spec-kitty-analyzer}"
VERSION="${SPEC_KITTY_ANALYZER_VERSION:-latest}"
BIN_NAME="spec-kitty-analyzer"
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
WORK_DIR=""

log() {
  printf '%s\n' "$*"
}

fail() {
  printf 'error: %s\n' "$*" >&2
  exit 1
}

cleanup() {
  if [ -n "${WORK_DIR}" ] && [ -d "${WORK_DIR}" ]; then
    rm -rf "${WORK_DIR}"
  fi
}
trap cleanup EXIT INT TERM

detect_os() {
  case "$(uname -s)" in
    Darwin) printf 'darwin' ;;
    Linux) printf 'linux' ;;
    *) fail "unsupported OS: $(uname -s)" ;;
  esac
}

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64) printf 'amd64' ;;
    arm64|aarch64) printf 'arm64' ;;
    *) fail "unsupported architecture: $(uname -m)" ;;
  esac
}

path_contains() {
  case ":${PATH}:" in
    *":$1:"*) return 0 ;;
    *) return 1 ;;
  esac
}

choose_install_dir() {
  if [ -n "${SPEC_KITTY_ANALYZER_BIN_DIR:-}" ]; then
    printf '%s' "${SPEC_KITTY_ANALYZER_BIN_DIR}"
    return
  fi
  if path_contains "${HOME}/.local/bin"; then
    printf '%s' "${HOME}/.local/bin"
    return
  fi
  if path_contains "/usr/local/bin" && [ -w "/usr/local/bin" ]; then
    printf '%s' "/usr/local/bin"
    return
  fi
  printf '%s' "${HOME}/.local/bin"
}

ensure_path() {
  dir="$1"
  if path_contains "${dir}"; then
    return
  fi
  if [ "${SPEC_KITTY_ANALYZER_NO_PATH_EDIT:-}" = "1" ]; then
    log "${dir} is not on PATH; SPEC_KITTY_ANALYZER_NO_PATH_EDIT=1 skipped shell profile update."
    return
  fi

  shell_name=$(basename "${SHELL:-sh}")
  case "${shell_name}" in
    zsh) profile="${HOME}/.zshrc" ;;
    bash) profile="${HOME}/.bashrc" ;;
    *) profile="${HOME}/.profile" ;;
  esac

  touch "${profile}"
  if ! grep -F "spec-kitty-analyzer installer" "${profile}" >/dev/null 2>&1; then
    {
      printf '\n# spec-kitty-analyzer installer\n'
      printf 'export PATH="%s:$PATH"\n' "${dir}"
    } >> "${profile}"
  fi
  PATH="${dir}:${PATH}"
  export PATH
  log "Added ${dir} to PATH in ${profile}. Open a new shell to inherit it."
}

download() {
  url="$1"
  dest="$2"
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$url" -o "$dest"
    return
  fi
  if command -v wget >/dev/null 2>&1; then
    wget -q "$url" -O "$dest"
    return
  fi
  fail "curl or wget is required to download ${url}"
}

asset_url() {
  os_name="$1"
  arch="$2"
  asset="${BIN_NAME}_${os_name}_${arch}.tar.gz"
  if [ "${VERSION}" = "latest" ]; then
    printf 'https://github.com/%s/releases/latest/download/%s' "${REPO}" "${asset}"
  else
    printf 'https://github.com/%s/releases/download/%s/%s' "${REPO}" "${VERSION}" "${asset}"
  fi
}

find_local_binary() {
  for candidate in \
    "${SCRIPT_DIR}/${BIN_NAME}" \
    "${SCRIPT_DIR}/../${BIN_NAME}" \
    "${SCRIPT_DIR}/bin/${BIN_NAME}" \
    "${SCRIPT_DIR}/../bin/${BIN_NAME}"; do
    if [ -f "${candidate}" ]; then
      printf '%s' "${candidate}"
      return
    fi
  done
}

find_skill_src() {
  for candidate in \
    "${SCRIPT_DIR}/skills/spec-kitty-analyzer/SKILL.md" \
    "${SCRIPT_DIR}/../skills/spec-kitty-analyzer/SKILL.md" \
    "${WORK_DIR}/skills/spec-kitty-analyzer/SKILL.md"; do
    if [ -f "${candidate}" ]; then
      printf '%s' "${candidate}"
      return
    fi
  done
}

install_skill() {
  root="$1"
  src="$2"
  if [ ! -d "${root}" ]; then
    return
  fi
  dest="${root}/spec-kitty-analyzer"
  mkdir -p "${dest}"
  cp "${src}" "${dest}/SKILL.md"
  chmod 0644 "${dest}/SKILL.md"
  log "Installed skill: ${dest}/SKILL.md"
}

install_dir=$(choose_install_dir)
mkdir -p "${install_dir}"

binary=$(find_local_binary || true)
if [ -z "${binary}" ]; then
  os_name=$(detect_os)
  arch=$(detect_arch)
  WORK_DIR=$(mktemp -d 2>/dev/null || mktemp -d -t spec-kitty-analyzer)
  archive="${WORK_DIR}/${BIN_NAME}.tar.gz"
  url=$(asset_url "${os_name}" "${arch}")
  log "Downloading ${url}"
  download "${url}" "${archive}"
  tar -xzf "${archive}" -C "${WORK_DIR}"
  binary="${WORK_DIR}/${BIN_NAME}"
fi

[ -f "${binary}" ] || fail "could not locate ${BIN_NAME} binary"
install -m 0755 "${binary}" "${install_dir}/${BIN_NAME}"
ensure_path "${install_dir}"
log "Installed CLI: ${install_dir}/${BIN_NAME}"

skill_src=$(find_skill_src || true)
if [ -n "${skill_src}" ]; then
  install_skill "${HOME}/.agents/skills" "${skill_src}"
  install_skill "${HOME}/.claude/skills" "${skill_src}"
else
  log "Skill source not found; CLI installed without agent skill."
fi

"${install_dir}/${BIN_NAME}" --version
