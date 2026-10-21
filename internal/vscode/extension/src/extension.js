'use strict';

// fdev for VS Code: shows, dims or hides the log calls in the code you are
// editing, colours them by level, and opens the fdev launcher in a terminal.
//
// What lives where:
//   scan.js         finds the logs in a text (no vscode, tested with node:test)
//   settings.js     the fdev.* settings, checked
//   decorations.js  the colours, the dim and the hidden look
//   launcher.js     "fdev: Open"
//
// The display mode (show, dim, hide) and the levels it applies to are kept in
// the workspace state, which VS Code stores outside the project: nothing of
// fdev is written into the project's files, so nothing of it reaches git. The
// fdev.logs.mode setting is only the mode a workspace starts in.

const vscode = require('vscode');
const scan = require('./scan');
const settingsModule = require('./settings');
const { Decorations } = require('./decorations');
const launcher = require('./launcher');

const MODE_KEY = 'fdev.logs.mode';
const LEVELS_KEY = 'fdev.logs.levels';
const MODES = settingsModule.MODES;
const UPDATE_DELAY = 150; // ms to wait after typing before the file is scanned again
const SELECTION_DELAY = 30;

const ICONS = { show: '$(eye)', dim: '$(circle-large-outline)', hide: '$(eye-closed)' };
const VERBS = { show: 'shown', dim: 'dimmed', hide: 'hidden' };

let context;
let settings;
let deco;
let logsItem = null;
let launcherItem = null;
let foldEmitter;
let providers = [];
let peeking = false;
let peekTimer = null;
let updateTimer = null;
let selectionTimer = null;
let foldChain = Promise.resolve();
const pendingDocs = new Set();
const pendingSelections = new Set();

// uri -> { version, languageId, results }
const cache = new Map();
// uri -> the start lines of the statements this extension folded
const folded = new Map();
// editor -> the revealed statements of its last render (to skip no-op renders)
const revealSignature = new WeakMap();

// ---------------------------------------------------------------------------
// State.

function storedMode() {
  const mode = context.workspaceState.get(MODE_KEY);
  return MODES.includes(mode) ? mode : settings.logs.mode;
}

/** The mode in force now: a temporary "peek" shows everything. */
function currentMode() {
  return peeking ? 'show' : storedMode();
}

function currentLevels() {
  const stored = context.workspaceState.get(LEVELS_KEY);
  const list = Array.isArray(stored) ? stored : settings.logs.levels;
  return new Set(list.filter((l) => scan.LEVELS.includes(l)));
}

function isSupported(document) {
  return settings.logs.enabled && settings.logs.languages.includes(document.languageId);
}

/** The logs of a document, from the cache while the text has not changed; null when it is too big. */
function resultsFor(document) {
  const key = document.uri.toString();
  const hit = cache.get(key);
  if (hit && hit.version === document.version && hit.languageId === document.languageId) {
    return hit.results;
  }
  let results = null;
  const size = document.offsetAt(document.lineAt(document.lineCount - 1).range.end);
  if (size <= settings.logs.maxFileSize) {
    results = scan.scan(document.getText(), document.languageId, settings.compiled);
  }
  cache.set(key, { version: document.version, languageId: document.languageId, results });
  return results;
}

/** How a log is drawn now: "show", "dim" or "hide" (a hidden one the cursor is on is dimmed). */
function variantOf(editor, document, r, mode, levels) {
  if (mode === 'show' || !levels.has(r.level)) return 'show';
  if (mode === 'hide' && settings.logs.revealOnCursor && editor && touches(editor, document, r)) {
    return 'dim';
  }
  return mode;
}

// Whether a selection is on the statement: on any of its lines when it is
// alone on them, else on the statement itself.
function touches(editor, document, r) {
  for (const sel of editor.selections) {
    if (r.wholeLines) {
      if (sel.start.line <= r.endLine && sel.end.line >= r.startLine) return true;
    } else {
      const range = new vscode.Range(document.positionAt(r.start), document.positionAt(r.end));
      if (range.intersection(sel)) return true;
    }
  }
  return false;
}

