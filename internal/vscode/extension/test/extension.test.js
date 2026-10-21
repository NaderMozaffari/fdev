'use strict';

// Runs the extension against a small fake of the "vscode" module: enough to
// check that activation, the modes, the hidden look, folding, hover and the
// status bar behave, without an editor.

const test = require('node:test');
const assert = require('node:assert');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const Module = require('node:module');

const pkg = JSON.parse(fs.readFileSync(path.join(__dirname, '..', 'package.json'), 'utf8'));
const defaults = {};
for (const [key, spec] of Object.entries(pkg.contributes.configuration.properties)) {
  defaults[key.replace(/^fdev\./, '')] = spec.default;
}

class Position {
  constructor(line, character) { this.line = line; this.character = character; }
  compareTo(o) { return this.line - o.line || this.character - o.character; }
  isBefore(o) { return this.compareTo(o) < 0; }
}
class Range {
  constructor(a, b, c, d) {
    if (a instanceof Position) { this.start = a; this.end = b; }
    else { this.start = new Position(a, b); this.end = new Position(c, d); }
  }
  intersection(o) {
    const start = this.start.isBefore(o.start) ? o.start : this.start;
    const end = this.end.isBefore(o.end) ? this.end : o.end;
    return end.isBefore(start) ? undefined : new Range(start, end);
  }
}
class EventEmitter {
  constructor() { this.fns = []; this.event = (fn) => { this.fns.push(fn); return { dispose() {} }; }; }
  fire(x) { this.fns.forEach((f) => f(x)); }
  dispose() {}
}

function makeDocument(text, languageId = 'dart') {
  const starts = [0];
  for (let i = 0; i < text.length; i++) if (text[i] === '\n') starts.push(i + 1);
  const doc = {
    uri: { scheme: 'file', fsPath: '/p/a.dart', toString: () => 'file:///p/a.dart' },
    languageId,
    version: 1,
    lineCount: starts.length,
    getText(range) {
      if (!range) return text;
      return text.slice(doc.offsetAt(range.start), doc.offsetAt(range.end));
    },
    offsetAt: (p) => starts[p.line] + p.character,
    positionAt(offset) {
      let line = 0;
      while (line + 1 < starts.length && starts[line + 1] <= offset) line++;
      return new Position(line, offset - starts[line]);
    },
    lineAt(line) {
      const s = starts[line];
      const e = line + 1 < starts.length ? starts[line + 1] - 1 : text.length;
      return { text: text.slice(s, e), range: new Range(line, 0, line, e - s) };
    },
  };
  return doc;
}

