# Integración Obertrack ↔ Obersuite

Contrato del lado de Obertrack. **Este documento manda**: si algo aquí no
coincide con lo que hace el código, es un fallo — pero mientras no se corrija,
lo que decide qué recibe Obersuite es el código, no lo que se acordó por chat.

Vive en Obertrack a propósito. Los códigos, los campos y las semánticas los fija
el servidor; tenerlos escritos en el repositorio del cliente garantiza que se
queden desactualizados sin que nadie se entere.

**Última revisión:** 10 de septiembre de 2026 · `payload_schema_version` 3

---

## 1. Cómo se autentica

Todas las rutas cuelgan de `/api/integrations/obersuite` y van con un token de
servicio estático, **no** con sesión de usuario:

```
X-Service-Token: <OBERSUITE_SERVICE_TOKEN>
```

Sin el token configurado en el servidor, el grupo entero responde **503**
(fail-closed). Con token ausente o incorrecto en la petición, **401**.

No hay rotación automática ni tokens con caducidad, y es deliberado: son dos
servicios nuestros hablando entre sí por una red que controlamos, y el mecanismo
de rotación sería más superficie de fallo que el riesgo que cubre. Si el token
se filtra, se cambia a mano en las dos partes.

### Límite de peticiones

120 por minuto y por IP sobre todo el grupo. El tráfico real son ~200 peticiones
al día entre 28 personas, así que el límite no está para racionar el uso normal:
está para que un bucle de reintentos no arrastre a la aplicación entera.

Al pasarse se devuelve **429** con `Retry-After: 60`. **El 429 no es un fallo de
la operación**: no hay que reintentarlo de inmediato ni marcar nada como
rechazado, hay que esperar el minuto. Es la única excepción a "cualquier código
que no sea 2xx significa que algo salió mal".

---

## 2. `GET /version` — qué está desplegado

```json
{ "service": "obertrack", "commit": "a1b2c3d", "payload_schema_version": 3 }
```

Existe porque el fallo más caro de esta integración no fue de código: fue que
ninguno de los dos lados podía saber si un cambio había llegado a producción. Se
deducía mirando el JSON recibido o contando bytes de respuesta, y se llegó a dar
por desplegado algo que seguía sin subir.

- `commit` sale de `BUILD_COMMIT` (que la imagen recibe como `--build-arg`) o de
  la información que Go incrusta al compilar desde git. Si no hay ninguna de las
  dos, dice `"desconocido"` — nunca vacío, porque un campo vacío parece un fallo
  y un campo ausente parece que no lo tenemos.
- `payload_schema_version` sube cuando se añade, se quita o cambia de
  significado un campo del padrón. Historia: **1** ficha inicial · **2** se
  añaden `last_contact_at`, `updated_at` y ETag · **3** se retiran los campos
  que nadie consumía.

Antes de abrir una incidencia por "esto no me llega", conviene mirar aquí.

---

## 3. `GET /companies` — el padrón de empresas

Devuelve un **array JSON plano**. Sin envoltorio, sin paginación. Se evaluaron
las dos cosas y se **descartaron**, no están pendientes: el padrón son ~90
empresas y con el ETag la respuesta casi siempre es un 304 de cero bytes.
Volverlo a proponer sin que el volumen haya cambiado es rehacer la discusión.

### Campos

| Campo | Tipo | Notas |
|---|---|---|
| `id` | número | El `company_id` que hay que mandar en el hire |
| `name` | texto | |
| `status` | `"active"` \| `"suspended"` | Una suspendida **rechaza contrataciones** (422) |
| `responsible_name` | texto | |
| `responsible_email` | texto | |
| `industry` | texto | |
| `country`, `state`, `city`, `address` | texto | Ubicación normalizada |
| `professionals_count` | número | Profesionales con empleo activo en la empresa **o** adscritos a ella directamente, sin duplicar |
| `boards_count` | número | Tableros del espacio de la empresa |
| `tasks_count` | número | Tareas del espacio de la empresa |
| `last_contact` | texto | En español, para mostrar: `"hoy"`, `"hace 3 días"`, `"nunca"` |
| `last_contact_at` | ISO 8601 o `null` | La misma fecha en crudo, para ordenar o comparar |
| `updated_at` | ISO 8601 | Base del corte incremental |

Las fechas opcionales vienen como `null` cuando nunca ocurrieron. **No** se
aplanan a la fecha cero: `"0001-01-01"` diría que pasó en el año 1.

**Una empresa borrada desaparece del array.** No hay campo `deleted` ni tumba.
Quien sincronice en incremental tiene que asumir que la ausencia es la baja, y
por eso una sincronización incremental no puede detectar bajas por sí sola: para
eso hace falta una pasada completa (sin `updated_since`) cada cierto tiempo.

### Campos que se retiraron y no van a volver

