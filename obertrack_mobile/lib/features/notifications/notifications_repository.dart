import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/api_client.dart';
import '../../core/providers.dart';
import '../../models/notification.dart';

class NotificationsRepository {
  NotificationsRepository(this._api);
  final ApiClient _api;

  /// GET /api/notifications → array plano (sin envoltorio).
  Future<List<AppNotification>> list() async {
    final resp = await _api.get('/notifications');
    final data = resp.data;
    if (resp.statusCode == 200 && data is List) {
      return data
          .whereType<Map<String, dynamic>>()
          .map(AppNotification.fromJson)
          .toList();
    }
    return const [];
  }

  /// GET /api/notifications/unread-count → { count }.
  Future<int> unreadCount() async {
    final resp = await _api.get('/notifications/unread-count');
    final data = resp.data;
    if (resp.statusCode == 200 && data is Map && data['count'] is num) {
      return (data['count'] as num).toInt();
    }
    return 0;
  }

  Future<void> markAsRead(int id) => _api.post('/notifications/$id/read');

  Future<void> markAllAsRead() => _api.post('/notifications/read-all');
}

final notificationsRepositoryProvider =
    Provider<NotificationsRepository>((ref) {
  return NotificationsRepository(ref.watch(apiClientProvider));
});

final notificationsListProvider =
    FutureProvider.autoDispose<List<AppNotification>>((ref) {
  return ref.watch(notificationsRepositoryProvider).list();
});

/// Contador de no leídas. Se refresca al invalidar y al llegar eventos por WS.
final unreadCountProvider = FutureProvider<int>((ref) {
  return ref.watch(notificationsRepositoryProvider).unreadCount();
});
