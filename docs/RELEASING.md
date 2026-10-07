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

Each binary is built with the version and the release repository in it, so
`fdev version` prints the tag and `fdev update` knows where to look. The
release notes list the commit subjects since the last tag.

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