`last_activity_at`, `hours_this_month`, `pending_hours`, `pending_count`,
`rejected_count`, `location`, `created_at`, `client_since`, `phone_number`,
`open_tickets`.

Dos motivos, y conviene no perderlos:

1. **Nadie los consumía.** Se confirmó recorriendo los dos puntos de parseo del
   lado de Obersuite.
2. **Rompían el ETag.** `last_activity_at` lo alimenta el contador de uso, que
   escribe cada 30 segundos: bastaba con que alguien tuviera la aplicación
   abierta para que el cuerpo cambiara y el ETag no acertara jamás. Los de
   operación tenían el mismo problema y además exponían cuánto trabaja la gente
   de cada cliente. `open_tickets` era el peor: costaba una subconsulta con
   normalización de teléfonos y habría rotado el ETag para llenar una pestaña
   que hoy enseña un número escrito a mano.

`location` era texto libre que la gente rellena a mano — llegó a contener
`"Responsable de la gestión y c…"` en producción. Para ubicación están
`country`/`state`/`city`, que sí están normalizados.

Hay una prueba (`TestListCompanies_NoSalenLosCamposRetirados`) que inspecciona
el JSON serializado y falla en cuanto cualquiera de ellos reaparezca.

### Caché: ETag e `If-None-Match`

```
ETag: W/"a1b2c3…"
Cache-Control: private, max-age=0, must-revalidate
```

Mandando `If-None-Match: <el ETag anterior>` se recibe **304** sin cuerpo.

El ETag se calcula sobre el **cuerpo exacto** que se devuelve, no sobre un
`MAX(updated_at)`. Es más caro, pero es lo correcto: la ficha lleva contadores
que cambian sin tocar la fila de la empresa, y el propio `"hace 8 días"` se
recalcula al pasar la medianoche. Un validador basado en `updated_at` diría "no
ha cambiado nada" mientras esos números ya son otros — peor que no cachear.

Es débil (`W/`) a propósito: garantiza equivalencia para quien lo consume, no
que los bytes sean idénticos.

**Los proxies pueden interferir.** Si el 304 no llega, lo primero que hay que
mirar es si algo por el camino está reescribiendo o eliminando el `ETag` /
`If-None-Match`, antes de suponer un fallo del servidor.

### Corte incremental

```
GET /companies?updated_since=2026-09-09T13:00:00Z
```

RFC 3339. Devuelve solo lo que cambió después de ese instante. **Sin el
parámetro devuelve el padrón entero**, que es como funcionaba antes.

Una fecha que no se entiende se rechaza con **400** en vez de ignorarse:
tratarla como "sin filtro" devolvería todo y quien llama creería estar
recibiendo solo lo nuevo. Ese es el fallo silencioso que se quiso evitar.

---

## 4. `POST /hire` — la contratación

Crea o reutiliza al profesional y le abre el empleo en la empresa.

### Cuerpo

```jsonc
{
  "external_id": "cand_8891",       // id del candidato en Obersuite
  "email": "ana@ejemplo.com",        // OBLIGATORIO
  "name": "Ana Pérez",               // OBLIGATORIO
  "company_id": 42,                  // OBLIGATORIO (el id de /companies)
  "identity_document": "12345678A",
  "phone_number": "+34600000000",
  "country": "España", "state": "Aragón", "city": "Zaragoza",
  "address": "Calle Mayor 1",
  "job_title": "Diseñadora",
  "started_at": "2026-09-15",        // YYYY-MM-DD; por defecto, hoy
  "cv": {
    "file_name": "cv-ana.pdf",
    "mime_type": "application/pdf",
    "content_base64": "JVBERi0…"
  }
}
```

El CV va **embebido en base64**, no como URL temporal, para que la contratación
sea atómica y aguante los reintentos sin depender de que un enlace siga vivo.

- Máximo **8 MB** ya decodificado.
- Tipos aceptados: PDF, DOC, DOCX, XLS, XLSX, JPEG, PNG, GIF, WEBP.
- **El CV es best-effort.** Si falla, la contratación **igual se completa** y se
  devuelve `cv_attached: false` con un `cv_warning` explicando qué pasó. No hay
  que reintentar el hire por un CV rechazado: se crearía trabajo duplicado sin
  arreglar el archivo.

### Respuesta (200)

```json
{
  "user_id": 1201, "employment_id": 3345, "obersuite_id": "cand_8891",
  "status": "created", "cv_attached": true, "induction_pending": true
}
```

`status` distingue tres cosas que **todas son éxito**:

| Valor | Qué pasó |
|---|---|
| `created` | Profesional nuevo + empleo nuevo |
| `rehired` | Ya existía la persona; se le abrió un empleo nuevo |
| `already_active` | Ya tenía empleo activo en esa empresa — **no-op idempotente** |

`already_active` responde **200**, no 409. Es lo que corta los reintentos del
webhook, y por eso desde esa rama **no se envía ningún correo**: reenviar la
inducción en cada reintento le llenaría la bandeja al profesional con el mismo
enlace.

