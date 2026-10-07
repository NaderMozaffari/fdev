/// Structured logs for the fdev log viewer.
///
/// With `--dart-define=FDEV_LOGS=true` (fdev passes it to `flutter run`)
/// every log is printed as a record fdev renders, filters and links to the
/// code that logged it (see docs/PROTOCOL.md). Without it the logs are plain
/// one-line text, for IDE consoles and `flutter run` on its own.
///
/// Logs are printed in debug builds only, unless [FdevLog.inRelease] is set.
///
/// ```dart
/// FdevLog.appPackage = 'my_app';
/// FdevLog.info('connected', tag: 'Billing');
/// ```
library;

import 'dart:convert';

enum FdevLevel { debug, info, success, warning, error, fatal }

enum FdevHttpPhase { request, response, error }

/// One step of an HTTP call; see [FdevLog.http].
class FdevHttp {
  const FdevHttp({
    required this.phase,
    required this.method,
    required this.url,
    this.status,
    this.duration,
    this.headers,
    this.body,
    this.error,
    this.caller,
    this.name,
    this.id,
  });

  final FdevHttpPhase phase;
  final String method;
  final String url;
  final int? status;
  final Duration? duration;
  final Map<String, Object?>? headers;

  /// A Map or List (shown as JSON) or text.
  final Object? body;

  /// What went wrong, for [FdevHttpPhase.error] without a response.
  final String? error;

  /// `StackTrace.current` taken where the request was made, so the log line
  /// links there instead of to the HTTP client's internals.
  final StackTrace? caller;

  /// A short name for the call, shown before the path: the method that made
  /// it (`getProfile`), for instance [FdevLog.member] of [caller].
  final String? name;

  /// The same on a request and its response (and error), so fdev pairs
  /// them even when calls to one URL overlap: e.g. the request options'
  /// `hashCode`. Without it, fdev pairs them by method and URL.
  final Object? id;
}

class FdevLog {
  FdevLog._();

  /// Whether records are printed: `--dart-define=FDEV_LOGS=true`.
  static const bool structured = bool.fromEnvironment('FDEV_LOGS');

  /// The app's package name; its frames are what the ↗ links to.
  static String? appPackage;

  /// Parts of paths that are logging helpers (e.g. `core/logger/`), skipped
  /// when looking for the code that logged.
  static List<String> skip = [];

  /// Also skipped for [http], e.g. `core/network/` for the API client.
  static List<String> skipHttp = [];

  /// Masks the values of [redactKeys] in headers, bodies and URL queries.
  static bool redact = true;

  /// Keys whose values are masked, compared in lower case.
  static Set<String> redactKeys = {
    'authorization', 'proxy-authorization', 'cookie', 'set-cookie',
    'x-api-key', 'api-key', 'apikey', 'api_key', 'x-auth-token',
    'password', 'passwd', 'new_password', 'old_password', 'secret',
    'client_secret', 'token', 'access_token', 'refresh_token', 'id_token',
    'otp', 'pin', 'cvv', 'card_number', 'private_key',
  };

  /// Log in release builds too.
  static bool inRelease = false;

  /// Where lines go; `print` by default.
  static void Function(String line) output = print;

  /// Chunk length in UTF-16 units: below the line limits of logcat and iOS
  /// even for 3-byte characters.
  static const int chunkSize = 300;

  /// Longer bodies are cut, to keep the device log from flooding.
  static int maxBodyLength = 128 * 1024;

  static void debug(Object? message, {String? tag}) =>
      log(FdevLevel.debug, message, tag: tag);
  static void info(Object? message, {String? tag}) =>
      log(FdevLevel.info, message, tag: tag);
  static void success(Object? message, {String? tag}) =>
      log(FdevLevel.success, message, tag: tag);
  static void warning(Object? message, {String? tag}) =>
      log(FdevLevel.warning, message, tag: tag);
  static void error(Object? message, {String? tag, StackTrace? stack}) =>
      log(FdevLevel.error, message, tag: tag, stack: stack);
  static void fatal(Object? message, {String? tag, StackTrace? stack}) =>
      log(FdevLevel.fatal, message, tag: tag, stack: stack);

  /// Keeps [value] at hand in fdev, under [name]: its values window (`k`)
  /// shows the newest of each name, to see and copy while the app runs, for
  /// a token, a user id, a push token. Unlike headers and bodies it is not
  /// masked: pass only what you want to see, in debug builds.
  ///
  /// ```dart
  /// FdevLog.value('accessToken', auth.accessToken);
  /// ```
  static void value(String name, Object? value, {String? tag}) {
    if (!_enabled) return;
    if (structured) {
      _emit({'l': 'value', 'k': name, 'v': value, if (tag != null) 't': tag});
      return;
    }
    output(tag == null ? 'DEBUG $name = $value' : 'DEBUG [$tag] $name = $value');
  }

