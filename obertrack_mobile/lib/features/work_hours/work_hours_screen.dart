import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/formatting.dart';
import '../../core/theme.dart';
import '../../models/work_hour.dart';
import '../../widgets/async_views.dart';
import '../auth/auth_controller.dart';
import 'work_hours_repository.dart';

class WorkHoursScreen extends ConsumerWidget {
  const WorkHoursScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final listAsync = ref.watch(workHoursListProvider);
    final user = ref.watch(currentUserProvider);
    final canRegister = user?.canEdit('hours') ?? true;

    return Scaffold(
      appBar: AppBar(title: const Text('Mis horas')),
      floatingActionButton: canRegister
          ? FloatingActionButton.extended(
              onPressed: () => _openRegister(context, ref),
              icon: const Icon(Icons.add),
              label: const Text('Registrar'),
            )
          : null,
      body: RefreshIndicator(
        onRefresh: () async {
          ref.invalidate(workHoursListProvider);
          ref.invalidate(workHoursSummaryProvider);
          await ref.read(workHoursListProvider.future);
        },
        child: listAsync.when(
          loading: () => const CenteredLoader(),
          error: (e, _) => ErrorRetry(
            message: e.toString().replaceFirst('Exception: ', ''),
            onRetry: () => ref.invalidate(workHoursListProvider),
          ),
          data: (page) {
            return ListView(
              padding: const EdgeInsets.all(12),
              children: [
                const _SummaryCard(),
                const SizedBox(height: 12),
                if (page.items.isEmpty)
                  const Padding(
                    padding: EdgeInsets.only(top: 60),
                    child: EmptyState(
                      icon: Icons.schedule_rounded,
                      title: 'Sin registros',
                      subtitle: 'Registra tu primera jornada con el botón +.',
                    ),
                  )
                else
                  for (final wh in page.items) _WorkHourTile(wh: wh),
              ],
            );
          },
        ),
      ),
    );
  }

  void _openRegister(BuildContext context, WidgetRef ref) {
    showModalBottomSheet(
      context: context,
      isScrollControlled: true,
      showDragHandle: true,
      builder: (_) => const _RegisterSheet(),
    );
  }
}

class _SummaryCard extends ConsumerWidget {
  const _SummaryCard();

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final async = ref.watch(workHoursSummaryProvider);
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('Resumen del mes',
                style: Theme.of(context).textTheme.titleSmall),
            const SizedBox(height: 12),
            async.when(
              loading: () => const SizedBox(
                  height: 48, child: Center(child: CircularProgressIndicator())),
              error: (_, _) => const Text('—'),
              data: (s) => Row(
                children: [
                  _SummaryStat(
                      label: 'Total', value: formatHours(s.total), color: Brand.blueViolet),
                  _SummaryStat(
                      label: 'Aprobadas',
                      value: formatHours(s.approved),
                      color: Brand.success),
                  _SummaryStat(
                      label: 'Pendientes',
                      value: formatHours(s.pending),
                      color: Brand.warning),
                  _SummaryStat(
                      label: 'Rechazadas',
                      value: formatHours(s.rejected),
                      color: Brand.danger),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _SummaryStat extends StatelessWidget {
  const _SummaryStat(
      {required this.label, required this.value, required this.color});
  final String label;
  final String value;
  final Color color;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Expanded(
      child: Column(
        children: [
          Text(value,
              style: theme.textTheme.titleLarge
                  ?.copyWith(fontWeight: FontWeight.w700, color: color)),
          const SizedBox(height: 2),
          Text(label,
              textAlign: TextAlign.center, style: theme.textTheme.bodySmall),
        ],
      ),
    );
  }
}

class _WorkHourTile extends StatelessWidget {
  const _WorkHourTile({required this.wh});
  final WorkHour wh;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Card(
      margin: const EdgeInsets.only(bottom: 8),
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
        child: Row(
          children: [
            Column(
              crossAxisAlignment: CrossAxisAlignment.center,
              children: [
                Text(formatHours(wh.hoursWorked),
                    style: theme.textTheme.titleLarge?.copyWith(
                        fontWeight: FontWeight.w700, color: Brand.blueViolet)),
                Text('h', style: theme.textTheme.bodySmall),
              ],
            ),
            const SizedBox(width: 16),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(formatDate(wh.workDate),
                      style: theme.textTheme.titleSmall
                          ?.copyWith(fontWeight: FontWeight.w600)),
                  const SizedBox(height: 2),
                  Text(wh.workType.label,
                      style: theme.textTheme.bodySmall?.copyWith(
                          color: theme.colorScheme.onSurfaceVariant)),
                  if (wh.activities.isNotEmpty) ...[
                    const SizedBox(height: 4),
                    Text(wh.activities,
                        maxLines: 2,
                        overflow: TextOverflow.ellipsis,
                        style: theme.textTheme.bodySmall),
                  ],
                ],
              ),
            ),
            const SizedBox(width: 8),
            _StatusBadge(status: wh.status),
          ],
        ),
      ),
    );
  }
}

class _StatusBadge extends StatelessWidget {
  const _StatusBadge({required this.status});
  final WorkHourStatus status;

