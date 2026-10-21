import 'package:fdev_log/fdev_log.dart';

// One const Log per file, named after what it logs.
const _log = Log('Billing');

void main() {
  FdevLog.appPackage = 'fdev_log_example';

  _log.info('Connected');
  _log.warning('No receipt, retrying');
  _log.debug({'sku': 'coins_100', 'state': 'pending'}); // a Map: shown as JSON
  _log.value('orderId', 'o-42');
  try {
    throw StateError('no purchase');
  } catch (e, st) {
    _log.error('Purchase failed', e, st);
  }
  _log.success('Purchase verified');
}
