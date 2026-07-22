package com.obertrack.obertrack_mobile

import io.flutter.embedding.android.FlutterFragmentActivity

// local_auth (huella / rostro) requiere una FragmentActivity para mostrar el
// diálogo biométrico del sistema, no la FlutterActivity por defecto.
class MainActivity : FlutterFragmentActivity()
