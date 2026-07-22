import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/theme.dart';
import '../../models/chat.dart';
import '../../models/user.dart';
import '../../widgets/async_views.dart';
import 'chat_repository.dart';
import 'chat_thread_screen.dart';

class ChatsScreen extends ConsumerWidget {
  const ChatsScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final async = ref.watch(channelsProvider);

    return Scaffold(
      appBar: AppBar(title: const Text('Chats')),
      floatingActionButton: FloatingActionButton(
        onPressed: () => _startNewDm(context, ref),
        child: const Icon(Icons.edit_outlined),
      ),
      body: RefreshIndicator(
        onRefresh: () async {
          ref.invalidate(channelsProvider);
          ref.invalidate(channelsUnreadProvider);
          await ref.read(channelsProvider.future);
        },
        child: async.when(
          loading: () => const CenteredLoader(),
          error: (e, _) => ErrorRetry(
            message: e.toString().replaceFirst('Exception: ', ''),
            onRetry: () => ref.invalidate(channelsProvider),
          ),
          data: (channels) {
            if (channels.isEmpty) {
              return ListView(
                children: const [
                  SizedBox(height: 120),
                  EmptyState(
                    icon: Icons.forum_outlined,
                    title: 'Sin conversaciones',
                    subtitle: 'Toca el botón para iniciar un chat.',
                  ),
                ],
              );
            }
            return ListView.separated(
              itemCount: channels.length,
              separatorBuilder: (_, _) => const Divider(height: 1, indent: 72),
              itemBuilder: (_, i) => _ChannelTile(channel: channels[i]),
            );
          },
        ),
      ),
    );
  }

  Future<void> _startNewDm(BuildContext context, WidgetRef ref) async {
    final users = await ref.read(chatRepositoryProvider).allUsers();
    if (!context.mounted) return;
    if (users.isEmpty) {
      ScaffoldMessenger.of(context).showSnackBar(const SnackBar(
          content: Text('No hay usuarios disponibles para chatear')));
      return;
    }
    final picked = await showModalBottomSheet<User>(
      context: context,
      isScrollControlled: true,
      showDragHandle: true,
      builder: (_) => _UserPicker(users: users),
    );
    if (picked == null || !context.mounted) return;
    try {
      final channelId = await ref.read(chatRepositoryProvider).createDm(picked.id);
      ref.invalidate(channelsProvider);
      if (!context.mounted) return;
      Navigator.of(context).push(MaterialPageRoute(
        builder: (_) => ChatThreadScreen(channelId: channelId, title: picked.name),
      ));
    } catch (e) {
      if (context.mounted) {
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(
            content: Text(e.toString().replaceFirst('Exception: ', ''))));
      }
    }
  }
}

class _ChannelTile extends StatelessWidget {
  const _ChannelTile({required this.channel});
  final Channel channel;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final hasUnread = channel.unreadCount > 0;
    return ListTile(
      contentPadding: const EdgeInsets.symmetric(horizontal: 16, vertical: 6),
      leading: CircleAvatar(
        radius: 24,
        backgroundColor: channel.isDirect
            ? Brand.blueViolet.withValues(alpha: 0.15)
            : Brand.azure.withValues(alpha: 0.15),
        child: channel.isDirect
            ? Text(channel.initials,
                style: const TextStyle(
                    color: Brand.blueViolet, fontWeight: FontWeight.w700))
            : Icon(
                channel.type == ChannelType.private
                    ? Icons.lock_outline
                    : Icons.tag,
                color: Brand.azure),
      ),
      title: Text(
        channel.displayName,
        maxLines: 1,
        overflow: TextOverflow.ellipsis,
        style: TextStyle(
            fontWeight: hasUnread ? FontWeight.w700 : FontWeight.w600),
      ),
      subtitle: channel.description.isNotEmpty
          ? Text(channel.description,
              maxLines: 1, overflow: TextOverflow.ellipsis)
          : Text(channel.isDirect ? 'Mensaje directo' : 'Canal',
              style: theme.textTheme.bodySmall),
      trailing: hasUnread
          ? Container(
              padding:
                  const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
              decoration: const BoxDecoration(
                color: Brand.orchid,
                borderRadius: BorderRadius.all(Radius.circular(20)),
              ),
              child: Text('${channel.unreadCount}',
                  style: const TextStyle(
                      color: Colors.white,
                      fontSize: 12,
                      fontWeight: FontWeight.w700)),
            )
          : null,
      onTap: () => Navigator.of(context).push(MaterialPageRoute(
        builder: (_) =>
            ChatThreadScreen(channelId: channel.id, title: channel.displayName),
      )),
    );
  }
}

class _UserPicker extends StatefulWidget {
  const _UserPicker({required this.users});
  final List<User> users;

  @override
  State<_UserPicker> createState() => _UserPickerState();
}

class _UserPickerState extends State<_UserPicker> {
  String _query = '';

  @override
  Widget build(BuildContext context) {
    final filtered = widget.users
        .where((u) =>
            u.name.toLowerCase().contains(_query.toLowerCase()) ||
            u.email.toLowerCase().contains(_query.toLowerCase()))
        .toList();
    return Padding(
      padding: EdgeInsets.only(
        left: 16,
        right: 16,
        top: 4,
        bottom: MediaQuery.of(context).viewInsets.bottom + 16,
      ),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('Nuevo mensaje',
              style: Theme.of(context)
                  .textTheme
                  .titleLarge
                  ?.copyWith(fontWeight: FontWeight.w700)),
          const SizedBox(height: 12),
          TextField(
            autofocus: true,
            decoration: const InputDecoration(
              hintText: 'Buscar persona…',
              prefixIcon: Icon(Icons.search),
            ),
            onChanged: (v) => setState(() => _query = v),
          ),
          const SizedBox(height: 8),
          Flexible(
            child: ListView.builder(
              shrinkWrap: true,
              itemCount: filtered.length,
              itemBuilder: (_, i) {
                final u = filtered[i];
                return ListTile(
                  leading: CircleAvatar(
                    backgroundColor: Brand.blueViolet.withValues(alpha: 0.15),
                    child: Text(u.initials,
                        style: const TextStyle(
                            color: Brand.blueViolet,
                            fontWeight: FontWeight.w700)),
                  ),
                  title: Text(u.name),
                  subtitle: Text(u.jobTitle.isNotEmpty ? u.jobTitle : u.email),
                  onTap: () => Navigator.pop(context, u),
                );
              },
            ),
          ),
        ],
      ),
    );
  }
}