// ---------------------------------------------------------------------------
// Drawing.

function apply(editor) {
  const document = editor.document;
  if (!isSupported(document)) {
    deco.clear(editor);
    return;
  }
  const results = resultsFor(document);
  if (!results) {
    deco.clear(editor);
    return;
  }
  const mode = currentMode();
  const levels = currentLevels();
  const revealed = [];
  const items = [];
  results.forEach((r, index) => {
    const variant = variantOf(editor, document, r, mode, levels);
    const range = new vscode.Range(document.positionAt(r.start), document.positionAt(r.end));
    const hiding = mode === 'hide' && levels.has(r.level);
    if (hiding && variant === 'dim') revealed.push(index);
    let marker = '';
    if (variant === 'hide') {
      let preview = '';
      if (settings.logs.hiddenMarker === 'label') {
        const body = document.getText(range);
        preview = scan.preview(body, { kind: r.kind, start: 0, end: body.length });
      }
      marker = deco.markerText(settings.logs.hiddenMarker, r.level, preview);
    }
    items.push({
      range,
      firstLine: new vscode.Range(range.start.line, 0, range.start.line, 0),
      level: r.level,
      variant,
      marker,
    });
  });
  deco.render(editor, items);
  revealSignature.set(editor, revealed.join(','));
}

function updateAll() {
  for (const editor of vscode.window.visibleTextEditors) apply(editor);
  updateStatus();
  syncFolds();
}

function refresh() {
  foldEmitter.fire();
  updateAll();
}

function scheduleDocument(document) {
  pendingDocs.add(document);
  clearTimeout(updateTimer);
  updateTimer = setTimeout(() => {
    for (const editor of vscode.window.visibleTextEditors) {
      if (pendingDocs.has(editor.document)) apply(editor);
    }
    pendingDocs.clear();
    updateStatus();
    syncFolds();
  }, UPDATE_DELAY);
}

// The cursor moved: only a change in which statements it touches is worth a
// new render.
function scheduleSelection(editor) {
  pendingSelections.add(editor);
  clearTimeout(selectionTimer);
  selectionTimer = setTimeout(() => {
    for (const e of pendingSelections) {
      if (!vscode.window.visibleTextEditors.includes(e)) continue;
      const results = isSupported(e.document) ? resultsFor(e.document) : null;
      if (!results) continue;
      const levels = currentLevels();
      const revealed = [];
      results.forEach((r, index) => {
        if (levels.has(r.level) && touches(e, e.document, r)) revealed.push(index);
      });
      if (revealed.join(',') !== revealSignature.get(e)) apply(e);
    }
    pendingSelections.clear();
    syncFolds();
  }, SELECTION_DELAY);
}

// ---------------------------------------------------------------------------
// Folding: a statement that spans several lines and is alone on them is
// folded while hidden, so it takes one line like its marker.

function foldRanges(document) {
  if (!settings.logs.fold || currentMode() !== 'hide' || !isSupported(document)) return [];
  const results = resultsFor(document);
  if (!results) return [];
  const levels = currentLevels();
  return results
    .filter((r) => r.wholeLines && r.endLine > r.startLine && levels.has(r.level))
    .map((r) => new vscode.FoldingRange(r.startLine, r.endLine, vscode.FoldingRangeKind.Region));
}

// Folds and unfolds, in the active editor (the fold commands act on it), what
// differs between the statements that should be folded and those this
// extension folded earlier. Runs one at a time.
function syncFolds() {
  foldChain = foldChain.then(doSyncFolds).catch(() => {});
}

