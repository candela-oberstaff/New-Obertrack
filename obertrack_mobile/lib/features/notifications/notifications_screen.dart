import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/formatting.dart';
import '../../core/theme.dart';
import '../../models/notification.dart';
import '../../widgets/async_views.dart';
import 'notifications_repository.dart';

class NotificationsScreen extends ConsumerWidget {
  const NotificationsScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final listAsync = ref.watch(notificationsListProvider);

    return Scaffold(
      appBar: AppBar(
        title: const Text('Notificaciones'),
        actions: [
          TextButton(
            onPressed: () async {
              await ref
                  .read(notificationsRepositoryProvider)
                  .markAllAsRead();
              ref.invalidate(notificationsListProvider);
              ref.invalidate(unreadCountProvider);
            },
            child: const Text('Marcar todo'),
          ),
        ],
      ),
      body: RefreshIndicator(
        onRefresh: () async {
          ref.invalidate(notificationsListProvider);
          ref.invalidate(unreadCountProvider);
          await ref.read(notificationsListProvider.future);
        },
        child: listAsync.when(
          loading: () => const CenteredLoader(),
          error: (e, _) => ErrorRetry(
            message: 'No se pudieron cargar las notificaciones',
            onRetry: () => ref.invalidate(notificationsListProvider),
          ),
          data: (items) {
            if (items.isEmpty) {
              return ListView(
                children: const [
                  SizedBox(height: 120),
                  EmptyState(
                    icon: Icons.notifications_none_rounded,
                    title: 'Sin notificaciones',
                    subtitle: 'Aquí verás tus avisos y alertas.',
                  ),
                ],
              );
            }
            return ListView.separated(
              padding: const EdgeInsets.symmetric(vertical: 8),
              itemCount: items.length,
              separatorBuilder: (_, _) => const Divider(height: 1),
              itemBuilder: (_, i) => _NotificationTile(item: items[i]),
            );
          },
        ),
      ),
    );
  }
}

class _NotificationTile extends ConsumerWidget {
  const _NotificationTile({required this.item});
  final AppNotification item;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final theme = Theme.of(context);
    return ListTile(
      onTap: item.isRead
          ? null
          : () async {
              await ref
                  .read(notificationsRepositoryProvider)
                  .markAsRead(item.id);
              ref.invalidate(notificationsListProvider);
              ref.invalidate(unreadCountProvider);
            },
      leading: CircleAvatar(
        backgroundColor: _iconColor(item.type).withValues(alpha: 0.15),
        child: Icon(_iconFor(item.type), color: _iconColor(item.type)),
      ),
      title: Text(
        item.title,
        style: TextStyle(
          fontWeight: item.isRead ? FontWeight.w500 : FontWeight.w700,
        ),
      ),
      subtitle: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          if (item.message.isNotEmpty) Text(item.message),
          const SizedBox(height: 2),
          Text(formatRelative(item.createdAt),
              style: theme.textTheme.bodySmall
                  ?.copyWith(color: theme.colorScheme.onSurfaceVariant)),
        ],
      ),
      isThreeLine: item.message.isNotEmpty,
      trailing: item.isRead
          ? null
          : Container(
              width: 10,
              height: 10,
              decoration: const BoxDecoration(
                color: Brand.orchid,
                shape: BoxShape.circle,
              ),
            ),
    );
  }

  IconData _iconFor(String type) {
    final t = type.toLowerCase();
    if (t.contains('task') || t.contains('tarea')) {
      return Icons.check_circle_outline;
    }
    if (t.contains('hour') || t.contains('hora')) return Icons.schedule;
    if (t.contains('incident') || t.contains('emergen')) {
      return Icons.warning_amber_rounded;
    }
    if (t.contains('chat') || t.contains('message') || t.contains('mensaje')) {
      return Icons.chat_bubble_outline;
    }
    return Icons.notifications_none_rounded;
  }

  Color _iconColor(String type) {
    final t = type.toLowerCase();
    if (t.contains('incident') || t.contains('emergen')) return Brand.danger;
    if (t.contains('hour') || t.contains('hora')) return Brand.azure;
    return Brand.blueViolet;
  }
}
