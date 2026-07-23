import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/api_client.dart';
import '../../core/providers.dart';
import '../../models/paginated.dart';
import '../../models/task.dart';

class TasksRepository {
  TasksRepository(this._api);
  final ApiClient _api;

  /// GET /api/tasks (paginado). Se puede filtrar por assignee_id / status.
  Future<Paginated<Task>> list({
    int page = 1,
    int limit = 20,
    int? assigneeId,
    TaskStatus? status,
  }) async {
    final query = <String, dynamic>{'page': page, 'limit': limit};
    if (assigneeId != null) query['assignee_id'] = assigneeId;
    if (status != null && status != TaskStatus.unknown) {
      query['status'] = status.wire;
    }
    final resp = await _api.get('/tasks', query: query);
    final data = resp.data;
    if (resp.statusCode == 200 && data is Map<String, dynamic>) {
      return Paginated.fromJson(data, Task.fromJson);
    }
    final msg = (data is Map && data['error'] is String)
        ? data['error'] as String
        : 'No se pudieron cargar las tareas';
    throw Exception(msg);
  }

  /// PUT /api/tasks/:id — cambia el estado (requiere permiso "edit" en tasks).
  Future<Task> updateStatus(int id, TaskStatus status) async {
    final resp = await _api.put('/tasks/$id', data: {'status': status.wire});
    final data = resp.data;
    if ((resp.statusCode == 200) && data is Map<String, dynamic>) {
      return Task.fromJson(data);
    }
    final msg = (data is Map && data['error'] is String)
        ? data['error'] as String
        : 'No se pudo actualizar la tarea';
    throw Exception(msg);
  }
}

final tasksRepositoryProvider = Provider<TasksRepository>((ref) {
  return TasksRepository(ref.watch(apiClientProvider));
});

/// Filtro de tareas: null = todas las visibles; con id = solo mías.
class TasksFilter {
  const TasksFilter({this.assigneeId, this.status});
  final int? assigneeId;
  final TaskStatus? status;

  TasksFilter copyWith({int? assigneeId, TaskStatus? status, bool clearStatus = false}) {
    return TasksFilter(
      assigneeId: assigneeId ?? this.assigneeId,
      status: clearStatus ? null : (status ?? this.status),
    );
  }
}

final tasksFilterProvider = StateProvider<TasksFilter>((ref) {
  return const TasksFilter();
});

final tasksListProvider =
    FutureProvider.autoDispose<Paginated<Task>>((ref) {
  final filter = ref.watch(tasksFilterProvider);
  return ref.watch(tasksRepositoryProvider).list(
        assigneeId: filter.assigneeId,
        status: filter.status,
      );
});
