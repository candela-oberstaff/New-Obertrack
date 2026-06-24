# Integración de Zoho Desk para Gestión de Tickets y WhatsApp

Este documento detalla el plan técnico, la arquitectura de integración y el flujo de autenticación para la conexión entre **Obertrack**, **Zoho Desk**, y los canales oficiales de **WhatsApp** asociados a Zoho Desk.

---

## 📐 Arquitectura del Sistema

La topología de comunicación y sincronización de mensajes está diseñada de la siguiente manera:

```
[ Cliente Final (WhatsApp) ]
            │
            ▼ (WhatsApp Oficial API / Meta Cloud)
   [ Zoho Desk Inbox ] ─── (Conversación unificada)
            │
            ▼ (Webhooks / API HTTPS REST)
[ Backend de Obertrack ] ─── (Express.js / Node.js)
            │
            ▼ (WebSockets / REST API)
[ Frontend de Obertrack ] ─── (Panel Personalizado de Agentes en React / Next.js)
```

---

## 🔑 Credenciales y Configuración de Zoho API

Para interactuar con la API de Zoho Desk, utilizamos una aplicación OAuth tipo **Web-Based** creada en la [Zoho API Console](https://api-console.zoho.com/).

### Credenciales de la Aplicación
* **Client ID:** definido en `ZOHO_CLIENT_ID`
* **Client Secret:** definido en `ZOHO_CLIENT_SECRET`
* **Refresh Token:** definido en `ZOHO_REFRESH_TOKEN`
* **Redirect URI:** definido en `ZOHO_REDIRECT_URI`
* **Ámbitos de API requeridos (Scopes):**
  * `Desk.tickets.ALL` (Acceso completo de lectura y escritura a tickets y conversaciones relacionadas)
  * `Desk.contacts.READ` (Lectura de perfiles de contactos y cuentas)
  * `Desk.contacts.WRITE` (Creación y actualización de contactos/cuentas)
  * `Desk.basic.READ` (Lectura de departamentos, organizaciones y agentes)
  * `Desk.search.READ` (Búsqueda de datos de tickets/contactos)

---

## 🔄 Flujo de Autenticación y Renovación de Tokens

Zoho utiliza OAuth 2.0. Dado que el `Access Token` expira cada **60 minutos**, el backend de Obertrack implementará un flujo automático utilizando el `Refresh Token` persistido de forma segura.

### 1. Obtención de Códigos Iniciales (Paso Manual de Autorización)
Se genera la URL de autorización para el Administrador:
```
https://accounts.zoho.com/oauth/v2/auth?scope=Desk.tickets.ALL,Desk.contacts.READ,Desk.contacts.WRITE,Desk.basic.READ,Desk.search.READ&client_id=1000.YVRU04LUXVH3SL302YRABIOE8HUY5N&response_type=code&access_type=offline&redirect_uri=https://obertrack.com/auth/zoho/callback
```
> **Nota:** El parámetro `access_type=offline` es obligatorio para que Zoho devuelva el `Refresh Token` que permite la renovación indefinida de credenciales.

### 2. Canje de Código por Tokens (POST)
El backend intercambia el código recibido por los tokens:
* **URL:** `https://accounts.zoho.com/oauth/v2/token`
* **Método:** `POST`
* **Cuerpo (x-www-form-urlencoded):**
  * `code`: `{AUTHORIZATION_CODE}`
  * `client_id`: `{ZOHO_CLIENT_ID}`
  * `client_secret`: `{ZOHO_CLIENT_SECRET}`
  * `redirect_uri`: `{YOUR_REDIRECT_URI}`
  * `grant_type`: `authorization_code`

### 3. Renovación Automática del Access Token
Cada hora, o cuando las APIs devuelvan un error `401 Unauthorized`, el backend realiza la renovación:
* **URL:** `https://accounts.zoho.com/oauth/v2/token`
* **Cuerpo (x-www-form-urlencoded):**
  * `refresh_token`: `{REFRESH_TOKEN}`
  * `client_id`: `{ZOHO_CLIENT_ID}`
  * `client_secret`: `{ZOHO_CLIENT_SECRET}`
  * `grant_type`: `refresh_token`

---

## 📞 Endpoints Críticos de Zoho Desk API a Utilizar

Todas las llamadas transaccionales requieren el header: `Authorization: Zoho-oauthtoken {ACCESS_TOKEN}` junto con `orgId: {ORGANIZATION_ID}`.

### 1. Listar Tickets
Obtiene todos los tickets ordenados por última actualización:
* **Endpoint:** `GET https://desk.zoho.com/api/v1/tickets?sortBy=-modifiedTime&limit=50`
* **Uso:** Rellena la lista principal del dashboard del agente.

### 2. Obtener Detalle de Ticket
Obtiene la información completa de un ticket específico. Es el endpoint base para conocer contacto, estado, canal, `source.extId`, agente asignado y metadatos necesarios antes de responder o dibujar el detalle.
* **Endpoint:** `GET https://desk.zoho.com/api/v1/tickets/{ticketId}`
* **Uso:** Alimenta la vista de detalle en `/tickets/:id` y permite extraer datos necesarios para mensajería oficial.

### 3. Obtener Conversaciones (Threads / WhatsApp History)
Obtiene el historial cronológico de un ticket específico:
* **Endpoint:** `GET https://desk.zoho.com/api/v1/tickets/{ticketId}/threads`
* **Uso:** Dibuja la ventana de chat y conversación interactiva en el panel del agente.

### 4. Responder al Ticket (Enviar mensaje de WhatsApp)
Agrega una respuesta al ticket que se despacha automáticamente a través de la integración de WhatsApp activa en Zoho Desk:
* **Endpoint:** `POST https://desk.zoho.com/api/v1/tickets/{ticketId}/threads`
* **Cuerpo (JSON):**
  ```json
  {
    "channel": "phone",
    "content": "Tu mensaje de respuesta aquí..."
  }
  ```

### 5. Actualizar Estado o Asignación del Ticket
* **Endpoint:** `PATCH https://desk.zoho.com/api/v1/tickets/{ticketId}`
* **Cuerpo (JSON):**
  ```json
  {
    "status": "Closed",
    "assigneeId": "{agentId}"
  }
  ```

---

## 🛠️ Plan de Trabajo de las Siguientes Fases

| Fase | Título | Foco Técnico |
| :--- | :--- | :--- |
| **Fase 1** | **Investigación y Conexión** | Configurar OAuth manual, canjear las credenciales provistas y validar las llamadas a la API REST de Zoho Desk. |
| **Fase 2** | **Backend Base** | Desarrollar el servidor en Node.js (Express), manejar el ciclo de vida del Refresh Token y exponer la API interna. |
| **Fase 3** | **Frontend Inicial** | Armar la UI de control del agente usando React/Next.js con Tailwind (Listados, ChatView, formularios de respuesta rápida). |
| **Fase 4** | **Tiempo Real** | Configurar Webhooks de Zoho Desk hacia nuestro backend y conectar WebSockets a la interfaz para actualizaciones automáticas. |
| **Fase 5** | **Automatización e IA** | Introducción opcional de análisis semántico, clasificación inteligente y respuestas automatizadas. |