async function doSyncFolds() {
  const editor = vscode.window.activeTextEditor;
  if (!editor) return;
  const document = editor.document;
  const key = document.uri.toString();
  const tracked = folded.get(key) || new Set();
  const desired = new Set();
  if (settings.logs.fold && currentMode() === 'hide' && isSupported(document)) {
    const results = resultsFor(document);
    const levels = currentLevels();
    for (const r of results || []) {
      if (!r.wholeLines || r.endLine <= r.startLine) continue;
      if (variantOf(editor, document, r, 'hide', levels) === 'hide') desired.add(r.startLine);
    }
  }
  // A fold this extension no longer offers (hide was left, a peek started, a
  // level was dropped) disappears with the folding provider's ranges, and
  // must not be unfolded by line: with no range of its own on that line the
  // command would act on the range around it, a function the person folded.
  // Only a statement that stays foldable and is revealed (the cursor is on
  // it) needs the command.
  const offered = new Set(foldRanges(document).map((r) => r.start));
  const toFold = [...desired].filter((l) => !tracked.has(l));
  const toUnfold = [...tracked].filter((l) => !desired.has(l) && offered.has(l));
  if (toFold.length === 0 && toUnfold.length === 0) {
    if (desired.size || tracked.size) folded.set(key, desired);
    return;
  }
  // The folding model asks the provider again after a change; give it a moment.
  await new Promise((resolve) => setTimeout(resolve, 120));
  if (vscode.window.activeTextEditor !== editor) return;
  if (toUnfold.length) {
    await vscode.commands.executeCommand('editor.unfold', { selectionLines: toUnfold, levels: 1 });
  }
  if (toFold.length) {
    await vscode.commands.executeCommand('editor.fold', { selectionLines: toFold, levels: 1 });
  }
  folded.set(key, desired);
}

// VS Code moves its own folds with the text; the lines this extension
// remembers must move the same way.
function shiftFolds(event) {
  const key = event.document.uri.toString();
  const tracked = folded.get(key);
  if (!tracked || tracked.size === 0) return;
  const changes = [...event.contentChanges].sort(
    (a, b) => b.range.start.compareTo(a.range.start),
  );
  let lines = [...tracked];
  for (const change of changes) {
    const first = change.range.start.line;
    const last = change.range.end.line;
    const added = (change.text.match(/\n/g) || []).length;
    const delta = added - (last - first);
    lines = lines
      .map((line) => {
        if (line < first) return line;
        if (line > last) return line + delta;
        if (line === first && first === last && added === 0) return line;
        return -1; // inside the replaced text: forget it
      })
      .filter((line) => line >= 0);
  }
  folded.set(key, new Set(lines));
}

// ---------------------------------------------------------------------------
// Hover: the code behind a marker.

