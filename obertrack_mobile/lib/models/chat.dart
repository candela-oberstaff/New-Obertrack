import 'json_utils.dart';
import 'user.dart';

enum ChannelType {
  public,
  private,
  direct,
  unknown;

  static ChannelType fromString(String? v) {
    switch (v) {
      case 'public':
        return ChannelType.public;
      case 'private':
        return ChannelType.private;
      case 'direct':
        return ChannelType.direct;
      default:
        return ChannelType.unknown;
    }
  }
}

/// Canal de la lista de Chats (GET /api/channels → ChannelWithUnread).
class Channel {
  Channel({
    required this.id,
    required this.name,
    required this.description,
    required this.type,
    required this.unreadCount,
    this.recipient,
    this.participants = const [],
    this.supervised = false,
  });

  final int id;
  final String name;
  final String description;
  final ChannelType type;
  final int unreadCount;

  /// En un DM propio, el OTRO usuario.
  final User? recipient;

  /// En DMs supervisados por superadmin, ambos miembros.
  final List<User> participants;
  final bool supervised;

  bool get isDirect => type == ChannelType.direct;

  /// Nombre a mostrar: en DMs el nombre interno es "DM-a-b", usamos la persona.
  String get displayName {
    if (isDirect) {
      if (recipient != null) return recipient!.name;
      if (participants.isNotEmpty) {
        return participants.map((u) => u.name).join(' · ');
      }
    }
    return name;
  }

  String get initials {
    final n = displayName.trim();
    if (n.isEmpty) return '#';
    final parts = n.split(RegExp(r'\s+'));
    if (parts.length == 1) return parts.first[0].toUpperCase();
    return (parts.first[0] + parts.last[0]).toUpperCase();
  }

  factory Channel.fromJson(Map<String, dynamic> j) {
    final rawParts = (j['participants'] as List?) ?? const [];
    return Channel(
      id: asInt(j['id']),
      name: asString(j['name']),
      description: asString(j['description']),
      type: ChannelType.fromString(j['type'] as String?),
      unreadCount: asInt(j['unread_count']),
      recipient: j['recipient'] is Map<String, dynamic>
          ? User.fromJson(j['recipient'] as Map<String, dynamic>)
          : null,
      participants: rawParts
          .whereType<Map<String, dynamic>>()
          .map(User.fromJson)
          .toList(),
      supervised: asBool(j['supervised']),
    );
  }
}

/// Mensaje de un canal (ChannelMessage).
class ChatMessage {
  ChatMessage({
    required this.id,
    required this.channelId,
    required this.userId,
    required this.content,
    required this.isEdited,
    required this.isDeleted,
    this.user,
    this.createdAt,
  });

  final int id;
  final int channelId;
  final int userId;
  final String content;
  final bool isEdited;
  final bool isDeleted;
  final User? user;
  final DateTime? createdAt;

  factory ChatMessage.fromJson(Map<String, dynamic> j) => ChatMessage(
        id: asInt(j['id']),
        channelId: asInt(j['channel_id']),
        userId: asInt(j['user_id']),
        content: asString(j['content']),
        isEdited: asBool(j['is_edited']),
        isDeleted: asBool(j['is_deleted']),
        user: j['user'] is Map<String, dynamic>
            ? User.fromJson(j['user'] as Map<String, dynamic>)
            : null,
        createdAt: parseDate(j['created_at']),
      );
}
