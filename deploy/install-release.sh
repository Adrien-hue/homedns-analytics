#!/usr/bin/env bash

set -Eeuo pipefail

readonly INSTALL_ROOT="${HOMEDNS_INSTALL_ROOT:-/opt/homedns}"
readonly APP_USER="${HOMEDNS_APP_USER:-homedns}"
readonly APP_GROUP="${HOMEDNS_APP_GROUP:-homedns}"

readonly DNS_SERVICE="${HOMEDNS_DNS_SERVICE:-homedns-dns.service}"
readonly HEALTH_URL="${HOMEDNS_HEALTH_URL:-http://127.0.0.1:8081/health}"

readonly HEALTH_ATTEMPTS="${HOMEDNS_HEALTH_ATTEMPTS:-15}"
readonly HEALTH_DELAY_SECONDS="${HOMEDNS_HEALTH_DELAY_SECONDS:-1}"

readonly DNS_BINARY_NAME="homedns-dns"

fail() {
  echo "Installation failed: $*" >&2
  exit 1
}

require_command() {
  local command_name="$1"

  command -v "${command_name}" >/dev/null 2>&1 ||
    fail "required command not found: ${command_name}"
}

require_root() {
  if [[ "${EUID}" -ne 0 ]]; then
    fail "this script must be run as root"
  fi
}

archive_contains() {
  local archive="$1"
  local expected_path="$2"

  tar -tzf "${archive}" |
    sed 's#^\./##' |
    grep -Fqx "${expected_path}"
}

validate_archive() {
  local archive="$1"

  [[ -f "${archive}" ]] ||
    fail "archive not found: ${archive}"

  tar -tzf "${archive}" >/dev/null ||
    fail "invalid archive: ${archive}"

  local required_path

  for required_path in \
    "RELEASE" \
    "${DNS_BINARY_NAME}"; do
    archive_contains \
      "${archive}" \
      "${required_path}" ||
      fail "archive does not contain ${required_path}"
  done
}

read_release_value() {
  local archive="$1"
  local key="$2"
  local release_content

  if ! release_content="$(
    tar -xOf "${archive}" RELEASE 2>/dev/null
  )"; then
    release_content="$(
      tar -xOf "${archive}" ./RELEASE
    )"
  fi

  printf '%s\n' "${release_content}" |
    awk -F= -v wanted_key="${key}" '
      $1 == wanted_key {
        print substr($0, index($0, "=") + 1)
        exit
      }
    '
}

prepare_layout() {
  install -d -o "${APP_USER}" -g "${APP_GROUP}" -m 0755 \
    "${INSTALL_ROOT}" \
    "${INSTALL_ROOT}/releases"

  install -d -o "${APP_USER}" -g "${APP_GROUP}" -m 0750 \
    "${INSTALL_ROOT}/shared" \
    "${INSTALL_ROOT}/shared/config" \
    "${INSTALL_ROOT}/shared/data" \
    "${INSTALL_ROOT}/shared/logs"

  install -d -o "${APP_USER}" -g "${APP_GROUP}" -m 0775 \
    "${INSTALL_ROOT}/shared/benchmarks"
}

migrate_existing_benchmarks() {
  local current_link="${INSTALL_ROOT}/current"
  local shared_benchmarks="${INSTALL_ROOT}/shared/benchmarks"

  [[ -e "${current_link}" ]] || return 0

  local current_release
  current_release="$(readlink -f "${current_link}")"

  local existing_benchmarks="${current_release}/benchmarks"

  [[ -d "${existing_benchmarks}" ]] || return 0
  [[ ! -L "${existing_benchmarks}" ]] || return 0

  echo "Migrating existing benchmark history..."

  cp -a "${existing_benchmarks}/." "${shared_benchmarks}/"
  chown -R "${APP_USER}:${APP_GROUP}" "${shared_benchmarks}"
}

normalize_permissions() {
  local release_dir="$1"

  chown -R "${APP_USER}:${APP_GROUP}" "${release_dir}"

  find "${release_dir}" -type d -exec chmod 0755 {} +
  find "${release_dir}" -type f -exec chmod 0644 {} +
  find "${release_dir}" -type f -name '*.sh' -exec chmod 0755 {} +

  chmod 0755 "${release_dir}/${DNS_BINARY_NAME}"
}

validate_extracted_release() {
  local release_dir="$1"

  [[ -f "${release_dir}/RELEASE" ]] ||
    fail "extracted release does not contain RELEASE"

  [[ -f "${release_dir}/${DNS_BINARY_NAME}" ]] ||
    fail "extracted release does not contain ${DNS_BINARY_NAME}"

  [[ -x "${release_dir}/${DNS_BINARY_NAME}" ]] ||
    fail "extracted DNS binary is not executable"

  file "${release_dir}/${DNS_BINARY_NAME}" |
    grep -q 'ELF 64-bit.*ARM aarch64' ||
    fail "extracted DNS binary is not a Linux ARM64 executable"
}

configure_shared_benchmarks() {
  local release_dir="$1"
  local release_benchmarks="${release_dir}/benchmarks"
  local shared_benchmarks="${INSTALL_ROOT}/shared/benchmarks"

  rm -rf "${release_benchmarks}"
  ln -s "${shared_benchmarks}" "${release_benchmarks}"
  chown -h "${APP_USER}:${APP_GROUP}" "${release_benchmarks}"
}

