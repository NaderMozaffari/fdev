#!/bin/sh
# Builds fdev's installers (installer/) into dist/:
#
#   fdev_macos.dmg          Install fdev.app, one app for Intel and Apple silicon
#   fdev_windows_setup.exe  for Windows on x64 (and on ARM, emulated)
#
# Each carries the release's fdev archive for its OS and CPU, so it installs
# without the internet; online, it downloads a newer release of the channel
# picked from RELEASE_REPO (as in release.sh). The archives come from dist/
# (release.sh builds them), or with --publish from the published release,
# so the installers carry the very files the release lists. Needs macOS with
# Xcode's command line tools; the Windows one is cross-compiled.
#
#   scripts/installer.sh v0.2.0-beta.1             build only
#   scripts/installer.sh v0.2.0-beta.1 --publish   and add them to the GitHub release
#
# Signing the Mac app: set MACOS_SIGN_IDENTITY to a "Developer ID
# Application: …" certificate in your keychain, and MACOS_NOTARY_PROFILE to
# a profile made once with `xcrun notarytool store-credentials`. The .dmg is
# then signed, notarized and stapled: macOS opens it without a warning, and
# firewalls such as Little Snitch show who made it. Without them the app gets
# an ad-hoc signature (Apple silicon runs nothing without one), and macOS
# asks to confirm it the first time (right-click › Open). The Windows .exe
# isn't signed, so SmartScreen asks too.
set -eu

version=${1:?usage: scripts/installer.sh vX.Y.Z [--publish]}
case $version in v*) ;; *) version=v$version ;; esac
publish=${2:-}
repo=${RELEASE_REPO:-$(gh repo view --json nameWithOwner -q .nameWithOwner 2>/dev/null || echo NaderMozaffari/fdev)}
[ "$(uname -s)" = Darwin ] || { echo "installer: build on macOS (the Mac app needs Xcode)" >&2; exit 1; }

cd "$(dirname "$0")/.."
root=$PWD
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"; rm -f "$root"/installer/rsrc_windows_*.syso "$root"/installer/bundle/fdev_*' EXIT

# The release archives the installers carry.
archives="fdev_darwin_amd64.tar.gz fdev_darwin_arm64.tar.gz fdev_windows_amd64.zip"
missing=
for a in $archives; do [ -f "dist/$a" ] || missing=1; done
if [ -n "$missing" ]; then
	if [ "$publish" = --publish ]; then
		mkdir -p dist
		for a in $archives; do gh release download "$version" --repo "$repo" --pattern "$a" --dir dist --clobber; done
	else
		scripts/release.sh "$version"
	fi
fi
# bundle puts the archive for one OS and CPU, alone, where go:embed takes it.
bundle() {
	rm -f "$root"/installer/bundle/fdev_*
	cp "$root/dist/$1" "$root/installer/bundle/"
}

