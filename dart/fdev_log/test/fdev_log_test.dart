import 'dart:convert';
import 'dart:io';

import 'package:fdev_log/fdev_log.dart';
import 'package:test/test.dart';

final _marker = RegExp(r'^⟪fd(?: (\d+) (\d+)/(\d+))?⟫(.*)$');

Map<String, Object?> decode(List<String> lines) =>
    jsonDecode(lines.map((l) => _marker.firstMatch(l)!.group(4)!).join())
        as Map<String, Object?>;

void main() {
  test('a short record is one line', () {
    final lines = FdevLog.encode({'l': 'info', 't': 'Billing', 'm': 'سلام'});
    expect(lines, hasLength(1));
    expect(decode(lines)['m'], 'سلام');
  });

  test('a long record is split into chunks that rejoin', () {
    final message = 'پرداخت 😀 ${'x' * 1000}\nline two';
    final lines = FdevLog.encode({'l': 'error', 'm': message});
    expect(lines.length, greaterThan(1));
    for (final (i, line) in lines.indexed) {
      final m = _marker.firstMatch(line)!;
      expect(m.group(2), '${i + 1}');
      expect(m.group(3), '${lines.length}');
      expect(m.group(4)!.length, lessThanOrEqualTo(FdevLog.chunkSize));
      final last = m.group(4)!.codeUnitAt(m.group(4)!.length - 1);
      expect(last >= 0xD800 && last <= 0xDBFF, isFalse);
    }
    expect(decode(lines)['m'], message);
  });

  test('location skips helpers, the SDK and other packages', () {
    FdevLog.appPackage = 'my_app';
    final trace = StackTrace.fromString('''
#0      Log.info (package:my_app/core/logger/logger.dart:40:5)
#1      Dio.fetch (package:dio/src/dio.dart:10:1)
#2      Wallet.load (package:my_app/features/wallet/wallet.dart:88:9)
''');
    expect(FdevLog.location(trace, skip: ['core/logger/']),
        'lib/features/wallet/wallet.dart:88:9');
    final web = StackTrace.fromString(
        'packages/my_app/features/wallet/wallet.dart 88:9  load');
    expect(FdevLog.location(web), 'lib/features/wallet/wallet.dart:88:9');
  });

  test('member names the API call from the caller stack', () {
    FdevLog.appPackage = 'my_app';
    final trace = StackTrace.fromString('''
#0      Client.request (package:my_app/core/network/client.dart:12:3)
#1      Api.getProfile.<anonymous closure> (package:my_app/core/api.dart:40:5)
#2      Wallet.load (package:my_app/features/wallet/wallet.dart:88:9)
''');
    expect(FdevLog.member(trace, skip: ['core/network/']), 'getProfile');
    final web =
        StackTrace.fromString('packages/my_app/core/api.dart 40:5  getProfile');
    expect(FdevLog.member(web), 'getProfile');
  });

  test('redacts secrets in headers, bodies and URLs', () {
    final body = FdevLog.redacted({
      'user': {'Password': 'hunter2', 'name': 'a'},
      'items': [
        {'token': 'abc'}
      ],
      'json': '{"refresh_token": "xyz", "ok": 1}',
    }) as Map;
    expect(body['user'], {'Password': '••• (7 chars)', 'name': 'a'});
    expect(body['items'], [
      {'token': '••• (3 chars)'}
    ]);
    expect(body['json'], {'refresh_token': '••• (3 chars)', 'ok': 1});

    final lines = <String>[];
    FdevLog.output = lines.add;
    FdevLog.http(const FdevHttp(
        phase: FdevHttpPhase.request,
        method: 'GET',
        url: 'https://api.test/v1?token=abc&page=2',
        headers: {'Authorization': 'Bearer abc'}));
    expect(lines.join(), isNot(contains('abc')));
    FdevLog.output = print;
  });

  test('objects with toJson and sets are logged as JSON', () {
    final out = <String>[];
    final before = FdevLog.output;
    FdevLog.output = out.add;
    addTearDown(() => FdevLog.output = before);
    FdevLog.info({
      'user': _User('Ali'),
      'roles': {'admin'}
    });
    expect(out.single, contains('"name": "Ali"'));
    expect(out.single, contains('"admin"'));
  });

  test('a value is printed for the values window', () {
    final out = <String>[];
    final before = FdevLog.output;
    FdevLog.output = out.add;
    addTearDown(() => FdevLog.output = before);
    FdevLog.value('accessToken', 'abc');
    expect(
        out.single,
        FdevLog.structured
            ? contains('"k":"accessToken"')
            : 'DEBUG accessToken = abc');
  });

  group('Log', () {
    late List<String> out;
    setUp(() {
      out = [];
      FdevLog.output = out.add;
    });
    tearDown(() {
      FdevLog.output = print;
      FdevLog.levels = {...FdevLevel.values};
    });

    // Plain text only: structured output is covered by the subprocess test.
    test('forwards the level and the tag', () {
      if (FdevLog.structured) return;
      const log = Log('Billing');
      log.debug('d');
      log.info('i');
      log.success('s');
      log.warning('w');
      log.error('e');
      log.fatal('f');
      expect(out, [
        'DEBUG [Billing] d',
        'INFO [Billing] i',
        'OK [Billing] s',
        'WARN [Billing] w',
        'ERROR [Billing] e',
        'FATAL [Billing] f',
      ]);
    });

    test('an error is appended to the message, its stack is kept', () {
      if (FdevLog.structured) return;
      const log = Log('Auth');
      log.warning('retrying', 'timeout');
      log.error('failed', 'bad code', StackTrace.fromString('#0 here'));
      final thrown = StateError('no session');
      log.fatal('broken', thrown);
      log.error(null, 'only the error');
      expect(out[0], 'WARN [Auth] retrying: timeout');
      expect(out[1], 'ERROR [Auth] failed: bad code');
      expect(out[2], '#0 here');
      expect(out[3], 'FATAL [Auth] broken: Bad state: no session');
      // An Error that was never thrown has no stack: nothing extra is printed.
      expect(out[4], 'ERROR [Auth] only the error');
      expect(out, hasLength(5));
    });

    test('the stack of a thrown Error is used when none is given', () {
      if (FdevLog.structured) return;
      Object? caught;
      try {
        throw StateError('x');
      } catch (e) {
        caught = e;
      }
      const Log('Auth').error('failed', caught);
      expect(out, hasLength(2));
      expect(out[1], contains('fdev_log_test.dart'));
    });

    test('a value and Maps go through the tag', () {
      if (FdevLog.structured) return;
      const log = Log('Auth');
      log.value('token', 'abc');
      log.info({'id': 7});
      expect(out.first, 'DEBUG [Auth] token = abc');
      expect(out[1], contains('"id": 7'));
    });

    test('levels drops what is not in the set', () {
      const log = Log('Billing');
      FdevLog.levels = {FdevLevel.error, FdevLevel.fatal};
      log.debug('d');
      log.info('i');
      log.success('s');
      log.warning('w');
      log.value('k', 1);
      expect(out, isEmpty);
      log.error('e');
      log.fatal('f');
      FdevLog.error('static');
      expect(out, hasLength(3));
      FdevLog.levels = {FdevLevel.info};
      out.clear();
      log.error('e');
      FdevLog.http(const FdevHttp(
          phase: FdevHttpPhase.error, method: 'GET', url: 'https://x.test'));
      expect(out, isEmpty);
    });

    test('the static API still works', () {
      if (FdevLog.structured) return;
      FdevLog.info('hello', tag: 'Old');
      FdevLog.error('bad', tag: 'Old', stack: StackTrace.fromString('#0 x'));
      expect(out, ['INFO [Old] hello', 'ERROR [Old] bad', '#0 x']);
    });

    test('the call site is the caller of Log, not fdev_log', () async {
      final result = await Process.run(Platform.resolvedExecutable, [
        'run',
        '--enable-asserts',
        '-DFDEV_LOGS=true',
        'test/fixtures/call_site.dart',
      ]);
      expect(result.exitCode, 0, reason: '${result.stderr}');
      final records = [
        for (final line in LineSplitter.split('${result.stdout}'))
          if (line.startsWith('⟪fd⟫')) decode([line]),
      ];
      expect(records, hasLength(2));
      for (final (i, r) in records.indexed) {
        expect(r['t'], 'Fixture');
        final at = r['at'] as String;
        expect(at, contains('test/fixtures/call_site.dart:${8 + i}:'),
            reason: 'at is $at');
        expect(at, isNot(contains('fdev_log/lib')));
      }
      expect(records[1]['m'], 'second: Bad state: boom');
    });
  });
}

class _User {
  _User(this.name);
  final String name;
  Map<String, Object?> toJson() => {'name': name};
}
