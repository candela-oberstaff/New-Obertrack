# Datos de demostración (seeder)

Comando que llena una base de datos de desarrollo con una aplicación *usable*:
dos empresas con su gente, tableros con tareas, seis semanas de jornadas, chat,
tickets de soporte, incidentes y notificaciones. Está pensado para que alguien
que acaba de clonar el repositorio no tenga que fabricar todo a mano por la UI.

Código: [`backend/cmd/seed/`](../backend/cmd/seed/).

## Cómo se corre

La base de datos **no publica su puerto al host** (decisión de seguridad,
hallazgo M-03), así que el camino normal es desde dentro de la red de Docker:

```bash
docker compose --profile seed run --rm seeder            # sembrar
docker compose --profile seed run --rm seeder -reset     # borrar lo anterior y volver a sembrar
docker compose --profile seed run --rm seeder -reset-only # solo borrar
```

El servicio `seeder` vive únicamente en `docker-compose.override.yaml` (nunca en
el compose de producción) y está bajo un *profile*: `docker compose up` no lo
levanta por accidente.

Si tienes Go y una base alcanzable desde el host, también vale:

```bash
cd backend && go run ./cmd/seed
```

Necesita las mismas variables que el backend para conectarse: `DB_HOST`,
`DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`, `DB_SSL_MODE`. Las toma del
entorno o del `.env` de la raíz. No necesita `JWT_SECRET` ni ninguna credencial
de integración.

### Opciones

| Flag | Por defecto | Para qué |
|---|---|---|
| `-reset` | `false` | Borra los datos de demo previos antes de sembrar |
| `-reset-only` | `false` | Solo borra y termina |
| `-password` | `Obertrack2026!` | Contraseña de todas las cuentas sembradas |
| `-migrate` | `true` | Corre las migraciones antes de sembrar (sirve para partir de una base vacía) |
| `-verbose` | `false` | Registra cada consulta SQL |
| `-force` | `false` | Permite correr con `GIN_MODE=release` |

## Cuentas

Contraseña de todas: **`Obertrack2026!`**

| Rol | Correo |
|---|---|
| Superadmin | `superadmin@demo.obertrack.test` |
| Analista de IT | `it@demo.obertrack.test` |
| Customer Success | `cs.carmen@demo.obertrack.test`, `cs.pedro@demo.obertrack.test` |
| Empresa · Acme S.A. | `acme.marta@demo.obertrack.test` |
| Manager · Acme | `acme.diego@demo.obertrack.test` |
| Profesionales · Acme | `acme.laura@`, `acme.valentina@`, `acme.hugo@` |
| Empresa · Globex Corp | `globex.gabriela@demo.obertrack.test` |
| Manager · Globex | `globex.andres@demo.obertrack.test` |
| Profesionales · Globex | `globex.sofia@`, `globex.mateo@` |

## Qué queda cargado

- **Empresas y equipos**: dos tenants con empleador, manager y profesionales, con
  su `employment` (expediente) y el manager principal en la tabla N-a-N.
- **Roles y grupos**: los cuatro presets por empresa (Colaborador, Supervisor,
  Solo lectura, Soporte), asignados, más un grupo "Equipo core".
- **Tableros**: dos, con sus tres fases y diez tareas repartidas entre los tres
  estados, con asignados, fechas y comentarios.
- **Jornadas**: ~30 días hábiles por profesional con la mezcla que hace falta
  para probar el módulo — aprobadas (histórico), pendientes (la cola del
  manager), una rechazada con motivo y una ausencia justificada.
- **Chat**: un canal `general` por empresa con conversación.
- **Soporte**: tres tickets, uno en cola, uno asignado y uno resuelto, cada uno
  con su canal privado y los agentes dentro (igual que los crea `ContactSupport`).
- **Incidentes**: uno abierto con respuestas del equipo y uno cerrado.
- **Notificaciones**: bandeja con algo que leer en varias cuentas.

Casos que vale la pena conocer porque son fáciles de pasar por alto al probar:

- **`acme.laura` trabaja en las dos empresas** → ejercita el switcher multiempresa.
- **`globex.mateo` tiene rol "Solo lectura"** → ejercita los permisos por módulo.

## Reglas de la casa

**Todo lo que crea el seeder vive bajo el dominio `demo.obertrack.test`.** No es
cosmético: es lo que hace que `-reset` sea quirúrgico. El borrado identifica las
filas por ese dominio (o por la empresa de alguien de ese dominio) y no toca nada
más, así que se puede correr en una base que tenga datos reales al lado. El TLD
`.test` está reservado por la RFC 2606, así que ningún correo de prueba puede
salir de verdad hacia una bandeja real.

Es **idempotente**: correrlo dos veces no duplica nada. Lo que ya existe se
respeta —si editaste una tarea sembrada, sigue como la dejaste—, salvo las
notificaciones, que se reescriben en cada corrida.

Se **niega a correr con `GIN_MODE=release`** salvo que se insista con `-force`:
crea cuentas con una contraseña conocida y publicada en este documento.

## Qué NO hace

No crea el superadmin de un despliegue real. Para eso siguen estando las rutas
de bootstrap `POST /api/seed/superadmin` (protegidas por `SEED_BOOTSTRAP_TOKEN`
y deshabilitadas en release), que son otra cosa: alta inicial en un entorno de
verdad, no datos de demostración.
