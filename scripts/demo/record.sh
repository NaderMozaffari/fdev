#!/usr/bin/env bash
# Records the README's GIFs and screenshots (docs/media) by running fdev in
# the demo project (project/), whose flutter, dart and adb are the fakes in
# bin/, with the keys in scenes/. Needs Go and agg
# (https://github.com/asciinema/agg/releases); JetBrains Mono is downloaded.
#
#	scripts/demo/record.sh              all of them
#	scripts/demo/record.sh hero themes  some of them
set -euo pipefail

here=$(cd "$(dirname "$0")" && pwd)
repo=$(cd "$here/../.." && pwd)
media=$repo/docs/media
agg=${AGG:-agg}
work=$(mktemp -d)
work=$(cd "$work" && pwd -P) # fdev sees the real path
trap 'rm -rf "$work"' EXIT
mkdir -p "$media" "$work/bin" "$work/casts" "$work/frames"

echo "building fdev and rec"
go build -o "$work/bin/rec" "$here/rec"
go build -ldflags "-X main.version=$(git -C "$repo" describe --tags --abbrev=0 2>/dev/null || echo dev)" -o "$work/bin/fdev" "$repo"

fonts=${FONT_DIR:-$work/fonts}
if [ ! -f "$fonts/JetBrainsMono-Regular.ttf" ]; then
	echo "downloading JetBrains Mono"
	mkdir -p "$fonts"
	curl -fsSL -o "$work/jbm.zip" https://github.com/JetBrains/JetBrainsMono/releases/download/v2.304/JetBrainsMono-2.304.zip
	unzip -qjo "$work/jbm.zip" 'fonts/ttf/JetBrainsMono-Regular.ttf' 'fonts/ttf/JetBrainsMono-Bold.ttf' 'fonts/ttf/JetBrainsMono-Italic.ttf' -d "$fonts"
fi

# The project, in a git repository of its own so the header shows a clean
# branch.
project=$work/acme_shop
cp -R "$here/project" "$project"
git -C "$project" init -q -b main
git -C "$project" add -A
git -C "$project" -c user.name=demo -c user.email=demo@example.com commit -qm "Acme Shop"

home=$work/home
case $(uname) in
Darwin) cache=$home/Library/Caches/fdev ;;
*) cache=$home/.cache/fdev ;;
esac
state=$cache/$(printf %s "$project" | shasum -a 256 | cut -c1-16).json

ago() { # ago <hours>
	date -u -v-"$1"H +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date -u -d "$1 hours ago" +%Y-%m-%dT%H:%M:%SZ
}

# fresh starts fdev's state over: a theme, a look for the logs, a few recent
# runs, and how long the build steps took last time (for the progress bars).
fresh() { # fresh <theme> [<look json>]
	rm -rf "$home"
	mkdir -p "$cache"
	printf '{"theme": "%s"}\n' "$1" >"$cache/ui.json"
	local look=""
	[ -n "${2:-}" ] && look=",
  \"look\": $2"
	cat >"$state" <<EOF
{
  "recent": [
    {"target": "dev", "env": {"STORE": "myket", "DEVICE": "R5CT21PIXEL8"}, "note": "Pixel 8", "at": "$(ago 2)"},
    {"target": "apk-prod", "env": {"STORE": "bazaar"}, "at": "$(ago 26)"},
    {"target": "ios-dev", "env": {"DEVICE": "6F1C2A9E-3B7D-4E21-9C55-1D2B3E4F5A6B"}, "note": "iPhone 16 Pro", "at": "$(ago 75)"}
  ],
  "answers": {"device:android": "R5CT21PIXEL8"},
  "durations": {
    "Running Gradle task 'assembleDevBazaarDebug'": 5.6,
    "Running Gradle task 'assembleDevMyketDebug'": 5.6,
    "Running Gradle task 'assembleDevDebug'": 5.6,
    "Running Gradle task 'assembleProdBazaarRelease'": 9.8,
    "Running Gradle task 'assembleProdRelease'": 9.8
  }$look
}
EOF
}

