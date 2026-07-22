import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';

import 'api_client.dart';
import 'token_store.dart';

/// Almacenamiento cifrado (Keychain en iOS, EncryptedSharedPreferences en Android).
final secureStorageProvider = Provider<FlutterSecureStorage>((ref) {
  return const FlutterSecureStorage(
    aOptions: AndroidOptions(encryptedSharedPreferences: true),
  );
});

final tokenStoreProvider = Provider<TokenStore>((ref) {
  return TokenStore(ref.watch(secureStorageProvider));
});

/// Cliente HTTP compartido. Cuando el refresh falla, invalida la sesión.
final apiClientProvider = Provider<ApiClient>((ref) {
  final tokens = ref.watch(tokenStoreProvider);
  return ApiClient(
    tokens,
    onSessionExpired: () {
      ref.read(sessionExpiredProvider.notifier).state++;
    },
  );
});

/// Contador que se incrementa cuando expira la sesión; el controlador de auth
/// lo observa para forzar el cierre de sesión y la redirección al login.
final sessionExpiredProvider = StateProvider<int>((ref) => 0);
