import 'package:flutter/material.dart';

void bootstrap(String flavor) {
  WidgetsFlutterBinding.ensureInitialized();
  debugPrint('Acme Shop starting ($flavor)');
  runApp(const SizedBox());
}
