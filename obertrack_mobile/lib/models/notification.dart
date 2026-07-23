import 'dart:convert';

import 'json_utils.dart';

class AppNotification {
  AppNotification({
    required this.id,
    required this.type,
    required this.title,
    required this.message,
    this.dataRaw = '',
    this.readAt,
    this.createdAt,
  });

  final int id;
  final String type;
  final String title;
  final String message;

  /// `data` viaja como STRING JSON en el backend; se decodifica bajo demanda.
  final String dataRaw;
  final DateTime? readAt;
  final DateTime? createdAt;

  bool get isRead => readAt != null;

  Map<String, dynamic> get data {
    if (dataRaw.isEmpty) return const {};
    try {
      final decoded = jsonDecode(dataRaw);
      return decoded is Map<String, dynamic> ? decoded : const {};
    } catch (_) {
      return const {};
    }
  }

  factory AppNotification.fromJson(Map<String, dynamic> j) => AppNotification(
        id: asInt(j['id']),
        type: asString(j['type']),
        title: asString(j['title']),
        message: asString(j['message']),
        dataRaw: asString(j['data']),
        readAt: parseDate(j['read_at']),
        createdAt: parseDate(j['created_at']),
      );

  AppNotification copyWith({DateTime? readAt}) => AppNotification(
        id: id,
        type: type,
        title: title,
        message: message,
        dataRaw: dataRaw,
        readAt: readAt ?? this.readAt,
        createdAt: createdAt,
      );
}
