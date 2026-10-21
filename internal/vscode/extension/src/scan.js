'use strict';

// Finds the log calls in a source file, without importing "vscode" so that
// the same code runs under `node --test`.
//
// A log is found in four steps:
//  1. A small lexer marks every character of the file as code, string or
//     comment, so that `// _log.info('x')` and `"call _log.info(now)"` are
//     never taken for logs, and parentheses inside strings are not counted.
//  2. The rules (regular expressions that end right after the opening "(")
//     are run over the text; matches that start outside code are dropped.
//  3. From the "(" the matching ")" is looked up through the lexer's marks,
//     then a trailing ";" and (when the statement is alone on its lines) a
//     trailing "// comment" are added.
//  4. Declarations of loggers (`const _log = Log('Billing');`) and imports of
//     logger libraries are found the same way and get the level
//     "declaration".
//
// Offsets in the results are positions in the text that was scanned, which
// is what VS Code's document.positionAt() expects, CRLF included.

/** The levels a log can have, in the order the settings list them. */
const LEVELS = [
  'debug',
  'info',
  'success',
  'warning',
  'error',
  'fatal',
  'value',
  'http',
  'declaration',
];

// Names the rules capture, mapped to the level they stand for: Log.warn is a
// warning, print() a debug line, Android's Log.e an error, Go's Fatalf fatal.
const ALIASES = {
  warn: 'warning',
  wtf: 'fatal',
  log: 'debug',
  print: 'debug',
  printf: 'debug',
  println: 'debug',
  debugprint: 'debug',
  debugprintstack: 'debug',
  debugprintthrottled: 'debug',
  dump: 'debug',
  dir: 'debug',
  table: 'debug',
  nslog: 'debug',
  os_log: 'debug',
  trace: 'debug',
  verbose: 'debug',
  fine: 'debug',
  finer: 'debug',
  finest: 'debug',
  request: 'http',
  response: 'http',
  information: 'info',
  notice: 'info',
  config: 'info',
  ok: 'success',
  err: 'error',
  severe: 'error',
  exception: 'error',
  critical: 'fatal',
  crit: 'fatal',
  fault: 'fatal',
  shout: 'fatal',
  fatalf: 'fatal',
  fatalln: 'fatal',
  panic: 'fatal',
  panicf: 'fatal',
  panicln: 'fatal',
  // The one-letter names of Android's Log, the logger package and friends.
  v: 'debug',
  t: 'debug',
  d: 'debug',
  i: 'info',
  s: 'success',
  w: 'warning',
  e: 'error',
  f: 'fatal',
};

/**
 * normalizeLevel turns whatever a rule captured ("warn", "W", "Println",
 * "wtf") into one of LEVELS. Unknown names become `fallback`.
 */
function normalizeLevel(name, fallback = 'debug') {
  if (typeof name !== 'string') return fallback;
  const key = name.trim().toLowerCase();
  if (LEVELS.includes(key)) return key;
  return ALIASES[key] || fallback;
}

// The rules. A rule's pattern must end right after the "(" that opens the
// call: the lexer finds the matching ")" from there. The level is the named
// group (?<level>...) when the pattern has one, else the rule's "level".
// These are also the defaults of the fdev.logs.rules setting in package.json
// (a test keeps the two equal).
const LOG_LEVELS_DART =
  'debug|info|success|warning|warn|error|fatal|wtf|value|http|request|response|' +
  'd|i|w|e|t|f|v|trace|verbose|severe|shout|fine|finer|finest|config';

