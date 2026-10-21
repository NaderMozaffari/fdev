# fdev

Log calls clutter code. This extension lets you **colour them by level, dim them or hide them with one key**, in any project and any of the languages below, without a single file of yours changing. It also opens the [fdev](https://github.com/NaderMozaffari/fdev) launcher and log viewer in an editor tab.

## Logs: show, dim, hide

Press `Ctrl+Alt+L` (`Cmd+Alt+L` on a Mac), or click the status bar item, to go through three modes:

| Mode | What you see |
| --- | --- |
| **show** | Every log call in the colour of its level, a dot in the gutter and a mark in the overview ruler. |
| **dim** (default) | The same, faded, so the logs stay readable but stop shouting. |
| **hide** | The statement disappears and a small marker such as `● info` takes its place. A call that spans several lines is folded into that one line. Hover over the marker to read the code. |

The status bar item shows the mode (an eye for show, an outlined circle for dim, a crossed-out eye for hide) and **how many logs the file has**.

In hide mode, a statement that your cursor or selection is on is drawn dimmed until you leave it, so you can edit it (`fdev.logs.revealOnCursor`).

### What counts as a log

- Calls: Dart `_log.info(...)`, `Log.error(...)`, `FdevLog.http(...)`, `print`, `debugPrint`; JavaScript and TypeScript `console.*`; Python `logging.*` / `logger.*` and `print`; Kotlin `Log.d(...)` and `Timber`; Swift `print`, `os_log`, `logger.*`; Go `log.*` and `slog.*`.
- **Logger declarations**, such as `static const _log = Log('Billing');` (`fdev.logs.includeDeclarations`).
- **Imports** of logger libraries, such as `import 'package:fdev_log/fdev_log.dart';` (`fdev.logs.includeImports`, `fdev.logs.importPattern`).

A call is followed to its matching `)` the way the language reads it: strings (including `'''...'''`, raw strings and template literals), `${...}` interpolation and comments are understood, so a `)` inside a string does not end the call and a log in a comment is not a log. A trailing `;` and a trailing `// comment` belong to the statement.

Each log has a level: `debug`, `info`, `success`, `warning`, `error`, `fatal`, `value`, `http`, or `declaration` (declarations and imports). Names are mapped to them: `warn` is `warning`, `wtf` is `fatal`, `print` and `debugPrint` are `debug`, `request` and `response` are `http`, and Android's `Log.e` is `error`.

### The mode lives outside your project

The mode you pick is kept in VS Code's workspace state, not in `.vscode/settings.json`, so **no fdev file appears in your project and nothing reaches git**. The setting `fdev.logs.mode` is only the mode a workspace starts in.

### Choose what dim and hide apply to

`fdev: Logs: Choose Levels` is a multi-select of levels. The levels you leave out stay as in show (coloured, not dimmed or hidden). Hide everything except errors, for example.

## Commands

| Command | What it does |
| --- | --- |
| `fdev: Open` | Opens the fdev launcher in a terminal named "fdev" in the editor area, in the workspace folder of the active file. Opening again focuses it. |
| `fdev: Logs: Cycle Mode` | show, dim, hide, show... (`Ctrl+Alt+L`, `Cmd+Alt+L` on a Mac, when the editor has focus). |
| `fdev: Logs: Show` / `Dim` / `Hide` | Picks a mode. |
| `fdev: Logs: Choose Levels` | Picks the levels dim and hide apply to. |
| `fdev: Logs: Reveal All Temporarily` | Shows every log for `fdev.logs.peekSeconds` seconds, then goes back. Run it again to go back at once. |
| `fdev: Logs: Use the Settings Again` | Forgets the mode and levels you picked and returns to the settings. |
| `fdev: Logs: Open Settings` | Opens this extension's settings. |

The keybinding is the usual VS Code kind: change it in Keyboard Shortcuts (search for `fdev.logs.cycleMode`).

## Settings

Everything works without any settings. Change what you like in Settings (search for `fdev`) or in `settings.json`.

| Setting | Default | What it does |
| --- | --- | --- |
| `fdev.logs.enabled` | `true` | Master switch. |
| `fdev.logs.mode` | `"dim"` | The mode a workspace starts in: `show`, `dim` or `hide`. |
| `fdev.logs.levels` | all nine | The levels dim and hide apply to. |
| `fdev.logs.languages` | `dart`, `javascript`, `typescript`, `javascriptreact`, `typescriptreact`, `python`, `kotlin`, `swift`, `go` | Language ids that are scanned. |
| `fdev.logs.rules` | built in | What counts as a log call: a list of `{ "pattern", "level", "languages" }` (see below). |
| `fdev.logs.includeDeclarations` | `true` | Count logger declarations as logs. |
| `fdev.logs.declarationRules` | built in | What counts as a logger declaration (same format as the rules). |
| `fdev.logs.includeImports` | `true` | Count logger imports as logs. |
| `fdev.logs.importPattern` | built in | Regular expression (flags `gm`) for the import lines to count. |
| `fdev.logs.colors` | see below | The colour of each level. |
| `fdev.logs.colorText` | `true` | Draw the call's text in its level colour. |
| `fdev.logs.background` | `false` | Draw a faint background of the level colour. |
| `fdev.logs.backgroundAlpha` | `0.12` | How strong that background is (0 to 1). |
| `fdev.logs.gutterIcons` | `true` | A dot in the gutter on the first line of a log. |
| `fdev.logs.overviewRuler` | `true` | A mark in the overview ruler. |
| `fdev.logs.fontStyle` | `"normal"` | `normal` or `italic`. |
| `fdev.logs.opacity` | `0.35` | How visible a dimmed log is (0.05 to 1). |
| `fdev.logs.hiddenMarker` | `"dot"` | What replaces a hidden statement: `dot` (`● info`), `label` (`● info “Fetching profile”`) or `none`. |
| `fdev.logs.fold` | `true` | Fold hidden statements that span several lines. |
| `fdev.logs.revealOnCursor` | `true` | Draw a hidden statement dimmed while the cursor is on it. |
| `fdev.logs.peekSeconds` | `5` | How long "Reveal All Temporarily" lasts. |
| `fdev.logs.maxFileSize` | `1048576` | Files with more characters than this are left alone. |
| `fdev.logs.statusBar` | `"right"` | Where the mode item goes: `left`, `right` or `off`. |
| `fdev.statusBar.launcher` | `true` | The `fdev` button in the status bar. |
| `fdev.path` | `""` | The fdev program; empty looks on the `PATH`, then `$GOBIN`, `$GOPATH/bin` and `~/go/bin`. User settings only. |

### Colours

```jsonc
"fdev.logs.colors": {
  "debug": "#8b949e",
  "info": "#58a6ff",
  "success": "#3fb950",
  "warning": "#d29922",
  "error": "#f85149",
  "fatal": "#ff4d94",
  "value": "#39c5cf",
  "http": "#bc8cff",
  "declaration": "#768390"
}
```

Any CSS colour works (`#rrggbb`, `#rrggbbaa`, `rgb(...)`); keep only the levels you want to change. The defaults read on dark and light themes. To tint the background instead of the text:

```jsonc
"fdev.logs.colorText": false,
"fdev.logs.background": true,
"fdev.logs.backgroundAlpha": 0.2
```

### Your own rules

A rule is a JavaScript regular expression that ends right after the `(` of the call; fdev finds the matching `)` itself. Take the level from a named group `(?<level>...)`, or give a fixed `level`. For example, to count a `track(...)` helper as an `info` log in Dart, copy the built-in rules and add:

```jsonc
{ "pattern": "\\banalytics\\.(?<level>track)\\s*\\(", "level": "info", "languages": ["dart"] }
```

An invalid pattern is reported once and skipped. Workspaces you have not trusted use the built-in rules.

## fdev launcher

`fdev: Open` (or the rocket `fdev` button in the status bar) runs `fdev` in an editor-area terminal, in the folder of the active file's workspace, with `FDEV_FROM_VSCODE=1` in its environment. If fdev is not installed it shows the command:

```sh
go install github.com/NaderMozaffari/fdev@latest
```

## Notes

- Hidden text uses a CSS trick (`display: none` through a decoration), the one other extensions use too. The line of a hidden single-line statement stays as a line with its marker; a multi-line statement alone on its lines is folded.
- Only the editors you can see are updated, edits are debounced, and files over `fdev.logs.maxFileSize` are skipped.
- The folding of hidden statements relies on VS Code's folding being on (`editor.folding`).
- The scanner reads each language's strings and comments, but not every corner of every language: a regular expression literal that holds a quote can confuse it for the rest of its line.

## License

MIT
