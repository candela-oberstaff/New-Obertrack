# Integración con Google Calendar

Vínculo **personal** entre cada usuario de Obertrack y su cuenta de Google, para
llevar la agenda de trabajo a su calendario.

**Estado: Fases 1, 2 y 3 completas.**
- **Fase 1** — vinculación de la cuenta personal y desconexión.
- **Fase 2** — sincronización de tareas → eventos en el calendario de cada
  asignado conectado (crear, actualizar, completar, reasignar, borrar).
- **Fase 3** — módulo de **Sesiones**: reuniones con sala de Google Meet
  convocadas desde Obertrack. Ver [sesiones-google-meet.md](sesiones-google-meet.md).

---

## 1. Por qué OAuth por usuario y no una cuenta de servicio

El requisito es que sirva **cualquier** cuenta de Google: un `@gmail.com`
personal o un dominio propio como los de Oberstaff.

| Enfoque | Alcance | Decisión |
|---|---|---|
| Domain-Wide Delegation (Service Account) | Solo un Workspace concreto | ❌ No es global |
| **OAuth per-user, consent screen "External"** | Cualquier cuenta de Google | ✅ Implementado |

Con el consent screen de tipo *External* no hay nada configurado por dominio:
cada persona autoriza su propia cuenta y Obertrack guarda su refresh token.

### El vínculo es por usuario, no por empresa

`google_calendar_accounts` tiene un `uniqueIndex` sobre `user_id`. El calendario
pertenece a la persona, así que el vínculo sobrevive a `SwitchCompany` y a las
altas y bajas de empleo. Un usuario que trabaja para dos empresas tiene un solo
calendario conectado.

---

## 2. Configuración en Google Cloud

1. Proyecto nuevo → habilitar **Google Calendar API**.
2. **OAuth consent screen → External.**
3. Scopes: `openid`, `email`, `https://www.googleapis.com/auth/calendar.events`.
4. Credencial **OAuth client ID → Web application**, con los redirect URIs:
   - `http://localhost:8080/api/integrations/google/callback` (dev)
   - `https://<dominio-prod>/api/integrations/google/callback`
5. Añadir cuentas de prueba y **enviar la app a verificación**.

> ⚠️ **Modo *testing*: los refresh tokens caducan a los 7 días** y hay un tope de
> 100 usuarios. Es comportamiento normal de Google, no un bug de Obertrack.
> Desaparece al superar la verificación (proceso de 2 a 6 semanas).

---

## 3. Variables de entorno

```env
GOOGLE_CALENDAR_ENABLED=true
GOOGLE_CLIENT_ID=...apps.googleusercontent.com
GOOGLE_CLIENT_SECRET=...
GOOGLE_REDIRECT_URI=https://app.obertrack.com/api/integrations/google/callback
GOOGLE_TOKEN_ENC_KEY=<openssl rand -base64 32>
FRONTEND_URL=https://app.obertrack.com
```

Con el flag apagado (default) el módulo no se construye, el panel no aparece en
Perfil y las rutas responden 503. **Con el flag encendido y alguna variable
vacía, el backend no arranca**: es preferible a que el botón "Conectar" falle en
mitad del consentimiento, dejando al usuario en una pantalla de error de Google.

`GOOGLE_REDIRECT_URI` debe coincidir **exactamente** con uno de los registrados
en Google Cloud, o el canje falla con `redirect_uri_mismatch`.

---

## 4. Flujo

```
Perfil → "Conectar"
   │
   ├─ POST /api/me/integrations/google/connect  → { auth_url }
   │     (JSON y no 302: la petición sale por XHR, un redirect lo seguiría axios)
   │
   ├─ navegador → accounts.google.com  (consentimiento del usuario)
   │
   └─ Google → GET /api/integrations/google/callback?code=…&state=…
         │  (ruta PÚBLICA; la identidad sale del state firmado, no de la sesión)
         ├─ canje del code en oauth2.googleapis.com/token
         ├─ 'sub' y 'email' del id_token
         ├─ tokens cifrados (AES-256-GCM) → upsert en google_calendar_accounts
         └─ 302 → {FRONTEND_URL}/profile?google=ok
```

