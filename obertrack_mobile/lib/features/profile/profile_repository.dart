import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/api_client.dart';
import '../../core/providers.dart';
import '../../models/cv.dart';

class ProfileRepository {
  ProfileRepository(this._api);
  final ApiClient _api;

  /// GET /api/me/cv → CVView (sin envoltorio).
  Future<CvView> myCv() async {
    final resp = await _api.get('/me/cv');
    final data = resp.data;
    if (resp.statusCode == 200 && data is Map<String, dynamic>) {
      return CvView.fromJson(data);
    }
    final msg = (data is Map && data['error'] is String)
        ? data['error'] as String
        : 'No se pudo cargar tu CV';
    throw Exception(msg);
  }
}

final profileRepositoryProvider = Provider<ProfileRepository>((ref) {
  return ProfileRepository(ref.watch(apiClientProvider));
});

/// CV vivo del profesional (auto-refresca al invalidar).
final myCvProvider = FutureProvider.autoDispose<CvView>((ref) {
  return ref.watch(profileRepositoryProvider).myCv();
});
