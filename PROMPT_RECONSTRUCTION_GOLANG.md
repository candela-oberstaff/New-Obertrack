# PROMPT COMPLETO: Obertrack - Sistema de Gestión de Personal

## Stack Propuesto

- **Backend:** Go (Golang) con Gin o Fiber
- **Frontend:** React 19 + TypeScript + Vite
- **Base de datos:** Postgres
- **Auth:** JWT
- **WebSocket:** Para chat en tiempo real (ws o gorilla/websocket)

---

## 1. MODELOS DE BASE DE DATOS

### Users (Usuarios)
| Campo | Tipo | Descripción |
|-------|------|-------------|
| id | BIGINT, PK | ID único |
| name | VARCHAR(255) | Nombre completo |
| email | VARCHAR(255), UNIQUE | Email |
| password | VARCHAR(255) | Hash bcrypt |
| avatar | VARCHAR(500) | URL del avatar |
| tipo_usuario | ENUM('empleador', 'empleado') | Tipo de usuario |
| is_manager | BOOLEAN | Si es manager |
| is_superadmin | BOOLEAN | Si es super admin |
| empleador_id | BIGINT, FK | ID del empleador (para empleados) |
| company_name | VARCHAR(255) | Nombre de empresa (empleadores) |
| job_title | VARCHAR(255) | Puesto de trabajo |
| phone_number | VARCHAR(50) | Teléfono |
| country | VARCHAR(100) | País |
| city | VARCHAR(100) | Ciudad |
| location | TEXT | Dirección |
| google_calendar_token | TEXT | Token Google Calendar |
| google_forms_token | TEXT | Token Google Forms |
| remember_token | VARCHAR(100) | Token remember me |
| email_verified_at | TIMESTAMP | Verificación email |
| created_at | TIMESTAMP | Fecha creación |
| updated_at | TIMESTAMP | Fecha actualización |

### Tasks (Tareas)
| Campo | Tipo | Descripción |
|-------|------|-------------|
| id | BIGINT, PK | ID único |
| title | VARCHAR(255) | Título |
| description | TEXT | Descripción |
| status | ENUM('por_hacer', 'en_proceso', 'finalizado') | Estado |
| priority | ENUM('low', 'medium', 'high', 'urgent') | Prioridad |
| start_date | DATE | Fecha inicio |
| end_date | DATE | Fecha límite |
| completed | BOOLEAN | Si está completada |
| created_by | BIGINT, FK | Usuario que creó |
| created_at | TIMESTAMP | Fecha creación |
| updated_at | TIMESTAMP | Actualización |

### Task_User (Tabla pivote task-asignados)
| Campo | Tipo |
|-------|------|
| task_id | BIGINT, FK |
| user_id | BIGINT, FK |

### Comments (Comentarios de tareas)
| Campo | Tipo | Descripción |
|-------|------|-------------|
| id | BIGINT, PK | ID único |
| task_id | BIGINT, FK | Tarea asociada |
| user_id | BIGINT, FK | Usuario que comenta |
| content | TEXT | Contenido |
| created_at | TIMESTAMP | Fecha creación |
| updated_at | TIMESTAMP | Actualización |

### Task_Attachments (Archivos de tareas)
| Campo | Tipo | Descripción |
|-------|------|-------------|
| id | BIGINT, PK | ID único |
| task_id | BIGINT, FK | Tarea asociada |
| uploaded_by | BIGINT, FK | Usuario que subió |
| filename | VARCHAR(255) | Nombre original |
| stored_filename | VARCHAR(255) | Nombre guardado |
| mime_type | VARCHAR(100) | Tipo MIME |
| file_size | BIGINT | Tamaño en bytes |
| created_at | TIMESTAMP | Fecha creación |

### WorkHours (Horas trabajadas)
| Campo | Tipo | Descripción |
|-------|------|-------------|
| id | BIGINT, PK | ID único |
| user_id | BIGINT, FK | Usuario |
| work_date | DATE | Fecha de trabajo |
| hours_worked | DECIMAL(5,2) | Horas trabajadas |
| start_time | TIME | Hora inicio |
| end_time | TIME | Hora fin |
| approved | BOOLEAN | Si está aprobada |
| approved_by | BIGINT, FK | Usuario que aprobó |
| approved_at | TIMESTAMP | Fecha aprobación |
| comments | TEXT | Comentarios |
| created_at | TIMESTAMP | Fecha creación |
| updated_at | TIMESTAMP | Actualización |

