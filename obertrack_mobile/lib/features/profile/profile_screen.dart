import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/biometric_service.dart';
import '../../core/formatting.dart';
import '../../core/theme.dart';
import '../../models/cv.dart';
import '../../models/user.dart';
import '../../widgets/async_views.dart';
import '../auth/auth_controller.dart';
import 'profile_repository.dart';

class ProfileScreen extends ConsumerWidget {
  const ProfileScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final user = ref.watch(currentUserProvider);
    if (user == null) return const CenteredLoader();

    // El CV vivo solo aplica a profesionales; para empleadores mostramos el
    // perfil sin la sección de expediente.
    final showCv = user.userType == UserType.profesional;

    return Scaffold(
      appBar: AppBar(
        title: const Text('Mi perfil'),
        actions: [
          IconButton(
            tooltip: 'Cerrar sesión',
            icon: const Icon(Icons.logout_rounded),
            onPressed: () => _confirmLogout(context, ref),
          ),
        ],
      ),
      body: RefreshIndicator(
        onRefresh: () async {
          await ref.read(authControllerProvider.notifier).refreshMe();
          ref.invalidate(myCvProvider);
        },
        child: ListView(
          padding: const EdgeInsets.all(16),
          children: [
            _ProfileHeader(user: user),
            const SizedBox(height: 20),
            _InfoCard(user: user),
            const SizedBox(height: 20),
            Text('Ajustes', style: Theme.of(context).textTheme.titleMedium),
            const SizedBox(height: 8),
            const _BiometricSetting(),
            if (showCv) ...[
              const SizedBox(height: 20),
              Text('Expediente laboral',
                  style: Theme.of(context).textTheme.titleMedium),
              const SizedBox(height: 8),
              const _CvSection(),
            ],
            const SizedBox(height: 32),
          ],
        ),
      ),
    );
  }

  Future<void> _confirmLogout(BuildContext context, WidgetRef ref) async {
    final ok = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Cerrar sesión'),
        content: const Text('¿Seguro que deseas salir?'),
        actions: [
          TextButton(
              onPressed: () => Navigator.pop(ctx, false),
              child: const Text('Cancelar')),
          FilledButton(
              onPressed: () => Navigator.pop(ctx, true),
              child: const Text('Salir')),
        ],
      ),
    );
    if (ok == true) {
      await ref.read(authControllerProvider.notifier).logout();
    }
  }
}

class _ProfileHeader extends StatelessWidget {
  const _ProfileHeader({required this.user});
  final User user;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Column(
      children: [
        Container(
          width: 88,
          height: 88,
          decoration: const BoxDecoration(
            gradient: LinearGradient(
              colors: [Brand.blueViolet, Brand.orchid],
              begin: Alignment.topLeft,
              end: Alignment.bottomRight,
            ),
            shape: BoxShape.circle,
          ),
          alignment: Alignment.center,
          child: Text(user.initials,
              style: const TextStyle(
                  color: Colors.white,
                  fontSize: 30,
                  fontWeight: FontWeight.w700)),
        ),
        const SizedBox(height: 12),
        Text(user.name,
            style: theme.textTheme.titleLarge
                ?.copyWith(fontWeight: FontWeight.w700)),
        const SizedBox(height: 4),
        Text(user.email,
            style: theme.textTheme.bodyMedium
                ?.copyWith(color: theme.colorScheme.onSurfaceVariant)),
        const SizedBox(height: 10),
        Wrap(
          spacing: 8,
          children: [
            Chip(
              label: Text(user.userType.label),
              backgroundColor: Brand.blueViolet.withValues(alpha: 0.12),
              labelStyle: const TextStyle(
                  color: Brand.indigo, fontWeight: FontWeight.w600),
            ),
            if (user.isManager)
              Chip(
                label: const Text('Manager'),
                backgroundColor: Brand.orchid.withValues(alpha: 0.12),
                labelStyle: const TextStyle(
                    color: Brand.orchid, fontWeight: FontWeight.w600),
              ),
          ],
        ),
      ],
    );
  }
}

class _InfoCard extends StatelessWidget {
  const _InfoCard({required this.user});
  final User user;

  @override
  Widget build(BuildContext context) {
    final rows = <(IconData, String, String)>[
      if (user.jobTitle.isNotEmpty)
        (Icons.badge_outlined, 'Puesto', user.jobTitle),
      if (user.companyName.isNotEmpty)
        (Icons.apartment_outlined, 'Empresa', user.companyName),
      if (user.phoneNumber.isNotEmpty)
        (Icons.phone_outlined, 'Teléfono', user.phoneNumber),
      if (_location(user).isNotEmpty)
        (Icons.place_outlined, 'Ubicación', _location(user)),
      if (user.industry.isNotEmpty)
        (Icons.business_center_outlined, 'Industria', user.industry),
    ];
    if (rows.isEmpty) return const SizedBox.shrink();

    return Card(
      child: Padding(
        padding: const EdgeInsets.symmetric(vertical: 4),
        child: Column(
          children: [
            for (var i = 0; i < rows.length; i++) ...[
              ListTile(
                leading: Icon(rows[i].$1),
                title: Text(rows[i].$2,
                    style: Theme.of(context).textTheme.bodySmall),
                subtitle: Text(rows[i].$3,
                    style: Theme.of(context)
                        .textTheme
                        .bodyLarge
                        ?.copyWith(fontWeight: FontWeight.w500)),
                dense: true,
              ),
              if (i < rows.length - 1)
                const Divider(height: 1, indent: 56),
            ],
          ],
        ),
      ),
    );
  }

  String _location(User u) {
    if (u.location.isNotEmpty) return u.location;
    return [u.city, u.state, u.country].where((s) => s.isNotEmpty).join(', ');
  }
}

