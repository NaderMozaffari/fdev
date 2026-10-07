# fdev log records

An app built with `--dart-define=FDEV_LOGS=true` prints each log as one
record. fdev finds records in `flutter run` output: after the logcat prefix
on Android (`I/flutter (1234): `), after `flutter: ` on iOS and macOS, and at
the start of the line on the web.

## Lines

```
⟪fd⟫<json>
⟪fd <id> <n>/<count>⟫<part of the json>
```

A record whose JSON is longer than 300 UTF-16 units is split into parts
of at most 300 units (logcat and the iOS log truncate long lines), never
inside a surrogate pair. Parts share an `<id>` (an increasing number) and are
joined in `<n>` order once all `<count>` have arrived. The JSON is on one
line: newlines in strings are escaped.

## Logs

| Field | | |
|---|---|---|
| `l` | level | `debug`, `info`, `success`, `warning`, `error` or `fatal` |
| `t` | tag | optional; shown before the message |
| `m` | message | text; may have newlines; a JSON object or array, or a Dart Map or List as `toString` prints it (`{id: 7, tags: [a, b]}`), alone or after some text, is pretty-printed |
| `s` | stack trace | optional, `StackTrace.toString()` |
| `at` | call site | optional, `lib/<path>.dart:<line>[:<col>]` relative to the project, or an absolute path |

```json
{"l":"info","t":"Billing","m":"connected","at":"lib/billing/billing.dart:42:7"}
```

## Network

| Field | | |
|---|---|---|
| `l` | | `http` |
| `p` | phase | `request`, `response` or `error` |
| `method` | | `GET`, `POST`, ... |
| `url` | | the full URL |
| `status` | | HTTP status, when there is a response |
| `ms` | | time since the request, for `response` and `error` |
| `headers` | | an object of strings |
| `body` | | JSON, or a string (a string holding JSON is shown as JSON) |
| `error` | | what went wrong, for `error` without a response |
| `at` | | the code that made the request |
| `name` | | optional; a short name for the call, like the method that made it (`getProfile`), shown before the path |
| `id` | | optional; the same on a request and its response, to pair them when calls to one URL overlap (else they pair by method and URL) |

```json
{"l":"http","p":"response","method":"GET","url":"https://api.example.com/v1/me","status":200,"ms":142,"body":{"id":7},"name":"getProfile"}
```

A request waits for its response (fdev shows it spinning) until one with
the same `id`, or method and URL, comes; a hot restart gives up on it.

## Values

| Field | | |
|---|---|---|
| `l` | | `value` |
| `k` | name | e.g. `accessToken` |
| `v` | value | a string, or any JSON |
| `t` | tag | optional |

A value to keep at hand: the viewer's values window (`k`) shows the newest
of each name, to copy.

```json
{"l":"value","k":"accessToken","v":"eyJhbGciOi..."}
```

Mask secrets before printing: device logs end up in bug reports and CI
output. `fdev_log` masks the values of keys like `authorization`, `cookie`,
`password` and `token` in headers, bodies and URL queries.

## Plain lines

Without records, fdev still understands `LEVEL [tag] message` with
`DEBUG`, `INFO`, `OK`, `WARN`, `ERROR`, `FATAL`, `REQUEST` or `RESPONSE`,
Flutter's exception blocks, and uncaught exceptions (`E/flutter`).
