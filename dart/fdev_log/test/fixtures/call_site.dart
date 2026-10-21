// Run by fdev_log_test.dart with FDEV_LOGS=true: logs through a Log from
// this file, so the test can check that `at` points here.
import 'package:fdev_log/fdev_log.dart';

const _log = Log('Fixture');

void main() {
  _log.info('first');
  _log.error('second', StateError('boom'));
}
