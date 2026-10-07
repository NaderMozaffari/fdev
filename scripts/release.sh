#!/bin/sh
# Builds the release archives into dist/: fdev for each OS and CPU, its
# checksums, and the install scripts set to the repository they install from.
#
#   scripts/release.sh v0.2.0-beta.1               build only
#   scripts/release.sh v0.2.0-beta.1 --publish     and make the GitHub release (gh)
#
# Versions are semantic (semver.org): v1.0.0 is a stable release, and one
# with a pre-release part (v0.2.0-beta.1, v1.0.0-rc.1) a beta, published as
# a GitHub pre-release so that installs and updates skip it unless asked
# for betas.
#
# RELEASE_REPO (owner/name, default this repository) is where the release
# goes, and what install scripts and `fdev update` download from: a public
# repository holding only releases lets anyone install without a token while
# the source stays private.
set -eu

version=${1:?usage: scripts/release.sh vX.Y.Z [--publish]}
case $version in v*) ;; *) version=v$version ;; esac
if ! printf '%s\n' "$version" | grep -Eq '^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z.-]+)?$'; then
	echo "release: $version isn't a semantic version: v1.2.3, or v1.2.3-beta.1 for a beta" >&2
	exit 2
fi
case $version in
*-*) kind=--prerelease ;; # a beta
*) kind=--latest ;;
esac
publish=${2:-}
repo=${RELEASE_REPO:-$(gh repo view --json nameWithOwner -q .nameWithOwner 2>/dev/null || echo NaderMozaffari/fdev)}

cd "$(dirname "$0")/.."
rm -rf dist && mkdir -p dist

for target in darwin/amd64 darwin/arm64 linux/amd64 linux/arm64 windows/amd64 windows/arm64; do
	os=${target%/*} arch=${target#*/}
	name=fdev_${os}_${arch}
	exe=fdev
	[ "$os" = windows ] && exe=fdev.exe
	echo "building $name"
	mkdir -p "dist/$name"
	CGO_ENABLED=0 GOOS=$os GOARCH=$arch go build -trimpath \
		-ldflags "-s -w -X main.version=$version -X main.releaseRepo=$repo" \
		-o "dist/$name/$exe" .
	cp LICENSE "dist/$name/"
	if [ "$os" = windows ]; then
		(cd "dist/$name" && zip -q "../$name.zip" ./*)
	else
		tar -czf "dist/$name.tar.gz" -C "dist/$name" fdev LICENSE
	fi
	rm -rf "dist/$name"
done

# The install scripts install from the repository the release is in.
sed "s|^DEFAULT_REPO=.*|DEFAULT_REPO=$repo|" scripts/install.sh >dist/install.sh
sed "s|^\$DefaultRepo = .*|\$DefaultRepo = '$repo'|" scripts/install.ps1 >dist/install.ps1

(cd dist && if command -v sha256sum >/dev/null; then sha256sum -- *.zip *.tar.gz; else shasum -a 256 -- *.zip *.tar.gz; fi >checksums.txt)
echo "dist/ ready for $repo $version"

[ "$publish" = --publish ] || exit 0

prev=$(git describe --tags --abbrev=0 "$version^" 2>/dev/null || true)
notes=$(git log --no-merges --pretty='- %s' ${prev:+"$prev.."}"$version" 2>/dev/null || true)
title="fdev $version"
[ "$kind" = --prerelease ] && title="$title (beta)"
gh release create "$version" dist/*.zip dist/*.tar.gz dist/checksums.txt dist/install.sh dist/install.ps1 \
	--repo "$repo" --title "$title" --notes "${notes:-fdev $version}" "$kind"