activate_release() {
  local release_dir="$1"
  local next_link="${INSTALL_ROOT}/current.next"
  local current_link="${INSTALL_ROOT}/current"

  rm -f "${next_link}"

  ln -s "${release_dir}" "${next_link}"
  chown -h "${APP_USER}:${APP_GROUP}" "${next_link}"

  mv -Tf "${next_link}" "${current_link}"
}

restart_service() {
  systemctl reset-failed "${DNS_SERVICE}" || true
  systemctl restart "${DNS_SERVICE}"
}

wait_for_health() {
  local attempt
  local response

  for ((attempt = 1; attempt <= HEALTH_ATTEMPTS; attempt++)); do
    response="$(
      curl \
        --fail \
        --silent \
        --max-time 2 \
        "${HEALTH_URL}" \
        2>/dev/null
    )" || response=""

    if [[ -n "${response}" ]] &&
      grep -Eq '"status"[[:space:]]*:[[:space:]]*"ok"' \
        <<<"${response}" &&
      grep -Eq '"ready"[[:space:]]*:[[:space:]]*true' \
        <<<"${response}"; then
      echo "Health check passed after ${attempt} attempt(s)."
      return 0
    fi

    if ((attempt < HEALTH_ATTEMPTS)); then
      sleep "${HEALTH_DELAY_SECONDS}"
    fi
  done

  return 1
}

rollback_release() {
  local previous_release="$1"

  echo
  echo "Deployment validation failed. Rolling back..." >&2

  if [[ -n "${previous_release}" &&
        -d "${previous_release}" &&
        -x "${previous_release}/${DNS_BINARY_NAME}" ]]; then
    activate_release "${previous_release}"

    if restart_service &&
      wait_for_health; then
      echo "Rollback completed successfully." >&2
      echo "Current: $(readlink -f "${INSTALL_ROOT}/current")" >&2
      return 0
    fi

    echo "Rollback release could not be restored cleanly." >&2
    return 1
  fi

  echo "No valid previous release is available for rollback." >&2
  systemctl stop "${DNS_SERVICE}" || true

  return 1
}

main() {
  require_root

  require_command awk
  require_command curl
  require_command file
  require_command getent
  require_command runuser
  require_command systemctl
  require_command tar

  [[ "$#" -eq 1 ]] ||
    fail "usage: $0 /path/to/homedns-<release-id>.tar.gz"

  local archive="$1"

  validate_archive "${archive}"

  local release_id
  release_id="$(read_release_value "${archive}" "release_id")"

  local version
  version="$(read_release_value "${archive}" "version")"

  local commit_sha
  commit_sha="$(read_release_value "${archive}" "commit_sha")"

  [[ -n "${release_id}" ]] ||
    fail "release_id is missing from RELEASE"

  [[ "${release_id}" =~ ^[0-9]{8}-[0-9]{6}$ ]] ||
    fail "invalid release_id: ${release_id}"

  [[ -n "${version}" ]] ||
    fail "version is missing from RELEASE"

  [[ -n "${commit_sha}" ]] ||
    fail "commit_sha is missing from RELEASE"

  id "${APP_USER}" >/dev/null 2>&1 ||
    fail "application user does not exist: ${APP_USER}"

  getent group "${APP_GROUP}" >/dev/null 2>&1 ||
    fail "application group does not exist: ${APP_GROUP}"

  prepare_layout
  migrate_existing_benchmarks

  local release_dir="${INSTALL_ROOT}/releases/${release_id}"

  [[ ! -e "${release_dir}" ]] ||
    fail "release already exists: ${release_dir}"

  local current_link="${INSTALL_ROOT}/current"
  local previous_release=""

  if [[ -L "${current_link}" ]]; then
    previous_release="$(readlink -f "${current_link}")"
  fi

  echo "Installing HomeDNS release ${release_id}..."
  echo "Version: ${version}"
  echo "Commit:  ${commit_sha}"

  install -d -o "${APP_USER}" -g "${APP_GROUP}" -m 0755 \
    "${release_dir}"

  if ! runuser -u "${APP_USER}" -- \
    tar -xzf "${archive}" -C "${release_dir}"; then
    rm -rf "${release_dir}"
    fail "archive extraction failed"
  fi

  normalize_permissions "${release_dir}"

  if ! validate_extracted_release "${release_dir}"; then
    rm -rf "${release_dir}"
    fail "release validation failed"
  fi

  configure_shared_benchmarks "${release_dir}"

  echo "Activating release..."
  activate_release "${release_dir}"

  echo "Restarting ${DNS_SERVICE}..."

  if ! restart_service; then
    rollback_release "${previous_release}" || true
    fail "service restart failed"
  fi

  echo "Waiting for health check..."

  if ! wait_for_health; then
    systemctl status "${DNS_SERVICE}" \
      --no-pager \
      --full >&2 || true

    journalctl -u "${DNS_SERVICE}" \
      --since "-2 minutes" \
      --no-pager >&2 || true

    rollback_release "${previous_release}" || true
    fail "health check failed: ${HEALTH_URL}"
  fi

  rm -f "${archive}"

  echo
  echo "Installation completed."
  echo "Release: ${release_id}"
  echo "Version: ${version}"
  echo "Commit:  ${commit_sha}"
  echo "Current: $(readlink -f "${INSTALL_ROOT}/current")"
  echo
  cat "${INSTALL_ROOT}/current/RELEASE"
}

main "$@"