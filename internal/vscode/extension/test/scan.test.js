'use strict';

const test = require('node:test');
const assert = require('node:assert');
const fs = require('node:fs');
const path = require('node:path');
const scan = require('../src/scan');

const compiled = scan.compile();

function find(text, lang = 'dart', config) {
  return scan.scan(text, lang, config ? scan.compile(config) : compiled);
}

// The text of each result, to compare against what the source says.
function texts(text, lang, config) {
  return find(text, lang, config).map((r) => text.slice(r.start, r.end));
}

test('normalizeLevel knows the aliases', () => {
  const cases = {
    warn: 'warning', WARN: 'warning', wtf: 'fatal', log: 'debug', print: 'debug',
    debugPrint: 'debug', request: 'http', response: 'http', i: 'info', d: 'debug',
    w: 'warning', e: 'error', v: 'debug', critical: 'fatal', Println: 'debug',
    Fatalf: 'fatal', info: 'info', success: 'success', value: 'value',
  };
  for (const [name, level] of Object.entries(cases)) {
    assert.strictEqual(scan.normalizeLevel(name), level, name);
  }
  assert.strictEqual(scan.normalizeLevel('nonsense'), 'debug');
  assert.strictEqual(scan.normalizeLevel('nonsense', 'info'), 'info');
  assert.strictEqual(scan.normalizeLevel(undefined, 'error'), 'error');
});

test('a single-line Dart call is found with its level and ;', () => {
  const src = "void f() {\n  _log.info('hello');\n}\n";
  const [r] = find(src);
  assert.strictEqual(src.slice(r.start, r.end), "_log.info('hello');");
  assert.strictEqual(r.level, 'info');
  assert.strictEqual(r.wholeLines, true);
  assert.strictEqual(r.startLine, 1);
  assert.strictEqual(r.endLine, 1);
  assert.strictEqual(r.kind, 'call');
});

test('every Dart level and alias of the default rule', () => {
  const want = {
    debug: 'debug', info: 'info', success: 'success', warning: 'warning', warn: 'warning',
    error: 'error', fatal: 'fatal', wtf: 'fatal', value: 'value', http: 'http',
    request: 'http', response: 'http',
  };
  for (const [name, level] of Object.entries(want)) {
    const [r] = find(`Log.${name}('x');\n`);
    assert.ok(r, name);
    assert.strictEqual(r.level, level, name);
  }
  assert.strictEqual(find('FdevLog.value("n", 3);')[0].level, 'value');
  assert.strictEqual(find('logger.d("x");')[0].level, 'debug');
  assert.strictEqual(find('_logger.severe("x");')[0].level, 'error');
});

test('names that merely end in log are not logs', () => {
  assert.deepStrictEqual(find("dialog.error('x');\nblog.info('x');\ncatalog.debug('x');"), []);
});

test('print, debugPrint and debugPrintStack are debug logs; .print is not', () => {
  const src = "print('a');\ndebugPrint('b');\ndebugPrintStack(stackTrace: s);\nfoo.print('c');\nsprint('d');\n";
  const rs = find(src);
  assert.strictEqual(rs.length, 3);
  assert.ok(rs.every((r) => r.level === 'debug'));
});

test('multi-line call with nested parentheses', () => {
  const src = [
    'void f() {',
    '  _log.error(',
    "    'failed: ${compute(a, (b) => b + 1)}',",
    '    error,',
    '    stack,',
    '  );',
    '  next();',
    '}',
  ].join('\n');
  const [r] = find(src);
  assert.ok(src.slice(r.start, r.end).endsWith(');'));
  assert.strictEqual(r.startLine, 1);
  assert.strictEqual(r.endLine, 5);
  assert.strictEqual(r.wholeLines, true);
  assert.ok(!src.slice(r.start, r.end).includes('next'));
});

test('strings holding ) and ; and fake log calls do not end or start a call', () => {
  const src = [
    "_log.info('a ) b ; c');",
    'x = "_log.info(not a log)";',
    "_log.warning(\"paren ( open\");",
    "y = 'print(nope)';",
  ].join('\n');
  const rs = find(src);
  assert.strictEqual(rs.length, 2);
  assert.strictEqual(src.slice(rs[0].start, rs[0].end), "_log.info('a ) b ; c');");
  assert.strictEqual(src.slice(rs[1].start, rs[1].end), '_log.warning("paren ( open");');
});

test('triple-quoted and raw strings', () => {
  const src = [
    "_log.info('''line ) one",
    "still ( inside ; ''');",
    "_log.info(r'raw \\ ) ');",
    '_log.info("""a ) b""");',
    "after();",
  ].join('\n');
  const rs = find(src);
  assert.strictEqual(rs.length, 3);
  assert.strictEqual(rs[0].startLine, 0);
  assert.strictEqual(rs[0].endLine, 1);
  assert.strictEqual(src.slice(rs[1].start, rs[1].end), "_log.info(r'raw \\ ) ');");
  assert.strictEqual(src.slice(rs[2].start, rs[2].end), '_log.info("""a ) b""");');
});

