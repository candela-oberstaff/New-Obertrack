# Módulo de Sesiones (Google Meet)

Reuniones con sala de **Google Meet** convocadas desde Obertrack, con invitados
internos y externos. Entrada propia en el sidebar (`/sesiones`), junto a Tareas y
Horas.

**Estado: núcleo, recurrencia y presencia en vivo completos.** Queda la Fase B
(difusión) y la edición por instancia de una serie — ver §10.

Depende de la [integración con Google Calendar](google-calendar-integration.md):
la sesión se crea en el calendario del organizador usando su vínculo personal.

---

## 1. Crear la sala no necesitó scopes nuevos

(Leer quién está dentro sí: ver §6, que añadió `meetings.space.readonly`.)

Un Meet no se pide a la API de Meet: se crea insertando el evento de calendario
con `?conferenceDataVersion=1` y un `conferenceData.createRequest` de tipo
`hangoutsMeet`. El scope que ya teníamos —`calendar.events`— cubre eso y también
añadir `attendees`, así que **no hubo que volver a pasar por verificación de
Google ni pedir reconsentimiento** a quien ya tenía la cuenta conectada.

Dos detalles de la API que condicionan el diseño:

**La conferencia se crea en diferido.** La respuesta puede traer
`conferenceData.createRequest.status.statusCode == "pending"` y todavía sin
enlace. Por eso existe `GetEvent`: al crear se reconsulta el evento, y si aun así
no está, se resuelve la primera vez que alguien abre la sesión
(`meetingService.Get`).

**Al editar NO se manda `conferenceDataVersion`.** Con la versión 0 (el defecto)
Google ignora la conferencia del cuerpo y **conserva** la que ya tenía el evento.
Pedir la versión 1 sin incluir la conferencia en el `PUT` la borraría, y mover la
hora de una reunión dejaría a todo el mundo sin enlace.

---

## 2. La cuenta que convoca

La sesión sale del **calendario del organizador**, con su vínculo personal. Se
descartó una cuenta corporativa única porque sería una credencial compartida y
nadie vería la reunión en su propia agenda.

La contrapartida es que **quien no tenga Google conectado no puede convocar**. En
vez de tratarlo como error, `/sesiones` muestra un estado vacío que explica el
módulo y ofrece "Conectar con Google". Eso resuelve de paso un problema que venía
de antes: el panel de Integraciones del Perfil no lo visitaba nadie, así que la
integración existía pero no se descubría.

`useGoogleConnection` (`frontend/src/hooks/useGoogleConnection.ts`) centraliza el
estado del vínculo y el retorno del consentimiento. Nació dentro de
`IntegracionesPanel` y se extrajo al añadir Sesiones: con dos copias, una de las
dos se habría quedado atrás.

---

## 3. Eventos con hora y zonas horarias

Las tareas son de día completo y esquivan la zona horaria; **una sesión no**.
`CalendarEventInput` soporta ahora las dos formas y el par de campos que se
rellene decide cuál (`StartDate/EndDate` → `date`; `StartAt/EndAt/TimeZone` →
`dateTime`). Google **rechaza** un evento que traiga las dos a la vez, de ahí el
`omitempty` en ambos campos de `googleEventTime`.

`TimeZone` es un identificador **IANA** (`America/Bogota`), no un offset. En una
serie recurrente el offset se rompe con el cambio de horario de verano y la
reunión se desplaza una hora. El frontend lo toma de
`Intl.DateTimeFormat().resolvedOptions().timeZone` y lo muestra bajo el
formulario; el backend lo valida contra la base IANA embebida (`import _
"time/tzdata"` en `cmd/main.go`, necesario porque la imagen es alpine).

---

## 4. Por qué es síncrono (y las tareas no)

La sincronización de tareas va por una cola con reintentos. Las sesiones **hablan
con Google dentro del request**, a propósito: quien convoca necesita el enlace de
Meet en el momento para poder pegarlo en un chat, y no se le puede responder "tu
sesión existirá dentro de veinte segundos".

La contrapartida —un fallo de red pierde la operación en vez de reintentarla— es
asumible porque hay una persona mirando que puede volver a intentarlo, y el error
que ve es el tipado de siempre.

**Compensación al fallar el guardado.** Si Google crea el evento pero la sesión
no se puede persistir, el evento **se borra**. Sin eso quedaría una reunión en el
calendario del organizador que Obertrack no sabe que existe y que nadie puede
cancelar desde la app. Cubierto por `TestCreateRollsBackGoogleEventWhenSaveFails`.

---

## 5. Modelo y permisos

`meeting_sessions` guarda la reunión (instantes en UTC + la zona con la que se
convocó, el `google_event_id`, el `meet_url` y el `RRULE` de la serie).
`meeting_attendees` guarda los invitados: `user_id` nulo = **externo**, del que
solo se conoce el correo. El correo se guarda siempre porque es lo que viaja a
Google; el `user_id` es lo que permite avisar además dentro de Obertrack.

