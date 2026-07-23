import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/formatting.dart';
import '../../core/theme.dart';
import '../../models/dashboard.dart';
import '../../models/user.dart';
import '../../models/work_hour.dart';
import '../../widgets/async_views.dart';
import '../auth/auth_controller.dart';
import '../notifications/notifications_repository.dart';
import '../notifications/notifications_screen.dart';
import 'home_repository.dart';

class HomeScreen extends ConsumerWidget {
  const HomeScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final user = ref.watch(currentUserProvider);
    final async = ref.watch(homeDataProvider);
    if (user == null) return const CenteredLoader();

    return Scaffold(
      body: RefreshIndicator(
        onRefresh: () async {
          ref.invalidate(homeDataProvider);
          await ref.read(homeDataProvider.future);
        },
        child: CustomScrollView(
          slivers: [
            SliverToBoxAdapter(child: _GreetingHeader(user: user)),
            SliverPadding(
              padding: const EdgeInsets.fromLTRB(16, 4, 16, 32),
              sliver: async.when(
                loading: () => const SliverToBoxAdapter(
                  child: Padding(
                    padding: EdgeInsets.only(top: 60),
                    child: CenteredLoader(),
                  ),
                ),
                error: (e, _) => SliverToBoxAdapter(
                  child: Padding(
                    padding: const EdgeInsets.only(top: 40),
                    child: ErrorRetry(
                      message: 'No se pudo cargar el panel',
                      onRetry: () => ref.invalidate(homeDataProvider),
                    ),
                  ),
                ),
                data: (d) => SliverToBoxAdapter(
                  child: _DashboardBody(user: user, data: d),
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _GreetingHeader extends StatelessWidget {
  const _GreetingHeader({required this.user});
  final User user;

  @override
  Widget build(BuildContext context) {
    final firstName = user.name.trim().split(RegExp(r'\s+')).first;
    final company = user.companyName;
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.fromLTRB(20, 24, 20, 28),
      decoration: const BoxDecoration(
        gradient: LinearGradient(
          colors: [Brand.indigo, Brand.blueViolet],
          begin: Alignment.topLeft,
          end: Alignment.bottomRight,
        ),
        borderRadius: BorderRadius.vertical(bottom: Radius.circular(24)),
      ),
      child: SafeArea(
        bottom: false,
        child: Row(
          children: [
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text('Hola,',
                      style: TextStyle(
                          color: Colors.white.withValues(alpha: 0.85),
                          fontSize: 15)),
                  Text(
                    firstName,
                    style: const TextStyle(
                        color: Colors.white,
                        fontSize: 26,
                        fontWeight: FontWeight.w700),
                  ),
                  const SizedBox(height: 4),
                  Text(
                    company.isNotEmpty
                        ? '${user.userType.label} · $company'
                        : user.userType.label,
                    style: TextStyle(
                        color: Colors.white.withValues(alpha: 0.85),
                        fontSize: 13),
                  ),
                ],
              ),
            ),
            const _NotificationsBell(),
            const SizedBox(width: 4),
            CircleAvatar(
              radius: 26,
              backgroundColor: Colors.white.withValues(alpha: 0.2),
              child: Text(user.initials,
                  style: const TextStyle(
                      color: Colors.white,
                      fontWeight: FontWeight.w700,
                      fontSize: 18)),
            ),
          ],
        ),
      ),
    );
  }
}

/// Campana de notificaciones (esquina superior derecha del Home) con contador.
class _NotificationsBell extends ConsumerWidget {
  const _NotificationsBell();

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final unread = ref.watch(unreadCountProvider).maybeWhen(
          data: (c) => c,
          orElse: () => 0,
        );
    return IconButton(
      tooltip: 'Notificaciones',
      onPressed: () {
        Navigator.of(context).push(MaterialPageRoute(
          builder: (_) => const NotificationsScreen(),
        ));
      },
      icon: Badge(
        isLabelVisible: unread > 0,
        label: Text('$unread'),
        child: const Icon(Icons.notifications_none_rounded,
            color: Colors.white, size: 28),
      ),
    );
  }
}

class _DashboardBody extends StatelessWidget {
  const _DashboardBody({required this.user, required this.data});
  final User user;
  final HomeData data;

  bool get _isAdmin =>
      user.isSuperadmin ||
      user.userType == UserType.superadmin ||
      user.userType == UserType.customerSuccess;

  @override
  Widget build(BuildContext context) {
    if (_isAdmin) {
      if (data.adminMetrics != null) return _AdminDashboard(data: data);
      // Es admin pero no se pudieron cargar las métricas (p.ej. token expirado):
      // mostramos reintentar en lugar de caer al panel de miembro.
      return const _AdminLoadError();
    }
    return _MemberDashboard(user: user, data: data);
  }
}

class _AdminLoadError extends ConsumerWidget {
  const _AdminLoadError();

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return Padding(
      padding: const EdgeInsets.only(top: 60),
      child: ErrorRetry(
        message: 'No se pudieron cargar las métricas del panel',
        onRetry: () => ref.invalidate(homeDataProvider),
      ),
    );
  }
}

// ---------------------------------------------------------------------------
// Superadmin / Customer Success
// ---------------------------------------------------------------------------

class _AdminDashboard extends StatelessWidget {
  const _AdminDashboard({required this.data});
  final HomeData data;

  @override
  Widget build(BuildContext context) {
    final m = data.adminMetrics!;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        if (m.inactiveWarning > 0)
          _AlertBanner(
            text:
                '${m.inactiveWarning} usuario(s) con alerta de inactividad',
          ),
        const _SectionTitle('Resumen general'),
        _StatGrid(cards: [
          _StatData('Usuarios activos', '${m.activeUsers}',
              Icons.groups_2_outlined, Brand.blueViolet,
              hint: 'de ${m.totalUsers}'),
          _StatData('Empresas', '${m.totalCompanies}',
              Icons.apartment_outlined, Brand.azure),
          _StatData('Profesionales', '${m.totalProfessionals}',
              Icons.badge_outlined, Brand.orchid),
          _StatData('Managers', '${m.totalManagers}',
              Icons.supervisor_account_outlined, Brand.indigo),
          _StatData('Horas pendientes', formatHours(m.pendingHours),
              Icons.hourglass_bottom, Brand.warning),
          _StatData('Activos hoy', '${m.activeToday}',
              Icons.bolt_outlined, Brand.success),
        ]),
        const SizedBox(height: 8),
        _StatGrid(cards: [
          _StatData('Horas aprobadas', formatHours(m.approvedHours),
              Icons.verified_outlined, Brand.success),
          _StatData('Tareas', '${m.totalTasks}',
              Icons.task_alt_outlined, Brand.blueViolet,
              hint: '${m.totalBoards} tableros'),
        ]),
        if (data.companies.isNotEmpty) ...[
          const _SectionTitle('Empresas'),
          ...data.companies.take(5).map((c) => _CompanyTile(company: c)),
        ],
        if (data.recentActivity.isNotEmpty) ...[
          const _SectionTitle('Actividad reciente'),
          ...data.recentActivity.take(8).map((a) => _ActivityTile(item: a)),
        ],
      ],
    );
  }
}

