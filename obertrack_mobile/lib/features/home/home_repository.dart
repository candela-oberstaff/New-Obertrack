import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/api_client.dart';
import '../../core/providers.dart';
import '../../models/dashboard.dart';
import '../../models/user.dart';
import '../../models/work_hour.dart';
import '../auth/auth_controller.dart';

/// Datos del Home. Todas las secciones son opcionales: la vista muestra lo que
/// haya podido cargar según el rol y los permisos.
class HomeData {
  HomeData({
    this.adminMetrics,
    this.recentActivity = const [],
    this.companies = const [],
    this.peopleCount,
    this.peopleLabel,
    this.pendingHoursCount,
    this.hoursSummary,
    this.taskTotals,
    this.unread = 0,
  });

  final AdminMetrics? adminMetrics;
  final List<ActivityItem> recentActivity;
  final List<CompanyMetric> companies;
  final int? peopleCount;
  final String? peopleLabel;
  final int? pendingHoursCount;
  final WorkHourSummary? hoursSummary;
  final TaskTotals? taskTotals;
  final int unread;
}

class HomeRepository {
  HomeRepository(this._api);
  final ApiClient _api;

  Future<T?> _safe<T>(Future<T> Function() f) async {
    try {
      return await f();
    } catch (_) {
      return null;
    }
  }

  Future<AdminMetrics?> _adminDashboard() async {
    final r = await _api.get('/admin/dashboard');
    if (r.statusCode == 200 && r.data is Map<String, dynamic>) {
      return AdminMetrics.fromJson(r.data as Map<String, dynamic>);
    }
    return null;
  }

  Future<List<ActivityItem>> _recentActivity() async {
    final r = await _api.get('/admin/recent-activity');
    if (r.statusCode == 200 && r.data is List) {
      return (r.data as List)
          .whereType<Map<String, dynamic>>()
          .map(ActivityItem.fromJson)
          .toList();
    }
    return const [];
  }

  Future<List<CompanyMetric>> _companies() async {
    final r = await _api.get('/admin/companies');
    if (r.statusCode == 200 && r.data is List) {
      return (r.data as List)
          .whereType<Map<String, dynamic>>()
          .map(CompanyMetric.fromJson)
          .toList();
    }
    return const [];
  }

  Future<int?> _arrayCount(String path) async {
    final r = await _api.get(path);
    if (r.statusCode == 200 && r.data is List) return (r.data as List).length;
    return null;
  }

  Future<WorkHourSummary?> _hoursSummary() async {
    final r = await _api.get('/work-hours/summary');
    if (r.statusCode == 200 && r.data is Map<String, dynamic>) {
      return WorkHourSummary.fromJson(r.data as Map<String, dynamic>);
    }
    return null;
  }

  Future<TaskTotals?> _taskTotals() async {
    final r = await _api.get('/tasks/status-counts');
    if (r.statusCode == 200 && r.data is Map<String, dynamic>) {
      return TaskTotals.fromStatusCounts(r.data as Map<String, dynamic>);
    }
    return null;
  }

  Future<int> _unread() async {
    final r = await _api.get('/notifications/unread-count');
    if (r.statusCode == 200 && r.data is Map && r.data['count'] is num) {
      return (r.data['count'] as num).toInt();
    }
    return 0;
  }

  /// Carga el Home adaptado al rol del usuario.
  Future<HomeData> load(User user) async {
    final isAdmin = user.isSuperadmin ||
        user.userType == UserType.superadmin ||
        user.userType == UserType.customerSuccess;
    final isEmployer = user.userType == UserType.empleador;

    if (isAdmin) {
      final results = await Future.wait([
        _safe(_adminDashboard),
        _safe(_recentActivity),
        _safe(_companies),
        _safe(_unread),
      ]);
      return HomeData(
        adminMetrics: results[0] as AdminMetrics?,
        recentActivity: (results[1] as List<ActivityItem>?) ?? const [],
        companies: (results[2] as List<CompanyMetric>?) ?? const [],
        unread: (results[3] as int?) ?? 0,
      );
    }

    if (isEmployer || user.isManager) {
      final peoplePath =
          isEmployer ? '/users/employees' : '/users/my-team';
      final results = await Future.wait([
        _safe(() => _arrayCount(peoplePath)),
        _safe(() => _arrayCount('/work-hours/pending')),
        _safe(_hoursSummary),
        _safe(_taskTotals),
        _safe(_unread),
      ]);
      return HomeData(
        peopleCount: results[0] as int?,
        peopleLabel: isEmployer ? 'Empleados' : 'Equipo',
        pendingHoursCount: results[1] as int?,
        hoursSummary: results[2] as WorkHourSummary?,
        taskTotals: results[3] as TaskTotals?,
        unread: (results[4] as int?) ?? 0,
      );
    }

    // Profesional (u otros): resumen personal.
    final results = await Future.wait([
      _safe(_hoursSummary),
      _safe(_taskTotals),
      _safe(_unread),
    ]);
    return HomeData(
      hoursSummary: results[0] as WorkHourSummary?,
      taskTotals: results[1] as TaskTotals?,
      unread: (results[2] as int?) ?? 0,
    );
  }
}

final homeRepositoryProvider = Provider<HomeRepository>((ref) {
  return HomeRepository(ref.watch(apiClientProvider));
});

final homeDataProvider = FutureProvider.autoDispose<HomeData>((ref) async {
  final user = ref.watch(currentUserProvider);
  if (user == null) return HomeData();
  return ref.watch(homeRepositoryProvider).load(user);
});
