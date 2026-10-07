# fdev_log

Logs for Flutter and Dart apps that [fdev](https://github.com/NaderMozaffari/fdev)
shows as one clean line each, filterable, with a link to the code that logged
them. Outside fdev they are plain one-line text. See the fdev README for
setup and [docs/PROTOCOL.md](../../docs/PROTOCOL.md) for the format.

```dart
FdevLog.appPackage = 'my_app';
FdevLog.info('connected', tag: 'Billing');
FdevLog.error(e, tag: 'Billing', stack: stack);
FdevLog.http(FdevHttp(phase: FdevHttpPhase.response, method: 'GET', url: url, status: 200));
FdevLog.info(user);                         // a Map, List or toJson() model: shown as JSON
FdevLog.value('accessToken', token);        // kept at hand in fdev's values window (k)
```

Secrets in headers, bodies and URLs are masked (`FdevLog.redactKeys`), and
nothing is logged in release builds unless `FdevLog.inRelease` is set.