// ---------------------------------------------------------------------------
// Empleador / Manager / Profesional
// ---------------------------------------------------------------------------

class _MemberDashboard extends StatelessWidget {
  const _MemberDashboard({required this.user, required this.data});
  final User user;
  final HomeData data;

  @override
  Widget build(BuildContext context) {
    final s = data.hoursSummary;
    final t = data.taskTotals;
    final cards = <_StatData>[];

    if (data.peopleCount != null) {
      cards.add(_StatData(
        data.peopleLabel ?? 'Personas',
        '${data.peopleCount}',
        Icons.groups_2_outlined,
        Brand.blueViolet,
      ));
    }
    if (data.pendingHoursCount != null) {
      cards.add(_StatData(
        'Horas por aprobar',
        '${data.pendingHoursCount}',
        Icons.pending_actions_outlined,
        Brand.warning,
      ));
    }
    if (s != null) {
      cards.add(_StatData('Horas del mes', formatHours(s.total),
          Icons.schedule_outlined, Brand.azure));
      cards.add(_StatData('Horas aprobadas', formatHours(s.approved),
          Icons.verified_outlined, Brand.success));
    }
    if (t != null) {
      cards.add(_StatData('Tareas activas', '${t.porHacer + t.enProceso}',
          Icons.task_alt_outlined, Brand.orchid,
          hint: '${t.finalizado} finalizadas'));
    }
    cards.add(_StatData('Avisos sin leer', '${data.unread}',
        Icons.notifications_none_rounded, Brand.indigo));

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        const _SectionTitle('Tu resumen'),
        _StatGrid(cards: cards),
        if (t != null && t.total > 0) ...[
          const _SectionTitle('Tareas por estado'),
          _TaskBreakdown(totals: t),
        ],
        if (s != null) ...[
          const _SectionTitle('Horas del mes'),
          _HoursBreakdown(summary: s),
        ],
      ],
    );
  }
}

