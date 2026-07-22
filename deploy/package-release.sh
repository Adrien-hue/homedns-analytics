#!/usr/bin/env bash

set -Eeuo pipefail

PROJECT_ROOT="$(git rev-parse --show-toplevel)"
cd "${PROJECT_ROOT}"

RELEASE_ID="${RELEASE_ID:-$(date +%Y%m%d-%H%M%S)}"
COMMIT_SHA="$(git rev-parse --short=12 HEAD)"
ARCHIVE_PATH="${1:-/tmp/homedns-${RELEASE_ID}.tar.gz}"

if [[ -n "$(git status --porcelain)" ]]; then
  echo "Error: the working tree is not clean." >&2
  echo "Commit or discard changes before packaging a release." >&2
  exit 1
fi

STAGING_DIR="$(mktemp -d)"

cleanup() {
  rm -rf "${STAGING_DIR}"
}

trap cleanup EXIT

mkdir -p \
  "${STAGING_DIR}/scripts/benchmarks"

# Current Sprint 01 runtime payload.
cp "${PROJECT_ROOT}/Makefile" \
  "${STAGING_DIR}/Makefile"

cp -R "${PROJECT_ROOT}/scripts/benchmarks/." \
  "${STAGING_DIR}/scripts/benchmarks/"

# Traceability without deploying the Git repository.
cat > "${STAGING_DIR}/RELEASE" <<EOF
release_id=${RELEASE_ID}
commit_sha=${COMMIT_SHA}
created_at=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
EOF

tar \
  --create \
  --gzip \
  --file="${ARCHIVE_PATH}" \
  --directory="${STAGING_DIR}" \
  .

echo "Release package created"
echo "Release ID: ${RELEASE_ID}"
echo "Commit:     ${COMMIT_SHA}"
echo "Archive:    ${ARCHIVE_PATH}"