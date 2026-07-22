import 'json_utils.dart';

/// Envoltorio de paginación del backend: `{ data, total, page, limit }`.
class Paginated<T> {
  Paginated({
    required this.items,
    required this.total,
    required this.page,
    required this.limit,
  });

  final List<T> items;
  final int total;
  final int page;
  final int limit;

  bool get hasMore => page * limit < total;

  factory Paginated.fromJson(
    Map<String, dynamic> json,
    T Function(Map<String, dynamic>) itemFromJson,
  ) {
    final rawList = (json['data'] as List?) ?? const [];
    return Paginated<T>(
      items: rawList
          .whereType<Map<String, dynamic>>()
          .map(itemFromJson)
          .toList(),
      total: asInt(json['total']),
      page: asInt(json['page'], 1),
      limit: asInt(json['limit'], rawList.length),
    );
  }
}
