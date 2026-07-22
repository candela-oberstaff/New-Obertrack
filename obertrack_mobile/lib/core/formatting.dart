import 'package:intl/intl.dart';

final _dayFmt = DateFormat('d MMM y', 'es');
final _dayShort = DateFormat('d MMM', 'es');
final _timeFmt = DateFormat('HH:mm', 'es');

String formatDate(DateTime? d) => d == null ? '—' : _dayFmt.format(d.toLocal());
String formatDateShort(DateTime? d) =>
    d == null ? '—' : _dayShort.format(d.toLocal());

/// Fecha relativa amable para listas (notificaciones, etc.).
String formatRelative(DateTime? d) {
  if (d == null) return '';
  final local = d.toLocal();
  final now = DateTime.now();
  final diff = now.difference(local);
  if (diff.inSeconds < 60) return 'ahora';
  if (diff.inMinutes < 60) return 'hace ${diff.inMinutes} min';
  if (diff.inHours < 24) return 'hace ${diff.inHours} h';
  if (diff.inDays == 1) return 'ayer';
  if (diff.inDays < 7) return 'hace ${diff.inDays} d';
  return _dayShort.format(local);
}

String formatTime(DateTime? d) => d == null ? '' : _timeFmt.format(d.toLocal());

/// Horas con una decimal como máximo: 8, 7.5…
String formatHours(double h) {
  if (h == h.roundToDouble()) return h.toStringAsFixed(0);
  return h.toStringAsFixed(1);
}
