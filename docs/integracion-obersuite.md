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
{
  "service": "obertrack",
  "commit": "ea245b64",
  "commit_source": "SOURCE_COMMIT",
  "payload_schema_version": 3,
  "started_at": "2026-09-10T18:15:45Z"
}
```

Existe porque el fallo más caro de esta integración no fue de código: fue que
ninguno de los dos lados podía saber si un cambio había llegado a producción. Se
deducía mirando el JSON recibido o contando bytes de respuesta, y se llegó a dar
por desplegado algo que seguía sin subir.

- `payload_schema_version` sube cuando se añade, se quita o cambia de
  significado un campo del padrón. Historia: **1** ficha inicial · **2** se
  añaden `last_contact_at`, `updated_at` y ETag · **3** se retiran los campos
  que nadie consumía. **Este es el número que le importa a quien consume**: el
  commit dice qué hay desplegado, la versión del esquema dice si le afecta.
- `commit` se busca en `BUILD_COMMIT` (la nuestra, vía `--build-arg`) y después
  en las que publica sola la plataforma: `SOURCE_COMMIT` (Coolify), `COMMIT_SHA`,
  `GIT_COMMIT`, `GITHUB_SHA`. En desarrollo cae a lo que Go incrusta al compilar
  desde git. Si no aparece por ningún lado dice `"desconocido"` en vez de
  inventárselo: un commit falso es peor que ninguno, porque se le cree.
- `commit_source` dice **de dónde** salió, y no es un adorno: un `"desconocido"`
  y un commit real se leen igual de bien, así que sin el origen no se distingue
  "no hay commit" de "el despliegue no publica la variable". Ese fue exactamente
  nuestro caso en el primer despliegue con este endpoint. La idea es de
  Obersuite, que lo resolvió antes en su lado.
- `started_at` delata un reinicio que nadie pidió. Si sube solo, algo se está
  cayendo y volviendo a levantar — y eso explica síntomas que, de otro modo, se
  persiguen por el lado equivocado.

Antes de abrir una incidencia por "esto no me llega", conviene mirar aquí.

Obersuite expone lo simétrico en `GET /api/version`, con `contrato_obertrack`
como su número de contrato hacia nosotros.

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
| `GET /companies/:id` (detalle) | **Reabierta** (10-sep-2026) | Se aparcó porque nadie la necesitaba. Apareció la pantalla que la necesita — ver abajo |
| Devolver campos de operación | **Descartada** | Ver §3: rompían el ETag y exponen datos de los clientes |

### El detalle de empresa, reabierto

Obersuite tiene una ficha de empresa con ocho pestañas —Uso, Profesionales,
Organigrama, Expediente, Actividad, Tickets, Archivados, Horarios— clonadas de
nuestra pantalla de Empresas. Hoy las ocho dicen "Sin datos disponibles" porque
retiraron los datos de ejemplo que llevaban; eso está bien resuelto, un hueco
honesto vale más que un número inventado.

Lo que hay que saber antes de construirlo:

- **La objeción del ETag no aplica.** Los campos de operación se quitaron del
  padrón porque rotaban el validador de una lista que se pide entera. Un detalle
  es por empresa y bajo demanda: no toca esa caché.
- **Los datos ya existen**, todos, como endpoints de administración. Es
  reexportar lo que la pantalla de Empresas ya muestra, no calcular nada nuevo.
- **Uso y Horarios son datos por persona** (cuánto entra cada profesional, qué
  jornada tiene) y **Expediente y Actividad son las notas internas de Customer
  Success sobre el cliente**. `CompanyEvent` no tiene campo de visibilidad, así
  que ahí es todo o nada. Decidido el 10-sep-2026: Obersuite es interno de
  Oberstaff, se tratan los ocho bloques igual.
#### Las rutas

Un bloque por ruta, bajo `/integrations/obersuite/companies/:id`. Sueltas y no
en una respuesta con todo dentro porque las pestañas se abren de una en una: la
que nadie abre no se pide, y entonces no cuesta nada.

| Ruta | Pestaña | Devuelve |
|---|---|---|
| `GET .../:id` | cabecera | Ficha completa, **incluida la operación del mes** (horas, pendientes, rechazadas) que se retiró del padrón |
| `GET .../:id/professionals` | Profesionales **y Horarios** | La plantilla. Los cuatro campos de horario viajan en cada fila: Horarios es una proyección de esta lista, no otro bloque |
| `GET .../:id/org-chart` | Organigrama | El árbol completo, sin recorte por rol |
| `GET .../:id/timeline` | **Expediente** | La cronología. `?category=`, `?person_id=`, `?page=` (50 por página) |
| `GET .../:id/attention` | **Actividad** | Inactividad + ausencias. `?days=`, `?month=`, `?year=` |
| `GET .../:id/tickets` | Tickets | Los de soporte, de cualquier origen |
| `GET .../:id/archived` | Archivados | Empleos terminados y cuentas desactivadas |
| `GET .../:id/usage` | Uso | Resumen, módulos y personas. `?days=` (30 por defecto), `?search=`, `?status=`, `?page=` |

**Cuidado con los nombres de dos pestañas.** En nuestra pantalla "Actividad"
**no** es la cronología: es el panel de inactividad y ausencias. La cronología
es "Expediente". Están clonadas las etiquetas, así que es fácil cablear una en
la otra y que parezca que funciona.

Todos los bloques validan el `:id` aunque venga de un padrón que acabamos de
servir: entre cachearlo y abrir una ficha pueden pasar horas. **404** si la
empresa ya no está, **400** si el id no es un número. Consultar por un id
inexistente devolvería listas vacías, que se leen como "esta empresa no tiene
nada".

**Los filtros del expediente viajan en la respuesta** (`categories`, con valor y
etiqueta) en vez de dejar que el cliente los escriba. Es la defensa contra la
deriva silenciosa: cuando aquí se renombra o se fusiona una categoría —pasó esta
semana con `staff`, que se metió dentro de `lifecycle`— una lista repetida del
otro lado sigue funcionando y enseñando un filtro que ya no devuelve nada. Lo
protege `TestCategoriasDelExpediente_CoincidenConLasDelFrontend`, que lee el
archivo del frontend y falla si las dos listas se separan.

`counts` trae más claves que `categories`, a propósito: `staff` y `management`
ya no se ofrecen como filtro pero sus movimientos siguen existiendo y se pueden
pedir por `?category=`. **Los chips se pintan desde `categories`, nunca
recorriendo las claves de `counts`.**

#### Los campos de `professionals[]`

| Campo | Tipo | Notas |
|---|---|---|
| `id` | número | El usuario en Obertrack |
| `name`, `email`, `avatar` | texto | `avatar` vacío si no tiene |
| `user_type` | texto | Siempre `"profesional"`: la lista se filtra a ese tipo para cuadrar con `professionals_count` |
| `is_active` | booleano | **Siempre presente.** Es la cuenta, no el empleo |
| `is_manager`, `is_supervisor` | booleano | Siempre presentes |
| `job_title` | texto | El del empleo **en esta empresa**; cae al del perfil si el empleo no lo tiene |
| `obersuite_id` | texto, **se omite si no hay** | El candidato en Obersuite: esta *persona* vino de allí |
| `hire_obersuite_id` | texto, **se omite si no hay** | **Esta contratación concreta** la hizo Obersuite |
| `started_at` | ISO 8601 o `null` | Ingreso **en esta empresa**. Nulo si está vinculada sin empleo escrito |
| `schedule_type`, `schedule_days`, `schedule_start_time`, `schedule_end_time` | texto | Del empleo **en esta empresa**. Vacíos si nadie le puso jornada |
| `hours_this_month` | número | Del mes corriente |
| `tasks_assigned`, `tasks_completed` | número | |
| `last_active` | ISO 8601 o `null` | Última jornada registrada |
| `is_primary_company` | booleano | Si esta es la empresa que el usuario tiene activa. `false` = trabaja aquí pero ve otra al entrar en la app |

Los dos identificadores de Obersuite **no son lo mismo** y por eso van los dos:
alguien puede venir de Obersuite (`obersuite_id`) y que este empleo concreto lo
abriéramos nosotros a mano (`hire_obersuite_id` ausente). Van como texto y se
omiten cuando no hay vínculo — no hay booleano `from_obersuite` ni `is_obersuite`.

**`is_active` siempre viene**, así que no hace falta suponer nada cuando falta:
no falta. Y es el estado de la *cuenta*; que alguien esté en esta lista ya
significa que su empleo aquí está activo.

##### La lista cuadra con el contador, y no cuadraba

`len(professionals)` es igual a `professionals_count` del padrón por
construcción: las dos consultas usan el mismo criterio —empresa principal **o**
empleo activo aquí—.

No era así. La consulta del panel de administración filtra solo por empresa
principal, y eso deja fuera a los **recontratados**: quien ya trabajaba en otra
empresa conserva su `empleador_id` y aquí solo gana un empleo. El padrón sí los
cuenta. Medido antes de arreglarlo: una empresa con `professionals_count = 4`
devolvía 3 personas.

Es justo la población que produce este puente —los `rehired`—, así que habría
salido a la primera. Y el segundo fallo era peor porque no se nota: al leer el
horario y la fecha de ingreso anclados a la empresa principal, un recontratado
salía con **los datos de su otra empresa**. Un dato equivocado que parece bueno.

#### Y lo que esto NO es

Es **de solo lectura**. Un espejo va en un sentido: si alguien escribe una nota
desde Obersuite, no vuelve. Conviene decirlo antes de que se prometa lo
contrario, porque la pregunta que se hizo fue si los cambios en uno se ven en el
otro, y la respuesta hoy es "los de aquí allí sí; los de allí aquí no".

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
