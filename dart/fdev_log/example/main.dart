import 'package:fdev_log/fdev_log.dart';

void main() {
  FdevLog.appPackage = 'fdev_log_example';
  FdevLog.info('connected', tag: 'Billing');
  FdevLog.warning({'sku': 'coins_100', 'state': 'pending'}, tag: 'Store');
  try {
    throw StateError('no purchase');
  } catch (e, stack) {
    FdevLog.error(e, tag: 'Store', stack: stack);
  }
}