function setup(text, overrides = {}) {
  const state = {
    decorations: new Map(),
    commandsRun: [],
    registered: {},
    folding: null,
    hover: null,
    status: [],
    memento: new Map(),
    types: [],
    on: {},
  };
  const doc = makeDocument(text);
  const editor = {
    document: doc,
    selections: [new Range(0, 0, 0, 0)],
    setDecorations(type, options) { state.decorations.set(type.key, options); },
  };
  let nextType = 0;
  const vscode = {
    Position, Range, EventEmitter,
    Uri: { file: (p) => ({ fsPath: p, scheme: 'file' }), parse: (s) => ({ data: s }) },
    ThemeIcon: class { constructor(id) { this.id = id; } },
    MarkdownString: class { constructor(v) { this.value = v; } },
    Hover: class { constructor(c, r) { this.contents = c; this.range = r; } },
    FoldingRange: class { constructor(s, e, k) { this.start = s; this.end = e; this.kind = k; } },
    FoldingRangeKind: { Region: 3 },
    DecorationRangeBehavior: { ClosedClosed: 1 },
    OverviewRulerLane: { Right: 4 },
    StatusBarAlignment: { Left: 1, Right: 2 },
    ViewColumn: { Active: -1 },
    window: {
      visibleTextEditors: [editor],
      activeTextEditor: editor,
      terminals: [],
      createTextEditorDecorationType: (options) => {
        const type = { key: `t${nextType++}`, options, disposed: false, dispose() { this.disposed = true; } };
        state.types.push(type);
        return type;
      },
      createStatusBarItem: () => {
        const item = { text: '', show() { this.shown = true; }, hide() { this.shown = false; }, dispose() {} };
        state.status.push(item);
        return item;
      },
      setStatusBarMessage() {},
      showWarningMessage() {},
      showErrorMessage: async () => undefined,
      showQuickPick: async () => undefined,
      onDidChangeActiveTextEditor: (fn) => { state.on.activeEditor = fn; return { dispose() {} }; },
      onDidChangeVisibleTextEditors: (fn) => { state.on.visibleEditors = fn; return { dispose() {} }; },
      onDidChangeTextEditorSelection: (fn) => { state.selection = fn; return { dispose() {} }; },
    },
    workspace: {
      getConfiguration: () => ({ get: (key) => (key in overrides ? overrides[key] : defaults[key]) }),
      onDidChangeConfiguration: (fn) => { state.on.configuration = fn; return { dispose() {} }; },
      onDidChangeTextDocument: (fn) => { state.on.textDocument = fn; return { dispose() {} }; },
      onDidCloseTextDocument: () => ({ dispose() {} }),
    },
    languages: {
      registerFoldingRangeProvider: (sel, p) => { state.folding = p; return { dispose() {} }; },
      registerHoverProvider: (sel, p) => { state.hover = p; return { dispose() {} }; },
    },
    commands: {
      registerCommand: (id, fn) => { state.registered[id] = fn; return { dispose() {} }; },
      executeCommand: async (id, args) => { state.commandsRun.push([id, args]); },
    },
  };
  const original = Module._load;
  Module._load = function (request, ...rest) {
    return request === 'vscode' ? vscode : original.call(this, request, ...rest);
  };
  for (const name of Object.keys(require.cache)) {
    if (name.includes(path.join('extension', 'src'))) delete require.cache[name];
  }
  const ext = require('../src/extension');
  const storage = fs.mkdtempSync(path.join(os.tmpdir(), 'fdev-ext-'));
  const context = {
    subscriptions: [],
    globalStorageUri: { fsPath: storage },
    workspaceState: {
      get: (k) => state.memento.get(k),
      update: (k, v) => { state.memento.set(k, v); return Promise.resolve(); },
    },
  };
  ext.activate(context);
  return { state, editor, doc, ext, context, vscode, restore: () => { Module._load = original; } };
}

const SRC = [
  "import 'package:fdev_log/fdev_log.dart';",
  "const _log = Log('Billing');",
  'void f() {',
  "  _log.info('start');",
  '  _log.error(',
  "    'failed',",
  '    e,',
  '  );',
  '}',
].join('\n');

const all = (state) => [...state.decorations.values()].flat();

test('dim is the default: colours and opacity, one mark per log', () => {
  const t = setup(SRC);
  try {
    assert.strictEqual(all(t.state).filter((o) => o.range).length > 0, true);
    const withOpacity = [...t.state.decorations.keys()];
    assert.ok(withOpacity.length >= 3);
    // 4 logs: an import, a declaration, an info call and an error call.
    const texts = [...t.state.decorations.entries()].filter(([, v]) => v.length && v[0].range.end.line >= v[0].range.start.line);
    assert.ok(texts.length > 0);
    assert.strictEqual(t.state.status[0].text, '$(circle-large-outline) 4');
    assert.match(t.state.status[0].tooltip, /4 logs in this file, dimmed/);
    assert.strictEqual(t.state.status[1].text, '$(rocket) fdev');
  } finally { t.restore(); }
});