// ---------------------------------------------------------------------------
// Widgets compartidos
// ---------------------------------------------------------------------------

class _SectionTitle extends StatelessWidget {
  const _SectionTitle(this.text);
  final String text;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(top: 20, bottom: 10),
      child: Text(text,
          style: Theme.of(context)
              .textTheme
              .titleMedium
              ?.copyWith(fontWeight: FontWeight.w700)),
    );
  }
}

class _StatData {
  _StatData(this.label, this.value, this.icon, this.color, {this.hint});
  final String label;
  final String value;
  final IconData icon;
  final Color color;
  final String? hint;
}

class _StatGrid extends StatelessWidget {
  const _StatGrid({required this.cards});
  final List<_StatData> cards;

  @override
  Widget build(BuildContext context) {
    return LayoutBuilder(builder: (context, constraints) {
      const gap = 12.0;
      final w = (constraints.maxWidth - gap) / 2;
      return Wrap(
        spacing: gap,
        runSpacing: gap,
        children: [
          for (final c in cards) SizedBox(width: w, child: _StatCard(data: c)),
        ],
      );
    });
  }
}

class _StatCard extends StatelessWidget {
  const _StatCard({required this.data});
  final _StatData data;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(14),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Container(
              width: 38,
              height: 38,
              decoration: BoxDecoration(
                color: data.color.withValues(alpha: 0.14),
                borderRadius: BorderRadius.circular(10),
              ),
              child: Icon(data.icon, color: data.color, size: 22),
            ),
            const SizedBox(height: 12),
            Text(data.value,
                style: theme.textTheme.headlineSmall
                    ?.copyWith(fontWeight: FontWeight.w700)),
            const SizedBox(height: 2),
            Text(data.label,
                style: theme.textTheme.bodySmall?.copyWith(
                    color: theme.colorScheme.onSurfaceVariant)),
            if (data.hint != null)
              Text(data.hint!,
                  style: theme.textTheme.bodySmall?.copyWith(
                      color: theme.colorScheme.outline, fontSize: 11)),
          ],
        ),
      ),
    );
  }
}

class _AlertBanner extends StatelessWidget {
  const _AlertBanner({required this.text});
  final String text;

  @override
  Widget build(BuildContext context) {
    return Container(
      margin: const EdgeInsets.only(top: 16),
      padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
      decoration: BoxDecoration(
        color: Brand.warning.withValues(alpha: 0.12),
        borderRadius: BorderRadius.circular(12),
      ),
      child: Row(
        children: [
          const Icon(Icons.warning_amber_rounded,
              color: Brand.warning, size: 22),
          const SizedBox(width: 10),
          Expanded(
              child: Text(text,
                  style: const TextStyle(
                      color: Brand.warning, fontWeight: FontWeight.w600))),
        ],
      ),
    );
  }
}

class _TaskBreakdown extends StatelessWidget {
  const _TaskBreakdown({required this.totals});
  final TaskTotals totals;

