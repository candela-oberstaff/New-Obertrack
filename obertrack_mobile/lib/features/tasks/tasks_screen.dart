import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/formatting.dart';
import '../../core/theme.dart';
import '../../models/task.dart';
import '../../widgets/async_views.dart';
import '../auth/auth_controller.dart';
import 'tasks_repository.dart';

class TasksScreen extends ConsumerWidget {
  const TasksScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final listAsync = ref.watch(tasksListProvider);
    final filter = ref.watch(tasksFilterProvider);
    final user = ref.watch(currentUserProvider);
    final canEdit = user?.canEdit('tasks') ?? true;

    return Scaffold(
      appBar: AppBar(
        title: const Text('Tareas'),
        bottom: PreferredSize(
          preferredSize: const Size.fromHeight(52),
          child: _FilterBar(
            filter: filter,
            myId: user?.id,
          ),
        ),
      ),
      body: RefreshIndicator(
        onRefresh: () async {
          ref.invalidate(tasksListProvider);
          await ref.read(tasksListProvider.future);
        },
        child: listAsync.when(
          loading: () => const CenteredLoader(),
          error: (e, _) => ErrorRetry(
            message: e.toString().replaceFirst('Exception: ', ''),
            onRetry: () => ref.invalidate(tasksListProvider),
          ),
          data: (page) {
            if (page.items.isEmpty) {
              return ListView(
                children: const [
                  SizedBox(height: 120),
                  EmptyState(
                    icon: Icons.task_alt_rounded,
                    title: 'Sin tareas',
                    subtitle: 'No hay tareas para este filtro.',
                  ),
                ],
              );
            }
            return ListView.builder(
              padding: const EdgeInsets.all(12),
              itemCount: page.items.length,
              itemBuilder: (_, i) => _TaskCard(
                task: page.items[i],
                canEdit: canEdit,
              ),
            );
          },
        ),
      ),
    );
  }
}

class _FilterBar extends ConsumerWidget {
  const _FilterBar({required this.filter, this.myId});
  final TasksFilter filter;
  final int? myId;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final notifier = ref.read(tasksFilterProvider.notifier);
    return SizedBox(
      height: 52,
      child: ListView(
        scrollDirection: Axis.horizontal,
        padding: const EdgeInsets.symmetric(horizontal: 12),
        children: [
          _Chip(
            label: 'Mías',
            selected: filter.assigneeId != null,
            onTap: () => notifier.update((f) => f.copyWith(
                  assigneeId: filter.assigneeId == null ? myId : null,
                )),
          ),
          const SizedBox(width: 8),
          for (final s in [
            TaskStatus.porHacer,
            TaskStatus.enProceso,
            TaskStatus.finalizado
          ]) ...[
            _Chip(
              label: s.label,
              selected: filter.status == s,
              onTap: () => notifier.update((f) => filter.status == s
                  ? f.copyWith(clearStatus: true)
                  : f.copyWith(status: s)),
            ),
            const SizedBox(width: 8),
          ],
        ],
      ),
    );
  }
}

class _Chip extends StatelessWidget {
  const _Chip(
      {required this.label, required this.selected, required this.onTap});
  final String label;
  final bool selected;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return FilterChip(
      label: Text(label),
      selected: selected,
      onSelected: (_) => onTap(),
      showCheckmark: false,
      selectedColor: Brand.blueViolet.withValues(alpha: 0.15),
      labelStyle: TextStyle(
        color: selected ? Brand.indigo : null,
        fontWeight: selected ? FontWeight.w600 : FontWeight.w500,
      ),
    );
  }
}

class _TaskCard extends ConsumerWidget {
  const _TaskCard({required this.task, required this.canEdit});
  final Task task;
  final bool canEdit;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final theme = Theme.of(context);
    return Card(
      margin: const EdgeInsets.only(bottom: 10),
      child: InkWell(
        onTap: () => _openDetail(context, ref),
        child: Padding(
          padding: const EdgeInsets.all(14),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Expanded(
                    child: Text(
                      task.title,
                      style: theme.textTheme.titleMedium?.copyWith(
                        fontWeight: FontWeight.w600,
                        decoration: task.completed
                            ? TextDecoration.lineThrough
                            : null,
                      ),
                    ),
                  ),
                  _PriorityDot(priority: task.priority),
                ],
              ),
              if (task.description.isNotEmpty) ...[
                const SizedBox(height: 4),
                Text(task.description,
                    maxLines: 2,
                    overflow: TextOverflow.ellipsis,
                    style: theme.textTheme.bodyMedium?.copyWith(
                        color: theme.colorScheme.onSurfaceVariant)),
              ],
              const SizedBox(height: 10),
              Row(
                children: [
                  _StatusChip(status: task.status),
                  const Spacer(),
                  if (task.endDate != null) ...[
                    Icon(Icons.event_outlined,
                        size: 15, color: theme.colorScheme.onSurfaceVariant),
                    const SizedBox(width: 4),
                    Text(formatDateShort(task.endDate),
                        style: theme.textTheme.bodySmall),
                  ],
                  if (task.commentCount > 0) ...[
                    const SizedBox(width: 12),
                    Icon(Icons.chat_bubble_outline,
                        size: 15, color: theme.colorScheme.onSurfaceVariant),
                    const SizedBox(width: 4),
                    Text('${task.commentCount}',
                        style: theme.textTheme.bodySmall),
                  ],
                ],
              ),
            ],
          ),
        ),
      ),
    );
  }

  void _openDetail(BuildContext context, WidgetRef ref) {
    showModalBottomSheet(
      context: context,
      isScrollControlled: true,
      showDragHandle: true,
      builder: (_) => _TaskDetailSheet(task: task, canEdit: canEdit),
    );
  }
}