const defaultRules = [
  {
    // Log('Tag').info(..) written as _log.info(..), Log.error(..), FdevLog.http(..)
    // and the logger / logging packages' own methods.
    pattern: String.raw`\b(?:_?log|_?logger|Log|Logger|FdevLog)\.(?<level>${LOG_LEVELS_DART})\s*\(`,
    languages: ['dart'],
  },
  {
    pattern: String.raw`(?<![\w$.])(?<level>print|debugPrint|debugPrintStack|debugPrintThrottled)\s*\(`,
    languages: ['dart'],
  },
  {
    pattern: String.raw`\bdeveloper\.log\s*\(`,
    level: 'debug',
    languages: ['dart'],
  },
  {
    pattern: String.raw`\bconsole\.(?<level>log|debug|info|warn|error|trace)\s*\(`,
    languages: ['javascript', 'typescript', 'javascriptreact', 'typescriptreact'],
  },
  {
    pattern: String.raw`\b(?:logging|logger|log|_logger|_log|LOGGER|LOG)\.(?<level>debug|info|warning|warn|error|critical|exception|fatal)\s*\(`,
    languages: ['python'],
  },
  {
    pattern: String.raw`(?<![\w$.])print\s*\(`,
    level: 'debug',
    languages: ['python'],
  },
  {
    pattern: String.raw`\b(?:Log|Timber)\.(?<level>v|d|i|w|e|wtf)\s*\(`,
    languages: ['kotlin'],
  },
  {
    pattern: String.raw`(?<![\w$.])(?<level>println|print)\s*\(`,
    languages: ['kotlin', 'swift'],
  },
  {
    pattern: String.raw`\b(?:logger|log)\.(?<level>trace|debug|info|notice|warning|error|fault|critical)\s*\(`,
    languages: ['swift'],
  },
  {
    pattern: String.raw`(?<![\w$.])(?<level>os_log|NSLog|debugPrint|dump)\s*\(`,
    languages: ['swift'],
  },
  {
    pattern: String.raw`\b(?:log|slog|logger)\.(?<level>Print|Printf|Println|Fatal|Fatalf|Fatalln|Panic|Panicf|Panicln|Debug|Info|Warn|Error)(?:Context)?\s*\(`,
    languages: ['go'],
  },
];

// Declarations of loggers, shown as level "declaration" when
// fdev.logs.includeDeclarations is on.
const defaultDeclarationRules = [
  {
    // const _log = Log('Billing');  static const _log = Log(...);
    // final _log = FdevLog...(  final Logger _log = Logger('x');
    pattern: String.raw`\b(?:static\s+)?(?:const|final|late\s+final|late|var)\s+(?:(?:Log|Logger|FdevLog)\s+)?\w+\s*=\s*(?:const\s+|new\s+)?(?:Log|Logger|FdevLog)(?:\.\w+)?\s*\(`,
    languages: ['dart'],
  },
  {
    pattern: String.raw`\b(?:const|let|var)\s+\w*(?:[Ll]og|LOG)\w*\s*=\s*(?:\w+\.)*(?:createLogger|getLogger|pino|debug)\s*\(`,
    languages: ['javascript', 'typescript', 'javascriptreact', 'typescriptreact'],
  },
  {
    pattern: String.raw`\b\w*(?:[Ll]og|LOG)\w*\s*=\s*(?:logging\.getLogger|structlog\.get_logger|get_logger|logging\.Logger)\s*\(`,
    languages: ['python'],
  },
  {
    pattern: String.raw`\b(?:private\s+|internal\s+)?(?:const\s+)?(?:val|var)\s+\w*(?:[Ll]og|LOG)\w*\s*=\s*(?:LoggerFactory\.getLogger|Logger\.getLogger|KotlinLogging\.logger)\s*\(`,
    languages: ['kotlin'],
  },
  {
    pattern: String.raw`\b(?:let|var)\s+\w*(?:[Ll]og|LOG)\w*\s*=\s*(?:Logger|OSLog)\s*\(`,
    languages: ['swift'],
  },
  {
    pattern: String.raw`\b\w*(?:[Ll]og|LOG)\w*\s*(?::=|=)\s*(?:slog\.New|log\.New|zap\.New\w*|logrus\.New)\s*\(`,
    languages: ['go'],
  },
];

