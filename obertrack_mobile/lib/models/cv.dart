import 'json_utils.dart';

/// Vista del CV vivo del profesional (GET /api/me/cv).
class CvView {
  CvView({
    required this.entries,
    required this.totalCompanies,
    required this.activeCompanies,
    required this.totalDays,
  });

  final List<CvEntry> entries;
  final int totalCompanies;
  final int activeCompanies;
  final int totalDays;

  factory CvView.fromJson(Map<String, dynamic> j) {
    final rawEntries = (j['entries'] as List?) ?? const [];
    return CvView(
      entries: rawEntries
          .whereType<Map<String, dynamic>>()
          .map(CvEntry.fromJson)
          .toList(),
      totalCompanies: asInt(j['total_companies']),
      activeCompanies: asInt(j['active_companies']),
      totalDays: asInt(j['total_days']),
    );
  }
}

class CvEntry {
  CvEntry({required this.employment, required this.summary});

  final EmploymentView employment;
  final ExpedienteSummary summary;

  factory CvEntry.fromJson(Map<String, dynamic> j) => CvEntry(
        employment: EmploymentView.fromJson(
            (j['employment'] as Map<String, dynamic>?) ?? const {}),
        summary: ExpedienteSummary.fromJson(
            (j['summary'] as Map<String, dynamic>?) ?? const {}),
      );
}

class EmploymentView {
  EmploymentView({
    required this.id,
    required this.jobTitle,
    required this.status,
    required this.companyName,
    required this.managerName,
    this.startedAt,
    this.endedAt,
    this.endReason = '',
  });

  final int id;
  final String jobTitle;
  final String status; // "active" | "ended"
  final String companyName;
  final String managerName;
  final DateTime? startedAt;
  final DateTime? endedAt;
  final String endReason;

  bool get isActive => status == 'active';

  factory EmploymentView.fromJson(Map<String, dynamic> j) => EmploymentView(
        id: asInt(j['id']),
        jobTitle: asString(j['job_title']),
        status: asString(j['status']),
        companyName: asString(j['company_name']),
        managerName: asString(j['manager_name']),
        startedAt: parseDate(j['started_at']),
        endedAt: parseDate(j['ended_at']),
        endReason: asString(j['end_reason']),
      );
}

class ExpedienteSummary {
  ExpedienteSummary({
    required this.daysEmployed,
    required this.totalHours,
    required this.approvedHours,
    required this.pendingHours,
    required this.tasksAssigned,
    required this.tasksCompleted,
    required this.absences,
  });

  final int daysEmployed;
  final double totalHours;
  final double approvedHours;
  final double pendingHours;
  final int tasksAssigned;
  final int tasksCompleted;
  final int absences;

  factory ExpedienteSummary.fromJson(Map<String, dynamic> j) =>
      ExpedienteSummary(
        daysEmployed: asInt(j['days_employed']),
        totalHours: asDouble(j['total_hours']),
        approvedHours: asDouble(j['approved_hours']),
        pendingHours: asDouble(j['pending_hours']),
        tasksAssigned: asInt(j['tasks_assigned']),
        tasksCompleted: asInt(j['tasks_completed']),
        absences: asInt(j['absences']),
      );
}
