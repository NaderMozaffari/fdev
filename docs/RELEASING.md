# Releasing fdev

## Versions and channels

fdev's versions are [semantic versions](https://semver.org), and come in
two channels:

| | Tag | On GitHub | Who gets it |
|---|---|---|---|
| **Beta** | `v0.2.0-beta.1`, `v0.2.0-beta.2`, `v1.0.0-rc.1` | a pre-release | `--beta` installs and updates, and `fdev update` from a beta |
| **Stable** | `v1.0.0`, `v1.0.1` | the latest release | everyone else |

A beta comes before its release: `v0.2.0-beta.1` < `v0.2.0-beta.2` <
`v0.2.0-rc.1` < `v0.2.0`. A beta's `fdev version` says so
(`fdev v0.2.0-beta.1 (beta)`), and so does its About page.

- The install scripts take the newest stable release, or with `--beta`
  (`FDEV_CHANNEL=beta`) the newest release of either. While there is no
  stable release, they take the newest beta and say so.
- `fdev update` keeps to the channel of the running fdev: a beta updates to
  the newest beta (or a newer stable release), a stable release to the
  newest stable one. `--beta` and `--stable` switch.

fdev is in beta until `v1.0.0`: tag each release `v0.x.y-beta.n`.

## Making a release

Push a version tag:

```sh
git tag v0.2.0-beta.1
git push origin v0.2.0-beta.1
```

The [Release workflow](../.github/workflows/release.yml) runs the tests,
then runs `scripts/release.sh`. That script builds fdev for macOS, Linux and
Windows (amd64 and arm64) and makes a GitHub release, a pre-release for a
beta. The release holds:

- `fdev_<os>_<arch>.tar.gz`, and `.zip` for Windows
- `checksums.txt`, the SHA-256 of each archive
- `install.sh` and `install.ps1`, set to install from this release's
  repository
- `fdev_macos.dmg` and `fdev_windows_setup.exe`, the installers
  (`installer/`), added by a second job on macOS once the release exists,
  which also adds them to `checksums.txt`

Each binary is built with the version and the release repository in it, so
`fdev version` prints the tag and `fdev update` knows where to look. The
release notes list the commit subjects since the last tag.

Each installer carries the release's archive for its OS and CPU (embedded
from `installer/bundle/`, where `scripts/installer.sh` copies it from the
published release), so it installs with no internet. Online, it asks
GitHub for the newest stable release and the newest beta, and downloads
one when it is newer than the archive it carries. The channel picked is the
one `fdev update` keeps to, as fdev's channel is that of its version.

The installers are a Wails app (Go and a webview) in a module of their own,
`installer/`, so fdev itself doesn't depend on Wails. Their screens are
`installer/frontend/index.html`; the download and checksum code is
`internal/update`'s. To build them yourself (on a Mac with Xcode's command
line tools):

```sh
scripts/installer.sh v0.2.0-beta.1             # dist/fdev_macos.dmg and dist/fdev_windows_setup.exe
scripts/installer.sh v0.2.0-beta.1 --publish   # and add them to that release
```

They connect to GitHub only: the fonts are in `installer/frontend/fonts`,
and `installer/build/InternetAccessPolicy.plist` tells firewalls such as
Little Snitch which hosts the Mac app talks to and why.

Signing the Mac app needs a *Developer ID Application* certificate (Apple
Developer Program; an *Apple Development* one can't sign for others) in the
keychain, and a notarytool profile made once:

```sh
xcrun notarytool store-credentials fdev --apple-id <you> --team-id <team>
MACOS_SIGN_IDENTITY="Developer ID Application: <name> (<team>)" MACOS_NOTARY_PROFILE=fdev \
  scripts/installer.sh v0.2.0-beta.1
```

Without them the app has an ad-hoc signature (Apple silicon runs nothing
without one), so macOS asks before opening it the first time and Little
Snitch lists it as not signed. The release workflow builds it that way, as
it has no certificate. The Windows `.exe` isn't signed either, so
SmartScreen warns about it.

To build or publish from your own machine instead (needs `gh`, `zip`):

```sh
scripts/release.sh v0.2.0-beta.1             # just dist/
scripts/release.sh v0.2.0-beta.1 --publish   # and the GitHub release
```

To make a published release a beta, or a beta stable, after the fact:

```sh
gh release edit v0.1.3 --prerelease     # a beta
gh release edit v1.0.0 --prerelease=false --latest
```

## Install scripts

The README installs with the scripts in `main`
(`https://raw.githubusercontent.com/NaderMozaffari/fdev/main/scripts/install.sh`),
which work whatever the newest release is. Each release also carries a
copy, for when raw.githubusercontent.com can't be reached.

The release can go to another repository: set the `RELEASE_REPO` variable
(owner/name) in this repository's **Settings → Secrets and variables →
Actions**, and the `RELEASE_TOKEN` secret to a token that can write to it.