  static bool get _enabled => inRelease || _debug;
  static final bool _debug = () {
    var debug = false;
    assert(debug = true);
    return debug;
  }();

  static const _plainLabels = {
    FdevLevel.debug: 'DEBUG',
    FdevLevel.info: 'INFO',
    FdevLevel.success: 'OK',
    FdevLevel.warning: 'WARN',
    FdevLevel.error: 'ERROR',
    FdevLevel.fatal: 'FATAL',
  };

  static void log(FdevLevel level, Object? message,
      {String? tag, StackTrace? stack}) {
    if (!_enabled) return;
    final text = _text(message);
    if (structured) {
      final at = location(StackTrace.current, skip: skip);
      _emit({
        'l': level.name,
        if (tag != null) 't': tag,
        'm': text,
        if (stack != null) 's': '$stack',
        if (at != null) 'at': at,
      });
      return;
    }
    final label = _plainLabels[level]!;
    output(tag == null ? '$label $text' : '$label [$tag] $text');
    if (stack != null) output('$stack');
  }

  static void http(FdevHttp log) {
    if (!_enabled) return;
    final url = redact ? _redactUrl(log.url) : log.url;
    final headers = log.headers == null ? null : redacted(log.headers);
    final body = log.body == null ? null : _body(redacted(log.body));
    if (structured) {
      final at = log.caller == null
          ? null
          : location(log.caller!, skip: [...skip, ...skipHttp]);
      _emit({
        'l': 'http',
        'p': log.phase.name,
        'method': log.method,
        'url': url,
        if (log.status != null) 'status': log.status,
        if (log.duration != null) 'ms': log.duration!.inMilliseconds,
        if (headers != null) 'headers': headers,
        if (body != null) 'body': body,
        if (log.error != null) 'error': log.error,
        if (at != null) 'at': at,
        if (log.name != null) 'name': log.name,
        if (log.id != null) 'id': '${log.id}',
      });
      return;
    }
    final took =
        log.duration == null ? '' : ' ${log.duration!.inMilliseconds}ms';
    output(switch (log.phase) {
      FdevHttpPhase.request => '→ ${log.method} $url',
      FdevHttpPhase.response => '← ${log.status} ${log.method} $url$took',
      FdevHttpPhase.error =>
        '✘ ${log.status ?? log.error} ${log.method} $url$took',
    });
    if (body != null) output(_text(body));
  }

  /// The lines of [record]: one, or numbered chunks when it is long.
  static List<String> encode(Map<String, Object?> record) {
    final json = jsonEncode(record, toEncodable: _encodable);
    if (json.length <= chunkSize) return ['⟪fd⟫$json'];
    final chunks = <String>[];
    var start = 0;
    while (start < json.length) {
      var end = start + chunkSize;
      if (end >= json.length) {
        end = json.length;
      } else if (_isHighSurrogate(json.codeUnitAt(end - 1))) {
        end--; // never split a surrogate pair
      }
      chunks.add(json.substring(start, end));
      start = end;
    }
    final id = ++_sequence;
    return [
      for (var i = 0; i < chunks.length; i++)
        '⟪fd $id ${i + 1}/${chunks.length}⟫${chunks[i]}',
    ];
  }

  static int _sequence = 0;

  static final _vmFrame = RegExp(
      r'\((package:([\w.]+)/|file://)([^)]+?\.dart):(\d+)(?::(\d+))?\)');
  static final _webFrame =
      RegExp(r'packages/([\w.]+)/(\S+?\.dart) (\d+)(?::(\d+))?');

  /// `path:line:col` of the first app frame of [trace] outside [skip]:
  /// `lib/...` for [appPackage], an absolute path for files outside packages
  /// (tests, scripts). SDK and other packages are skipped.
  static String? location(StackTrace trace, {List<String> skip = const []}) {
    for (final frame in '$trace'.split('\n')) {
      final String path;
      final String? line, column;
      final vm = _vmFrame.firstMatch(frame);
      final web = vm == null ? _webFrame.firstMatch(frame) : null;
      if (vm != null) {
        final file = vm.group(3)!;
        if (vm.group(1) == 'file://') {
          if (file.contains('/flutter/packages/') ||
              file.contains('/.pub-cache/') ||
              file.contains('/fdev_log/lib/')) {
            continue;
          }
          path = Uri.decodeFull(file);
        } else {
          if (!_isApp(vm.group(2)!)) continue;
          path = 'lib/$file';
        }
        (line, column) = (vm.group(4), vm.group(5));
      } else if (web != null) {
        if (!_isApp(web.group(1)!)) continue;
        path = 'lib/${web.group(2)}';
        (line, column) = (web.group(3), web.group(4));
      } else {
        continue;
      }
      if (skip.any(path.contains)) continue;
      return column == null ? '$path:$line' : '$path:$line:$column';
    }
    return null;
  }