### Endpoints

| Método | Ruta | Auth |
|---|---|---|
| `GET` | `/api/integrations/google/callback` | Pública (state firmado) |
| `GET` | `/api/me/integrations/google/status` | Sesión |
| `POST` | `/api/me/integrations/google/connect` | Sesión |
| `DELETE` | `/api/me/integrations/google` | Sesión |

No hay endpoints para listar o elegir calendario: el scope es el mínimo
(`calendar.events`), que no permite enumerar calendarios, y los eventos van
siempre al principal (`primary`). Es una contrapartida deliberada para
simplificar la verificación de Google.

Ninguna ruta autenticada recibe un `user_id`: todas operan sobre la sesión, así
que no hay forma de tocar el vínculo de otra persona.

---

## 5. Decisiones de seguridad

**Tokens cifrados en reposo.** Un refresh token en claro equivale a la llave del
calendario del usuario. Se guardan con AES-256-GCM (`utils.SecretSealer`), nonce
aleatorio por operación y `json:"-"` en el modelo como defensa en profundidad.

**El `state` se firma con una clave derivada, no con `JWT_SECRET`.** El state
viaja por la URL, así que queda en el historial, en logs de proxy y en la
cabecera `Referer`. Firmado con el secreto de sesión, alguien podría presentarlo
como cookie `access_token`. Se usa `HMAC-SHA256(JWT_SECRET, "google-oauth-state-v1")`
más una audiencia propia: un state nunca vale como sesión ni al revés.
Cubierto por `TestStateTokenIsNotUsableAsSession`.

**El callback es público a propósito.** Llega como navegación del navegador
desde Google. Si dependiera de la sesión, un access token expirado durante el
consentimiento devolvería un JSON 401 a pantalla completa. Además, así el mismo
endpoint servirá para el deep link de la app móvil.

**Protección de redirect abierto.** El `return_to` lo elige el frontend y vuelve
dentro del state; `safeReturnTo` lo acota a rutas internas (rechaza esquemas,
hosts y `//` protocolo-relativo). Sin esto, un enlace preparado llevaría al
usuario a un dominio ajeno justo después de autenticarse.

**Se revoca en Google antes de borrar.** Si solo se borrara la fila, el permiso
seguiría concedido y Obertrack aparecería para siempre en la lista de apps con
acceso de la cuenta.

**`access_type=offline` + `prompt=consent` son obligatorios.** Sin ellos Google
deja de devolver refresh token a partir del segundo consentimiento de la misma
cuenta, y la integración se rompe semanas después sin ninguna señal al conectar.

---

## 6. Estados del vínculo

| Estado | Significado | UI |
|---|---|---|
| *(sin fila)* | Nunca vinculó | Botón "Conectar con Google Calendar" |
| `active` | Funcionando | Cuenta, calendario destino, "Desconectar" |
| `needs_reauth` | Google rechazó el refresh token | Banner ámbar + "Reconectar" |

`needs_reauth` se alcanza cuando Google responde `invalid_grant` (el usuario
revocó el acceso, cambió su contraseña, o el token caducó en modo *testing*). El
vínculo **no se borra**: se conserva para poder mostrar el aviso en vez de
perder el estado en silencio. Los endpoints devuelven **409** en ese caso, no
500: no es un fallo del sistema sino algo que el usuario resuelve reconectando.

---

## 7. Fase 2 — Sincronización de tareas → eventos

Una tarea de Obertrack **con fecha** se refleja como un evento en el Google
Calendar de **cada asignado que tenga su cuenta conectada y activa**. Es
unidireccional (Obertrack → Google).

### Eventos de día completo

