#!/usr/bin/env bash
#
# build.sh - Build or install af with a real version, commit, and build date
# stamped in via ldflags.
#
# Why this exists: cmd/af/version.go's VersionInfo/GitCommit/BuildDate are
# ldflags-settable so a released binary can carry real provenance. A plain
# `go build ./cmd/af` (no ldflags) still gets a correct VERSION number from
# the source default in version.go, but GitCommit/BuildDate stay "unknown"
# unless this script (or an equivalent ldflags invocation) is used. rk's
# `rk doctor` (D6 stale-binary detection) parses the version this stamps.
#
# The version string itself is never hardcoded here — it's read out of
# cmd/af/version.go so there is exactly one place that can drift (guarded by
# TestVersionInfo_MatchesLatestChangelogEntry in cmd/af/version_test.go).
#
# Usage:
#   scripts/build.sh          # go build -> ./af
#   scripts/build.sh build    # same as above
#   scripts/build.sh install  # go install -> $GOBIN (or $GOPATH/bin, or ~/go/bin)

set -euo pipefail
cd "$(dirname "$0")/.."

VERSION=$(grep -oP '(?<=VersionInfo = ")[^"]+' cmd/af/version.go)
if [[ -z "$VERSION" ]]; then
  echo "build.sh: could not read VersionInfo default from cmd/af/version.go" >&2
  exit 1
fi

COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS="-X main.VersionInfo=${VERSION} -X main.GitCommit=${COMMIT} -X main.BuildDate=${DATE}"

MODE="${1:-build}"
case "$MODE" in
  build)
    go build -ldflags "$LDFLAGS" -o af ./cmd/af
    echo "built ./af  (version ${VERSION}, commit ${COMMIT}, built ${DATE})"
    ;;
  install)
    go install -ldflags "$LDFLAGS" ./cmd/af
    GOBIN_DIR=$(go env GOBIN)
    if [[ -z "$GOBIN_DIR" ]]; then
      GOBIN_DIR="$(go env GOPATH)/bin"
    fi
    echo "installed af to ${GOBIN_DIR}/af  (version ${VERSION}, commit ${COMMIT}, built ${DATE})"
    ;;
  *)
    echo "usage: $0 [build|install]" >&2
    exit 1
    ;;
esac
