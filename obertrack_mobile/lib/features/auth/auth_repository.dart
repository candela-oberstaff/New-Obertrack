import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/api_client.dart';
import '../../core/providers.dart';
import '../../core/token_store.dart';
import '../../models/user.dart';

class AuthRepository {
  AuthRepository(this._api, this._tokens);

  final ApiClient _api;
  final TokenStore _tokens;

  /// Inicia sesión. El backend setea los tokens vía `Set-Cookie`; el
  /// [ApiClient] los captura y persiste. Devuelve el usuario (`{"user": ...}`).
  Future<User> login(String email, String password) async {
    final resp =
        await _api.postAuth('/auth/login', data: {'email': email, 'password': password});
    if (resp.statusCode == 200) {
      final data = resp.data;
      final userJson = (data is Map && data['user'] is Map)
          ? data['user'] as Map<String, dynamic>
          : data as Map<String, dynamic>;
      return User.fromJson(userJson);
    }
    throw _errorFrom(resp.statusCode, resp.data);
  }

  /// Usuario actual + permisos (GET /auth/me devuelve el User SIN envoltorio).
  Future<User> me() async {
    final resp = await _api.get('/auth/me');
    if (resp.statusCode == 200 && resp.data is Map<String, dynamic>) {
      return User.fromJson(resp.data as Map<String, dynamic>);
    }
    throw _errorFrom(resp.statusCode, resp.data);
  }

  Future<void> logout() async {
    try {
      await _api.post('/auth/logout');
    } catch (_) {
      // Ignoramos fallos de red al cerrar sesión: limpiamos localmente igual.
    } finally {
      await _tokens.clear();
    }
  }

  Object _errorFrom(int? status, dynamic data) {
    if (data is Map && data['error'] is String) {
      return AuthException(data['error'] as String);
    }
    if (status == 401) return const AuthException('Credenciales inválidas');
    return const AuthException('No se pudo iniciar sesión');
  }
}

class AuthException implements Exception {
  const AuthException(this.message);
  final String message;
  @override
  String toString() => message;
}

final authRepositoryProvider = Provider<AuthRepository>((ref) {
  return AuthRepository(
    ref.watch(apiClientProvider),
    ref.watch(tokenStoreProvider),
  );
});
