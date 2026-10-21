# fdev_log

Logs for Flutter and Dart apps that [fdev](https://github.com/NaderMozaffari/fdev)
shows as one clean line each, filterable, with a link to the code that logged
them. Outside fdev they are plain one-line text. See the fdev README for
setup and [docs/PROTOCOL.md](../../docs/PROTOCOL.md) for the format.

One `const Log` per file, named after what it logs:

```dart
import 'package:fdev_log/fdev_log.dart';

const _log = Log('Billing');

_log.info('Connected');
_log.success('Purchase verified: $orderId');
_log.warning('No receipt, retrying');
_log.error('Purchase failed', e, st);        // the message becomes "Purchase failed: <e>"
_log.debug(user);                            // a Map, List or toJson() model: shown as JSON
_log.info('Uploading $p%', update: 'upload'); // one row that changes, not one per call
_log.value('accessToken', token);            // kept at hand in fdev's values window (k)
```

Set the app's package once at startup, so the `↗` link goes to your code:

```dart
FdevLog.appPackage = 'my_app';
```

## Which level when

- `debug`: details for whoever works on that code (payloads, states).
- `info`: a step of a flow worth seeing in the timeline.
- `success`: an operation the user asked for finished.
- `warning`: something unexpected that the app works around.
- `error`: an operation failed; pass the error and its stack (an `Error`
  carries its own, so `_log.error('...', e)` is enough for those).
- `fatal`: a bug: an uncaught error, a state that must not happen.

## Turning levels off

`FdevLog.levels` is the set of levels that are logged, all by default; a log
of another level is dropped. Fill it from dart-defines to switch levels off
at build time, e.g. `flutter run --dart-define=LOG_DEBUG_ENABLED=false`:

```dart
FdevLog.levels = {
  if (const bool.fromEnvironment('LOG_DEBUG_ENABLED', defaultValue: true))
    FdevLevel.debug,
  if (const bool.fromEnvironment('LOG_INFO_ENABLED', defaultValue: true))
    FdevLevel.info,
  if (const bool.fromEnvironment('LOG_SUCCESS_ENABLED', defaultValue: true))
    FdevLevel.success,
  if (const bool.fromEnvironment('LOG_WARNING_ENABLED', defaultValue: true))
    FdevLevel.warning,
  if (const bool.fromEnvironment('LOG_ERROR_ENABLED', defaultValue: true))
    FdevLevel.error,
  if (const bool.fromEnvironment('LOG_FATAL_ENABLED', defaultValue: true))
    FdevLevel.fatal,
};
```

`value` follows `debug`, and an HTTP error follows `error`. (`bool.fromEnvironment`
must be `const`, which is why each flag is written out.)

## The static API

`Log` forwards to `FdevLog`, which stays available for code that has no tag
to give or wants to set one per call:

```dart
FdevLog.info('connected', tag: 'Billing');
FdevLog.error(e, tag: 'Billing', stack: stack);
FdevLog.http(FdevHttp(phase: FdevHttpPhase.response, method: 'GET', url: url, status: 200));
```

Secrets in headers, bodies and URLs are masked (`FdevLog.redactKeys`), and
nothing is logged in release builds unless `FdevLog.inRelease` is set.
