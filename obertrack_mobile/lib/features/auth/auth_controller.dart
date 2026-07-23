import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/biometric_service.dart';
import '../../core/providers.dart';
import '../../core/token_store.dart';
import '../../models/user.dart';
import 'auth_repository.dart';

/// - unknown: aún resolviendo (splash).
/// - unauthenticated: sin sesión → login.
/// - locked: hay sesión guardada pero protegida con huella → desbloquear.
/// - authenticated: dentro.
enum AuthStatus { unknown, unauthenticated, locked, authenticated }

class AuthState {
  const AuthState({
    required this.status,
    this.user,
    this.loading = false,
    this.error,
    this.biometricEnabled = false,
  });

  final AuthStatus status;
  final User? user;
  final bool loading;
  final String? error;
  final bool biometricEnabled;

  AuthState copyWith({
    AuthStatus? status,
    User? user,
    bool? loading,
    String? error,
    bool? biometricEnabled,
    bool clearError = false,
    bool clearUser = false,
  }) {
    return AuthState(
      status: status ?? this.status,
      user: clearUser ? null : (user ?? this.user),
      loading: loading ?? this.loading,
      error: clearError ? null : (error ?? this.error),
      biometricEnabled: biometricEnabled ?? this.biometricEnabled,
    );
  }

  static const initial = AuthState(status: AuthStatus.unknown);
}

class AuthController extends StateNotifier<AuthState> {
  AuthController(this._repo, this._tokens, this._ref) : super(AuthState.initial) {
    _bootstrap();
    // Cierra sesión automáticamente cuando el refresh de tokens falla.
    _ref.listen<int>(sessionExpiredProvider, (prev, next) {
      if (next > 0 && state.status == AuthStatus.authenticated) {
        _forceLogout();
      }
    });
  }

  final AuthRepository _repo;
  final TokenStore _tokens;
  final Ref _ref;

  /// Al arrancar: si hay token guardado y la huella está activada, quedamos
  /// "bloqueados" hasta el desbloqueo biométrico; si no, validamos con /me.
  Future<void> _bootstrap() async {
    final bioEnabled = await _ref.read(biometricPrefsProvider).isEnabled;
    final hasSession = await _tokens.hasSession;
    if (!hasSession) {
      state = state.copyWith(
        status: AuthStatus.unauthenticated,
        biometricEnabled: false,
      );
      return;
    }
    if (bioEnabled) {
      state = state.copyWith(status: AuthStatus.locked, biometricEnabled: true);
      return;
    }
    try {
      final user = await _repo.me();
      state = state.copyWith(status: AuthStatus.authenticated, user: user);
    } catch (_) {
      await _tokens.clear();
      state =
          state.copyWith(status: AuthStatus.unauthenticated, clearUser: true);
    }
  }

  Future<bool> login(String email, String password) async {
    state = state.copyWith(loading: true, clearError: true);
    try {
      final user = await _repo.login(email, password);
      final bioEnabled = await _ref.read(biometricPrefsProvider).isEnabled;
      state = state.copyWith(
        status: AuthStatus.authenticated,
        user: user,
        loading: false,
        biometricEnabled: bioEnabled,
      );
      return true;
    } on AuthException catch (e) {
      state = state.copyWith(loading: false, error: e.message);
      return false;
    } catch (e) {
      state = state.copyWith(loading: false, error: 'No se pudo iniciar sesión');
      return false;
    }
  }

  /// Desbloqueo con huella desde el estado [AuthStatus.locked].
  Future<void> unlockWithBiometrics() => loginWithBiometrics();

  /// Abre el panel biométrico y, si se verifica, entra usando la sesión
  /// guardada. Funciona desde cualquier estado del login (no depende de que el
  /// arranque haya detectado "bloqueado"). Devuelve un mensaje de error para la
  /// UI, o null si todo salió bien / el usuario canceló.
  Future<String?> loginWithBiometrics() async {
    final available = await _ref.read(biometricServiceProvider).isAvailable;
    if (!available) {
      return 'Este dispositivo no tiene huella configurada.';
    }
    final hasSession = await _tokens.hasSession;
    if (!hasSession) {
      return 'No hay una sesión guardada en este equipo. '
          'Inicia sesión con tu contraseña una vez y vuelve a intentarlo.';
    }
    final ok = await _ref.read(biometricServiceProvider).authenticate();
    if (!ok) return null; // el usuario canceló: sin error.

    state = state.copyWith(loading: true, clearError: true);
    try {
      final user = await _repo.me();
      state = state.copyWith(
          status: AuthStatus.authenticated, user: user, loading: false);
      return null;
    } catch (_) {
      await _disableBiometricsSilently();
      await _tokens.clear();
      state = const AuthState(
        status: AuthStatus.unauthenticated,
        error: 'Tu sesión expiró. Inicia sesión con tu contraseña.',
      );
      return null;
    }
  }

  /// Activa la huella (tras verificarla una vez). Requiere sesión activa.
  Future<bool> enableBiometrics() async {
    final ok = await _ref.read(biometricServiceProvider).authenticate(
        reason: 'Verifica tu huella para activar el acceso rápido');
    if (!ok) return false;
    await _ref.read(biometricPrefsProvider).setEnabled(true);
    state = state.copyWith(biometricEnabled: true);
    return true;
  }

  Future<void> disableBiometrics() async {
    await _ref.read(biometricPrefsProvider).setEnabled(false);
    state = state.copyWith(biometricEnabled: false);
  }

  Future<void> _disableBiometricsSilently() async {
    await _ref.read(biometricPrefsProvider).setEnabled(false);
  }

  Future<void> logout() async {
    await _repo.logout();
    await _disableBiometricsSilently();
    state = const AuthState(status: AuthStatus.unauthenticated);
  }

  Future<void> _forceLogout() async {
    await _tokens.clear();
    await _disableBiometricsSilently();
    state = const AuthState(
      status: AuthStatus.unauthenticated,
      error: 'Tu sesión expiró. Inicia sesión de nuevo.',
    );
  }

  /// Recarga /auth/me (p.ej. tras cambios de perfil).
  Future<void> refreshMe() async {
    try {
      final user = await _repo.me();
      state = state.copyWith(user: user);
    } catch (_) {}
  }
}

final authControllerProvider =
    StateNotifierProvider<AuthController, AuthState>((ref) {
  return AuthController(
    ref.watch(authRepositoryProvider),
    ref.watch(tokenStoreProvider),
    ref,
  );
});

/// Atajo al usuario actual.
final currentUserProvider = Provider<User?>((ref) {
  return ref.watch(authControllerProvider).user;
});
