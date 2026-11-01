#!/bin/sh
# Writes the Homebrew formula for a release, from the checksums that
# scripts/release.sh produced.
#
#   scripts/formula.sh v0.1.0 [download-base-url]
#
# The base URL defaults to the GitHub release; pass a file:// URL to test
# the formula against packages built locally.
set -eu

tag="${1:?usage: scripts/formula.sh v0.1.0 [base-url]}"
version="${tag#v}"
root="$(cd "$(dirname "$0")/.." && pwd)"
base="${2:-https://github.com/bkodzo/po/releases/download/${tag}}"

sums="$root/dist/checksums.txt"
sha_for() { awk -v f="po-${tag}-darwin-$1.tar.gz" '$2 == f {print $1}' "$sums"; }
arm="$(sha_for arm64)"
amd="$(sha_for amd64)"
[ -n "$arm" ] && [ -n "$amd" ] || { echo "checksums for $tag not found in $sums; run scripts/release.sh $tag first" >&2; exit 1; }

sed -e "s|@VERSION@|${version}|g" \
    -e "s|@BASE@|${base}|g" \
    -e "s|@SHA_ARM64@|${arm}|g" \
    -e "s|@SHA_AMD64@|${amd}|g" \
    "$root/packaging/homebrew/po.rb.in"
