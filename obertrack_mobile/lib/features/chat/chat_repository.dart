import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/api_client.dart';
import '../../core/providers.dart';
import '../../models/chat.dart';
import '../../models/json_utils.dart';
import '../../models/user.dart';

class ChatRepository {
  ChatRepository(this._api);
  final ApiClient _api;

  /// GET /api/channels → array de ChannelWithUnread.
  Future<List<Channel>> channels() async {
    final r = await _api.get('/channels');
    if (r.statusCode == 200 && r.data is List) {
      return (r.data as List)
          .whereType<Map<String, dynamic>>()
          .map(Channel.fromJson)
          .toList();
    }
    final msg = (r.data is Map && r.data['error'] is String)
        ? r.data['error'] as String
        : 'No se pudieron cargar los chats';
    throw Exception(msg);
  }

  /// GET /api/channels/unread/total → { total_unread }.
  Future<int> unreadTotal() async {
    final r = await _api.get('/channels/unread/total');
    if (r.statusCode == 200 && r.data is Map && r.data['total_unread'] is num) {
      return (r.data['total_unread'] as num).toInt();
    }
    return 0;
  }

  /// GET /api/channels/:id/messages → array (máx 100). Se ordena ascendente.
  Future<List<ChatMessage>> messages(int channelId) async {
    final r = await _api.get('/channels/$channelId/messages');
    if (r.statusCode == 200 && r.data is List) {
      final list = (r.data as List)
          .whereType<Map<String, dynamic>>()
          .map(ChatMessage.fromJson)
          .where((m) => !m.isDeleted)
          .toList();
      list.sort((a, b) => a.id.compareTo(b.id));
      return list;
    }
    final msg = (r.data is Map && r.data['error'] is String)
        ? r.data['error'] as String
        : 'No se pudieron cargar los mensajes';
    throw Exception(msg);
  }

  /// POST /api/channels/:id/messages → mensaje creado (201).
  Future<ChatMessage> send(int channelId, String content) async {
    final r =
        await _api.post('/channels/$channelId/messages', data: {'content': content});
    if ((r.statusCode == 201 || r.statusCode == 200) &&
        r.data is Map<String, dynamic>) {
      return ChatMessage.fromJson(r.data as Map<String, dynamic>);
    }
    final msg = (r.data is Map && r.data['error'] is String)
        ? r.data['error'] as String
        : 'No se pudo enviar el mensaje';
    throw Exception(msg);
  }

  Future<void> markRead(int channelId) =>
      _api.post('/channels/$channelId/read');

  /// GET /api/channels/all-users → usuarios para iniciar un DM.
  Future<List<User>> allUsers() async {
    final r = await _api.get('/channels/all-users');
    if (r.statusCode == 200 && r.data is List) {
      return (r.data as List)
          .whereType<Map<String, dynamic>>()
          .map(User.fromJson)
          .toList();
    }
    return const [];
  }

  /// POST /api/channels/dm → find-or-create; devuelve el id del canal.
  Future<int> createDm(int recipientId) async {
    final r = await _api.post('/channels/dm', data: {'recipient_id': recipientId});
    if ((r.statusCode == 200 || r.statusCode == 201) &&
        r.data is Map<String, dynamic>) {
      return asInt((r.data as Map<String, dynamic>)['id']);
    }
    final msg = (r.data is Map && r.data['error'] is String)
        ? r.data['error'] as String
        : 'No se pudo iniciar la conversación';
    throw Exception(msg);
  }
}

final chatRepositoryProvider = Provider<ChatRepository>((ref) {
  return ChatRepository(ref.watch(apiClientProvider));
});

final channelsProvider = FutureProvider.autoDispose<List<Channel>>((ref) {
  return ref.watch(chatRepositoryProvider).channels();
});

/// Total de mensajes sin leer entre todos los canales (badge de la pestaña).
final channelsUnreadProvider = FutureProvider<int>((ref) {
  return ref.watch(chatRepositoryProvider).unreadTotal();
});

final messagesProvider =
    FutureProvider.autoDispose.family<List<ChatMessage>, int>((ref, channelId) {
  return ref.watch(chatRepositoryProvider).messages(channelId);
});