test('hide mode hides the text, adds markers, folds multi-line statements, hover shows the code', async () => {
  const t = setup(SRC);
  try {
    t.state.registered['fdev.logs.hide']();
    // Hidden ranges use the display:none type; markers are empty ranges with a contentText.
    const hidden = all(t.state).filter((o) => o.renderOptions === undefined && o.range && o.range.start.compareTo(o.range.end) !== 0);
    assert.ok(hidden.length >= 4);
    const markers = all(t.state).filter((o) => o.renderOptions);
    assert.deepStrictEqual(
      markers.map((m) => m.renderOptions.before.contentText).sort(),
      ['● error', '● info', '● logger'], // the import under the cursor is revealed, not hidden
    );
    assert.strictEqual(t.state.status[0].text, '$(eye-closed) 4');

    // Folding ranges: only the multi-line, whole-line statement.
    const ranges = t.state.folding.provideFoldingRanges(t.doc);
    assert.deepStrictEqual(ranges.map((r) => [r.start, r.end]), [[4, 7]]);

    // The cursor is on line 0 (the import): it is revealed, so it is dimmed, not hidden.
    const dimmedImport = [...t.state.decorations.values()].flat().some((o) => o.range.start.line === 0 && !o.renderOptions);
    assert.ok(dimmedImport);

    // Hover on the marker of the info call.
    const hover = t.state.hover.provideHover(t.doc, new Position(3, 2));
    assert.ok(hover);
    assert.match(hover.contents.value, /_log\.info\('start'\);/);
    // The cursor's own line (the import) is revealed: no hover there.
    assert.strictEqual(t.state.hover.provideHover(t.doc, new Position(0, 2)), undefined);

    // The fold command ran for the multi-line statement.
    await new Promise((r) => setTimeout(r, 300));
    const fold = t.state.commandsRun.find(([id]) => id === 'editor.fold');
    assert.ok(fold, 'editor.fold was run');
    assert.deepStrictEqual(fold[1].selectionLines, [4]);

    // Leaving hide takes the range away from the folding provider, which
    // unfolds it by itself. The command is not run: on a line without a range
    // of ours it would unfold the range around it (a function the person folded).
    t.state.registered['fdev.logs.show']();
    assert.deepStrictEqual(t.state.folding.provideFoldingRanges(t.doc), []);
    await new Promise((r) => setTimeout(r, 300));
    assert.strictEqual(t.state.commandsRun.some(([id]) => id === 'editor.unfold'), false);
  } finally { t.restore(); }
});

test('the cursor on a folded statement unfolds it (and only it), and folds it again when it leaves', async () => {
  const t = setup(SRC);
  const folds = () => t.state.commandsRun.filter(([id]) => id === 'editor.fold' || id === 'editor.unfold');
  try {
    t.state.registered['fdev.logs.hide']();
    await new Promise((r) => setTimeout(r, 300));
    assert.deepStrictEqual(folds().map(([id, a]) => [id, a.selectionLines]), [['editor.fold', [4]]]);

    t.editor.selections = [new Range(5, 2, 5, 2)];
    t.state.selection({ textEditor: t.editor });
    await new Promise((r) => setTimeout(r, 300));
    assert.deepStrictEqual(folds().map(([id, a]) => [id, a.selectionLines]).slice(1), [['editor.unfold', [4]]]);

    t.editor.selections = [new Range(3, 0, 3, 0)];
    t.state.selection({ textEditor: t.editor });
    await new Promise((r) => setTimeout(r, 300));
    // The cursor is on the info call now; the multi-line statement is folded again, once.
    assert.deepStrictEqual(folds().map(([id, a]) => [id, a.selectionLines]).slice(2), [['editor.fold', [4]]]);

    // No change in what the cursor touches: no command at all.
    t.editor.selections = [new Range(3, 1, 3, 1)];
    t.state.selection({ textEditor: t.editor });
    await new Promise((r) => setTimeout(r, 300));
    assert.strictEqual(folds().length, 3);
  } finally { t.restore(); }
});

test('typing does not run a fold command again', async () => {
  const t = setup(SRC);
  try {
    t.state.registered['fdev.logs.hide']();
    await new Promise((r) => setTimeout(r, 300));
    const before = t.state.commandsRun.length;
    t.doc.version++;
    t.state.on.textDocument({ document: t.doc, contentChanges: [{ range: new Range(3, 0, 3, 0), text: 'x' }] });
    await new Promise((r) => setTimeout(r, 400));
    assert.strictEqual(t.state.commandsRun.length, before);
  } finally { t.restore(); }
});