Un correo "externo" que resulta pertenecer a alguien con cuenta se **promueve a
interno**, para que reciba campanita y DM y no solo el correo de Google
(`TestResolveAttendeesPromotesKnownEmail`).

### Endpoints

| Método | Ruta | Permiso |
|---|---|---|
| GET | `/api/meetings` (`?past=true`, `?task_id=`) | `meetings` view |
| GET | `/api/meetings/upcoming` | `meetings` view |
| GET | `/api/meetings/:id` | `meetings` view |
| GET | `/api/meetings/:id/presence` | `meetings` view |
| POST | `/api/meetings` | `meetings` edit |
| PUT | `/api/meetings/:id` | `meetings` edit |
| DELETE | `/api/meetings/:id` (cancela) | `meetings` edit |

Ninguna recibe un `organizer_id`: quien convoca sale de la sesión. Ver una sesión
exige ser organizador o invitado; **editar y cancelar, solo el organizador**,
aunque se tenga `edit` del módulo (`TestOnlyOrganizerCanEditOrCancel`).

> ⚠️ `/meetings/upcoming` convive con `/meetings/:id`. Gin entra en pánico al
> arrancar si un segmento estático choca con un wildcard hermano, así que esa
> convivencia está fijada por `meeting_routes_test.go` en vez de descubrirse con
> el contenedor cayéndose.

### El módulo RBAC `meetings` necesitó backfill

`EffectivePermissions` lee una clave ausente como `none`. Los roles ya creados en
producción no tenían `meetings`, así que **el módulo habría nacido invisible**
para toda empresa con roles configurados, y el síntoma (una entrada del menú que
no aparece) no apunta a su causa. La migración
`202607281410_backfill_meetings_permission` la añade heredando de `tasks`: quien
puede crear tareas puede convocar reuniones.

Detalle de implementación: el `WHERE` usa `->>'meetings' IS NULL` y **no** el
operador jsonb `?`, porque GORM interpretaría ese signo como placeholder de
parámetro.

---

## 6. Presencia en vivo (API de Meet)

La API de **Calendar** no sabe nada de quién está conectado; eso vive en la API
de **Meet** (`meet.googleapis.com/v2`), que es otra API aunque comparta la
credencial del usuario.

### Scope: `meetings.space.readonly`, y no `.created`

`meetings.space.created` solo alcanza a las salas que la app creó **a través de
la API de Meet**. Las nuestras las crea Calendar con `conferenceData.createRequest`,
así que ese scope no las vería. Hace falta `meetings.space.readonly`, que cubre
cualquier sala a la que el usuario tenga acceso.

> ⚠️ Es un scope **sensible**. En modo *testing* funciona con los usuarios de
> prueba, pero **publicar la app exigirá pasar la verificación de Google**. Hasta
> aquí habíamos evitado ese proceso manteniendo el scope mínimo; añadirlo fue una
> decisión consciente a cambio del contador de asistentes.
>
> Además, **todo el que conectó antes tiene que reconectar**: su token no lleva
> el scope nuevo y Google responde 403.

### Cómo se resuelve

Dos llamadas, y la segunda solo cuando hace falta:

1. `GET /v2/spaces/{meetingCode}` — el código sale del propio enlace
   (`meet.google.com/nnv-fbhe-wpc` → `nnv-fbhe-wpc`), que la API acepta como
   alias del space. Si no trae `activeConference`, la sala está vacía y **ahí
   termina**: nos ahorramos la segunda llamada y la cuota.
2. `GET /v2/{conferenceRecord}/participants?filter=latestEndTime IS NULL` — ese
   filtro es como la API marca a quien **sigue dentro**, frente al histórico de
   quien ya salió.

Se consulta con la credencial del **organizador**, no con la de quien pregunta:
Meet exige ser dueño de la sala o estar dentro, y un invitado que todavía no ha
entrado no cumple ninguna de las dos. Como el permiso para ver la sesión ya se
comprobó antes, no se filtra nada que el que pregunta no pudiera ver entrando.

### Tres finales que se distinguen a propósito

| Respuesta | Significado | Qué hace la UI |
|---|---|---|
| `401` | Credencial revocada → `needs_reauth` | Banner de reconexión (como siempre) |
| `403` | Token viejo sin el scope → `ErrMeetScopeMissing` | "Reconecta Google para ver quién está dentro" |
| `404` | La sala aún no existe (nadie entró nunca) | "Sala vacía" — **no es un error** |

El `403` se separa de `needs_reauth` deliberadamente: la cuenta sigue sirviendo
para crear, editar y cancelar, y marcarla como revocada rompería el módulo entero
por un contador. El `404` se trata como sala vacía porque el frontend consulta
esto en bucle y un contador no debe pintar la tarjeta de rojo.

