import 'json_utils.dart';

enum WorkType {
  complete,
  absence,
  recover;

  static WorkType fromString(String? v) {
    switch (v) {
      case 'absence':
        return WorkType.absence;
      case 'recover':
        return WorkType.recover;
      case 'complete':
      default:
        return WorkType.complete;
    }
  }

  String get wire => switch (this) {
        WorkType.complete => 'complete',
        WorkType.absence => 'absence',
        WorkType.recover => 'recover',
      };

  String get label => switch (this) {
        WorkType.complete => 'Jornada completa',
        WorkType.absence => 'Ausencia',
        WorkType.recover => 'Recuperación',
      };
}

/// El estado no es un campo único: se deriva de `approved` / `rejected`.
enum WorkHourStatus {
  pending,
  approved,
  rejected;

  String get label => switch (this) {
        WorkHourStatus.pending => 'Pendiente',
        WorkHourStatus.approved => 'Aprobada',
        WorkHourStatus.rejected => 'Rechazada',
      };
}

class WorkHour {
  WorkHour({
    required this.id,
    required this.workDate,
    required this.workType,
    required this.hoursWorked,
    required this.approved,
    required this.rejected,
    this.activities = '',
    this.comments = '',
    this.absenceReason = '',
    this.rejectionReason = '',
    this.approvedAt,
    this.rejectedAt,
  });

  final int id;
  final DateTime? workDate;
  final WorkType workType;
  final double hoursWorked;
  final bool approved;
  final bool rejected;
  final String activities;
  final String comments;
  final String absenceReason;
  final String rejectionReason;
  final DateTime? approvedAt;
  final DateTime? rejectedAt;

  WorkHourStatus get status {
    if (approved) return WorkHourStatus.approved;
    if (rejected) return WorkHourStatus.rejected;
    return WorkHourStatus.pending;
  }

  factory WorkHour.fromJson(Map<String, dynamic> j) => WorkHour(
        id: asInt(j['id']),
        workDate: parseDate(j['work_date']),
        workType: WorkType.fromString(j['work_type'] as String?),
        hoursWorked: asDouble(j['hours_worked']),
        approved: asBool(j['approved']),
        rejected: asBool(j['rejected']),
        activities: asString(j['activities']),
        comments: asString(j['comments']),
        absenceReason: asString(j['absence_reason']),
        rejectionReason: asString(j['rejection_reason']),
        approvedAt: parseDate(j['approved_at']),
        rejectedAt: parseDate(j['rejected_at']),
      );
}

/// Resumen mensual: `{ total_hours, approved_hours, pending_hours, rejected_hours }`.
class WorkHourSummary {
  WorkHourSummary({
    required this.total,
    required this.approved,
    required this.pending,
    required this.rejected,
  });

  final double total;
  final double approved;
  final double pending;
  final double rejected;

  factory WorkHourSummary.fromJson(Map<String, dynamic> j) => WorkHourSummary(
        total: asDouble(j['total_hours']),
        approved: asDouble(j['approved_hours']),
        pending: asDouble(j['pending_hours']),
        rejected: asDouble(j['rejected_hours']),
      );
}