class _CvSection extends ConsumerWidget {
  const _CvSection();

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final cvAsync = ref.watch(myCvProvider);
    return cvAsync.when(
      loading: () => const Padding(
        padding: EdgeInsets.symmetric(vertical: 32),
        child: CenteredLoader(),
      ),
      error: (e, _) => ErrorRetry(
        message: 'No se pudo cargar tu expediente',
        onRetry: () => ref.invalidate(myCvProvider),
      ),
      data: (cv) {
        if (cv.entries.isEmpty) {
          return const Card(
            child: Padding(
              padding: EdgeInsets.all(24),
              child: EmptyState(
                icon: Icons.folder_open_outlined,
                title: 'Sin empleos registrados',
              ),
            ),
          );
        }
        return Column(
          children: [
            _CvSummaryRow(cv: cv),
            const SizedBox(height: 12),
            for (final entry in cv.entries) _CvEntryCard(entry: entry),
          ],
        );
      },
    );
  }
}

class _CvSummaryRow extends StatelessWidget {
  const _CvSummaryRow({required this.cv});
  final CvView cv;

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        _Stat(label: 'Empresas', value: '${cv.totalCompanies}'),
        _Stat(label: 'Activas', value: '${cv.activeCompanies}'),
        _Stat(label: 'Días', value: '${cv.totalDays}'),
      ],
    );
  }
}

class _Stat extends StatelessWidget {
  const _Stat({required this.label, required this.value});
  final String label;
  final String value;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Expanded(
      child: Card(
        child: Padding(
          padding: const EdgeInsets.symmetric(vertical: 14),
          child: Column(
            children: [
              Text(value,
                  style: theme.textTheme.headlineSmall?.copyWith(
                      fontWeight: FontWeight.w700, color: Brand.blueViolet)),
              const SizedBox(height: 2),
              Text(label, style: theme.textTheme.bodySmall),
            ],
          ),
        ),
      ),
    );
  }
}

class _CvEntryCard extends StatelessWidget {
  const _CvEntryCard({required this.entry});
  final CvEntry entry;

  @override
  Widget build(BuildContext context) {
    final e = entry.employment;
    final s = entry.summary;
    final theme = Theme.of(context);
    return Card(
      margin: const EdgeInsets.only(bottom: 12),
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Expanded(
                  child: Text(
                    e.jobTitle.isEmpty ? 'Puesto' : e.jobTitle,
                    style: theme.textTheme.titleMedium
                        ?.copyWith(fontWeight: FontWeight.w600),
                  ),
                ),
                _StatusPill(active: e.isActive),
              ],
            ),
            const SizedBox(height: 2),
            Text(e.companyName,
                style: theme.textTheme.bodyMedium
                    ?.copyWith(color: theme.colorScheme.onSurfaceVariant)),
            const SizedBox(height: 6),
            Text(
              '${formatDateShort(e.startedAt)} — ${e.isActive ? 'Actualidad' : formatDateShort(e.endedAt)}',
              style: theme.textTheme.bodySmall,
            ),
            if (e.managerName.isNotEmpty) ...[
              const SizedBox(height: 4),
              Text('Manager: ${e.managerName}',
                  style: theme.textTheme.bodySmall),
            ],
            const Divider(height: 20),
            Wrap(
              spacing: 16,
              runSpacing: 8,
              children: [
                _MiniStat(
                    label: 'Horas aprob.',
                    value: formatHours(s.approvedHours)),
                _MiniStat(
                    label: 'Tareas',
                    value: '${s.tasksCompleted}/${s.tasksAssigned}'),
                _MiniStat(label: 'Ausencias', value: '${s.absences}'),
              ],
            ),
          ],
        ),
      ),
    );
  }
}

class _StatusPill extends StatelessWidget {
  const _StatusPill({required this.active});
  final bool active;

  @override
  Widget build(BuildContext context) {
    final color = active ? Brand.success : Theme.of(context).colorScheme.outline;
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.12),
        borderRadius: BorderRadius.circular(20),
      ),
      child: Text(active ? 'Activo' : 'Finalizado',
          style: TextStyle(
              color: color, fontSize: 12, fontWeight: FontWeight.w600)),
    );
  }
}

class _MiniStat extends StatelessWidget {
  const _MiniStat({required this.label, required this.value});
  final String label;
  final String value;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(value,
            style: theme.textTheme.titleSmall
                ?.copyWith(fontWeight: FontWeight.w700)),
        Text(label, style: theme.textTheme.bodySmall),
      ],
    );
  }
}

/// Interruptor para activar/desactivar el inicio de sesión con huella.
/// Solo se muestra si el dispositivo tiene biometría configurada.
class _BiometricSetting extends ConsumerWidget {
  const _BiometricSetting();

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final available = ref.watch(biometricAvailableProvider).maybeWhen(
          data: (v) => v,
          orElse: () => false,
        );
    if (!available) return const SizedBox.shrink();

    final enabled = ref.watch(
        authControllerProvider.select((s) => s.biometricEnabled));

    return Card(
      child: SwitchListTile(
        secondary: const Icon(Icons.fingerprint, color: Brand.blueViolet),
        title: const Text('Iniciar sesión con huella'),
        subtitle: Text(enabled
            ? 'Activado en este dispositivo'
            : 'Entra más rápido sin escribir tu contraseña'),
        value: enabled,
        onChanged: (want) async {
          final notifier = ref.read(authControllerProvider.notifier);
          if (want) {
            final ok = await notifier.enableBiometrics();
            if (!ok && context.mounted) {
              ScaffoldMessenger.of(context).showSnackBar(const SnackBar(
                  content: Text('No se pudo verificar la huella')));
            }
          } else {
            await notifier.disableBiometrics();
          }
        },
      ),
    );
  }
}