### RecoveryHours (Horas de recuperación)
| Campo | Tipo | Descripción |
|-------|------|-------------|
| id | BIGINT, PK | ID único |
| user_id | BIGINT, FK | Usuario |
| recovery_date | DATE | Fecha de recuperación |
| hours_recovered | DECIMAL(5,2) | Horas recuperadas |
| activities | TEXT | Actividades realizadas |
| approved | BOOLEAN | Si está aprobada |
| created_at | TIMESTAMP | Fecha creación |
| updated_at | TIMESTAMP | Actualización |

### MassEmailLogs (Logs de emails masivos)
| Campo | Tipo | Descripción |
|-------|------|-------------|
| id | BIGINT, PK | ID único |
| user_id | BIGINT, FK | Usuario que envió |
| segment | VARCHAR(50) | Segmento destinatario |
| subject | VARCHAR(255) | Asunto |
| recipient_count | INT | Cantidad destinatarios |
| created_at | TIMESTAMP | Fecha envío |

### Notifications (Notificaciones)
| Campo | Tipo | Descripción |
|-------|------|-------------|
| id | BIGINT, PK | ID único |
| user_id | BIGINT, FK | Usuario destino |
| type | VARCHAR(50) | Tipo notificación |
| title | VARCHAR(255) | Título |
| message | TEXT | Mensaje |
| data | JSON | Datos adicionales |
| read_at | TIMESTAMP | Fecha lectura |
| created_at | TIMESTAMP | Fecha creación |

---

## 2. ROLES Y PERMISOS

| Rol | Condición | Acceso |
|-----|-----------|--------|
| Super Admin | `is_superadmin = true` | TODO el sistema |
| Empleador | `tipo_usuario = 'empleador'` | Su empresa, sus profesionales |
| Manager | `is_manager = true` | Equipo que gestiona |
| Profesional | `tipo_usuario = 'empleado'` | Solo sus tareas y horas |

### Permisos específicos:
- `admin.access` - Panel admin
- `empresa.access` - Panel empresa
- `manager.access` - Gestión de equipo
- `professional.access` - Acceso empleado
- `tasks.create` - Crear tareas
- `tasks.assign` - Asignar tareas
- `chat.access` - Acceso al chat
- `ai.access` - Acceso a AI
- `whatsapp.access` - Acceso WhatsApp
- `work-hours.register` - Registrar horas
- `work-hours.approve` - Aprobar horas
- `mass-communication` - Comunicaciones masivas

---

## 3. FUNCIONALIDADES PRINCIPALES

### A. Dashboard
- **Dashboard Unificado:** Muestra información diferente según el rol
- **Stats cards:** Tarjetas con estadísticas (profesionales, empresas, tareas, horas)
- **Resumen de profesionales:** Lista de profesionales con horas y tareas
- **Tareas pendientes:** Lista de tareas del usuario
- **Fechas límite:** Próximas fechas de entrega

### B. Gestión de Tareas (CRUD completo)
- **Crear tarea:** Título, descripción, asignar a profesional, fechas, prioridad
- **Editar tarea:** Modificar todos los campos
- **Eliminar tarea:** Soft delete
- **Cambiar estado:** Por hacer → En proceso → Finalizado
- **Cambiar prioridad:** Low → Medium → High → Urgent
- **Comentarios:** Sistema de comentarios en tareas
- **Archivos:** Subir y descargar archivos adjuntos
- **Notificaciones:** Al crear, asignar, completar

### C. Registro de Horas
- **Registro diario:** Fecha, horas trabajadas, hora inicio/fin
- **Comentarios:** Agregar notas
- **Aprobación:** Manager/empleador aprueba horas
- **Horas de recuperación:** Registrar horas de recuperación
- **Resumen semanal/mensual:** Estadísticas de horas

### D. Reports / Reportes
- **Reporte por profesional:** Horas trabajadas por fecha
- **Filtros por mes:** Seleccionar mes específico
- **Descargar PDF:** Exportar reporte en PDF
- **Resumen por empresa:** Todas las horas de la empresa

### E. Chat
- **Chat en tiempo real:** WebSocket
- **Salas por empresa:** Chat privado por empresa
- **Mensajes:** Texto, emojis
- **Notificaciones:** Nuevos mensajes