# The colors of each theme's terminal, for agg: the background and text,
# then the 16 ANSI colors.
ansi=282a2e,f25d7c,52d39a,fecf4d,6b8cf5,b97cf5,5fd7d7,c5c8c6,4d4d4d,ff7b97,73f5b5,ffe082,8fa9ff,d4a5ff,8be9fd,ffffff
colors() {
	case $1 in
	dracula) echo 282a36,f8f8f2 ;;
	nord) echo 2e3440,eceff4 ;;
	tokyo-night) echo 1a1b26,c0caf5 ;;
	catppuccin) echo 1e1e2e,cdd6f4 ;;
	gruvbox) echo 282828,ebdbb2 ;;
	latte) echo eff1f5,4c4f69 ;;
	solarized) echo fdf6e3,586e75 ;;
	*) echo 171717,dddddd ;;
	esac
}

theme=auto
record() { # record <name> <scene> <cols> <rows> fdev [args]
	local name=$1 scene=$2 cols=$3 rows=$4
	shift 4
	local c
	c=$(colors "$theme")
	echo "recording $name"
	(cd "$project" && env -i HOME="$home" XDG_CACHE_HOME="$home/.cache" LANG=en_US.UTF-8 \
		PATH="$here/bin:$work/bin:/usr/local/bin:/usr/bin:/bin" TERM=xterm-256color COLORTERM=truecolor \
		"$work/bin/rec" -scene "$here/scenes/$scene.scene" -o "$work/casts/$name.cast" \
		-cols "$cols" -rows "$rows" -bg "#${c%,*}" -fg "#${c#*,}" -- "$@")
}

render() { # render <name> [agg options]: the cast as a GIF
	local name=$1
	shift
	"$agg" -q --font-dir "$fonts" --font-family "JetBrains Mono" --font-size 15 --line-height 1.3 \
		--theme "$(colors "$theme"),$ansi" "$@" "$work/casts/$name.cast" "$work/frames/$name.gif"
}

shot() { # shot <name> <marker> <file>: a frame of the cast, as docs/media/<file>.png
	render "$1" --select "marker:$2"
	mv "$work/frames/$1.gif" "$work/frames/$3.gif"
	"$work/bin/rec" png "$work/frames/$3.gif" "$media/$3.png"
}

hero() {
	theme=auto
	fresh auto
	record hero hero 120 32 fdev
	for m in menu:launcher targets:targets store:ask progress:progress logs:viewer network:network filter:filter; do
		shot hero "${m%%:*}" "${m#*:}"
	done
	render hero --idle-time-limit 2 --last-frame-duration 2
	mv "$work/frames/hero.gif" "$media/hero.gif"
}

features() {
	theme=auto
	fresh auto
	record features features 120 32 fdev dev
	for m in values:values select:select settings:settings; do
		shot features "${m%%:*}" "${m#*:}"
	done
}

# themes is the viewer in a few themes and layouts, as a slideshow.
themes() {
	local frames=() look
	for t in auto:lines:text dracula:columns:text tokyo-night:table:badge latte:columns:badge gruvbox:lines:badge nord:table:text; do
		theme=${t%%:*}
		look=$(printf '{"layout": "%s", "labels": "%s", "spacing": 0, "time": "clock", "dir": ".fdev/logs", "group": "pause"}' \
			"$(echo "$t" | cut -d: -f2)" "${t##*:}")
		fresh "$theme" "$look"
		record "look-$theme" look 120 32 fdev dev
		render "look-$theme" --select marker:shot
		frames+=("$work/frames/look-$theme.gif")
	done
	"$work/bin/rec" png "$work/frames/look-tokyo-night.gif" "$media/table.png"
	"$work/bin/rec" join -delay 2.6s -o "$media/themes.gif" "${frames[@]}"
}

for scene in "${@:-hero features themes}"; do
	for s in $scene; do "$s"; done
done
ls -la "$media"
