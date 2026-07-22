import 'package:flutter/foundation.dart';

/// Configuración de entorno de la app.
///
/// La URL base puede sobrescribirse en tiempo de compilación:
///   flutter run --dart-define=API_BASE_URL=https://obertrack.com
///
/// Por defecto apunta al backend en Docker local. Ten en cuenta que el host
/// `localhost` desde un emulador Android NO es la máquina de desarrollo: hay
/// que usar `10.0.2.2`. En el simulador de iOS sí funciona `localhost`.
class AppConfig {
  AppConfig._();

  /// Base sin el sufijo `/api`. Ej: http://10.0.2.2:8080
  static const String _defaultHost = String.fromEnvironment(
    'API_BASE_URL',
    defaultValue: _autoDefault,
  );

  // En Android el emulador enruta la máquina host por 10.0.2.2.
  // En iOS/otros, localhost. Se resuelve en runtime más abajo.
  static const String _autoDefault = 'auto';

  static String get baseHost {
    if (_defaultHost != _autoDefault) return _defaultHost;
    if (defaultTargetPlatform == TargetPlatform.android) {
      return 'http://10.0.2.2:8080';
    }
    return 'http://localhost:8080';
  }

  /// Prefijo de la API REST.
  static String get apiBaseUrl => '$baseHost/api';

  /// Base para conexiones WebSocket (ws/wss según el esquema http/https).
  static String get wsBaseUrl {
    final host = baseHost;
    if (host.startsWith('https://')) {
      return host.replaceFirst('https://', 'wss://');
    }
    return host.replaceFirst('http://', 'ws://');
  }

  static const Duration connectTimeout = Duration(seconds: 15);
  static const Duration receiveTimeout = Duration(seconds: 20);
}
