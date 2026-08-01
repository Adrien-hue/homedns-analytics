#!/usr/bin/env bash

set -Eeuo pipefail

PROJECT_ROOT="$(git rev-parse --show-toplevel)"
cd "${PROJECT_ROOT}"

RELEASE_ID="${RELEASE_ID:-$(date +%Y%m%d-%H%M%S)}"
COMMIT_SHA="$(git rev-parse --short=12 HEAD)"
CREATED_AT="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"

VERSION="${HOMEDNS_VERSION:-v0.2.0-rc2}"

ARCHIVE_PATH="${1:-/tmp/homedns-${RELEASE_ID}.tar.gz}"

DNS_BINARY_NAME="homedns-dns"
DNS_BINARY_PATH="${PROJECT_ROOT}/dns/${DNS_BINARY_NAME}"

MODULE_PATH="github.com/Adrien-hue/homedns-analytics/dns"

fail() {
  echo "Error: $*" >&2
  exit 1
}

require_command() {
  local command_name="$1"

  command -v "${command_name}" >/dev/null 2>&1 ||
    fail "required command not found: ${command_name}"
}

validate_working_tree() {
  if [[ -n "$(git status --porcelain)" ]]; then
    fail "the working tree is not clean.
Commit or discard changes before packaging a release."
  fi
}

build_dns_binary() {
  echo "Building Linux ARM64 DNS binary..."

  (
    cd "${PROJECT_ROOT}/dns"

    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=arm64 \
    go build \
      -trimpath \
      -ldflags="-s -w \
        -X ${MODULE_PATH}/internal/version.Version=${VERSION} \
        -X ${MODULE_PATH}/internal/version.Commit=${COMMIT_SHA} \
        -X ${MODULE_PATH}/internal/version.BuildTime=${CREATED_AT}" \
      -o "${DNS_BINARY_PATH}" \
      ./cmd/homedns-dns
  )
}

validate_dns_binary() {
  [[ -f "${DNS_BINARY_PATH}" ]] ||
    fail "DNS binary was not created: ${DNS_BINARY_PATH}"

  [[ -x "${DNS_BINARY_PATH}" ]] ||
    fail "DNS binary is not executable: ${DNS_BINARY_PATH}"

  local file_output
  file_output="$(file "${DNS_BINARY_PATH}")"

  [[ "${file_output}" == *"ELF 64-bit"* ]] ||
    fail "DNS binary is not a 64-bit ELF executable: ${file_output}"

  [[ "${file_output}" == *"ARM aarch64"* ]] ||
    fail "DNS binary is not built for ARM64: ${file_output}"

  [[ "${file_output}" == *"statically linked"* ]] ||
    fail "DNS binary is not statically linked: ${file_output}"
}

write_release_metadata() {
  local destination="$1"

  cat > "${destination}" <<EOF
release_id=${RELEASE_ID}
version=${VERSION}
commit_sha=${COMMIT_SHA}
created_at=${CREATED_AT}
dns_binary=${DNS_BINARY_NAME}
EOF
}

create_archive() {
  local staging_dir="$1"

  mkdir -p \
    "${staging_dir}/scripts/benchmarks"

  install \
    -m 0755 \
    "${DNS_BINARY_PATH}" \
    "${staging_dir}/${DNS_BINARY_NAME}"

  install \
    -m 0644 \
    "${PROJECT_ROOT}/Makefile" \
    "${staging_dir}/Makefile"

  cp -R \
    "${PROJECT_ROOT}/scripts/benchmarks/." \
    "${staging_dir}/scripts/benchmarks/"

  write_release_metadata \
    "${staging_dir}/RELEASE"

  tar \
    --create \
    --gzip \
    --file="${ARCHIVE_PATH}" \
    --directory="${staging_dir}" \
    .
}

validate_archive() {
  tar -tzf "${ARCHIVE_PATH}" >/dev/null ||
    fail "release archive is invalid: ${ARCHIVE_PATH}"

  for required_path in \
    "./RELEASE" \
    "./Makefile" \
    "./homedns-dns" \
    "./scripts/benchmarks/benchmark.sh" \
    "./scripts/benchmarks/collect-baseline.sh"; do
    tar -tzf "${ARCHIVE_PATH}" |
      grep -qx "${required_path}" ||
      fail "release archive is missing ${required_path}"
  done
}

main() {
  require_command git
  require_command go
  require_command file
  require_command tar

  validate_working_tree

  local staging_dir
  staging_dir="$(mktemp -d)"

  cleanup() {
    rm -rf "${staging_dir}"
    rm -f "${DNS_BINARY_PATH}"
  }

  trap cleanup EXIT

  build_dns_binary
  validate_dns_binary
  create_archive "${staging_dir}"
  validate_archive

  echo
  echo "Release package created"
  echo "Release ID: ${RELEASE_ID}"
  echo "Version:    ${VERSION}"
  echo "Commit:     ${COMMIT_SHA}"
  echo "Archive:    ${ARCHIVE_PATH}"
}

main "$@"