  static final _vmMember = RegExp(r'^#\d+\s+(.+?) \(');
  static final _webMember = RegExp(r'\.dart \d+(?::\d+)?\s+(\S+)$');

  /// The method of the first app frame of [trace] outside [skip], without
  /// its class or closures: `getProfile` for `Api.getProfile.<anonymous
  /// closure>`. With the HTTP client's files in [skip], the API call's name.
  static String? member(StackTrace trace, {List<String> skip = const []}) {
    for (final frame in '$trace'.split('\n')) {
      if (location(StackTrace.fromString(frame), skip: skip) == null) continue;
      final name = (_vmMember.firstMatch(frame) ?? _webMember.firstMatch(frame))
          ?.group(1);
      if (name == null) return null;
      final parts =
          name.split('.').where((p) => p.isNotEmpty && !p.startsWith('<'));
      return parts.isEmpty ? null : parts.last;
    }
    return null;
  }

  static bool _isApp(String package) =>
      appPackage == null ? package != 'fdev_log' : package == appPackage;

  /// [value] with the values of [redactKeys] masked, in maps at any depth.
  /// A string that holds a JSON object or array is decoded first.
  static Object? redacted(Object? value) {
    if (!redact) return value;
    if (value is Map) {
      return {
        for (final e in value.entries)
          '${e.key}': redactKeys.contains('${e.key}'.toLowerCase())
              ? _mask(e.value)
              : redacted(e.value),
      };
    }
    if (value is List) return [for (final v in value) redacted(v)];
    if (value is String) {
      final trimmed = value.trimLeft();
      if (trimmed.startsWith('{') || trimmed.startsWith('[')) {
        try {
          return redacted(jsonDecode(value));
        } catch (_) {}
      }
    }
    return value;
  }

  static String _mask(Object? value) =>
      value == null || '$value'.isEmpty ? '' : '••• (${'$value'.length} chars)';

  static String _redactUrl(String url) {
    final uri = Uri.tryParse(url);
    if (uri == null || !uri.hasQuery) return url;
    final hidden = uri.queryParametersAll.keys
        .where((k) => redactKeys.contains(k.toLowerCase()));
    if (hidden.isEmpty) return url;
    return uri.replace(queryParameters: {
      for (final e in uri.queryParametersAll.entries)
        e.key: redactKeys.contains(e.key.toLowerCase()) ? ['•••'] : e.value,
    }).toString();
  }

  static void _emit(Map<String, Object?> record) {
    for (final line in encode(record)) {
      output(line);
    }
  }

  static bool _isHighSurrogate(int unit) => unit >= 0xD800 && unit <= 0xDBFF;

  /// Maps, lists and sets are logged as JSON; so are objects with a
  /// `toJson()` (models), in them or on their own.
  static String _text(Object? value) {
    if (value is String) return value;
    if (value is Map || value is Iterable || _hasToJson(value)) {
      try {
        return const JsonEncoder.withIndent('  ', _encodable).convert(value);
      } catch (_) {}
    }
    return '$value';
  }

  static Object? _encodable(Object? o) {
    if (o is Iterable) return o.toList();
    if (o is DateTime) return o.toIso8601String();
    if (o is Enum) return o.name;
    try {
      return (o as dynamic).toJson();
    } catch (_) {
      return '$o';
    }
  }

  static bool _hasToJson(Object? o) {
    if (o == null || o is num || o is bool) return false;
    try {
      (o as dynamic).toJson;
      return true;
    } catch (_) {
      return false;
    }
  }

  static Object? _body(Object? body) {
    if (body is Map || body is List) {
      try {
        final json = jsonEncode(body, toEncodable: _encodable);
        if (json.length <= maxBodyLength) return body;
        return '${json.substring(0, maxBodyLength)}… (${json.length} chars)';
      } catch (_) {}
    }
    final text = '$body';
    return text.length <= maxBodyLength
        ? text
        : '${text.substring(0, maxBodyLength)}… (${text.length} chars)';
  }
}
