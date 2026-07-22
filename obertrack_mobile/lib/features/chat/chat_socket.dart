import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:web_socket_channel/io.dart';

import '../../core/config.dart';
import '../../core/providers.dart';
import 'chat_repository.dart';

/// Mantiene abierta la conexión `/ws/channels` mientras haya sesión y refresca
/// la lista de chats, el contador y los mensajes abiertos ante cada evento.
class ChannelsSocket {
  ChannelsSocket(this._ref);
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
        Uri.parse('${AppConfig.wsBaseUrl}/ws/channels'),
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
    // Refrescamos desde el REST (fuente de verdad): lista, contador y mensajes.
    _ref.invalidate(channelsProvider);
    _ref.invalidate(channelsUnreadProvider);
    _ref.invalidate(messagesProvider);
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

final channelsSocketProvider = Provider<ChannelsSocket>((ref) {
  final socket = ChannelsSocket(ref);
  ref.onDispose(socket.dispose);
  return socket;
});
