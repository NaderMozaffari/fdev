import 'dart:convert';

import 'package:fdev_log/fdev_log.dart';
import 'package:test/test.dart';

final _marker = RegExp(r'^⟪fd(?: (\d+) (\d+)/(\d+))?⟫(.*)$');

Map<String, Object?> decode(List<String> lines) => jsonDecode(
    lines.map((l) => _marker.firstMatch(l)!.group(4)!).join()) as Map<String, Object?>;

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
    FdevLog.info({'user': _User('Ali'), 'roles': {'admin'}});
    expect(out.single, contains('"name": "Ali"'));
    expect(out.single, contains('"admin"'));
  });

  test('a value is printed for the values window', () {
    final out = <String>[];
    final before = FdevLog.output;
    FdevLog.output = out.add;
    addTearDown(() => FdevLog.output = before);
    FdevLog.value('accessToken', 'abc');
    expect(out.single, FdevLog.structured ? contains('"k":"accessToken"') : 'DEBUG accessToken = abc');
  });
}

class _User {
  _User(this.name);
  final String name;
  Map<String, Object?> toJson() => {'name': name};
}