test('comments hide logs and parentheses', () => {
  const src = [
    "// _log.info('no');",
    "/* _log.info('no'); */",
    "/// _log.info('no');",
    "_log.info('yes' /* ) */ ); // note",
    '/* a /* nested */ _log.error("no") */',
    "_log.debug('x', // )",
    "  1);",
  ].join('\n');
  const rs = find(src);
  assert.strictEqual(rs.length, 2);
  assert.strictEqual(rs[0].level, 'info');
  assert.strictEqual(src.slice(rs[0].start, rs[0].end), "_log.info('yes' /* ) */ ); // note");
  assert.strictEqual(rs[1].level, 'debug');
  assert.strictEqual(rs[1].endLine, 6);
});

test('Dart interpolation nests strings and braces', () => {
  const src = "_log.info('a ${b('c)${d(\")\")}')} e'); next('(');\n";
  const [r] = find(src);
  assert.strictEqual(src.slice(r.start, r.end), "_log.info('a ${b('c)${d(\")\")}')} e');");
  assert.strictEqual(find(src).length, 1);
});

test('JS template literals and console calls', () => {
  const src = [
    'console.log(`a ${f(`b ${c}`)} )`);',
    "console.warn('w');",
    'console.error("e"); // trailing',
    "const s = 'console.log(x)';",
    '// console.info(1)',
    'console.table(x)',
  ].join('\n');
  const rs = find(src, 'javascript');
  assert.deepStrictEqual(rs.map((r) => r.level), ['debug', 'warning', 'error']);
  assert.strictEqual(src.slice(rs[0].start, rs[0].end), 'console.log(`a ${f(`b ${c}`)} )`);');
  assert.ok(src.slice(rs[2].start, rs[2].end).endsWith('// trailing'));
  assert.strictEqual(rs[2].wholeLines, true);
});

test('Python logging, print and # comments', () => {
  const src = [
    'logger.info("a ) b")',
    '# logger.error("no")',
    'logging.warning(',
    '    "multi",',
    ')',
    'x = a // b',
    'print("p")',
    'logger.exception("boom")',
    'logger.critical("c")',
    "s = '''logger.info(\"inside\")'''",
  ].join('\n');
  const rs = find(src, 'python');
  assert.deepStrictEqual(rs.map((r) => r.level), ['info', 'warning', 'debug', 'error', 'fatal']);
  assert.strictEqual(rs[1].startLine, 2);
  assert.strictEqual(rs[1].endLine, 4);
  assert.strictEqual(rs[0].wholeLines, true);
});

test('Kotlin, Swift and Go', () => {
  assert.deepStrictEqual(
    find('Log.d(TAG, "x")\nLog.e(TAG, "y")\nTimber.w("z")\nprintln("p")', 'kotlin').map((r) => r.level),
    ['debug', 'error', 'warning', 'debug'],
  );
  assert.deepStrictEqual(
    find('print("a")\nlogger.error("b")\nNSLog("c")\nlog.fault("d")', 'swift').map((r) => r.level),
    ['debug', 'error', 'debug', 'fatal'],
  );
  assert.deepStrictEqual(
    find('log.Printf("a %d", 1)\nslog.Error("b")\nlog.Fatalf("c")', 'go').map((r) => r.level),
    ['debug', 'error', 'fatal'],
  );
  // A Go rune and a raw string do not confuse the lexer.
  assert.strictEqual(find("c := '\"'\nlog.Println(`a ) b`)\n", 'go').length, 1);
  // Swift interpolation.
  const sw = 'print("a \\(f(")")) b")\nlogger.info("c")\n';
  assert.deepStrictEqual(find(sw, 'swift').map((r) => r.level), ['debug', 'info']);
});

test('rules only run for their languages', () => {
  assert.strictEqual(find('console.log(1)', 'dart').length, 0);
  assert.strictEqual(find('print(1)', 'javascript').length, 0);
});

test('a log inside an expression is found but is not whole-line', () => {
  const src = "foo(onError: (e) => _log.error('x', e), other: 1);\n";
  const [r] = find(src);
  assert.strictEqual(src.slice(r.start, r.end), "_log.error('x', e)");
  assert.strictEqual(r.wholeLines, false);
  const src2 = "if (debug) _log.info('x');\n";
  assert.strictEqual(find(src2)[0].wholeLines, false);
  const src3 = "_log.info('a'); other();\n";
  assert.strictEqual(find(src3)[0].wholeLines, false);
});

