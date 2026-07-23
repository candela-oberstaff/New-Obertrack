import 'package:flutter_secure_storage/flutter_secure_storage.dart';

/// Almacenamiento seguro de los tokens de sesión.
///
/// El backend entrega los tokens como cookies httpOnly (`access_token` /
/// `refresh_token`). Un cliente nativo SÍ puede leer la cabecera `Set-Cookie`
/// (httpOnly solo bloquea el acceso desde JavaScript en el navegador), así que
/// extraemos ambos valores y los guardamos aquí de forma cifrada. Luego el
/// access token viaja como `Authorization: Bearer <token>` en cada petición.
class TokenStore {
  TokenStore(this._storage);

  final FlutterSecureStorage _storage;

  static const _kAccess = 'access_token';
  static const _kRefresh = 'refresh_token';

  Future<void> save({required String access, required String refresh}) async {
    await _storage.write(key: _kAccess, value: access);
    await _storage.write(key: _kRefresh, value: refresh);
  }

  Future<void> saveAccess(String access) =>
      _storage.write(key: _kAccess, value: access);

  Future<String?> get accessToken => _storage.read(key: _kAccess);
  Future<String?> get refreshToken => _storage.read(key: _kRefresh);

  Future<bool> get hasSession async => (await accessToken) != null;

  Future<void> clear() async {
    await _storage.delete(key: _kAccess);
    await _storage.delete(key: _kRefresh);
  }
}