function hoverFor(document, position) {
  if (currentMode() !== 'hide' || !isSupported(document)) return undefined;
  const results = resultsFor(document);
  if (!results) return undefined;
  const levels = currentLevels();
  const editor = vscode.window.visibleTextEditors.find((e) => e.document === document);
  const offset = document.offsetAt(position);
  for (const r of results) {
    if (offset < r.start || offset >= r.end) continue;
    if (variantOf(editor, document, r, 'hide', levels) !== 'hide') continue;
    const range = new vscode.Range(document.positionAt(r.start), document.positionAt(r.end));
    const text = dedent(document.getText(range), document.lineAt(range.start.line).text);
    const longest = (text.match(/`+/g) || []).reduce((n, run) => Math.max(n, run.length), 0);
    const fence = '`'.repeat(Math.max(3, longest + 1));
    const md = new vscode.MarkdownString(`${fence}${document.languageId}\n${text}\n${fence}`);
    return new vscode.Hover(md, range);
  }
  return undefined;
}

// Takes off the indentation of the statement's first line from the others.
function dedent(text, firstLine) {
  const indent = /^[ \t]*/.exec(firstLine)[0];
  if (!indent) return text;
  return text
    .split(/\r?\n/)
    .map((line, i) => (i > 0 && line.startsWith(indent) ? line.slice(indent.length) : line))
    .join('\n');
}

function registerProviders() {
  for (const p of providers) p.dispose();
  providers = [];
  const selector = settings.logs.languages.map((language) => ({ language }));
  if (selector.length === 0) return;
  providers.push(
    vscode.languages.registerFoldingRangeProvider(selector, {
      onDidChangeFoldingRanges: foldEmitter.event,
      provideFoldingRanges: (document) => foldRanges(document),
    }),
    vscode.languages.registerHoverProvider(selector, {
      provideHover: (document, position) => hoverFor(document, position),
    }),
  );
}

// ---------------------------------------------------------------------------
// Status bar.

function createStatusItems() {
  if (logsItem) logsItem.dispose();
  if (launcherItem) launcherItem.dispose();
  logsItem = null;
  launcherItem = null;
  if (settings.logs.statusBar !== 'off') {
    logsItem = vscode.window.createStatusBarItem(
      settings.logs.statusBar === 'left'
        ? vscode.StatusBarAlignment.Left
        : vscode.StatusBarAlignment.Right,
      100,
    );
    logsItem.name = 'fdev logs';
    logsItem.command = 'fdev.logs.cycleMode';
  }
  if (settings.launcherButton) {
    launcherItem = vscode.window.createStatusBarItem(vscode.StatusBarAlignment.Left, 0);
    launcherItem.name = 'fdev';
    launcherItem.text = '$(rocket) fdev';
    launcherItem.tooltip = 'Open fdev (launcher and log viewer) in a terminal';
    launcherItem.command = 'fdev.open';
    launcherItem.show();
  }
}

function updateStatus() {
  if (!logsItem) return;
  const editor = vscode.window.activeTextEditor;
  if (!editor || !isSupported(editor.document)) {
    logsItem.hide();
    return;
  }
  const mode = currentMode();
  const results = resultsFor(editor.document);
  const next = MODES[(MODES.indexOf(storedMode()) + 1) % MODES.length];
  const levels = currentLevels();
  const lines = [];
  if (!results) {
    logsItem.text = `${ICONS[mode]} –`;
    lines.push('This file is larger than fdev.logs.maxFileSize, so its logs are left alone.');
  } else {
    logsItem.text = `${ICONS[mode]} ${results.length}`;
    lines.push(
      `${results.length} ${results.length === 1 ? 'log' : 'logs'} in this file, ${VERBS[mode]}` +
        (peeking ? ' for a moment (peek).' : '.'),
    );
    if (mode !== 'show' && levels.size < scan.LEVELS.length) {
      lines.push(`${VERBS[mode]} levels: ${[...levels].join(', ') || 'none'}`);
    }
  }
  lines.push(`Click to switch to ${next}. Ctrl+Alt+L (Cmd+Alt+L on a Mac) does the same.`);
  logsItem.tooltip = `fdev logs\n${lines.join('\n')}`;
  logsItem.show();
}

// ---------------------------------------------------------------------------
// Commands.

function flash(text) {
  vscode.window.setStatusBarMessage(`fdev: ${text}`, 2500);
}

function setMode(mode) {
  endPeek(false);
  context.workspaceState.update(MODE_KEY, mode);
  flash(`logs ${VERBS[mode]}`);
  refresh();
}

function cycleMode() {
  const current = storedMode();
  setMode(MODES[(MODES.indexOf(current) + 1) % MODES.length]);
}

function endPeek(update = true) {
  clearTimeout(peekTimer);
  peekTimer = null;
  if (!peeking) return;
  peeking = false;
  if (update) refresh();
}

// Shows every log as in "show" for fdev.logs.peekSeconds, then goes back; a
// second call goes back at once.
function peek() {
  if (peeking) {
    endPeek();
    return;
  }
  if (storedMode() === 'show') {
    flash('logs are already shown');
    return;
  }
  peeking = true;
  peekTimer = setTimeout(() => endPeek(), settings.logs.peekSeconds * 1000);
  refresh();
}

async function chooseLevels() {
  const current = currentLevels();
  const items = scan.LEVELS.map((level) => ({
    label: level,
    description: level === 'declaration' ? 'logger declarations and imports' : undefined,
    picked: current.has(level),
  }));
  const picked = await vscode.window.showQuickPick(items, {
    canPickMany: true,
    title: 'fdev: levels that dim and hide apply to',
    placeHolder: 'The levels you leave out stay as in "show" (coloured, not dimmed or hidden)',
  });
  if (!picked) return;
  context.workspaceState.update(
    LEVELS_KEY,
    picked.map((p) => p.label),
  );
  refresh();
}

function resetMode() {
  endPeek(false);
  context.workspaceState.update(MODE_KEY, undefined);
  context.workspaceState.update(LEVELS_KEY, undefined);
  flash(`logs back to the settings (${settings.logs.mode})`);
  refresh();
}

// ---------------------------------------------------------------------------
// Settings.

let lastErrorText = '';

function loadSettings() {
  settings = settingsModule.read();
  deco.configure(settings.logs);
  cache.clear();
  const errors = settings.compiled.errors;
  const text = errors.join('\n');
  if (errors.length && text !== lastErrorText) {
    vscode.window.showWarningMessage(
      `fdev: ${errors[0]}${errors.length > 1 ? ` (and ${errors.length - 1} more)` : ''} - fix it in the fdev.logs settings.`,
    );
  }
  lastErrorText = text;
}

// ---------------------------------------------------------------------------

function activate(ctx) {
  context = ctx;
  deco = new Decorations(ctx);
  foldEmitter = new vscode.EventEmitter();
  loadSettings();
  createStatusItems();
  registerProviders();

  const command = (id, fn) => ctx.subscriptions.push(vscode.commands.registerCommand(id, fn));
  command('fdev.logs.cycleMode', cycleMode);
  command('fdev.logs.show', () => setMode('show'));
  command('fdev.logs.dim', () => setMode('dim'));
  command('fdev.logs.hide', () => setMode('hide'));
  command('fdev.logs.chooseLevels', chooseLevels);
  command('fdev.logs.peek', peek);
  command('fdev.logs.resetMode', resetMode);
  command('fdev.logs.openSettings', () =>
    vscode.commands.executeCommand('workbench.action.openSettings', '@ext:nadermozaffari.fdev'),
  );
  command('fdev.open', () => launcher.open(settings));

  ctx.subscriptions.push(
    foldEmitter,
    { dispose: () => deco.dispose() },
    { dispose: () => providers.forEach((p) => p.dispose()) },
    { dispose: () => logsItem && logsItem.dispose() },
    { dispose: () => launcherItem && launcherItem.dispose() },
    { dispose: () => [updateTimer, selectionTimer, peekTimer].forEach(clearTimeout) },
    vscode.workspace.onDidChangeConfiguration((e) => {
      if (!e.affectsConfiguration('fdev')) return;
      loadSettings();
      createStatusItems();
      registerProviders();
      refresh();
    }),
    vscode.workspace.onDidChangeTextDocument((e) => {
      if (e.contentChanges.length === 0) return;
      shiftFolds(e);
      if (isSupported(e.document)) scheduleDocument(e.document);
    }),
    vscode.workspace.onDidCloseTextDocument((document) => {
      const key = document.uri.toString();
      cache.delete(key);
      folded.delete(key);
    }),
    vscode.window.onDidChangeActiveTextEditor(() => {
      updateAll();
    }),
    vscode.window.onDidChangeVisibleTextEditors(() => {
      updateAll();
    }),
    vscode.window.onDidChangeTextEditorSelection((e) => {
      if (currentMode() === 'hide' && settings.logs.revealOnCursor) scheduleSelection(e.textEditor);
    }),
  );

  updateAll();
}

function deactivate() {
  clearTimeout(updateTimer);
  clearTimeout(selectionTimer);
  clearTimeout(peekTimer);
}

module.exports = { activate, deactivate };