El frontend solo consulta cuando la sesión **está en curso o a punto**
(`startsSoon`), cada 20s y sin reintentos: preguntar por una reunión de la semana
que viene sería gastar cuota para que siempre conteste "vacía".

---

## 7. Series recurrentes

Una serie es **un evento recurrente en Google con un único enlace de Meet** —el
comportamiento por defecto y el que se espera de una reunión periódica—. Se
soportan diaria, semanal, quincenal y mensual, terminando nunca, en una fecha
(`UNTIL`) o tras N repeticiones (`COUNT`).

### El bug que obligó a añadir `series_ends_at`

`start_at`/`end_at` describen solo la **primera** ocurrencia. El listado filtraba
por `end_at >= now`, así que **una sesión diaria desaparecía de "Próximas" en
cuanto terminaba su primer día** y se iba a "Pasadas" para siempre — justo el
caso de uso que motivó la recurrencia.

Ahora se persiste `series_ends_at` (el fin de la ÚLTIMA ocurrencia; `NULL` = la
serie no termina) y el filtro es sobre esa columna. La migración
`202607281600_add_meeting_series_end` la rellena con `end_at` para las sesiones
no recurrentes ya existentes; las recurrentes se quedan en `NULL`, que es
correcto porque hasta ahora no había forma de ponerles fin.

### La próxima ocurrencia se calcula, no se guarda

`NextStartAt`/`NextEndAt` son `gorm:"-"`: se calculan al vuelo en `decorate()`
desde la RRULE. Persistirlas obligaría a un job que las fuera corrigiendo con el
paso del tiempo. El listado además **ordena por ellas**, no por `start_at`: una
serie creada hace un mes tiene un `start_at` viejo y saldría siempre la primera.

Por lo mismo, `/upcoming` recorta **después** de ordenar en Go y no con un
`LIMIT` en SQL, que se llevaría las series con `start_at` más antiguo en vez de
las reuniones más cercanas.

### El salto entre ocurrencias es en hora local

`occurrenceAt` avanza sobre la fecha local con `time.Date` en vez de sumar 24h.
Es la razón de guardar la zona IANA: una reunión de las 09:00 debe seguir siendo
a las 09:00 **después del cambio de horario de verano**, y sumar duraciones fijas
la desplazaría una hora. Cubierto por
`TestNextOccurrenceKeepsLocalTimeAcrossDST`.

### Reglas acotadas a propósito

`ParseRecurrence` acepta solo `FREQ` (DAILY/WEEKLY/MONTHLY), `INTERVAL`, `COUNT`
y `UNTIL`, y **rechaza el resto** (`BYDAY`, `BYSETPOS`…). Aceptar cualquier regla
del estándar obligaría a implementar su expansión completa para poder calcular
las ocurrencias nosotros. La regla se **normaliza** antes de guardarla, así que
en la BD no conviven dos formas de decir lo mismo.

---

## 8. Avisos

A los invitados **internos**: campanita (`notifSvc.CreateNotification`) y DM del
bot "Obertrack" con el enlace, igual que hace `task_service`. A los **externos**
los invita Google por correo (`sendUpdates=all`), que es el único canal que
tenemos con ellos — el formulario lo advierte antes de crear la sesión.

`sendUpdates=all` solo se manda cuando hay invitados; en los eventos de tarea no
hay ninguno, así que la Fase 2 no cambia de comportamiento (`TestEventQuery`).

---

## 9. Archivos

| Capa | Archivo |
|---|---|
| Modelos | `models/meeting.go` |
| Migraciones | `202607281400_add_meetings`, `202607281410_backfill_meetings_permission` |
| Repositorio | `repository/meeting_repository.go` |
| Servicio | `service/meeting_service.go` |
| Cliente Google | `service/google_calendar_service.go` (`CalendarEventInput`, `GetEvent`, `buildEventPayload`) |
| Handler | `handlers/meeting.go` |
| Rutas | `routes/work_routes.go` |
| Frontend | `pages/Meetings.tsx`, `services/meeting.service.ts`, `hooks/useGoogleConnection.ts` |

---

## 10. Pendiente

**Fase B — difusión:**
- `POST /meetings/:id/share` para publicar la sesión en un canal del chat
  (reusando `channelService.SendMessage` + el `broadcast` de
  `postSupportSystemMessage`).
- Widget "Próxima sesión" en el Dashboard, sobre `/meetings/upcoming` (el
  endpoint ya existe).
- Enlace sesión↔tarea en `TaskDetailPanel` (el campo `task_id` y el filtro del
  listado ya existen; falta la UI).

**Edición por instancia** (lo único que queda de la Fase C). Hoy **editar o
cancelar afecta a toda la serie**, y así se advierte en el formulario. Soportar
excepciones obliga a manejar `/events/{id}/instances` y `originalStartTime`, y
multiplica la superficie de error; se dejó fuera a conciencia.