// Import lines of logger libraries, shown as level "declaration" when
// fdev.logs.includeImports is on. It is run on the whole file with the "g"
// and "m" flags, so ^ is the start of a line, and a match is a whole line.
const defaultImportPattern = [
  // Dart, JS, TS: the URI (quoted) mentions a logger.
  String.raw`^[ \t]*(?:import|export)\b[^\n;]*['"][^'"\n]*(?:logger|fdev_log|logging|loguru|pino|winston)[^'"\n]*['"][^\n;]*;?`,
  // Python, Kotlin, Swift, Go.
  String.raw`^[ \t]*(?:import|from)\s+[\w.]*(?:logging|logger|loguru|structlog)\b[^\n]*`,
  String.raw`^[ \t]*import\s+(?:android\.util\.Log|timber\.log\.Timber|os\.log|OSLog|Logging)\b[^\n]*`,
].join('|');

// ---------------------------------------------------------------------------
// The lexer.

const KIND_CODE = 0;
const KIND_STRING = 1;
const KIND_COMMENT = 2;

// What differs between languages for the lexer: the line comment, whether
// "/* */" exists (and nests), the quotes, whether '''...''' exists, whether
// r'..' is raw, backtick strings, and ${...} / \(...) interpolation.
const SYNTAX = {
  dart: {
    line: '//', block: true, nested: true, quotes: `'"`, triple: true, rawPrefix: true,
    backtick: null, interp: 'dollar', interpIn: 'all',
  },
  js: {
    line: '//', block: true, nested: false, quotes: `'"`, triple: false, rawPrefix: false,
    backtick: 'template', interp: 'dollar', interpIn: 'backtick',
  },
  python: {
    line: '#', block: false, nested: false, quotes: `'"`, triple: true, rawPrefix: true,
    backtick: null, interp: null, interpIn: 'all',
  },
  kotlin: {
    line: '//', block: true, nested: true, quotes: `'"`, triple: true, rawPrefix: false,
    backtick: null, interp: 'dollar', interpIn: 'dquote',
  },
  swift: {
    line: '//', block: true, nested: true, quotes: `"`, triple: true, rawPrefix: false,
    backtick: null, interp: 'swift', interpIn: 'dquote',
  },
  go: {
    line: '//', block: true, nested: false, quotes: `'"`, triple: false, rawPrefix: false,
    backtick: 'raw', interp: null, interpIn: 'all',
  },
  generic: {
    line: '//', block: true, nested: false, quotes: `'"`, triple: false, rawPrefix: false,
    backtick: null, interp: null, interpIn: 'all',
  },
};

const SYNTAX_OF = {
  dart: 'dart',
  javascript: 'js',
  javascriptreact: 'js',
  typescript: 'js',
  typescriptreact: 'js',
  python: 'python',
  kotlin: 'kotlin',
  swift: 'swift',
  go: 'go',
};

function syntaxOf(languageId) {
  return SYNTAX[SYNTAX_OF[languageId] || 'generic'];
}

const WORD = /[\w$]/;
const RAW_LETTER = /[rRbBuUfF]/;

// Whether the quote at i is the start of a raw string: r'..' in Dart, r'..',
// rb'..', fr'..' in Python. The prefix must not be the end of a longer name.
function isRawPrefix(text, i) {
  let j = i - 1;
  let letters = 0;
  let raw = false;
  while (j >= 0 && letters < 2 && RAW_LETTER.test(text[j])) {
    if (text[j] === 'r' || text[j] === 'R') raw = true;
    j--;
    letters++;
  }
  if (j >= 0 && WORD.test(text[j])) return false;
  return raw && letters > 0;
}

/**
 * mask marks every character of `text` as code (0), string (1) or comment
 * (2). The delimiters of a string are string; the code inside ${...} of an
 * interpolated string is code, so that a call there still counts its
 * parentheses. A ' or " string that is not closed ends at the end of its
 * line, so a regular expression literal like /"/ in JavaScript cannot swallow
 * the rest of the file.
 */