### F. AI Chat
- **Chat con IA:** Integración con API de IA
- **Historial:** Guardar conversaciones

### G. WhatsApp
- **Chat WhatsApp:** Integración con Waha (WhatsApp HTTP API)
- **Envío masivo:** Enviar mensajes a varios contactos

### H. Google Integrations
- **Google Calendar:** Conectar cuenta, sincronizar eventos
- **Google Forms:** Crear y gestionar formularios

### I. Email Masivo
- **Segmentación:** Todos los profesionales, empresas, alertas rojas, amarillas
- **Plantillas:** Guardar y usar plantillas
- **Estadísticas:** Logs de envíos

---

## 4. RUTAS DEL API (Go/Gin)

### Auth
```
POST   /api/auth/register
POST   /api/auth/login
POST   /api/auth/logout
GET    /api/auth/me
POST   /api/auth/refresh
```

### Users
```
GET    /api/users
POST   /api/users
GET    /api/users/{id}
PUT    /api/users/{id}
DELETE /api/users/{id}
POST   /api/users/{id}/toggle-status
PUT    /api/users/{id}/promote-manager
PUT    /api/users/{id}/toggle-superadmin
```

### Tasks
```
GET    /api/tasks
POST   /api/tasks
GET    /api/tasks/{id}
PUT    /api/tasks/{id}
DELETE /api/tasks/{id}
POST   /api/tasks/{id}/toggle-completion
GET    /api/tasks/{id}/details
POST   /api/tasks/{id}/comments
POST   /api/tasks/{id}/attachments
GET    /api/tasks/attachments/{id}/download
```

### Work Hours
```
GET    /api/work-hours
POST   /api/work-hours
PUT    /api/work-hours/{id}
POST   /api/work-hours/approve
POST   /api/work-hours/approve-days
GET    /api/work-hours/download-report
```

### Recovery Hours
```
GET    /api/recovery-hours
POST   /api/recovery-hours
PUT    /api/recovery-hours/{id}
POST   /api/recovery-hours/{id}/approve
```

### Reports
```
GET    /api/reports
GET    /api/reports/professional/{id}
GET    /api/reports/download/{id}
```

### Chat
```
GET    /api/chat/messages
POST   /api/chat/messages
WS     /ws/chat
```

### Integrations
```
GET    /api/google/calendar/connect
GET    /api/google/calendar/callback
POST   /api/google/calendar/disconnect
GET    /api/google/forms/connect
GET    /api/google/forms/callback
POST   /api/whatsapp/send
GET    /api/whatsapp/status
```

---

## 5. COMPONENTES FRONTEND (React + TypeScript)

### Pages

```typescript
// Estructura de páginas
src/
├── pages/
│   ├── Dashboard.tsx           // Dashboard unificado
│   ├── Login.tsx
│   ├── admin/
│   │   ├── Dashboard.tsx
│   │   ├── Users/
│   │   │   ├── Index.tsx
│   │   │   └── Create.tsx
│   │   ├── Companies/
│   │   │   └── Index.tsx
│   │   └── Professionals/
│   │       └── Index.tsx
│   ├── empresa/
│   │   ├── Dashboard.tsx
│   │   └── Tasks/
│   │       ├── Index.tsx
│   │       └── Create.tsx
│   ├── manager/
│   │   └── Tasks/
│   │       ├── Index.tsx
│   │       ├── Create.tsx
│   │       └── Edit.tsx
│   ├── profesional/
│   │   ├── Tasks/
│   │   │   └── Index.tsx
│   │   └── RegisterHours.tsx
│   ├── reports/
│   │   └── Index.tsx
│   └── profile/
│       └── Edit.tsx
```

### Components

```typescript
// Componentes reutilizables
src/components/
├── common/
│   ├── Button.tsx
│   ├── Input.tsx
│   ├── Select.tsx
│   ├── Modal.tsx
│   ├── Dropdown.tsx
│   ├── DataTable.tsx
│   ├── Avatar.tsx
│   └── Loading.tsx
├── layout/
│   ├── Layout.tsx
│   ├── Header.tsx
│   ├── Sidebar.tsx
│   └── AuthLayout.tsx
├── tasks/
│   ├── TaskCard.tsx
│   ├── TaskList.tsx
│   ├── TaskForm.tsx
│   └── TaskDetails.tsx
└── permissions/
    └── Can.tsx        // Componente para verificar permisos
```

