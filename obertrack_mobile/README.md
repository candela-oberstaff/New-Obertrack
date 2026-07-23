# Obertrack Mobile

App móvil (Flutter) de **Obertrack**, cliente del backend Go/Gin del proyecto
`Obertrack`. Es un **proyecto aparte** del monorepo web/backend: vive como
carpeta hermana y solo consume la API REST + WebSocket.

Una sola app que se **adapta al rol** tras el login (profesional, empleador,
manager, superadmin…) mostrando los módulos permitidos.

## Módulos incluidos (MVP)

| Módulo | Pantalla | Endpoints |
|---|---|---|
| **Login + sesión** | `login_screen` | `POST /api/auth/login`, `POST /api/auth/refresh`, `GET /api/auth/me` |
| **Perfil / CV** | `profile_screen` | `GET /api/auth/me`, `GET /api/me/cv` |
| **Notificaciones** | `notifications_screen` | `GET /api/notifications`, `/unread-count`, `/:id/read`, `/read-all` + `ws://…/ws/notifications` |
| **Tareas** | `tasks_screen` | `GET /api/tasks`, `PUT /api/tasks/:id` |
| **Horas** | `work_hours_screen` | `GET /api/work-hours`, `/summary`, `POST /api/work-hours` |

## Autenticación

El backend entrega los tokens como **cookies httpOnly** (`access_token` /
`refresh_token`). Un cliente nativo sí puede leer la cabecera `Set-Cookie`
(httpOnly solo bloquea el acceso desde el navegador), así que la app:

1. Hace login y **extrae los tokens del `Set-Cookie`** ([`ApiClient`](lib/core/api_client.dart)).
2. Los guarda cifrados con `flutter_secure_storage` ([`TokenStore`](lib/core/token_store.dart)).
3. Envía `Authorization: Bearer <access_token>` en cada request (el middleware lo acepta).
4. Ante un `401`, refresca de forma transparente con `POST /auth/refresh` y reintenta.

## Configuración del backend

Por defecto apunta al **Docker local**:
- Android emulador → `http://10.0.2.2:8080` (el host de la máquina de desarrollo).
- iOS simulador / escritorio → `http://localhost:8080`.

Para apuntar a otro servidor, usa `--dart-define`:

```bash
# Producción
flutter run --dart-define=API_BASE_URL=https://obertrack.com

# Un backend en tu LAN
flutter run --dart-define=API_BASE_URL=http://192.168.1.50:8080
```

> **Nota de seguridad:** el `AndroidManifest` habilita `usesCleartextTraffic`
> para permitir HTTP en desarrollo (`10.0.2.2`). Para una build de producción
> contra HTTPS conviene restringirlo con un `network_security_config`.

## Arquitectura

```
lib/
  core/          config, tema, cliente HTTP, almacenamiento, router, formato
  models/        modelos Dart mapeados 1:1 al JSON del backend
  features/
    auth/        repositorio + estado de sesión (Riverpod) + login
    profile/     perfil y CV vivo
    notifications/ lista, contador, socket en tiempo real
    tasks/       lista, filtros y cambio de estado
    work_hours/  lista, resumen mensual y registro de jornada
    home/        splash + shell con navegación adaptada al rol
  widgets/       vistas compartidas (loader, error, vacío)
```

- **Estado:** Riverpod (`flutter_riverpod`).
- **HTTP:** Dio con interceptores (Bearer + refresh).
- **Routing:** go_router con redirección según el estado de auth.
- **Realtime:** `web_socket_channel` (IOWebSocketChannel con header Bearer).

## Ejecutar

```bash
flutter pub get
flutter run            # con un emulador/dispositivo conectado
flutter test           # tests de modelos/parseo
flutter analyze
```

La primera compilación Android descarga Gradle y sus dependencias, así que
requiere conexión a internet.

## Próximos módulos sugeridos

Chat/canales (`/ws/chat`, `/ws/channels`), incidentes/emergencias
(`/api/admin/incidents`) y, para el empleador, aprobación de horas y gestión de
equipo — todos con endpoints ya disponibles en el backend.
