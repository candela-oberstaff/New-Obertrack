import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:intl/intl.dart';

import '../../core/api_client.dart';
import '../../core/providers.dart';
import '../../models/paginated.dart';
import '../../models/work_hour.dart';

final _apiDate = DateFormat('yyyy-MM-dd');

class WorkHoursRepository {
  WorkHoursRepository(this._api);
  final ApiClient _api;

  /// GET /api/work-hours (paginado, más recientes primero según el backend).
  Future<Paginated<WorkHour>> list({int page = 1, int limit = 30}) async {
    final resp =
        await _api.get('/work-hours', query: {'page': page, 'limit': limit});
    final data = resp.data;
    if (resp.statusCode == 200 && data is Map<String, dynamic>) {
      return Paginated.fromJson(data, WorkHour.fromJson);
    }
    final msg = (data is Map && data['error'] is String)
        ? data['error'] as String
        : 'No se pudieron cargar los registros';
    throw Exception(msg);
  }

  /// GET /api/work-hours/summary → resumen del mes en curso.
  Future<WorkHourSummary> summary() async {
    final resp = await _api.get('/work-hours/summary');
    final data = resp.data;
    if (resp.statusCode == 200 && data is Map<String, dynamic>) {
      return WorkHourSummary.fromJson(data);
    }
    return WorkHourSummary(total: 0, approved: 0, pending: 0, rejected: 0);
  }

  /// POST /api/work-hours. El backend calcula las horas para complete/absence;
  /// solo se respeta hours_worked en recover y absence_hours en absence.
  Future<WorkHour> create({
    required DateTime workDate,
    required WorkType workType,
    String activities = '',
    String comments = '',
    String absenceReason = '',
    double? hoursWorked,
    double? absenceHours,
  }) async {
    final body = <String, dynamic>{
      'work_date': _apiDate.format(workDate),
      'work_type': workType.wire,
      'activities': activities,
      'comments': comments,
    };
    if (workType == WorkType.recover && hoursWorked != null) {
      body['hours_worked'] = hoursWorked;
    }
    if (workType == WorkType.absence) {
      body['absence_reason'] = absenceReason;
      if (absenceHours != null) body['absence_hours'] = absenceHours;
    }

    final resp = await _api.post('/work-hours', data: body);
    final data = resp.data;
    if ((resp.statusCode == 201 || resp.statusCode == 200) &&
        data is Map<String, dynamic>) {
      return WorkHour.fromJson(data);
    }
    final msg = (data is Map && data['error'] is String)
        ? data['error'] as String
        : 'No se pudo registrar la jornada';
    throw Exception(msg);
  }
}

final workHoursRepositoryProvider = Provider<WorkHoursRepository>((ref) {
  return WorkHoursRepository(ref.watch(apiClientProvider));
});

final workHoursListProvider =
    FutureProvider.autoDispose<Paginated<WorkHour>>((ref) {
  return ref.watch(workHoursRepositoryProvider).list();
});

final workHoursSummaryProvider =
    FutureProvider.autoDispose<WorkHourSummary>((ref) {
  return ref.watch(workHoursRepositoryProvider).summary();
});