  @override
  Widget build(BuildContext context) {
    final color = switch (status) {
      WorkHourStatus.pending => Brand.warning,
      WorkHourStatus.approved => Brand.success,
      WorkHourStatus.rejected => Brand.danger,
    };
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 5),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.12),
        borderRadius: BorderRadius.circular(20),
      ),
      child: Text(status.label,
          style: TextStyle(
              color: color, fontSize: 11.5, fontWeight: FontWeight.w600)),
    );
  }
}

class _RegisterSheet extends ConsumerStatefulWidget {
  const _RegisterSheet();

  @override
  ConsumerState<_RegisterSheet> createState() => _RegisterSheetState();
}

class _RegisterSheetState extends ConsumerState<_RegisterSheet> {
  DateTime _date = DateTime.now();
  WorkType _type = WorkType.complete;
  final _activities = TextEditingController();
  final _absenceReason = TextEditingController();
  final _recoverHours = TextEditingController(text: '8');
  bool _saving = false;
  String? _error;

  @override
  void dispose() {
    _activities.dispose();
    _absenceReason.dispose();
    _recoverHours.dispose();
    super.dispose();
  }

  Future<void> _pickDate() async {
    final picked = await showDatePicker(
      context: context,
      initialDate: _date,
      firstDate: DateTime.now().subtract(const Duration(days: 365)),
      lastDate: DateTime.now(),
      locale: const Locale('es'),
    );
    if (picked != null) setState(() => _date = picked);
  }

  Future<void> _submit() async {
    setState(() {
      _saving = true;
      _error = null;
    });
    try {
      await ref.read(workHoursRepositoryProvider).create(
            workDate: _date,
            workType: _type,
            activities: _activities.text.trim(),
            absenceReason: _absenceReason.text.trim(),
            hoursWorked: _type == WorkType.recover
                ? double.tryParse(_recoverHours.text.replaceAll(',', '.'))
                : null,
          );
      ref.invalidate(workHoursListProvider);
      ref.invalidate(workHoursSummaryProvider);
      if (mounted) {
        Navigator.pop(context);
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('Jornada registrada')),
        );
      }
    } catch (e) {
      setState(() {
        _saving = false;
        _error = e.toString().replaceFirst('Exception: ', '');
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Padding(
      padding: EdgeInsets.only(
        left: 20,
        right: 20,
        top: 4,
        bottom: MediaQuery.of(context).viewInsets.bottom + 24,
      ),
      child: SingleChildScrollView(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('Registrar jornada',
                style: theme.textTheme.titleLarge
                    ?.copyWith(fontWeight: FontWeight.w700)),
            const SizedBox(height: 16),
            ListTile(
              contentPadding: EdgeInsets.zero,
              leading: const Icon(Icons.event_outlined),
              title: const Text('Fecha'),
              subtitle: Text(formatDate(_date)),
              trailing: const Icon(Icons.edit_calendar_outlined),
              onTap: _pickDate,
            ),
            const SizedBox(height: 8),
            Text('Tipo', style: theme.textTheme.labelLarge),
            const SizedBox(height: 8),
            SegmentedButton<WorkType>(
              segments: const [
                ButtonSegment(
                    value: WorkType.complete, label: Text('Completa')),
                ButtonSegment(
                    value: WorkType.absence, label: Text('Ausencia')),
                ButtonSegment(
                    value: WorkType.recover, label: Text('Recuper.')),
              ],
              selected: {_type},
              onSelectionChanged: (s) => setState(() => _type = s.first),
            ),
            const SizedBox(height: 16),
            if (_type == WorkType.recover)
              TextField(
                controller: _recoverHours,
                keyboardType:
                    const TextInputType.numberWithOptions(decimal: true),
                decoration: const InputDecoration(
                  labelText: 'Horas a recuperar',
                  prefixIcon: Icon(Icons.timelapse),
                ),
              ),
            if (_type == WorkType.absence)
              TextField(
                controller: _absenceReason,
                decoration: const InputDecoration(
                  labelText: 'Motivo de la ausencia',
                  prefixIcon: Icon(Icons.info_outline),
                ),
              ),
            if (_type == WorkType.complete)
              TextField(
                controller: _activities,
                maxLines: 3,
                decoration: const InputDecoration(
                  labelText: 'Actividades realizadas',
                  alignLabelWithHint: true,
                ),
              ),
            if (_type == WorkType.complete)
              Padding(
                padding: const EdgeInsets.only(top: 8),
                child: Text(
                  'El servidor calcula las horas de una jornada completa.',
                  style: theme.textTheme.bodySmall?.copyWith(
                      color: theme.colorScheme.onSurfaceVariant),
                ),
              ),
            if (_error != null) ...[
              const SizedBox(height: 12),
              Text(_error!, style: const TextStyle(color: Brand.danger)),
            ],
            const SizedBox(height: 20),
            FilledButton(
              onPressed: _saving ? null : _submit,
              child: _saving
                  ? const SizedBox(
                      width: 22,
                      height: 22,
                      child: CircularProgressIndicator(
                          strokeWidth: 2.5,
                          valueColor: AlwaysStoppedAnimation(Colors.white)))
                  : const Text('Guardar'),
            ),
          ],
        ),
      ),
    );
  }
}
