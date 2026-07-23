import 'json_utils.dart';

/// Tipos de cuenta del backend (valores canónicos).
enum UserType {
  empleador,
  profesional,
  superadmin,
  analistaIt,
  customerSuccess,
  unknown;

  static UserType fromString(String? v) {
    switch (v) {
      case 'empleador':
        return UserType.empleador;
      case 'profesional':
        return UserType.profesional;
      case 'superadmin':
        return UserType.superadmin;
      case 'analista_it':
        return UserType.analistaIt;
      case 'customer_success':
        return UserType.customerSuccess;
      default:
        return UserType.unknown;
    }
  }

  /// Etiqueta legible para la UI.
  String get label {
    switch (this) {
      case UserType.empleador:
        return 'Empleador';
      case UserType.profesional:
        return 'Profesional';
      case UserType.superadmin:
        return 'Superadmin';
      case UserType.analistaIt:
        return 'Analista IT';
      case UserType.customerSuccess:
        return 'Customer Success';
      case UserType.unknown:
        return 'Usuario';
    }
  }
}

/// Empresa donde el profesional tiene un empleo activo (switcher multi-empresa).
class CompanyRef {
  CompanyRef({required this.id, required this.name});
  final int id;
  final String name;

  factory CompanyRef.fromJson(Map<String, dynamic> j) =>
      CompanyRef(id: asInt(j['id']), name: asString(j['name']));
}

class User {
  User({
    required this.id,
    required this.name,
    required this.email,
    required this.userType,
    this.avatar = '',
    this.isManager = false,
    this.isSuperadmin = false,
    this.isActive = true,
    this.empleadorId,
    this.managerId,
    this.companyName = '',
    this.industry = '',
    this.jobTitle = '',
    this.phoneNumber = '',
    this.country = '',
    this.state = '',
    this.city = '',
    this.location = '',
    this.identityDocument = '',
    this.address = '',
    this.permissions,
    this.companies = const [],
    this.createdAt,
  });

  final int id;
  final String name;
  final String email;
  final String avatar;
  final UserType userType;
  final bool isManager;
  final bool isSuperadmin;
  final bool isActive;
  final int? empleadorId;
  final int? managerId;
  final String companyName;
  final String industry;
  final String jobTitle;
  final String phoneNumber;
  final String country;
  final String state;
  final String city;
  final String location;
  final String identityDocument;
  final String address;

  /// Permisos efectivos por módulo (solo en /auth/login y /auth/me).
  final Map<String, String>? permissions;

  /// Empresas del switcher multi-empresa (solo en endpoints de auth).
  final List<CompanyRef> companies;
  final DateTime? createdAt;

  bool get isEmployer => userType == UserType.empleador || isSuperadmin;

  /// ¿Tiene al menos "view" en el módulo? Ausencia de mapa = comportamiento
  /// histórico (sin restricción) => devolvemos true.
  bool canView(String module) {
    final p = permissions;
    if (p == null) return true;
    final level = p[module];
    return level == 'view' || level == 'edit';
  }

  bool canEdit(String module) {
    final p = permissions;
    if (p == null) return true;
    return p[module] == 'edit';
  }

  String get initials {
    final parts = name.trim().split(RegExp(r'\s+'));
    if (parts.isEmpty || parts.first.isEmpty) return '?';
    if (parts.length == 1) return parts.first[0].toUpperCase();
    return (parts.first[0] + parts.last[0]).toUpperCase();
  }

  factory User.fromJson(Map<String, dynamic> j) {
    Map<String, String>? perms;
    final rawPerms = j['permissions'];
    if (rawPerms is Map) {
      perms = rawPerms.map((k, v) => MapEntry(k.toString(), v.toString()));
    }
    final rawCompanies = (j['companies'] as List?) ?? const [];
    return User(
      id: asInt(j['id']),
      name: asString(j['name']),
      email: asString(j['email']),
      avatar: asString(j['avatar']),
      userType: UserType.fromString(j['user_type'] as String?),
      isManager: asBool(j['is_manager']),
      isSuperadmin: asBool(j['is_superadmin']),
      isActive: asBool(j['is_active'], true),
      empleadorId: asIntOrNull(j['empleador_id']),
      managerId: asIntOrNull(j['manager_id']),
      companyName: asString(j['company_name']),
      industry: asString(j['industry']),
      jobTitle: asString(j['job_title']),
      phoneNumber: asString(j['phone_number']),
      country: asString(j['country']),
      state: asString(j['state']),
      city: asString(j['city']),
      location: asString(j['location']),
      identityDocument: asString(j['identity_document']),
      address: asString(j['address']),
      permissions: perms,
      companies: rawCompanies
          .whereType<Map<String, dynamic>>()
          .map(CompanyRef.fromJson)
          .toList(),
      createdAt: parseDate(j['created_at']),
    );
  }
}
