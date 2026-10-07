#!/bin/sh
# Installs fdev, or updates it: the newest stable release for this OS and
# CPU, checked against its checksums, into ~/.fdev/bin (FDEV_INSTALL_DIR to
# change it), which goes on the PATH of each shell you have: zsh, bash, fish
# and sh. Needs curl or wget.
#
#   curl -fsSL https://raw.githubusercontent.com/NaderMozaffari/fdev/main/scripts/install.sh | sh
#   curl -fsSL .../install.sh | sh -s -- --beta       the newest beta too
#   curl -fsSL .../install.sh | sh -s -- v0.2.0       that version
#
# Or FDEV_CHANNEL=beta and FDEV_VERSION=v0.2.0. Until there is a stable
# release, it installs the newest beta, and says so. A token (GH_TOKEN /
# GITHUB_TOKEN, or the GitHub CLI logged in) raises GitHub's rate limit.
set -eu

DEFAULT_REPO=NaderMozaffari/fdev
repo=${FDEV_REPO:-$DEFAULT_REPO}
dir=${FDEV_INSTALL_DIR:-$HOME/.fdev/bin}
channel=${FDEV_CHANNEL:-stable}
version=${FDEV_VERSION:-}
for arg in "$@"; do
	case $arg in
	--beta) channel=beta ;;
	--stable) channel=stable ;;
	v[0-9]* | [0-9]*) version=v${arg#v} ;;
	*) echo "fdev: unknown option $arg (--beta, --stable or a version)" >&2; exit 2 ;;
	esac
done

# writable dir: dir, or the folder it would be made in, can be written.
writable() {
	w=$1
	while [ ! -d "$w" ]; do w=$(dirname "$w"); done
	[ -w "$w" ]
}
if ! writable "$dir"; then
	echo "fdev: can't write to $dir" >&2
	echo "  pick a folder you own: FDEV_INSTALL_DIR=\$HOME/bin sh" >&2
	exit 1
fi

case $(uname -s) in
Darwin) os=darwin ;;
Linux) os=linux ;;
*) echo "fdev: $(uname -s) isn't supported by this script; on Windows use install.ps1" >&2; exit 1 ;;
esac
case $(uname -m) in
x86_64 | amd64) arch=amd64 ;;
arm64 | aarch64) arch=arm64 ;;
*) echo "fdev: no build for $(uname -m)" >&2; exit 1 ;;
esac
asset=fdev_${os}_${arch}.tar.gz

token=${FDEV_GITHUB_TOKEN:-${GH_TOKEN:-${GITHUB_TOKEN:-}}}
if [ -z "$token" ] && command -v gh >/dev/null 2>&1; then
	token=$(gh auth token 2>/dev/null || true)
fi

get() { # url accept
	if command -v curl >/dev/null 2>&1; then
		if [ -n "$token" ]; then
			curl -fsSL -H "Accept: $2" -H "Authorization: Bearer $token" "$1"
		else
			curl -fsSL -H "Accept: $2" "$1"
		fi
	elif command -v wget >/dev/null 2>&1; then
		if [ -n "$token" ]; then
			wget -qO- --header="Accept: $2" --header="Authorization: Bearer $token" "$1"
		else
			wget -qO- --header="Accept: $2" "$1"
		fi
	else
		echo "fdev: needs curl or wget" >&2
		return 1
	fi
}

api=https://api.github.com/repos/$repo/releases
unreadable() {
	echo "fdev: can't read the releases of $repo" >&2
	echo "  is github.com reachable? (a token in GITHUB_TOKEN helps if GitHub limits you)" >&2
	exit 1
}

# The release: the version asked for, or the newest of the channel. GitHub
# lists releases newest first; a beta is a pre-release.
if [ -z "$version" ]; then
	list=$(get "$api?per_page=100" application/vnd.github+json) || unreadable
	# newest beta|stable: the first release of the channel, by its fields
	# (tag_name comes before draft and prerelease).
	newest() {
		printf '%s\n' "$list" | tr ',{}[]' '\n\n\n\n\n' | awk -v beta="$1" '
			/^ *"tag_name": *"/ { match($0, /"tag_name": *"[^"]*"/); t = substr($0, RSTART, RLENGTH); sub(/.*"tag_name": *"/, "", t); sub(/"$/, "", t); tag = t; draft = 0 }
			/^ *"draft": *true/ { draft = 1 }
			/^ *"prerelease": *(true|false)/ {
				if (tag != "" && !draft && (beta == "beta" || ($0 ~ /false/ && tag !~ /-/))) { print tag; exit }
				tag = ""
			}'
	}
	version=$(newest "$channel")
	if [ -z "$version" ] && [ "$channel" = stable ]; then
		version=$(newest beta)
		[ -n "$version" ] && echo "fdev: there is no stable release yet, so this is the newest beta: $version"
	fi
	[ -n "$version" ] || { echo "fdev: $repo has no releases yet" >&2; exit 1; }
fi
release=$(get "$api/tags/$version" application/vnd.github+json) || {
	echo "fdev: there is no release $version in $repo" >&2
	exit 1
}
# One JSON field a line, pretty-printed or not.
release=$(printf '%s\n' "$release" | tr ',{}[]' '\n\n\n\n\n')
tag=$(printf '%s\n' "$release" | sed -n 's/^ *"tag_name": *"\([^"]*\)".*/\1/p' | head -n1)