function mask(text, syntax) {
  const n = text.length;
  const kind = new Uint8Array(n);
  // Contexts: { t: 's', q, raw, interp, multi } for a string, or
  // { t: 'i', open, close, depth } for the code of an interpolation.
  const stack = [];
  let i = 0;
  while (i < n) {
    const top = stack.length ? stack[stack.length - 1] : null;
    const c = text[i];

    if (top && top.t === 's') {
      if (!top.raw && c === '\\') {
        kind[i] = KIND_STRING;
        if (top.interp && syntax.interp === 'swift' && text[i + 1] === '(') {
          kind[i + 1] = KIND_STRING;
          stack.push({ t: 'i', open: '(', close: ')', depth: 0 });
          i += 2;
          continue;
        }
        if (i + 1 < n) kind[i + 1] = KIND_STRING;
        i += 2;
        continue;
      }
      if (!top.multi && c === '\n') {
        stack.pop();
        i++;
        continue;
      }
      if (text.startsWith(top.q, i)) {
        kind.fill(KIND_STRING, i, i + top.q.length);
        i += top.q.length;
        stack.pop();
        continue;
      }
      if (top.interp && syntax.interp === 'dollar' && c === '$' && text[i + 1] === '{') {
        kind[i] = kind[i + 1] = KIND_STRING;
        stack.push({ t: 'i', open: '{', close: '}', depth: 0 });
        i += 2;
        continue;
      }
      kind[i] = KIND_STRING;
      i++;
      continue;
    }

    // Code: the top level, or inside ${...}.
    if (top) {
      if (c === top.open) {
        top.depth++;
      } else if (c === top.close) {
        if (top.depth === 0) {
          kind[i] = KIND_STRING;
          stack.pop();
          i++;
          continue;
        }
        top.depth--;
      }
    }
    if (text.startsWith(syntax.line, i)) {
      let j = text.indexOf('\n', i);
      if (j < 0) j = n;
      kind.fill(KIND_COMMENT, i, j);
      i = j;
      continue;
    }
    if (syntax.block && c === '/' && text[i + 1] === '*') {
      let depth = 1;
      let j = i + 2;
      while (j < n && depth > 0) {
        if (text[j] === '*' && text[j + 1] === '/') {
          depth--;
          j += 2;
        } else if (syntax.nested && text[j] === '/' && text[j + 1] === '*') {
          depth++;
          j += 2;
        } else {
          j++;
        }
      }
      kind.fill(KIND_COMMENT, i, j);
      i = j;
      continue;
    }
    if (syntax.backtick && c === '`') {
      kind[i] = KIND_STRING;
      stack.push({
        t: 's', q: '`', multi: true,
        raw: syntax.backtick === 'raw',
        interp: syntax.backtick === 'template',
      });
      i++;
      continue;
    }
    if (syntax.quotes.includes(c)) {
      const triple = syntax.triple && text.startsWith(c + c + c, i);
      const q = triple ? c + c + c : c;
      const raw = syntax.rawPrefix && isRawPrefix(text, i);
      const interp =
        !raw &&
        syntax.interp !== null &&
        (syntax.interpIn === 'all' || (syntax.interpIn === 'dquote' && c === '"'));
      kind.fill(KIND_STRING, i, i + q.length);
      stack.push({ t: 's', q, raw, interp, multi: triple });
      i += q.length;
      continue;
    }
    i++;
  }
  return kind;
}

// ---------------------------------------------------------------------------
// Rules.

function toLanguageSet(languages) {
  if (!Array.isArray(languages) || languages.length === 0) return null;
  return new Set(languages.filter((l) => typeof l === 'string').map((l) => l.toLowerCase()));
}

function compileRuleList(rules, errors, forcedLevel) {
  const out = [];
  if (!Array.isArray(rules)) return out;
  for (const rule of rules) {
    if (!rule || typeof rule.pattern !== 'string' || rule.pattern === '') {
      errors.push('A rule needs a "pattern" string.');
      continue;
    }
    let re;
    try {
      re = new RegExp(rule.pattern, 'g');
    } catch (e) {
      errors.push(`Rule /${rule.pattern}/: ${e.message}`);
      continue;
    }
    out.push({
      re,
      level: forcedLevel || (rule.level ? normalizeLevel(rule.level) : null),
      languages: toLanguageSet(rule.languages),
    });
  }
  return out;
}

