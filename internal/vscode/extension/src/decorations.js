'use strict';

// Draws the logs: one decoration type per level and look, created when first
// needed and disposed when a setting changes. A type is the CSS VS Code
// applies; the ranges it is applied to are set per editor.
//
// Looks:
//   show  the level colour (text, faint background), a dot in the gutter and
//         a mark in the overview ruler.
//   dim   the same with the text's opacity lowered.
//   hide  the statement is `display: none` (the well-known textDecoration
//         hack) and a small marker is drawn in its place. The text and the
//         marker are two decorations: a `before` of the hidden text would be
//         hidden with it, so the marker sits on an empty range at its start.

const fs = require('fs');
const path = require('path');
const vscode = require('vscode');
const { withAlpha, dotSvg, hash } = require('./color');

// What a level is called on a marker.
const SHORT = { declaration: 'logger' };

class Decorations {
  /** @param {vscode.ExtensionContext} context */
  constructor(context) {
    this.context = context;
    this.settings = null;
    this.types = new Map();
    this.icons = new Map();
    this.used = new WeakMap();
    this.iconDir = null;
  }

  /** configure replaces the settings and drops every type built from the old ones. */
  configure(logs) {
    this.disposeTypes();
    this.settings = logs;
  }

  disposeTypes() {
    // A look that is switched off (no gutter dot and no ruler mark) is kept as null.
    for (const type of this.types.values()) if (type) type.dispose();
    this.types.clear();
    this.icons.clear();
    this.used = new WeakMap();
  }

  dispose() {
    this.disposeTypes();
  }

  type(key, make) {
    if (this.types.has(key)) return this.types.get(key);
    const options = make();
    const type = options ? vscode.window.createTextEditorDecorationType(options) : null;
    this.types.set(key, type);
    return type;
  }

  // The gutter dot of a colour: a small SVG file in the extension's global
  // storage (gutterIconPath needs a Uri), or a data: Uri when it cannot be
  // written.
  iconUri(color, opacity) {
    const key = `${color}|${opacity}`;
    let uri = this.icons.get(key);
    if (uri) return uri;
    const svg = dotSvg(color, opacity);
    try {
      if (!this.iconDir) {
        this.iconDir = this.context.globalStorageUri.fsPath;
        fs.mkdirSync(this.iconDir, { recursive: true });
      }
      const file = path.join(this.iconDir, `dot-${hash(key)}.svg`);
      fs.writeFileSync(file, svg);
      uri = vscode.Uri.file(file);
    } catch (e) {
      uri = vscode.Uri.parse('data:image/svg+xml;utf8,' + encodeURIComponent(svg));
    }
    this.icons.set(key, uri);
    return uri;
  }

  textType(level, variant) {
    return this.type(`text:${level}:${variant}`, () => {
      const s = this.settings;
      const color = s.colors[level];
      const options = { rangeBehavior: vscode.DecorationRangeBehavior.ClosedClosed };
      if (s.colorText) options.color = color;
      if (s.background) options.backgroundColor = withAlpha(color, s.backgroundAlpha);
      if (s.fontStyle === 'italic') options.fontStyle = 'italic';
      if (variant === 'dim') options.opacity = String(s.opacity);
      return options;
    });
  }

  // The dot in the gutter and the mark in the overview ruler, on the first
  // line of a log. Null when both are switched off.
  markType(level, variant) {
    return this.type(`mark:${level}:${variant}`, () => {
      const s = this.settings;
      if (!s.gutterIcons && !s.overviewRuler) return null;
      const color = s.colors[level];
      const fade = variant === 'dim' ? s.opacity : 1;
      const options = { isWholeLine: true };
      if (s.gutterIcons) {
        options.gutterIconPath = this.iconUri(color, fade);
        options.gutterIconSize = 'contain';
      }
      if (s.overviewRuler) {
        options.overviewRulerColor = withAlpha(color, fade);
        options.overviewRulerLane = vscode.OverviewRulerLane.Right;
      }
      return options;
    });
  }

  hideType() {
    return this.type('hide', () => ({
      textDecoration: 'none; display: none;',
      rangeBehavior: vscode.DecorationRangeBehavior.ClosedClosed,
    }));
  }

  hiddenMarkType(level) {
    return this.type(`hidden:${level}`, () => {
      const color = this.settings.colors[level];
      return {
        rangeBehavior: vscode.DecorationRangeBehavior.ClosedClosed,
        before: {
          contentText: '●',
          color,
          fontStyle: this.settings.fontStyle === 'italic' ? 'italic' : 'normal',
          margin: '0 0.15em',
          textDecoration: 'none; font-size: 0.85em; opacity: 0.85; cursor: default;',
        },
      };
    });
  }

  /**
   * render draws `items` in `editor`, and clears what an earlier render drew
   * there that is not part of this one. An item:
   *
   *   { range, firstLine, level, variant: 'show' | 'dim' | 'hide',
   *     marker: the text of the hidden marker ('' for none) }
   */
  render(editor, items) {
    const buckets = new Map();
    const add = (type, option) => {
      if (!type) return;
      let bucket = buckets.get(type);
      if (!bucket) buckets.set(type, (bucket = []));
      bucket.push(option);
    };

    for (const it of items) {
      if (it.variant === 'hide') {
        add(this.hideType(), { range: it.range });
        if (it.marker) {
          add(this.hiddenMarkType(it.level), {
            range: new vscode.Range(it.range.start, it.range.start),
            renderOptions: { before: { contentText: it.marker } },
          });
        }
        add(this.markType(it.level, 'dim'), { range: it.firstLine });
      } else {
        add(this.textType(it.level, it.variant), { range: it.range });
        add(this.markType(it.level, it.variant), { range: it.firstLine });
      }
    }

    const before = this.used.get(editor);
    if (before) {
      for (const type of before) {
        if (!buckets.has(type)) editor.setDecorations(type, []);
      }
    }
    for (const [type, options] of buckets) editor.setDecorations(type, options);
    this.used.set(editor, new Set(buckets.keys()));
  }

  /** clear removes everything this class drew in `editor`. */
  clear(editor) {
    const before = this.used.get(editor);
    if (!before) return;
    for (const type of before) editor.setDecorations(type, []);
    this.used.delete(editor);
  }

  /** The text a hidden statement leaves behind: "● info" or "● info “Fetching…”". */
  markerText(style, level, preview) {
    if (style === 'none') return '';
    const name = SHORT[level] || level;
    if (style === 'label' && preview) return `● ${name} “${preview}”`;
    return `● ${name}`;
  }
}

module.exports = { Decorations };