### Hooks Personalizados

```typescript
// Hooks útiles
src/hooks/
├── useAuth.ts       // Auth context
├── useTasks.ts     // Gestión de tareas
├── useWorkHours.ts // Horas trabajadas
└── usePermissions.ts // Permisos del usuario
```

### Servicios API

```typescript
// API client
src/services/
├── api.ts          // Axios instance
├── auth.service.ts
├── tasks.service.ts
├── workHours.service.ts
└── users.service.ts
```

---

## 6. FUNCIONALIDADES EN TIEMPO REAL

### WebSocket Events
- `chat.message` - Nuevo mensaje
- `task.created` - Tarea creada
- `task.updated` - Tarea actualizada
- `task.completed` - Tarea completada
- `work_hours.approved` - Horas aprobadas
- `notification` - Nueva notificación

---

## 7. INTEGRACIONES EXTERNAS

### Google Calendar API
- OAuth2 flow
- Listar eventos
- Crear eventos
- Sincronizar con tareas

### Google Forms API
- Listar formularios
- Obtener respuestas
- Crear formulario

### WhatsApp (Waha)
- Enviar mensajes
- Recibir mensajes
- Webhook para mensajes entrantes

### Brevo (Email)
- Envío de emails transaccionales
- Emails masivos
- Plantillas

### AI (OpenAI/GPT)
- Chat con IA
- Respuestas automáticas

---

## 8. AUTENTICACIÓN JWT

### Flujo
1. **Login:** POST /auth/login → retorna access_token + refresh_token
2. **Refresh:** POST /auth/refresh → retorna nuevo access_token
3. **Middleware:** Verificar JWT en cada request
4. **Logout:** Blacklist del token

### Claims del JWT
```json
{
  "sub": "user_id",
  "email": "user@email.com",
  "role": "empleado",
  "is_manager": false,
  "is_superadmin": false,
  "exp": "timestamp"
}
```

---

## 9. MIDDLEWARES (Go)

```go
// Middlewares necesarios
- AuthMiddleware       // Verificar JWT
- RoleMiddleware      // Verificar rol
- PermissionMiddleware // Verificar permisos específicos
- CORS               // Cross-origin
- RateLimit          // Rate limiting
- Logger             // Logging de requests
```

---

## 10. BASE DE DATOS - MIGRACIONES

```sql
-- Tablas principales
CREATE TABLE users (...);
CREATE TABLE tasks (...);
CREATE TABLE task_user (...);  -- pivote
CREATE TABLE comments (...);
CREATE TABLE task_attachments (...);
CREATE TABLE work_hours (...);
CREATE TABLE recovery_hours (...);
CREATE TABLE mass_email_logs (...);
CREATE TABLE notifications (...);
```

---

## 11. COSAS IMPORTANTES A CONSIDERAR

### Funcionalidades legacy que mantener:
- Cálculo de horas trabajadas vs objetivo (160 horas/mes)
- Estados de actividad: verde (activo), amarillo (advertencia), rojo (alerta)
- Recordatorios de horas no registradas
- Notificaciones en tiempo real
- Calendario de reuniones (Google Calendar)

### UI/UX:
- Diseño responsive (mobile-first)
- Tema claro/oscuro
- Loading states
- Manejo de errores
- Validaciones en frontend y backend

### Performance:
- Paginación
- Cache de consultas frecuentes
- Optimización de queries (eager loading)
- Compresión de assets

---

## EJEMPLO DE PROMPT PARA RECONSTRUIR

```markdown
Crea una aplicación completa de gestión de personal con las siguientes características:

**Stack:**
- Backend: Go con Gin framework
- Frontend: React 18 + TypeScript + Vite
- Base de datos: MySQL
- Auth: JWT

**Modelos:**
[Incluir todos los modelos de arriba]

**Funcionalidades:**
[Incluir todas las funcionalidades]

**Rutas:**
[Incluir todas las rutas API]

**Roles:**
- Super Admin
- Empleador  
- Manager
- Empleado

Implementa autenticación JWT, paginación, validaciones, manejo de archivos, y tiempo real con WebSocket.
```

---

Este documento contiene TODO lo necesario para reconstruir Obertrack completamente en Go + React/TypeScript.
