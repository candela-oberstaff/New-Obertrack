import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:obertrack_mobile/models/task.dart';
import 'package:obertrack_mobile/models/user.dart';
import 'package:obertrack_mobile/models/work_hour.dart';

void main() {
  test('UserType parsea los valores canónicos del backend', () {
    expect(UserType.fromString('profesional'), UserType.profesional);
    expect(UserType.fromString('empleador'), UserType.empleador);
    expect(UserType.fromString('customer_success'), UserType.customerSuccess);
    expect(UserType.fromString(null), UserType.unknown);
  });

  test('WorkHour deriva el estado de approved/rejected', () {
    WorkHour make({bool a = false, bool r = false}) => WorkHour.fromJson({
          'id': 1,
          'work_date': '2026-07-01T00:00:00Z',
          'work_type': 'complete',
          'hours_worked': 8,
          'approved': a,
          'rejected': r,
        });
    expect(make().status, WorkHourStatus.pending);
    expect(make(a: true).status, WorkHourStatus.approved);
    expect(make(r: true).status, WorkHourStatus.rejected);
  });

  test('TaskStatus round-trip wire/label', () {
    expect(TaskStatus.fromString('en_proceso'), TaskStatus.enProceso);
    expect(TaskStatus.enProceso.wire, 'en_proceso');
  });

  test('User.canView respeta permisos y el default histórico', () {
    final withPerms = User.fromJson({
      'id': 1,
      'name': 'Ana',
      'email': 'a@b.com',
      'user_type': 'profesional',
      'permissions': {'tasks': 'view'},
    });
    expect(withPerms.canView('tasks'), true);
    expect(withPerms.canEdit('tasks'), false);
    expect(withPerms.canView('hours'), false);

    final noPerms = User.fromJson({
      'id': 2,
      'name': 'Beto',
      'email': 'b@b.com',
      'user_type': 'profesional',
    });
    expect(noPerms.canView('anything'), true);
  });

  testWidgets('El binding de widgets funciona', (tester) async {
    await tester.pumpWidget(const MaterialApp(home: Scaffold()));
    expect(find.byType(Scaffold), findsOneWidget);
  });
}