/**
 * compile prepares the settings for scan(): the regular expressions are built
 * once per settings change, not once per scan. Rules that are not valid are
 * left out and reported in `errors`.
 *
 *   config = { rules, includeDeclarations, declarationRules,
 *              includeImports, importPattern }
 *
 * Anything missing is taken from the defaults above.
 */
function compile(config = {}) {
  const errors = [];
  const rules = compileRuleList(config.rules || defaultRules, errors, null);
  const declarations =
    config.includeDeclarations === false
      ? []
      : compileRuleList(config.declarationRules || defaultDeclarationRules, errors, 'declaration');
  let importRe = null;
  if (config.includeImports !== false) {
    const source =
      typeof config.importPattern === 'string' && config.importPattern !== ''
        ? config.importPattern
        : defaultImportPattern;
    try {
      importRe = new RegExp(source, 'gm');
    } catch (e) {
      errors.push(`importPattern: ${e.message}`);
      importRe = new RegExp(defaultImportPattern, 'gm');
    }
  }
  return { rules, declarations, importRe, errors };
}

// ---------------------------------------------------------------------------
// Scanning.

function lineStartsOf(text) {
  const starts = [0];
  let i = text.indexOf('\n');
  while (i >= 0) {
    starts.push(i + 1);
    i = text.indexOf('\n', i + 1);
  }
  return starts;
}

// The 0-based line of an offset, by binary search.
function lineAt(starts, offset) {
  let lo = 0;
  let hi = starts.length - 1;
  while (lo < hi) {
    const mid = (lo + hi + 1) >> 1;
    if (starts[mid] <= offset) lo = mid;
    else hi = mid - 1;
  }
  return lo;
}

// The index of the ")" that closes the "(" just before `from`, plus one; -1
// when the file ends first (the call is being typed).
function findClose(text, kind, from) {
  let depth = 1;
  for (let j = from; j < text.length; j++) {
    if (kind[j] !== KIND_CODE) continue;
    const c = text.charCodeAt(j);
    if (c === 40) depth++;
    else if (c === 41 && --depth === 0) return j + 1;
  }
  return -1;
}

// `this.` / `widget.` / `a.b.` in front of a call belong to it:
// `this._log.info(..)` is one statement, so it can be alone on its line.
const RECEIVER = /(?:[A-Za-z_$][\w$]*[?!]?\.[ \t]*)+$/;

function extendStart(text, kind, start) {
  const from = Math.max(0, start - 160);
  const m = RECEIVER.exec(text.slice(from, start));
  if (!m) return start;
  let s = from + m.index;
  if (s > 0 && WORD.test(text[s - 1])) return start; // the slice cut a name in two
  if (kind[s] !== KIND_CODE) return start;
  return s;
}

/**
 * scan returns the logs in `text`, sorted by position:
 *
 *   { start, end, level, kind, wholeLines, startLine, endLine }
 *
 * `start`/`end` are offsets (end is exclusive); `kind` is "call",
 * "declaration" or "import". `wholeLines` is true when the statement is alone
 * on its lines: only blanks before it on its first line and after it on its
 * last (a trailing // comment is part of it, and `end` includes it).
 *
 * `compiled` is the result of compile(); `languageId` picks the rules and the
 * lexer's syntax.
 */