test('a settings change replaces the decoration types: none is left behind, none is made per keystroke', async () => {
  const overrides = {};
  const t = setup(SRC, overrides);
  try {
    t.state.registered['fdev.logs.hide']();
    const made = t.state.types.length;
    for (let i = 0; i < 5; i++) {
      t.doc.version++;
      t.state.on.textDocument({ document: t.doc, contentChanges: [{ range: new Range(3, 0, 3, 0), text: 'x' }] });
      await new Promise((r) => setTimeout(r, 200));
    }
    assert.strictEqual(t.state.types.length, made, 'typing made new types');

    overrides['logs.opacity'] = 0.5;
    t.state.on.configuration({ affectsConfiguration: () => true });
    assert.ok(t.state.types.slice(0, made).every((x) => x.disposed), 'old types were not disposed');
    assert.ok(t.state.types.length > made, 'new types were not made');
    assert.ok(t.state.types.slice(made).every((x) => !x.disposed));
  } finally { t.restore(); }
});

test('a settings change with the gutter dot and the ruler mark off does not throw', () => {
  const overrides = { 'logs.gutterIcons': false, 'logs.overviewRuler': false };
  const t = setup(SRC, overrides);
  try {
    assert.doesNotThrow(() => t.state.on.configuration({ affectsConfiguration: () => true }));
    assert.doesNotThrow(() => t.state.registered['fdev.logs.hide']());
    overrides['logs.gutterIcons'] = true;
    assert.doesNotThrow(() => t.state.on.configuration({ affectsConfiguration: () => true }));
    assert.ok(all(t.state).length > 0);
  } finally { t.restore(); }
});

test('cycle goes show, dim, hide and the mode is kept in the workspace state only', () => {
  const t = setup(SRC);
  try {
    t.state.registered['fdev.logs.cycleMode']();
    assert.strictEqual(t.state.memento.get('fdev.logs.mode'), 'hide');
    t.state.registered['fdev.logs.cycleMode']();
    assert.strictEqual(t.state.memento.get('fdev.logs.mode'), 'show');
    t.state.registered['fdev.logs.cycleMode']();
    assert.strictEqual(t.state.memento.get('fdev.logs.mode'), 'dim');
    t.state.registered['fdev.logs.resetMode']();
    assert.strictEqual(t.state.memento.get('fdev.logs.mode'), undefined);
  } finally { t.restore(); }
});

test('levels not chosen stay as in show', () => {
  const t = setup(SRC);
  try {
    t.context.workspaceState.update('fdev.logs.levels', ['error']);
    t.state.registered['fdev.logs.hide']();
    const markers = all(t.state).filter((o) => o.renderOptions).map((m) => m.renderOptions.before.contentText);
    assert.deepStrictEqual(markers, ['● error']);
  } finally { t.restore(); }
});

test('a label marker carries the start of the message', () => {
  const t = setup(SRC, { 'logs.hiddenMarker': 'label' });
  try {
    t.state.registered['fdev.logs.hide']();
    const markers = all(t.state).filter((o) => o.renderOptions).map((m) => m.renderOptions.before.contentText);
    assert.ok(markers.includes('● info “start”'));
    assert.ok(markers.includes('● error “failed”'));
  } finally { t.restore(); }
});

test('a file in an unsupported language is left alone', () => {
  const t = setup(SRC, { 'logs.languages': ['python'] });
  try {
    assert.strictEqual(all(t.state).length, 0);
    assert.strictEqual(t.state.status[0].shown, false);
  } finally { t.restore(); }
});

test('the master switch turns everything off', () => {
  const t = setup(SRC, { 'logs.enabled': false });
  try {
    assert.strictEqual(all(t.state).length, 0);
  } finally { t.restore(); }
});

test('every command in package.json is registered and every keybinding or menu command is declared', () => {
  const t = setup(SRC);
  try {
    const declared = pkg.contributes.commands.map((c) => c.command);
    for (const id of declared) assert.ok(t.state.registered[id], `${id} is not registered`);
    for (const id of Object.keys(t.state.registered)) assert.ok(declared.includes(id), `${id} is not declared`);
    for (const k of pkg.contributes.keybindings) assert.ok(declared.includes(k.command), k.command);
    for (const items of Object.values(pkg.contributes.menus || {})) {
      for (const m of items) assert.ok(declared.includes(m.command), m.command);
    }
  } finally { t.restore(); }
});
