import 'json_utils.dart';
import 'user.dart';

enum TaskStatus {
  porHacer,
  enProceso,
  finalizado,
  unknown;

  static TaskStatus fromString(String? v) {
    switch (v) {
      case 'por_hacer':
        return TaskStatus.porHacer;
      case 'en_proceso':
        return TaskStatus.enProceso;
      case 'finalizado':
        return TaskStatus.finalizado;
      default:
        return TaskStatus.unknown;
    }
  }

  String get wire {
    switch (this) {
      case TaskStatus.porHacer:
        return 'por_hacer';
      case TaskStatus.enProceso:
        return 'en_proceso';
      case TaskStatus.finalizado:
        return 'finalizado';
      case TaskStatus.unknown:
        return 'por_hacer';
    }
  }

  String get label {
    switch (this) {
      case TaskStatus.porHacer:
        return 'Por hacer';
      case TaskStatus.enProceso:
        return 'En proceso';
      case TaskStatus.finalizado:
        return 'Finalizado';
      case TaskStatus.unknown:
        return '—';
    }
  }
}

enum TaskPriority {
  low,
  medium,
  high,
  urgent;

  static TaskPriority fromString(String? v) {
    switch (v) {
      case 'low':
        return TaskPriority.low;
      case 'high':
        return TaskPriority.high;
      case 'urgent':
        return TaskPriority.urgent;
      case 'medium':
      default:
        return TaskPriority.medium;
    }
  }

  String get label {
    switch (this) {
      case TaskPriority.low:
        return 'Baja';
      case TaskPriority.medium:
        return 'Media';
      case TaskPriority.high:
        return 'Alta';
      case TaskPriority.urgent:
        return 'Urgente';
    }
  }
}

class Task {
  Task({
    required this.id,
    required this.title,
    required this.description,
    required this.status,
    required this.priority,
    required this.completed,
    this.boardId = 0,
    this.startDate,
    this.endDate,
    this.assignees = const [],
    this.creator,
    this.boardName = '',
    this.commentCount = 0,
  });

  final int id;
  final String title;
  final String description;
  final TaskStatus status;
  final TaskPriority priority;
  final bool completed;
  final int boardId;
  final DateTime? startDate;
  final DateTime? endDate;
  final List<User> assignees;
  final User? creator;
  final String boardName;
  final int commentCount;

  factory Task.fromJson(Map<String, dynamic> j) {
    final rawAssignees = (j['assignees'] as List?) ?? const [];
    final board = j['board'];
    final comments = (j['comments'] as List?) ?? const [];
    return Task(
      id: asInt(j['id']),
      title: asString(j['title']),
      description: asString(j['description']),
      status: TaskStatus.fromString(j['status'] as String?),
      priority: TaskPriority.fromString(j['priority'] as String?),
      completed: asBool(j['completed']),
      boardId: asInt(j['board_id']),
      startDate: parseDate(j['start_date']),
      endDate: parseDate(j['end_date']),
      assignees: rawAssignees
          .whereType<Map<String, dynamic>>()
          .map(User.fromJson)
          .toList(),
      creator: j['creator'] is Map<String, dynamic>
          ? User.fromJson(j['creator'] as Map<String, dynamic>)
          : null,
      boardName:
          board is Map<String, dynamic> ? asString(board['name']) : '',
      commentCount: comments.length,
    );
  }
}
