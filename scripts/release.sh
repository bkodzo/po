#!/bin/sh
# Builds the release packages into dist/: one tarball per Mac architecture,
# each holding po, po-view and po-window, plus a checksums file.
#
#   scripts/release.sh v0.1.0
#
# It runs the same way on a developer's Mac and in the release workflow.
set -eu

version="${1:?usage: scripts/release.sh v0.1.0}"
root="$(cd "$(dirname "$0")/.." && pwd)"
out="$root/dist"

cd "$root"
rm -rf "$out"
mkdir -p "$out"

build_one() {
	goarch="$1" swiftarch="$2"
	name="po-${version}-darwin-${goarch}"
	stage="$out/$name"
	mkdir -p "$stage"

	echo "== $name"
	for cmd in po po-view; do
		GOOS=darwin GOARCH="$goarch" CGO_ENABLED=0 \
			go build -trimpath -ldflags "-s -w -X main.version=${version}" -o "$stage/$cmd" "./cmd/$cmd"
	done

	swift build -c release --package-path desktop --arch "$swiftarch" >/dev/null
	bin="$(swift build -c release --package-path desktop --arch "$swiftarch" --show-bin-path)"
	cp "$bin/po-window" "$stage/po-window"

	cat > "$stage/INSTALL.txt" <<TXT
Po ${version}

Put po, po-window and po-view in a folder on your PATH, for example:

    mkdir -p ~/.local/bin
    cp po po-window po-view ~/.local/bin/

Then run:

    po init

It explains what it will change, asks first, and keeps a backup.

If macOS says it cannot verify these programs, they are not signed yet. Allow
them with:

    xattr -dr com.apple.quarantine ~/.local/bin/po ~/.local/bin/po-window ~/.local/bin/po-view

Installing with Homebrew does not need that step.
TXT

	tar -C "$out" -czf "$out/$name.tar.gz" "$name"
	rm -rf "$stage"
}

build_one arm64 arm64
build_one amd64 x86_64

(cd "$out" && shasum -a 256 ./*.tar.gz | sed 's| \./| |' > checksums.txt)
echo
echo "== built:"
ls -l "$out"
echo
cat "$out/checksums.txt"