function scan(text, languageId, compiled = compile()) {
  if (!text) return [];
  const language = (languageId || '').toLowerCase();
  const syntax = syntaxOf(language);
  const kind = mask(text, syntax);
  const starts = lineStartsOf(text);
  const found = [];

  // Whether only blanks (or comments) surround [start, end) on its lines;
  // widens `end` over a trailing comment.
  function surround(start, end) {
    let ls = text.lastIndexOf('\n', start - 1) + 1;
    for (let p = ls; p < start; p++) {
      const ch = text[p];
      if (ch !== ' ' && ch !== '\t' && ch !== '﻿') return { whole: false, end };
    }
    let le = text.indexOf('\n', end);
    if (le < 0) le = text.length;
    let last = end;
    for (let p = end; p < le; p++) {
      const ch = text[p];
      if (ch === ' ' || ch === '\t' || ch === '\r') continue;
      if (kind[p] !== KIND_COMMENT) return { whole: false, end };
      last = p + 1;
    }
    return { whole: true, end: last };
  }

  function add(start, end, level, what, extend) {
    if (extend) start = extendStart(text, kind, start);
    if (extend && text[end] !== undefined) {
      // A trailing ";" belongs to the statement.
      let j = end;
      while (text[j] === ' ' || text[j] === '\t') j++;
      if (text[j] === ';' && kind[j] === KIND_CODE) end = j + 1;
    }
    const s = surround(start, end);
    found.push({
      start,
      end: s.end,
      level,
      kind: what,
      wholeLines: s.whole,
      startLine: lineAt(starts, start),
      endLine: lineAt(starts, Math.max(start, s.end - 1)),
    });
  }

  function run(list, what) {
    for (const rule of list) {
      if (rule.languages && !rule.languages.has(language)) continue;
      const re = rule.re;
      re.lastIndex = 0;
      let m;
      while ((m = re.exec(text)) !== null) {
        if (m[0].length === 0) {
          re.lastIndex++;
          continue;
        }
        const start = m.index;
        if (kind[start] !== KIND_CODE) continue;
        const matchEnd = start + m[0].length;
        let end = matchEnd;
        let open = -1;
        if (text[matchEnd - 1] === '(') {
          open = matchEnd;
        } else {
          let j = matchEnd;
          while (text[j] === ' ' || text[j] === '\t') j++;
          if (text[j] === '(' && kind[j] === KIND_CODE) open = j + 1;
        }
        if (open >= 0) {
          end = findClose(text, kind, open);
          if (end < 0) continue;
        }
        const level =
          what === 'declaration'
            ? 'declaration'
            : normalizeLevel(m.groups && m.groups.level, rule.level || 'debug');
        add(start, end, level, what, true);
        if (end > re.lastIndex) re.lastIndex = end; // do not look for logs inside this one
      }
    }
  }

  run(compiled.rules, 'call');
  run(compiled.declarations, 'declaration');

  if (compiled.importRe) {
    const re = compiled.importRe;
    re.lastIndex = 0;
    let m;
    while ((m = re.exec(text)) !== null) {
      if (m[0].length === 0) {
        re.lastIndex++;
        continue;
      }
      let start = m.index;
      while (start < m.index + m[0].length && (text[start] === ' ' || text[start] === '\t')) start++;
      if (kind[start] !== KIND_CODE) continue;
      let end = m.index + m[0].length;
      while (end > start && /\s/.test(text[end - 1])) end--;
      if (end <= start) continue;
      add(start, end, 'declaration', 'import', false);
    }
  }

  // Sorted by position; a log inside another one (the argument of a log call
  // that logs) is dropped, and so is one that overlaps the previous.
  found.sort((a, b) => a.start - b.start || b.end - a.end);
  const out = [];
  let lastEnd = -1;
  for (const r of found) {
    if (r.start < lastEnd) continue;
    out.push(r);
    lastEnd = r.end;
  }
  return out;
}

/**
 * preview returns the first string literal of a call's arguments, cut to
 * `max` characters, for the "label" marker: `_log.info('Fetching profile')`
 * gives "Fetching profile". Declarations and imports give "".
 */
function preview(text, result, max = 40) {
  if (result.kind !== 'call') return '';
  const body = text.slice(result.start, result.end);
  const open = body.indexOf('(');
  if (open < 0) return '';
  const m = /(['"`])((?:\\.|(?!\1)[^\\\n])*)\1/.exec(body.slice(open));
  if (!m) return '';
  const s = m[2].replace(/\s+/g, ' ').trim();
  return s.length > max ? s.slice(0, Math.max(1, max - 1)) + '…' : s;
}

module.exports = {
  LEVELS,
  KIND_CODE,
  KIND_STRING,
  KIND_COMMENT,
  defaultRules,
  defaultDeclarationRules,
  defaultImportPattern,
  normalizeLevel,
  mask,
  syntaxOf,
  compile,
  scan,
  preview,
};