test('a receiver chain belongs to the call', () => {
  const src = "  this._log.info('x');\n  widget.log.error('y');\n";
  const rs = find(src);
  assert.strictEqual(src.slice(rs[0].start, rs[0].end), "this._log.info('x');");
  assert.strictEqual(rs[0].wholeLines, true);
  assert.strictEqual(src.slice(rs[1].start, rs[1].end), "widget.log.error('y');");
  assert.strictEqual(rs[1].wholeLines, true);
});

test('a log nested in a log is one result', () => {
  const src = "_log.info(foo(_log.value('a', 1)));\n";
  assert.strictEqual(find(src).length, 1);
});

test('an unfinished call is skipped while typing', () => {
  assert.deepStrictEqual(find("void f() {\n  _log.info('x'\n}\n"), []);
});

test('declarations and imports', () => {
  const src = [
    "import 'package:flutter/material.dart';",
    "import 'package:fdev_log/fdev_log.dart' show Log;",
    "import '../core/logger/logger.dart';",
    'class A {',
    "  static const _log = Log('Billing');",
    "  final _log2 = FdevLog.tagged('x');",
    "  final Logger _l = Logger('x');",
    '  var n = 1;',
    '}',
    "const _log = Log('Top');",
  ].join('\n');
  const rs = find(src);
  assert.deepStrictEqual(rs.map((r) => r.level), Array(6).fill('declaration'));
  assert.deepStrictEqual(rs.map((r) => r.kind), ['import', 'import', 'declaration', 'declaration', 'declaration', 'declaration']);
  assert.strictEqual(src.slice(rs[0].start, rs[0].end), "import 'package:fdev_log/fdev_log.dart' show Log;");
  assert.strictEqual(src.slice(rs[2].start, rs[2].end), "static const _log = Log('Billing');");
  assert.ok(rs.every((r) => r.wholeLines));
});

test('declarations and imports can be switched off', () => {
  const src = "import 'package:fdev_log/fdev_log.dart';\nconst _log = Log('x');\n_log.info('y');\n";
  assert.strictEqual(find(src).length, 3);
  assert.strictEqual(find(src, 'dart', { includeDeclarations: false }).length, 2);
  assert.strictEqual(find(src, 'dart', { includeImports: false }).length, 2);
  assert.strictEqual(find(src, 'dart', { includeDeclarations: false, includeImports: false }).length, 1);
});

test('a custom import pattern', () => {
  const src = "import 'package:my/telemetry.dart';\nimport 'package:fdev_log/fdev_log.dart';\n";
  const rs = find(src, 'dart', { importPattern: String.raw`^import\s+'package:my/telemetry\.dart';` });
  assert.strictEqual(rs.length, 1);
  assert.strictEqual(rs[0].startLine, 0);
});

test('imports in comments and strings are not imports', () => {
  const src = "// import 'package:fdev_log/fdev_log.dart';\nvar s = '''\nimport 'logger.dart';\n''';\n";
  assert.deepStrictEqual(find(src), []);
});

test('Python and JS declarations and imports', () => {
  const py = 'import logging\nfrom loguru import logger\nlog = logging.getLogger(__name__)\nimport os\n';
  assert.deepStrictEqual(find(py, 'python').map((r) => r.kind), ['import', 'import', 'declaration']);
  const js = "import pino from 'pino';\nconst logger = pino({ level: 'info' });\nimport fs from 'fs';\n";
  assert.deepStrictEqual(find(js, 'typescript').map((r) => r.kind), ['import', 'declaration']);
});

test('custom rules: fixed level, named group, languages, invalid regex', () => {
  const cfg = {
    rules: [
      { pattern: String.raw`\btrace\s*\(`, level: 'warn' },
      { pattern: String.raw`\bemit\.(?<level>ok|boom)\s*\(`, level: 'info', languages: ['dart'] },
      { pattern: '(unclosed' },
      { nope: true },
    ],
  };
  const compiledCfg = scan.compile(cfg);
  assert.strictEqual(compiledCfg.errors.length, 2);
  const rs = scan.scan("trace('a');\nemit.ok('b');\nemit.boom('c');\n", 'dart', compiledCfg);
  assert.deepStrictEqual(rs.map((r) => r.level), ['warning', 'success', 'info']);
  assert.strictEqual(scan.scan("emit.ok('b');", 'python', compiledCfg).length, 0);
});