`induction_pending: true` significa que la persona quedó sin acceso hasta
aprobar la capacitación; se le mandó el enlace a la landing en vez de las
credenciales.

### Identidad: cómo se decide si la persona ya existe

**Primero por `external_id`, después por email.** El orden importa: el email
solo identifica mientras nadie lo cambie, y en cuanto un candidato se registra
con uno y llega el alta con otro, resolver por email crea una segunda cuenta de
la misma persona.

Mandar `external_id` siempre que se tenga. La primera contratación que llega con
él lo estampa también sobre las personas que ya estaban aquí de antes, y a
partir de ahí quedan identificadas en los dos sistemas.

### Códigos de respuesta

Esta tabla **es** el contrato. La lógica de reintentos de Obersuite depende
literalmente de ella.

| Código | Significa | ¿Reintentar? |
|---|---|---|
| **200** | Contratación materializada (`created`, `rehired` o `already_active`) | No: ya está |
| **400** | Falta un campo o su valor no vale (email mal escrito, sin `company_id`) | **No.** Hay que corregir el dato |
| **401 / 503** | Token incorrecto / token no configurado en el servidor | No: es configuración |
| **404** | El `company_id` no existe o no es una empresa | **No.** Hay que corregir el dato |
| **409** | Ya hay una cuenta con ese email y **no** es un profesional (empresa, superadmin, CS): no se puede convertir | **No.** Intervención humana |
| **422** | La empresa está suspendida | **No.** Hay que reactivarla en Obertrack |
| **429** | Límite de peticiones | Sí, **tras esperar `Retry-After`** |
| **500** | Fallo nuestro | **Sí** |

Regla de una línea: **los 4xx no se reintentan; el 429 se espera; el 500 sí.**

Un error que no reconocemos cae a 500 a propósito. Si el fallo es nuestro, que
lo reintenten es lo correcto; darlo por 400 les haría descartar en firme una
contratación que sí podía salir.

> **El 409 no es éxito.** Se dice explícitamente porque llegó a escribirse
> `esExito(409) = true` del lado de Obersuite, suponiendo qué significaba. Con
> eso, una contratación rechazada se habría marcado como HIRED y nadie se habría
> enterado. Se corrigió antes de producción.

### Los mensajes de error se le enseñan a una persona

Obersuite muestra nuestro `error` **tal cual** al reclutador. Por eso:

- Los **4xx** traen un texto accionable en español (`"la empresa está
  suspendida: hay que reactivarla en Obertrack antes de contratar"`).
- Los **5xx** traen un texto genérico; el detalle técnico se queda en nuestro
  log. Un `pq: connection refused` en la cara de quien contrata no le dice nada
  y encima filtra cómo estamos hechos por dentro.

### Los 4xx no crean nada

Validación → búsqueda de la empresa → resolución de identidad ocurren **antes**
de cualquier escritura. Se puede probar `400`, `404` y `422` contra producción
sin que quede ningún profesional ni empleo a medias.

El `409` sí requiere una cuenta existente que no sea profesional, pero tampoco
escribe.

---

## 5. Lo que se decidió NO hacer

Que quede escrito evita volver a discutirlo cada trimestre.

| Propuesta | Estado | Por qué |
|---|---|---|
| Paginación en `/companies` | **Descartada** | ~90 empresas y ETag: la respuesta habitual es un 304 vacío |
| Envoltorio `{data, meta}` | **Descartada** | Rompe a los dos consumidores actuales sin dar nada a cambio |
| Rotación automática de tokens | **Descartada** | Dos servicios nuestros; el mecanismo sería más frágil que el riesgo |
| `GET /companies/:id` (detalle) | **Aparcada** | Nadie la necesita hoy. Se retoma si aparece una pantalla de detalle |
| Devolver campos de operación | **Descartada** | Ver §3: rompían el ETag y exponen datos de los clientes |

---

## 6. Al cambiar algo de aquí

1. Subir `PayloadSchemaVersion` en `backend/internal/handlers/version.go` si
   cambia la forma del padrón.
2. Actualizar este documento **en el mismo commit**.
3. Avisar a Obersuite con el commit concreto, y que ellos comprueben
   `GET /version` **después** del despliegue. No dar nada por desplegado sin ese
   paso: es exactamente el error que motivó el endpoint.

### Dónde está cada cosa

| Qué | Archivo |
|---|---|
| Rutas y token | `backend/internal/routes/public_routes.go` |
| Handlers, ETag, códigos | `backend/internal/handlers/onboarding.go` |
| Versión y esquema | `backend/internal/handlers/version.go` |
| Lógica de hire y padrón | `backend/internal/service/onboarding_service.go` |
| SQL del padrón | `backend/internal/repository/user_repository.go` |
| Límite de peticiones | `backend/internal/middleware/ratelimit.go` |