# 0.2.0 for Info.plist and Windows' file version, which want plain numbers.
numeric=${version#v}
numeric=${numeric%%-*}
ldflags="-s -w -X main.version=$version -X main.repo=$repo"

cd installer

echo "building Install fdev.app"
app="$tmp/dmg/Install fdev.app"
mkdir -p "$app/Contents/MacOS" "$app/Contents/Resources"
for arch in amd64 arm64; do
	bundle "fdev_darwin_$arch.tar.gz"
	CGO_ENABLED=1 GOOS=darwin GOARCH=$arch \
		CGO_CFLAGS=-mmacosx-version-min=13.0 CGO_LDFLAGS=-mmacosx-version-min=13.0 \
		go build -tags desktop,production -trimpath -ldflags "$ldflags" -o "$tmp/installer_$arch" . 2>&1 |
		grep -v -e 'deprecated' -e '^ *[0-9]* |' -e '^ *|' -e 'note:' -e '^#' || true
	[ -f "$tmp/installer_$arch" ] || { echo "installer: the $arch build failed" >&2; exit 1; }
done
lipo -create -output "$app/Contents/MacOS/installer" "$tmp/installer_amd64" "$tmp/installer_arm64"

# The icon, from build/appicon.png at every size macOS asks for.
set_=$tmp/icon.iconset
mkdir -p "$set_"
for s in 16 32 128 256 512; do
	sips -z $s $s build/appicon.png --out "$set_/icon_${s}x${s}.png" >/dev/null
	sips -z $((s * 2)) $((s * 2)) build/appicon.png --out "$set_/icon_${s}x${s}@2x.png" >/dev/null
done
iconutil -c icns "$set_" -o "$app/Contents/Resources/icon.icns"

cat >"$app/Contents/Info.plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleName</key><string>Install fdev</string>
	<key>CFBundleDisplayName</key><string>Install fdev</string>
	<key>CFBundleIdentifier</key><string>dev.fdev.installer</string>
	<key>CFBundleExecutable</key><string>installer</string>
	<key>CFBundleIconFile</key><string>icon</string>
	<key>CFBundlePackageType</key><string>APPL</string>
	<key>CFBundleShortVersionString</key><string>$numeric</string>
	<key>CFBundleVersion</key><string>$numeric</string>
	<key>LSMinimumSystemVersion</key><string>13.0</string>
	<key>NSHighResolutionCapable</key><true/>
</dict>
</plist>
EOF
# What the app connects to and why, for Little Snitch and the like.
cp build/InternetAccessPolicy.plist "$app/Contents/Resources/"

dmg=$root/dist/fdev_macos.dmg
if [ -n "${MACOS_SIGN_IDENTITY:-}" ]; then
	# The hardened runtime and a secure timestamp are what notarization asks for.
	codesign --force --options runtime --timestamp --sign "$MACOS_SIGN_IDENTITY" "$app"
else
	codesign --force --sign - "$app"
fi
rm -f "$dmg"
hdiutil create -quiet -volname "Install fdev" -srcfolder "$tmp/dmg" -format UDZO "$dmg"
if [ -n "${MACOS_SIGN_IDENTITY:-}" ]; then
	codesign --force --timestamp --sign "$MACOS_SIGN_IDENTITY" "$dmg"
	if [ -n "${MACOS_NOTARY_PROFILE:-}" ]; then
		echo "notarizing fdev_macos.dmg (a few minutes)"
		xcrun notarytool submit "$dmg" --keychain-profile "$MACOS_NOTARY_PROFILE" --wait
		xcrun stapler staple "$dmg"
	fi
fi

echo "building fdev_windows_setup.exe"
# The icon, version and manifest (sharp text on high-DPI screens) as a .syso
# that go build links in.
go run github.com/tc-hib/go-winres@v0.3.3 simply --arch amd64 --icon build/appicon.png --manifest gui \
	--product-name fdev --file-description "Install fdev" \
	--product-version "$numeric.0" --file-version "$numeric.0" --original-filename fdev_windows_setup.exe
bundle fdev_windows_amd64.zip
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 \
	go build -tags desktop,production -trimpath -ldflags "$ldflags -H windowsgui" -o "$root/dist/fdev_windows_setup.exe" .

cd "$root/dist"
echo "dist/fdev_macos.dmg and dist/fdev_windows_setup.exe ready for $repo $version"

[ "$publish" = --publish ] || exit 0

# checksums.txt lists them too, after the archives release.sh put there.
gh release download "$version" --repo "$repo" --pattern checksums.txt --clobber
grep -v -e ' fdev_macos.dmg$' -e ' fdev_windows_setup.exe$' checksums.txt >"$tmp/sums" || true
shasum -a 256 fdev_macos.dmg fdev_windows_setup.exe >>"$tmp/sums"
mv "$tmp/sums" checksums.txt
gh release upload "$version" fdev_macos.dmg fdev_windows_setup.exe checksums.txt --repo "$repo" --clobber
