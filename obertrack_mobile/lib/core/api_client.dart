import 'dart:async';

import 'package:dio/dio.dart';

import 'config.dart';
import 'token_store.dart';

/// Se dispara cuando el refresh falla y hay que cerrar sesión.
typedef OnSessionExpired = void Function();

/// Cliente HTTP central de la app.
///
/// Responsabilidades:
///  - Inyectar `Authorization: Bearer <access_token>` en cada petición.
///  - Extraer los tokens de la cabecera `Set-Cookie` en login/register/refresh.
///  - Refrescar el access token de forma transparente ante un 401 y reintentar
///    la petición original una sola vez.
class ApiClient {
  ApiClient(this._tokens, {this.onSessionExpired}) {
    _dio = Dio(
      BaseOptions(
        baseUrl: AppConfig.apiBaseUrl,
        connectTimeout: AppConfig.connectTimeout,
        receiveTimeout: AppConfig.receiveTimeout,
        // No lanzar en 4xx: dejamos que las capas superiores lean el body de error.
        validateStatus: (s) => s != null && s < 500,
        headers: {'Accept': 'application/json'},
      ),
    );

    _dio.interceptors.add(
      InterceptorsWrapper(
        onRequest: (options, handler) async {
          if (options.extra['skipAuth'] != true) {
            final token = await _tokens.accessToken;
            if (token != null) {
              options.headers['Authorization'] = 'Bearer $token';
            }
          }
          handler.next(options);
        },
        onResponse: (response, handler) async {
          // Persistimos los tokens ANTES de entregar la respuesta, para que el
          // login no se dé por completado sin la sesión guardada.
          await _captureTokensFromResponse(response);
          // Un 401 llega como respuesta válida (validateStatus<500).
          if (response.statusCode == 401 &&
              response.requestOptions.extra['retried'] != true &&
              response.requestOptions.extra['skipAuth'] != true) {
            final refreshed = await _tryRefresh();
            if (refreshed) {
              final retried = await _retry(response.requestOptions);
              return handler.resolve(retried);
            }
            onSessionExpired?.call();
          }
          handler.next(response);
        },
        onError: (err, handler) async {
          handler.next(err);
        },
      ),
    );
  }

  late final Dio _dio;
  final TokenStore _tokens;
  final OnSessionExpired? onSessionExpired;

  Completer<bool>? _refreshing;

  Dio get raw => _dio;

  // ---- Peticiones tipadas de conveniencia -------------------------------

  Future<Response<T>> get<T>(String path, {Map<String, dynamic>? query}) =>
      _dio.get<T>(path, queryParameters: query);

  Future<Response<T>> post<T>(String path, {Object? data}) =>
      _dio.post<T>(path, data: data);

  Future<Response<T>> put<T>(String path, {Object? data}) =>
      _dio.put<T>(path, data: data);

  Future<Response<T>> delete<T>(String path, {Object? data}) =>
      _dio.delete<T>(path, data: data);

  /// Login: no envía Bearer y captura los tokens del `Set-Cookie`.
  Future<Response> postAuth(String path, {Object? data}) =>
      _dio.post(path, data: data, options: Options(extra: {'skipAuth': true}));

  // ---- Manejo de tokens vía Set-Cookie ----------------------------------

  Future<void> _captureTokensFromResponse(Response response) async {
    final cookies = response.headers.map['set-cookie'];
    if (cookies == null || cookies.isEmpty) return;
    String? access;
    String? refresh;
    for (final raw in cookies) {
      final pair = _firstCookiePair(raw);
      if (pair == null) continue;
      if (pair.key == 'access_token' && pair.value.isNotEmpty) {
        access = pair.value;
      } else if (pair.key == 'refresh_token' && pair.value.isNotEmpty) {
        refresh = pair.value;
      }
    }
    if (access != null && refresh != null) {
      await _tokens.save(access: access, refresh: refresh);
    } else if (access != null) {
      await _tokens.saveAccess(access);
    }
  }

  /// De "name=value; Path=/; HttpOnly" devuelve MapEntry(name, value).
  MapEntry<String, String>? _firstCookiePair(String setCookie) {
    final firstSegment = setCookie.split(';').first.trim();
    final eq = firstSegment.indexOf('=');
    if (eq <= 0) return null;
    return MapEntry(
      firstSegment.substring(0, eq).trim(),
      firstSegment.substring(eq + 1).trim(),
    );
  }

  Future<bool> _tryRefresh() {
    // Coalesce: si ya hay un refresh en curso, esperar su resultado.
    final inflight = _refreshing;
    if (inflight != null) return inflight.future;

    final completer = Completer<bool>();
    _refreshing = completer;

    () async {
      try {
        final refresh = await _tokens.refreshToken;
        if (refresh == null) {
          completer.complete(false);
          return;
        }
        final resp = await _dio.post(
          '/auth/refresh',
          options: Options(
            extra: {'skipAuth': true, 'retried': true},
            headers: {'Cookie': 'refresh_token=$refresh'},
          ),
        );
        if (resp.statusCode == 200) {
          await _captureTokensFromResponse(resp);
          completer.complete(true);
        } else {
          completer.complete(false);
        }
      } catch (_) {
        completer.complete(false);
      } finally {
        _refreshing = null;
      }
    }();

    return completer.future;
  }

  Future<Response> _retry(RequestOptions options) {
    final token = _tokens.accessToken;
    return token.then((t) {
      final headers = Map<String, dynamic>.from(options.headers);
      if (t != null) headers['Authorization'] = 'Bearer $t';
      return _dio.request(
        options.path,
        data: options.data,
        queryParameters: options.queryParameters,
        options: Options(
          method: options.method,
          headers: headers,
          extra: {...options.extra, 'retried': true},
        ),
      );
    });
  }
}

/// Extrae el mensaje de error del cuerpo `{"error": "..."}` del backend.
String apiErrorMessage(Object error, {String fallback = 'Ocurrió un error'}) {
  if (error is DioException) {
    final data = error.response?.data;
    if (data is Map && data['error'] is String) return data['error'] as String;
    if (error.type == DioExceptionType.connectionTimeout ||
        error.type == DioExceptionType.connectionError) {
      return 'No se pudo conectar con el servidor';
    }
  }
  return fallback;
}