Las tareas tienen `start_date`/`end_date` como fecha sin hora, así que se mapean
a eventos **all-day** (campo `date`, no `dateTime`). Esto **esquiva por completo
la zona horaria**. El `end.date` de Google es exclusivo, así que una tarea de un
solo día va de ese día al siguiente (la conversión vive en `buildEventPayload`).

### Cola durable (outbox), no llamadas en el request

Al mutar una tarea NO se llama a Google dentro del request: se encola un job en
`calendar_sync_jobs` (rápido) y un worker en segundo plano lo aplica con
reintentos. Así una caída de la API de Google —o un reinicio del backend— no
pierde ni bloquea la operación del usuario. Mismo principio que `report_runs`.

- Hay **un job por (tarea, usuario)**: el fallo de un asignado que revocó el
  acceso no frena la sincronización de los demás.
- Jobs pendientes anteriores de la misma (tarea, usuario) se descartan
  (`SupersedePendingJobs`): si una tarea se editó tres veces antes de correr el
  worker, solo importa el último estado.

### Reintentos: backoff exponencial y errores que no se reintentan

El worker corre cada 20 s, pero **un job fallido no se reintenta en el siguiente
tick**: `next_attempt_at` marca cuándo vuelve a ser elegible, y la espera escala
por `CalendarSyncMaxAttempts` (6) intentos:

| Tras el intento | Espera |
|---|---|
| 1 | 30 s |
| 2 | 2 min |
| 3 | 8 min |
| 4 | 30 min |
| 5 | 2 h |
| 6 | *(agotado → `failed`)* |

Ventana total de recuperación ≈ **2 h 40 min**. Antes eran reintentos cada 20 s,
así que los intentos se gastaban en unos dos minutos: una caída de la API de
Google o un pico de cuota algo más largo dejaba los jobs en `failed`, y un
`failed` **no se reejecuta nunca** — el evento quedaba sin crear hasta que
alguien volviera a editar la tarea.

Un job esperando su backoff **no bloquea la cola**: el filtro de
`ClaimPendingJobs` lo deja fuera y el resto sigue procesándose. Y como la fecha
vive en la BD, el backoff sobrevive a un reinicio del backend.

No todos los fallos merecen esa ventana. `retryAfterFailure` la niega a los que
no mejoran esperando, para no quemar cuota ni intentos:

| Error | ¿Reintenta? |
|---|---|
| Corte de red, `429` (cuota), `5xx` | Sí, con backoff |
| `408` (timeout del lado de Google) | Sí, con backoff |
| `401` → `needs_reauth` | No: lo arregla el usuario reconectando |
| Resto de `4xx` → `ErrGooglePermanent` | No: la petición está mal, repetirla da lo mismo |

La clasificación vive en `classifyEventError` / `isTransientStatus`. Cubierta por
`TestIsTransientStatus`, `TestProcessJobSchedulesBackoffOnTransientError`,
`TestProcessJobExhaustsAtMaxAttempts` y
`TestProcessJobDoesNotRetryPermanentFailures`.

### Ciclo de vida (enganchado en `taskService` vía callback inyectado)

| Acción en Obertrack | Efecto en Calendar |
|---|---|
| Crear tarea con fecha | Crea el evento para cada asignado conectado |
| Cambiar la fecha | Actualiza el evento |
| Completar / reabrir | Actualiza el título (✓ al completar) |
| Reasignar | Crea para el nuevo, borra para el que sale |
| Quitar la fecha | Borra el evento (queda sin reflejo) |
| Borrar la tarea | Borra el evento en todos los calendarios |

`OnTaskChanged` **reconcilia**: calcula el estado objetivo (asignados conectados
si la tarea tiene fecha) y lo compara con los enlaces existentes, encolando
upserts y borrados según la diferencia. Si el usuario borró el evento a mano en
Google, el siguiente update lo detecta (`ErrEventGone`) y lo re-crea.

