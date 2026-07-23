import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:local_auth/local_auth.dart';
import 'package:local_auth_android/local_auth_android.dart';

import 'providers.dart';

/// Envuelve `local_auth` para el desbloqueo por huella / rostro.
class BiometricService {
  BiometricService(this._auth);
  final LocalAuthentication _auth;

  /// ¿El dispositivo tiene hardware biométrico disponible y configurado?
  Future<bool> get isAvailable async {
    try {
      final supported = await _auth.isDeviceSupported();
      if (!supported) return false;
      final canCheck = await _auth.canCheckBiometrics;
      final enrolled = await _auth.getAvailableBiometrics();
      return canCheck && enrolled.isNotEmpty;
    } catch (_) {
      return false;
    }
  }

  /// Lanza el diálogo del sistema. Devuelve true si la identidad se verificó.
  Future<bool> authenticate({
    String reason = 'Confirma tu identidad para entrar a Obertrack',
  }) async {
    try {
      return await _auth.authenticate(
        localizedReason: reason,
        options: const AuthenticationOptions(
          biometricOnly: true,
          stickyAuth: true,
        ),
        authMessages: const [
          AndroidAuthMessages(
            signInTitle: 'Inicio de sesión con huella',
            cancelButton: 'Cancelar',
            biometricHint: 'Verifica tu identidad',
          ),
        ],
      );
    } catch (_) {
      return false;
    }
  }
}

final biometricServiceProvider = Provider<BiometricService>((ref) {
  return BiometricService(LocalAuthentication());
});

/// ¿El dispositivo soporta biometría? (para mostrar u ocultar la opción).
final biometricAvailableProvider = FutureProvider<bool>((ref) {
  return ref.watch(biometricServiceProvider).isAvailable;
});

/// Preferencia persistida: si el usuario activó el inicio con huella.
class BiometricPrefs {
  BiometricPrefs(this._ref);
  final Ref _ref;
  static const _key = 'biometric_enabled';
  static const _offeredKey = 'biometric_offered';

  Future<bool> get isEnabled async {
    final v = await _ref.read(secureStorageProvider).read(key: _key);
    return v == '1';
  }

  Future<void> setEnabled(bool value) async {
    final storage = _ref.read(secureStorageProvider);
    if (value) {
      await storage.write(key: _key, value: '1');
    } else {
      await storage.delete(key: _key);
    }
  }

  /// ¿Ya le ofrecimos activar la huella? (para no preguntar cada vez).
  Future<bool> get wasOffered async {
    final v = await _ref.read(secureStorageProvider).read(key: _offeredKey);
    return v == '1';
  }

  Future<void> markOffered() =>
      _ref.read(secureStorageProvider).write(key: _offeredKey, value: '1');
}

final biometricPrefsProvider = Provider<BiometricPrefs>((ref) {
  return BiometricPrefs(ref);
});
