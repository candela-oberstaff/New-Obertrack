# Módulo de Mensajería Unificada (Zoho Desk API) - Fase 1: WhatsApp

## 1. Estrategia de Sincronización Delta (Long Polling)
Para optimizar el uso de la API de Zoho Desk y mejorar la experiencia de usuario, se ha implementado un sistema de sincronización basado en deltas.

### Mecanismo de Polling
- **Carga Inicial (Cold Start):** El frontend realiza una consulta completa sin filtros de tiempo.
- **Sincronización Recurrente:** Cada 30 segundos, el frontend consulta al backend enviando el parámetro `modifiedSince` con el timestamp del ticket más recientemente modificado.
- **Merge en Frontend:** Los nuevos resultados se combinan con el estado local basándose en el `zoho_id`, evitando parpadeos de carga y mutaciones innecesarias del DOM.

## 2. Implementación Técnica

### Backend (Go)
- **Servicio:** `internal/service/zoho_service.go`
  - Método `ListWhatsAppTickets` ahora admite `modifiedTimeRange`.
- **Handlers:** `internal/handlers/whatsapp_handler.go`
  - Los endpoints `/api/chats/me` y `/api/chats/unassigned` procesan el query param `modifiedSince` y formatean el rango para Zoho (`ISO8601,ISO8601`).

### Frontend (TypeScript/React)
- **Servicio:** `src/services/ticket.service.ts`
  - Actualización de tipos e interfaces para soportar `modifiedSince`.
- **Componente:** `src/pages/WhatsApp.tsx`
  - Uso de `lastModified` state para persistir el cursor de tiempo.
  - Implementación de `useEffect` con `setInterval` para polling de lista de chats (30s) y mensajes (10s).

## 3. Mapeo de Datos (WhatsApp)

| Campo Frontend | Origen Zoho | Descripción |
|----------------|-------------|-------------|
| `zoho_id` | `id` | ID único del ticket. |
| `contact_name` | `contactName` | Nombre del remitente. |
| `contact_phone`| `phone` | Número de WhatsApp. |
| `modified_time`| `modifiedTime`| ISO String para ordenamiento y polling. |

## 4. Flujos de Trabajo
1. **Asignación:** Los agentes pueden tomar chats de la pestaña "Sin Asignar". Al hacerlo, el ticket se actualiza en Zoho Desk y se mueve a la pestaña "Mis Chats".
2. **Mensajería Instantánea:** El envío de respuestas utiliza el endpoint `/sendReply` con `channel: "WhatsApp"`, lo que garantiza que el mensaje llegue al dispositivo del cliente como un mensaje de WhatsApp real y no como un comentario interno.
