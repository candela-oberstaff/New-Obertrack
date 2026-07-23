import 'json_utils.dart';

/// Métricas globales del panel (GET /api/admin/dashboard) — superadmin / CS.
class AdminMetrics {
  AdminMetrics({
    required this.totalUsers,
    required this.activeUsers,
    required this.totalCompanies,
    required this.totalProfessionals,
    required this.totalManagers,
    required this.totalHoursWorked,
    required this.approvedHours,
    required this.pendingHours,
    required this.totalTasks,
    required this.totalBoards,
    required this.activeToday,
    required this.inactiveWarning,
  });

  final int totalUsers;
  final int activeUsers;
  final int totalCompanies;
  final int totalProfessionals;
  final int totalManagers;
  final double totalHoursWorked;
  final double approvedHours;
  final double pendingHours;
  final int totalTasks;
  final int totalBoards;
  final int activeToday;
  final int inactiveWarning;

  factory AdminMetrics.fromJson(Map<String, dynamic> j) => AdminMetrics(
        totalUsers: asInt(j['total_users']),
        activeUsers: asInt(j['active_users']),
        totalCompanies: asInt(j['total_companies']),
        totalProfessionals: asInt(j['total_professionals']),
        totalManagers: asInt(j['total_managers']),
        totalHoursWorked: asDouble(j['total_hours_worked']),
        approvedHours: asDouble(j['approved_hours']),
        pendingHours: asDouble(j['pending_hours']),
        totalTasks: asInt(j['total_tasks']),
        totalBoards: asInt(j['total_boards']),
        activeToday: asInt(j['active_today']),
        inactiveWarning: asInt(j['inactive_warning']),
      );
}

/// Item de actividad reciente (GET /api/admin/recent-activity).
class ActivityItem {
  ActivityItem({
    required this.type,
    required this.user,
    required this.company,
    required this.details,
    this.timestamp,
  });

  final String type;
  final String user;
  final String company;
  final String details;
  final DateTime? timestamp;

  factory ActivityItem.fromJson(Map<String, dynamic> j) => ActivityItem(
        type: asString(j['type']),
        user: asString(j['user']),
        company: asString(j['company']),
        details: asString(j['details']),
        timestamp: parseDate(j['timestamp']),
      );
}

/// Métrica por empresa (GET /api/admin/companies).
class CompanyMetric {
  CompanyMetric({
    required this.id,
    required this.name,
    required this.professionals,
    required this.hoursThisMonth,
    required this.tasksCompleted,
    required this.activeUsers,
  });

  final int id;
  final String name;
  final int professionals;
  final double hoursThisMonth;
  final int tasksCompleted;
  final int activeUsers;

  factory CompanyMetric.fromJson(Map<String, dynamic> j) => CompanyMetric(
        id: asInt(j['id']),
        name: asString(j['name']),
        professionals: asInt(j['professionals']),
        hoursThisMonth: asDouble(j['hours_this_month']),
        tasksCompleted: asInt(j['tasks_completed']),
        activeUsers: asInt(j['active_users']),
      );
}

/// Totales de tareas por estado, agregados desde /api/tasks/status-counts
/// (que viene anidado por tablero: {boardId: {estado: n}}).
class TaskTotals {
  TaskTotals({this.porHacer = 0, this.enProceso = 0, this.finalizado = 0});

  final int porHacer;
  final int enProceso;
  final int finalizado;

  int get total => porHacer + enProceso + finalizado;

  /// Suma los conteos de todos los tableros.
  factory TaskTotals.fromStatusCounts(Map<String, dynamic> byBoard) {
    var todo = 0, doing = 0, done = 0;
    for (final entry in byBoard.values) {
      if (entry is Map) {
        todo += asInt(entry['por_hacer']);
        doing += asInt(entry['en_proceso']);
        done += asInt(entry['finalizado']);
      }
    }
    return TaskTotals(porHacer: todo, enProceso: doing, finalizado: done);
  }
}