test('CRLF line endings keep offsets and lines right', () => {
  const src = "void f() {\r\n  _log.info(\r\n    'a',\r\n  ); // c\r\n  g();\r\n}\r\n";
  const [r] = find(src);
  assert.strictEqual(r.startLine, 1);
  assert.strictEqual(r.endLine, 3);
  assert.strictEqual(r.wholeLines, true);
  const body = src.slice(r.start, r.end);
  assert.ok(body.startsWith('_log.info('));
  assert.ok(body.endsWith('); // c'));
  assert.ok(!body.endsWith('\r'));
});

test('a trailing block comment after a statement counts too', () => {
  const src = "_log.info('a'); /* why */\n";
  const [r] = find(src);
  assert.strictEqual(r.wholeLines, true);
  assert.strictEqual(src.slice(r.start, r.end), "_log.info('a'); /* why */");
});

test('preview takes the first string of the arguments', () => {
  const src = "_log.info('Fetching profile for ${id}', extra: 'x');\n_log.error(e);\n_log.warning(\"a very long message that goes on and on and on forever\");";
  const rs = find(src);
  assert.strictEqual(scan.preview(src, rs[0]), 'Fetching profile for ${id}');
  assert.strictEqual(scan.preview(src, rs[1]), '');
  const long = scan.preview(src, rs[2], 20);
  assert.strictEqual(long.length, 20);
  assert.ok(long.endsWith('…'));
  const decl = find("const _log = Log('x');")[0];
  assert.strictEqual(scan.preview("const _log = Log('x');", decl), '');
});

test('an empty file and a file without logs', () => {
  assert.deepStrictEqual(find(''), []);
  assert.deepStrictEqual(find('void main() {}\n'), []);
});

test('a JavaScript regex literal with a quote cannot swallow the file', () => {
  const src = 'const re = /"/;\nconsole.log("x");\n';
  assert.strictEqual(find(src, 'javascript').length, 1);
});

test('a 2000-line file is scanned fast', () => {
  const lines = [];
  for (let i = 0; i < 400; i++) {
    lines.push(`void f${i}() {`);
    lines.push(`  _log.info('step ${i} (${'${x}'})');`);
    lines.push('  final s = """a ( b""";');
    lines.push('  // _log.error("no");');
    lines.push('}');
  }
  const src = lines.join('\n');
  assert.strictEqual(src.split('\n').length, 2000);
  const t0 = process.hrtime.bigint();
  const rs = find(src);
  const ms = Number(process.hrtime.bigint() - t0) / 1e6;
  assert.strictEqual(rs.length, 400);
  assert.ok(ms < 500, `took ${ms} ms`);
});

test('package.json defaults equal the defaults in scan.js', () => {
  const pkg = JSON.parse(fs.readFileSync(path.join(__dirname, '..', 'package.json'), 'utf8'));
  const props = pkg.contributes.configuration.properties;
  assert.deepStrictEqual(props['fdev.logs.rules'].default, scan.defaultRules);
  assert.deepStrictEqual(props['fdev.logs.declarationRules'].default, scan.defaultDeclarationRules);
  assert.strictEqual(props['fdev.logs.importPattern'].default, scan.defaultImportPattern);
  assert.deepStrictEqual(props['fdev.logs.levels'].default, scan.LEVELS);
});

test('tricky Dart: braces and quotes in interpolation, comments after a statement, lambdas, cascades', () => {
  const src = [
    'void f() {',
    "  _log.info('a ${m['x']} ${ {1: 2}[1] } \"}\" ${g(\"(\")}'); // it's ) here",
    '  list.forEach((e) => _log.info(\'x $e\'));',
    '  list.forEach((e) {',
    "    _log.warn('y'); /* z */",
    '  });',
    "  sb..write('a')..write('b');",
    "  _log.info('', error: '');",
    "  _log.error(r'\\', ref: \"$x )\");",
    '}',
    '',
  ].join('\n');
  const found = find(src);
  assert.deepStrictEqual(
    found.map((r) => [r.level, r.startLine, r.endLine, r.wholeLines]),
    [
      ['info', 1, 1, true],
      ['info', 2, 2, false],
      ['warning', 4, 4, true],
      ['info', 7, 7, true],
      ['error', 8, 8, true],
    ],
  );
  assert.strictEqual(src.slice(found[0].start, found[0].end).endsWith("// it's ) here"), true);
  assert.strictEqual(src.slice(found[2].start, found[2].end).endsWith('/* z */'), true);
  // The call in the arrow function is the statement's expression, not the
  // whole line: hiding it must leave the forEach around it.
  assert.strictEqual(src.slice(found[1].start, found[1].end), "_log.info('x $e')");
});

test('a statement with a keyword in front of it is not whole-line', () => {
  const src = "void f() async {\n  await _log.info('x');\n  if (a) _log.info('y');\n  this._log.info('z');\n}\n";
  assert.deepStrictEqual(
    find(src).map((r) => r.wholeLines),
    [false, false, true],
  );
});
