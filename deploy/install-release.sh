#!/usr/bin/env bash

set -Eeuo pipefail

readonly INSTALL_ROOT="${HOMEDNS_INSTALL_ROOT:-/opt/homedns}"
readonly APP_USER="${HOMEDNS_APP_USER:-homedns}"
readonly APP_GROUP="${HOMEDNS_APP_GROUP:-homedns}"

fail() {
  echo "Installation failed: $*" >&2
  exit 1
}

require_root() {
  if [[ "${EUID}" -ne 0 ]]; then
    fail "this script must be run as root"
  fi
}

validate_archive() {
  local archive="$1"

  [[ -f "${archive}" ]] ||
    fail "archive not found: ${archive}"

  tar -tzf "${archive}" >/dev/null ||
    fail "invalid archive: ${archive}"

  tar -tzf "${archive}" | grep -qx './RELEASE' ||
    fail "archive does not contain ./RELEASE"
}

read_release_value() {
  local archive="$1"
  local key="$2"

  tar -xOf "${archive}" ./RELEASE |
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

normalize_permissions() {
  local release_dir="$1"

  chown -R "${APP_USER}:${APP_GROUP}" "${release_dir}"

  find "${release_dir}" -type d -exec chmod 0755 {} +
  find "${release_dir}" -type f -exec chmod 0644 {} +
  find "${release_dir}" -type f -name '*.sh' -exec chmod 0755 {} +
}

configure_shared_benchmarks() {
  local release_dir="$1"
  local release_benchmarks="${release_dir}/benchmarks"
  local shared_benchmarks="${INSTALL_ROOT}/shared/benchmarks"

  if [[ -d "${release_benchmarks}" && ! -L "${release_benchmarks}" ]]; then
    cp -a "${release_benchmarks}/." "${shared_benchmarks}/"
  fi

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

main() {
  require_root

  [[ "$#" -eq 1 ]] ||
    fail "usage: $0 /path/to/homedns-<release-id>.tar.gz"

  local archive="$1"

  validate_archive "${archive}"

  local release_id
  release_id="$(read_release_value "${archive}" "release_id")"

  [[ -n "${release_id}" ]] ||
    fail "release_id is missing from RELEASE"

  [[ "${release_id}" =~ ^[0-9]{8}-[0-9]{6}$ ]] ||
    fail "invalid release_id: ${release_id}"

  id "${APP_USER}" >/dev/null 2>&1 ||
    fail "application user does not exist: ${APP_USER}"

  getent group "${APP_GROUP}" >/dev/null 2>&1 ||
    fail "application group does not exist: ${APP_GROUP}"

  prepare_layout

  local release_dir="${INSTALL_ROOT}/releases/${release_id}"

  [[ ! -e "${release_dir}" ]] ||
    fail "release already exists: ${release_dir}"

  echo "Installing HomeDNS release ${release_id}..."

  install -d -o "${APP_USER}" -g "${APP_GROUP}" -m 0755 \
    "${release_dir}"

  if ! runuser -u "${APP_USER}" -- \
    tar -xzf "${archive}" -C "${release_dir}"; then
    rm -rf "${release_dir}"
    fail "archive extraction failed"
  fi

  normalize_permissions "${release_dir}"
  configure_shared_benchmarks "${release_dir}"
  activate_release "${release_dir}"

  rm -f "${archive}"

  echo
  echo "Installation completed."
  echo "Release: ${release_id}"
  echo "Current: $(readlink -f "${INSTALL_ROOT}/current")"
  echo
  cat "${INSTALL_ROOT}/current/RELEASE"
}

main "$@"