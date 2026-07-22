import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:web_socket_channel/io.dart';

import '../../core/config.dart';
import '../../core/providers.dart';
import 'notifications_repository.dart';

/// Mantiene abierta la conexión `/ws/notifications` mientras haya sesión.
/// Ante cualquier evento entrante, invalida los providers de lista y contador
/// para que la UI se actualice en tiempo real. Reconecta con backoff simple.
class NotificationsSocket {
  NotificationsSocket(this._ref);
  final Ref _ref;

  IOWebSocketChannel? _channel;
  StreamSubscription? _sub;
  Timer? _reconnect;
  bool _disposed = false;

  Future<void> start() async {
    if (_disposed) return;
    final token = await _ref.read(tokenStoreProvider).accessToken;
    if (token == null) return;

    try {
      final channel = IOWebSocketChannel.connect(
        Uri.parse('${AppConfig.wsBaseUrl}/ws/notifications'),
        headers: {'Authorization': 'Bearer $token'},
        pingInterval: const Duration(seconds: 30),
      );
      _channel = channel;
      _sub = channel.stream.listen(
        (_) => _onEvent(),
        onDone: _scheduleReconnect,
        onError: (_) => _scheduleReconnect(),
        cancelOnError: true,
      );
    } catch (_) {
      _scheduleReconnect();
    }
  }

  void _onEvent() {
    // No parseamos el payload: basta con refrescar desde el REST (fuente de verdad).
    _ref.invalidate(unreadCountProvider);
    _ref.invalidate(notificationsListProvider);
  }

  void _scheduleReconnect() {
    if (_disposed) return;
    _reconnect?.cancel();
    _reconnect = Timer(const Duration(seconds: 5), start);
  }

  void dispose() {
    _disposed = true;
    _reconnect?.cancel();
    _sub?.cancel();
    _channel?.sink.close();
  }
}

/// Ciclo de vida del socket ligado al scope de Riverpod.
final notificationsSocketProvider = Provider<NotificationsSocket>((ref) {
  final socket = NotificationsSocket(ref);
  ref.onDispose(socket.dispose);
  return socket;
});