# Each asset's API URL comes just before its name.
asset_url() {
	printf '%s\n' "$release" | awk -v want="$1" '
		/"url": *"[^"]*\/releases\/assets\// { match($0, /https:[^"]*/); url = substr($0, RSTART, RLENGTH) }
		/"name": *"/ { if (index($0, "\"" want "\"")) { print url; exit } }'
}
url=$(asset_url "$asset")
sums=$(asset_url checksums.txt)
if [ -z "$url" ] || [ -z "$sums" ]; then
	echo "fdev: release $tag has no $asset" >&2
	exit 1
fi

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
case $tag in
*-*) echo "downloading fdev $tag ($asset), a beta: a pre-release, it may have bugs" ;;
*) echo "downloading fdev $tag ($asset)" ;;
esac
get "$url" application/octet-stream >"$tmp/$asset"
get "$sums" application/octet-stream >"$tmp/checksums.txt"

want=$(awk -v f="$asset" '$2 == f || $2 == "*" f { print $1 }' "$tmp/checksums.txt")
if command -v sha256sum >/dev/null 2>&1; then got=$(sha256sum "$tmp/$asset" | cut -d' ' -f1); else got=$(shasum -a 256 "$tmp/$asset" | cut -d' ' -f1); fi
if [ -z "$want" ] || [ "$want" != "$got" ]; then
	echo "fdev: checksum mismatch for $asset" >&2
	exit 1
fi

tar -xzf "$tmp/$asset" -C "$tmp" fdev
mkdir -p "$dir"
mv -f "$tmp/fdev" "$dir/fdev"
chmod 755 "$dir/fdev"
echo "installed $("$dir/fdev" version) in $dir"

# drop file line: takes out the line, and the "# fdev" above it, that an
# earlier installer added.
drop() {
	grep -qsxF "$2" "$1" || return 0
	kept=$(awk -v l="$2" '{ b[NR] = $0 }
		END {
			for (i = 1; i <= NR; i++) if (b[i] == l) {
				s[i] = 1
				if (b[i-1] == "# fdev") { s[i-1] = 1; if (b[i-2] == "") s[i-2] = 1 }
			}
			for (i = 1; i <= NR; i++) if (!s[i]) print b[i]
		}' "$1") && printf '%s\n' "$kept" >"$1"
}

# Copies the earlier installers put in ~/.local/bin or ~/bin.
for old in "$HOME/.local/bin" "$HOME/bin"; do
	if [ "$old" != "$dir" ] && [ -f "$old/fdev" ] && "$old/fdev" version 2>/dev/null | grep -q '^fdev '; then
		rm -f "$old/fdev" && echo "removed the old $old/fdev"
		for f in "${ZDOTDIR:-$HOME}/.zshrc" "$HOME/.bash_profile" "$HOME/.bashrc" "$HOME/.profile"; do
			drop "$f" "export PATH=\"$old:\$PATH\""
		done
	fi
done

# The PATH line for each shell's startup file, written once. $HOME stays
# as it is, so the files still work if the home folder moves.
case $dir in
"$HOME"/*) shown="\$HOME${dir#"$HOME"}" ;;
*) shown=$dir ;;
esac
shell=$(basename "${SHELL:-sh}")
added=
add() { # file line
	[ -f "$1" ] && grep -qsF "$2" "$1" && return 0
	mkdir -p "$(dirname "$1")" 2>/dev/null || true
	if printf '\n# fdev\n%s\n' "$2" >>"$1" 2>/dev/null; then
		added="$added $1"
	else
		echo "fdev: can't write $1; add this line to it yourself: $2" >&2
	fi
}
line="export PATH=\"$shown:\$PATH\""
zshrc=${ZDOTDIR:-$HOME}/.zshrc
# zsh: the default shell on macOS.
if [ "$shell" = zsh ] || [ -f "$zshrc" ] || [ "$os" = darwin ]; then add "$zshrc" "$line"; fi
# bash: macOS terminals start login shells, which read ~/.bash_profile;
# Linux ones read ~/.bashrc.
if [ "$os" = darwin ]; then bashrc=$HOME/.bash_profile; else bashrc=$HOME/.bashrc; fi
if [ "$shell" = bash ] || [ -f "$bashrc" ]; then add "$bashrc" "$line"; fi
# fish: a file of its own in conf.d.
if [ "$shell" = fish ] || [ -d "$HOME/.config/fish" ]; then
	add "$HOME/.config/fish/conf.d/fdev.fish" "contains $dir \$PATH; or set -gx PATH $dir \$PATH"
fi
# sh, dash, ksh, and Linux desktop sessions.
case $shell in zsh | bash | fish) ;; *) add "$HOME/.profile" "$line" ;; esac
if [ -f "$HOME/.profile" ] && [ "$os" = linux ]; then add "$HOME/.profile" "$line"; fi

for f in $added; do echo "added $dir to your PATH in $f"; done
case ":$PATH:" in
*":$dir:"*) ;;
*) echo "open a new terminal to use fdev, or in this one: export PATH=\"$dir:\$PATH\"" ;;
esac