  @override
  Widget build(BuildContext context) {
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          children: [
            _bar(context, 'Por hacer', totals.porHacer, totals.total,
                Theme.of(context).colorScheme.outline),
            const SizedBox(height: 12),
            _bar(context, 'En proceso', totals.enProceso, totals.total,
                Brand.azure),
            const SizedBox(height: 12),
            _bar(context, 'Finalizado', totals.finalizado, totals.total,
                Brand.success),
          ],
        ),
      ),
    );
  }

  Widget _bar(BuildContext context, String label, int value, int total,
      Color color) {
    final ratio = total == 0 ? 0.0 : value / total;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          children: [
            Expanded(
                child: Text(label,
                    style: Theme.of(context).textTheme.bodyMedium)),
            Text('$value',
                style: Theme.of(context)
                    .textTheme
                    .bodyMedium
                    ?.copyWith(fontWeight: FontWeight.w700)),
          ],
        ),
        const SizedBox(height: 6),
        ClipRRect(
          borderRadius: BorderRadius.circular(6),
          child: LinearProgressIndicator(
            value: ratio,
            minHeight: 8,
            backgroundColor: color.withValues(alpha: 0.15),
            valueColor: AlwaysStoppedAnimation(color),
          ),
        ),
      ],
    );
  }
}

class _HoursBreakdown extends StatelessWidget {
  const _HoursBreakdown({required this.summary});
  final WorkHourSummary summary;

  @override
  Widget build(BuildContext context) {
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Row(
          children: [
            _cell(context, 'Total', summary.total, Brand.blueViolet),
            _cell(context, 'Aprobadas', summary.approved, Brand.success),
            _cell(context, 'Pendientes', summary.pending, Brand.warning),
            _cell(context, 'Rechazadas', summary.rejected, Brand.danger),
          ],
        ),
      ),
    );
  }

  Widget _cell(BuildContext context, String label, double value, Color color) {
    return Expanded(
      child: Column(
        children: [
          Text(formatHours(value),
              style: Theme.of(context)
                  .textTheme
                  .titleLarge
                  ?.copyWith(fontWeight: FontWeight.w700, color: color)),
          const SizedBox(height: 2),
          Text(label,
              textAlign: TextAlign.center,
              style: Theme.of(context).textTheme.bodySmall),
        ],
      ),
    );
  }
}

class _CompanyTile extends StatelessWidget {
  const _CompanyTile({required this.company});
  final CompanyMetric company;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Card(
      margin: const EdgeInsets.only(bottom: 8),
      child: ListTile(
        leading: CircleAvatar(
          backgroundColor: Brand.blueViolet.withValues(alpha: 0.14),
          child: const Icon(Icons.apartment_outlined, color: Brand.blueViolet),
        ),
        title: Text(company.name,
            style: const TextStyle(fontWeight: FontWeight.w600)),
        subtitle: Text(
            '${company.professionals} prof. · ${company.activeUsers} activos'),
        trailing: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          crossAxisAlignment: CrossAxisAlignment.end,
          children: [
            Text(formatHours(company.hoursThisMonth),
                style: theme.textTheme.titleSmall
                    ?.copyWith(fontWeight: FontWeight.w700)),
            Text('horas/mes', style: theme.textTheme.bodySmall),
          ],
        ),
      ),
    );
  }
}

class _ActivityTile extends StatelessWidget {
  const _ActivityTile({required this.item});
  final ActivityItem item;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 8),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Container(
            margin: const EdgeInsets.only(top: 4),
            width: 8,
            height: 8,
            decoration: const BoxDecoration(
                color: Brand.blueViolet, shape: BoxShape.circle),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  item.details.isNotEmpty ? item.details : item.type,
                  style: theme.textTheme.bodyMedium
                      ?.copyWith(fontWeight: FontWeight.w500),
                ),
                const SizedBox(height: 2),
                Text(
                  [
                    if (item.user.isNotEmpty) item.user,
                    if (item.company.isNotEmpty) item.company,
                    formatRelative(item.timestamp),
                  ].where((s) => s.isNotEmpty).join(' · '),
                  style: theme.textTheme.bodySmall?.copyWith(
                      color: theme.colorScheme.onSurfaceVariant),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}
