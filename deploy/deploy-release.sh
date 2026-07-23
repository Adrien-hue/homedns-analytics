#!/usr/bin/env bash

set -Eeuo pipefail

readonly PROJECT_ROOT="$(
  cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd
)"

readonly PI_HOST="${HOMEDNS_PI_HOST:-homedns}"
readonly PI_USER="${HOMEDNS_PI_USER:-joyteaser}"
readonly REMOTE_TMP="${HOMEDNS_REMOTE_TMP:-/tmp}"

fail() {
  echo "Deployment failed: $*" >&2
  exit 1
}

require_command() {
  local command_name="$1"

  command -v "${command_name}" >/dev/null 2>&1 ||
    fail "required command not found: ${command_name}"
}

find_latest_archive() {
  local archives=(/tmp/homedns-*.tar.gz)

  [[ -f "${archives[0]}" ]] ||
    fail "release archive was not created"

  printf '%s\n' "${archives[@]}" |
    sort |
    tail -n 1
}

cleanup_remote_installer() {
  local remote_installer="$1"

  ssh "${PI_USER}@${PI_HOST}" \
    "rm -f '${remote_installer}'" >/dev/null 2>&1 || true
}

main() {
  require_command ssh
  require_command scp

  cd "${PROJECT_ROOT}"

  echo "Building HomeDNS release package..."
  ./deploy/package-release.sh

  local archive
  archive="$(find_latest_archive)"

  local archive_name
  archive_name="$(basename "${archive}")"

  local remote_archive="${REMOTE_TMP}/${archive_name}"
  local remote_installer="${REMOTE_TMP}/homedns-install-release.sh"

  echo
  echo "Target: ${PI_USER}@${PI_HOST}"
  echo "Archive: ${archive_name}"

  trap 'cleanup_remote_installer "${remote_installer}"' EXIT

  echo
  echo "Copying release archive..."
  scp "${archive}" \
    "${PI_USER}@${PI_HOST}:${remote_archive}"

  echo "Copying release installer..."
  scp "${PROJECT_ROOT}/deploy/install-release.sh" \
    "${PI_USER}@${PI_HOST}:${remote_installer}"

  echo "Installing release..."
  ssh -t "${PI_USER}@${PI_HOST}" \
    "chmod 0755 '${remote_installer}' &&
     sudo '${remote_installer}' '${remote_archive}'"

  trap - EXIT
  cleanup_remote_installer "${remote_installer}"

  echo
  echo "HomeDNS deployment completed successfully."
}

main "$@"