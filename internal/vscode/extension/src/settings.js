'use strict';

// Reads the fdev.* settings into one plain object, with every value checked
// and clamped, so the rest of the extension never meets an undefined colour
// or an opacity of 7.

const vscode = require('vscode');
const scan = require('./scan');

const DEFAULT_COLORS = {
  debug: '#8b949e',
  info: '#58a6ff',
  success: '#3fb950',
  warning: '#d29922',
  error: '#f85149',
  fatal: '#ff4d94',
  value: '#39c5cf',
  http: '#bc8cff',
  declaration: '#768390',
};

const DEFAULT_LANGUAGES = [
  'dart',
  'javascript',
  'typescript',
  'javascriptreact',
  'typescriptreact',
  'python',
  'kotlin',
  'swift',
  'go',
];

const MODES = ['show', 'dim', 'hide'];

function clamp(value, min, max, fallback) {
  const n = Number(value);
  if (!Number.isFinite(n)) return fallback;
  return Math.min(max, Math.max(min, n));
}

function oneOf(value, allowed, fallback) {
  return allowed.includes(value) ? value : fallback;
}

function stringList(value, fallback) {
  if (!Array.isArray(value)) return fallback;
  return value.filter((v) => typeof v === 'string');
}

/**
 * read returns the current settings. The `compiled` rules are built here, so
 * a caller that keeps this object (and replaces it on a settings change)
 * compiles the regular expressions once.
 */
function read() {
  const cfg = vscode.workspace.getConfiguration('fdev');
  const colors = { ...DEFAULT_COLORS };
  const userColors = cfg.get('logs.colors');
  if (userColors && typeof userColors === 'object') {
    for (const level of scan.LEVELS) {
      if (typeof userColors[level] === 'string' && userColors[level].trim() !== '') {
        colors[level] = userColors[level].trim();
      }
    }
  }

  const levels = stringList(cfg.get('logs.levels'), scan.LEVELS).filter((l) =>
    scan.LEVELS.includes(l),
  );

  const logs = {
    enabled: cfg.get('logs.enabled') !== false,
    mode: oneOf(cfg.get('logs.mode'), MODES, 'dim'),
    levels,
    languages: stringList(cfg.get('logs.languages'), DEFAULT_LANGUAGES).map((l) => l.toLowerCase()),
    colors,
    colorText: cfg.get('logs.colorText') !== false,
    background: cfg.get('logs.background') === true,
    backgroundAlpha: clamp(cfg.get('logs.backgroundAlpha'), 0, 1, 0.12),
    gutterIcons: cfg.get('logs.gutterIcons') !== false,
    overviewRuler: cfg.get('logs.overviewRuler') !== false,
    fontStyle: oneOf(cfg.get('logs.fontStyle'), ['normal', 'italic'], 'normal'),
    opacity: clamp(cfg.get('logs.opacity'), 0.05, 1, 0.35),
    hiddenMarker: oneOf(cfg.get('logs.hiddenMarker'), ['dot', 'label', 'none'], 'dot'),
    fold: cfg.get('logs.fold') !== false,
    revealOnCursor: cfg.get('logs.revealOnCursor') !== false,
    peekSeconds: clamp(cfg.get('logs.peekSeconds'), 1, 120, 5),
    maxFileSize: clamp(cfg.get('logs.maxFileSize'), 1024, 1024 * 1024 * 1024, 1024 * 1024),
    statusBar: oneOf(cfg.get('logs.statusBar'), ['left', 'right', 'off'], 'right'),
  };

  const compiled = scan.compile({
    rules: Array.isArray(cfg.get('logs.rules')) ? cfg.get('logs.rules') : undefined,
    includeDeclarations: cfg.get('logs.includeDeclarations') !== false,
    declarationRules: Array.isArray(cfg.get('logs.declarationRules'))
      ? cfg.get('logs.declarationRules')
      : undefined,
    includeImports: cfg.get('logs.includeImports') !== false,
    importPattern: cfg.get('logs.importPattern'),
  });

  const pathSetting = cfg.get('path');
  return {
    logs,
    compiled,
    path: typeof pathSetting === 'string' ? pathSetting.trim() : '',
    launcherButton: cfg.get('statusBar.launcher') !== false,
  };
}

module.exports = { read, DEFAULT_COLORS, DEFAULT_LANGUAGES, MODES };