class _TaskDetailSheet extends ConsumerStatefulWidget {
  const _TaskDetailSheet({required this.task, required this.canEdit});
  final Task task;
  final bool canEdit;

  @override
  ConsumerState<_TaskDetailSheet> createState() => _TaskDetailSheetState();
}

class _TaskDetailSheetState extends ConsumerState<_TaskDetailSheet> {
  bool _saving = false;

  Future<void> _setStatus(TaskStatus status) async {
    if (status == widget.task.status) return;
    setState(() => _saving = true);
    try {
      await ref
          .read(tasksRepositoryProvider)
          .updateStatus(widget.task.id, status);
      ref.invalidate(tasksListProvider);
      if (mounted) Navigator.pop(context);
    } catch (e) {
      if (mounted) {
        setState(() => _saving = false);
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(
            content: Text(e.toString().replaceFirst('Exception: ', ''))));
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final t = widget.task;
    final theme = Theme.of(context);
    return Padding(
      padding: EdgeInsets.only(
        left: 20,
        right: 20,
        top: 4,
        bottom: MediaQuery.of(context).viewInsets.bottom + 24,
      ),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(t.title,
              style: theme.textTheme.titleLarge
                  ?.copyWith(fontWeight: FontWeight.w700)),
          const SizedBox(height: 6),
          Wrap(spacing: 8, children: [
            _StatusChip(status: t.status),
            Chip(
                label: Text('Prioridad ${t.priority.label}'),
                visualDensity: VisualDensity.compact),
          ]),
          if (t.description.isNotEmpty) ...[
            const SizedBox(height: 14),
            Text(t.description, style: theme.textTheme.bodyMedium),
          ],
          if (t.boardName.isNotEmpty) ...[
            const SizedBox(height: 14),
            Row(children: [
              const Icon(Icons.dashboard_outlined, size: 16),
              const SizedBox(width: 6),
              Text(t.boardName, style: theme.textTheme.bodyMedium),
            ]),
          ],
          if (t.assignees.isNotEmpty) ...[
            const SizedBox(height: 8),
            Row(children: [
              const Icon(Icons.people_outline, size: 16),
              const SizedBox(width: 6),
              Expanded(
                  child: Text(t.assignees.map((u) => u.name).join(', '),
                      style: theme.textTheme.bodyMedium)),
            ]),
          ],
          if (widget.canEdit) ...[
            const SizedBox(height: 20),
            Text('Cambiar estado', style: theme.textTheme.labelLarge),
            const SizedBox(height: 8),
            if (_saving)
              const Center(child: Padding(
                padding: EdgeInsets.all(8),
                child: CircularProgressIndicator(),
              ))
            else
              Wrap(
                spacing: 8,
                children: [
                  for (final s in [
                    TaskStatus.porHacer,
                    TaskStatus.enProceso,
                    TaskStatus.finalizado
                  ])
                    ChoiceChip(
                      label: Text(s.label),
                      selected: t.status == s,
                      onSelected: (_) => _setStatus(s),
                    ),
                ],
              ),
          ],
        ],
      ),
    );
  }
}

class _PriorityDot extends StatelessWidget {
  const _PriorityDot({required this.priority});
  final TaskPriority priority;

  @override
  Widget build(BuildContext context) {
    final color = switch (priority) {
      TaskPriority.low => Brand.success,
      TaskPriority.medium => Brand.azure,
      TaskPriority.high => Brand.warning,
      TaskPriority.urgent => Brand.danger,
    };
    return Tooltip(
      message: 'Prioridad ${priority.label}',
      child: Container(
        margin: const EdgeInsets.only(top: 4, left: 8),
        width: 10,
        height: 10,
        decoration: BoxDecoration(color: color, shape: BoxShape.circle),
      ),
    );
  }
}

class _StatusChip extends StatelessWidget {
  const _StatusChip({required this.status});
  final TaskStatus status;

  @override
  Widget build(BuildContext context) {
    final color = switch (status) {
      TaskStatus.porHacer => Theme.of(context).colorScheme.outline,
      TaskStatus.enProceso => Brand.azure,
      TaskStatus.finalizado => Brand.success,
      TaskStatus.unknown => Theme.of(context).colorScheme.outline,
    };
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.12),
        borderRadius: BorderRadius.circular(20),
      ),
      child: Text(status.label,
          style: TextStyle(
              color: color, fontSize: 12, fontWeight: FontWeight.w600)),
    );
  }
}