### Enlace tarea↔evento

`calendar_event_links` mapea `(task_id, user_id) → google_event_id` (único por
par). Sin esta tabla solo se sabría crear, y cada guardado duplicaría el evento.

### Qué pasa al desconectar

Desconectar borra la fila de `google_calendar_accounts` **y los enlaces de ese
usuario** (`OnAccountDisconnected`, cableado como hook en `deps.go`).

Los eventos ya escritos **se quedan** en el calendario del usuario: la
desconexión revoca el permiso en Google antes de borrar nada, así que a partir
de ahí no hay credencial con la que eliminarlos. El usuario puede borrarlos a
mano. La contrapartida es que, si vuelve a conectar la misma cuenta, las tareas
que se editen después crean un evento nuevo junto al que quedó huérfano.

> Este era el origen de un bug: al no limpiar los enlaces, cada edición
> posterior de una tarea que ese usuario tuvo sincronizada encolaba un `delete`
> que fallaba con `ErrGoogleAccountNotFound`, gastaba sus cinco intentos y
> dejaba el enlace vivo para repetirlo en la edición siguiente. `processDelete`
> trata ahora ese error como "no hay dónde borrar" y limpia el enlace igual.
> Cubierto por `TestProcessDeleteWithoutAccountCleansLink`.

Reconectar tras `needs_reauth` **no** pasa por aquí: usa el mismo flujo de
conexión, `Upsert` conserva la fila y los enlaces siguen intactos, así que los
eventos existentes se actualizan en vez de duplicarse.

---

## 8. Archivos

| Capa | Archivo |
|---|---|
| Cifrado | `backend/internal/utils/crypto.go` |
| Modelos | `models/google_calendar.go`, `models/calendar_sync.go` |
| Migraciones | `202607231200_add_google_calendar_accounts`, `202607231600_add_calendar_sync` |
| Repositorios | `repository/google_calendar_repository.go`, `repository/calendar_sync_repository.go` |
| Servicios | `service/google_calendar_service.go`, `service/calendar_sync_service.go` (worker) |
| Handler | `handlers/google_calendar.go` |
| Enganche tareas | `service/task_service.go` (`SetCalendarSync`) |
| Rutas | `routes/account_routes.go`, `routes/public_routes.go` |
| Frontend | `services/google-calendar.service.ts`, `components/Profile/IntegracionesPanel.tsx` |

---

## 9. Pendiente (Fase 3+)

- **Backfill al conectar**: hoy solo se sincronizan tareas que se mutan DESPUÉS
  de conectar la cuenta. Al vincular, encolar upserts de las tareas con fecha ya
  asignadas al usuario.
- **Recuperar los jobs agotados**: un job en `failed` no se reejecuta nunca. Con
  la ventana de 2 h 40 min hace falta un incidente muy largo para llegar ahí,
  pero cuando pasa el evento queda sin crear hasta que alguien edite la tarea.
  Falta o bien un reintento manual desde admin, o bien un barrido periódico que
  reconcilie las tareas con fecha contra sus enlaces.
- **`Retry-After` en los 429**: hoy un 429 entra en la tabla de backoff genérica
  e ignora la cabecera con la que Google dice exactamente cuánto esperar.
- **El worker asume una sola réplica**: `ClaimPendingJobs` no bloquea filas
  porque solo hay una goroutine procesando. Si se escala horizontalmente hace
  falta `SELECT … FOR UPDATE SKIP LOCKED`.
- Jornadas aprobadas e incidentes → eventos. Ojo: `WorkHour` guarda fecha y hora
  en columnas separadas y sin timezone; hay que resolver la zona del usuario
  antes de mandar eventos con hora (los de tarea son all-day y no aplican).
- Bidireccional: push notifications de Google, sync tokens, conflictos. Duplica
  la complejidad; evaluar tras ver el uso real.
- App móvil: mismo backend, callback vía deep link.
