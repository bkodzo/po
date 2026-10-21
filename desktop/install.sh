#!/bin/sh
# Builds the desktop window and installs it next to the po command.
set -e
cd "$(dirname "$0")"
swift build -c release
dest="$(go env GOBIN)"
[ -n "$dest" ] || dest="$(go env GOPATH)/bin"
mkdir -p "$dest"
cp .build/release/po-window "$dest/po-window"
echo "installed $dest/po-window"
echo "start it with: po window"
