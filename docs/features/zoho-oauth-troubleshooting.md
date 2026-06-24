# Guía de Configuración y Resolución de Problemas: Zoho Desk OAuth & WhatsApp

Esta guía detalla el proceso completo para configurar, autorizar y solucionar problemas de la integración entre **Obertrack** y **Zoho Desk** para el canal oficial de **WhatsApp (Instant Messaging)** y la sincronización de agentes.

---

## 🔍 1. Diagnóstico e Historial del Problema

Durante el desarrollo de la integración, nos encontramos con dos errores críticos en la comunicación con la API de Zoho Desk:

### Error A: 403 SCOPE_MISMATCH en WhatsApp (`/im/sessions`)
Al intentar consultar el historial de chats o enviar mensajes a través de los endpoints de mensajería instantánea (`/api/v1/im/sessions/...`), la API devolvía:
```json
{
  "errorCode": "SCOPE_MISMATCH",
  "message": "The OAuth Token does not contain the scope to perform this operation."
}
```
Esto ocurría porque el token original se había generado únicamente con permisos estándar de tickets (`ZohoDesk.tickets.READ`, etc.), omitiendo los permisos necesarios del módulo de chat/WhatsApp.

### Error B: Ámbito de OAuth no válido / Ámbito no proporcionado
Al intentar solucionar el `SCOPE_MISMATCH` indicando scopes intuitivos como `Desk.im.ALL` o `Desk.conversations.ALL`, la pantalla de inicio de sesión de Zoho mostraba una página de error que bloqueaba la autorización del usuario:
> **Ámbito de OAuth no válido. Ámbito no proporcionado.**

Esto sucede porque Zoho API Console es extremadamente estricta y **rechaza la solicitud completa si tan solo uno de los scopes provistos no es exactamente un scope oficial habilitado**.

---

## 🔑 2. Tabla de Ámbitos (Scopes) Oficiales y Validados

De acuerdo a la documentación técnica de Zoho Desk y las pruebas realizadas en producción, los únicos scopes válidos y requeridos para que Obertrack funcione al 100% son:

| Scope (Ámbito) | Nivel | Descripción y Uso en Obertrack |
| :--- | :--- | :--- |
| **`Desk.tickets.ALL`** | Lectura/Escritura | Permite ver tickets, crear hilos (threads) y actualizar campos principales. |
| **`Desk.InstantMessages.READ`** | Lectura | **Requerido para WhatsApp**: Permite listar canales de mensajería y obtener detalles/historiales de sesiones de chat activas (`/im/sessions/{id}`). |
| **`Desk.InstantMessages.CREATE`** | Escritura | **Requerido para WhatsApp**: Permite enviar mensajes salientes y disparar plantillas oficiales de WhatsApp para iniciar/renovar conversaciones (`/initiateSession`). |
| **`Desk.InstantMessages.UPDATE`** | Escritura | **Requerido para WhatsApp**: Permite modificar metadatos y estados de las sesiones de chat. |
| **`Desk.contacts.READ`** | Lectura | Permite obtener perfiles de contacto (clientes) vinculados al número de WhatsApp. |
| **`Desk.contacts.WRITE`** | Escritura | Permite dar de alta o actualizar contactos cuando se inicia un chat desde Obertrack. |
| **`Desk.basic.READ`** | Lectura | Permite listar agentes activos, departamentos y organizaciones para mapear el responsable del ticket. |
| **`Desk.basic.CREATE`** | Escritura | Requerido para operaciones de asignación y creación de recursos básicos. |
| **`Desk.search.READ`** | Lectura | Habilita las búsquedas globales de tickets y clientes por teléfono/correo. |
| **`Desk.events.ALL`** | Completo | Permite la suscripción y recepción de eventos en tiempo real (webhooks de mensajes entrantes). |
| **`Desk.settings.ALL`** | Completo | Garantiza acceso de lectura/escritura a las configuraciones de los canales de soporte. |

---

## 🔄 3. Proceso Paso a Paso para Generar y Renovar Credenciales

Dado que el `Access Token` de Zoho expira cada 60 minutos, la aplicación utiliza un flujo automático basado en un `Refresh Token` permanente. Para generar este token inicial por primera vez o re-autorizar scopes, sigue estos pasos:

### Paso 1: Generar URL de Autorización (Paso Manual de Administrador)
El administrador debe ingresar a una URL estructurada con los scopes validados y las credenciales de la aplicación registradas en [Zoho API Console](https://api-console.zoho.com/):

```text
https://accounts.zoho.com/oauth/v2/auth?scope=Desk.tickets.ALL,Desk.contacts.READ,Desk.contacts.WRITE,Desk.basic.READ,Desk.basic.CREATE,Desk.search.READ,Desk.events.ALL,Desk.settings.ALL,Desk.InstantMessages.READ,Desk.InstantMessages.CREATE,Desk.InstantMessages.UPDATE&client_id=TU_CLIENT_ID&response_type=code&access_type=offline&redirect_uri=TU_REDIRECT_URI
```
> ⚠️ **Importante**: El parámetro `access_type=offline` es obligatorio para que Zoho devuelva el `refresh_token`.

### Paso 2: Obtener el Código de Autorización
1. Copia y pega la URL en tu navegador.
2. Inicia sesión con la cuenta de Zoho del administrador de la organización.
3. Haz clic en **Aceptar** para dar consentimiento a los permisos.
4. Serás redirigido a tu URL de callback. Copia el parámetro `code` de la URL, el cual se ve así:
   `code=1000.xxxxxx.xxxxxx`

### Paso 3: Intercambiar Código por Refresh Token
Realiza una petición `POST` al endpoint de Zoho para recibir los tokens definitivos. Puedes hacerlo desde una terminal usando PowerShell o cURL:

#### En PowerShell:
```powershell
Invoke-RestMethod -Uri 'https://accounts.zoho.com/oauth/v2/token' -Method Post -Body @{
    code = 'EL_CODIGO_COPIADO'
    client_id = 'TU_CLIENT_ID'
    client_secret = 'TU_CLIENT_SECRET'
    redirect_uri = 'TU_REDIRECT_URI'
    grant_type = 'authorization_code'
}
```

#### En cURL (Bash):
```bash
curl -X POST https://accounts.zoho.com/oauth/v2/token \
  -d "code=EL_CODIGO_COPIADO" \
  -d "client_id=TU_CLIENT_ID" \
  -d "client_secret=TU_CLIENT_SECRET" \
  -d "redirect_uri=TU_REDIRECT_URI" \
  -d "grant_type=authorization_code"
```

El servidor de Zoho responderá con un JSON similar a este:
```json
{
  "access_token": "1000.access_token_temporal...",
  "refresh_token": "1000.refresh_token_permanente...",
  "scope": "Desk.tickets.ALL Desk.contacts.READ ... Desk.InstantMessages.READ Desk.InstantMessages.CREATE Desk.InstantMessages.UPDATE",
  "api_domain": "https://www.zohoapis.com",
  "token_type": "Bearer",
  "expires_in": 3600
}
```

### Paso 4: Guardar Token y Reiniciar Backend
1. Abre tu archivo `.env` en la raíz del backend.
2. Actualiza la variable con el nuevo token obtenido:
   ```env
   ZOHO_REFRESH_TOKEN=1000.refresh_token_permanente...
   ```
3. Reinicia tu servidor backend Go para que lea la nueva variable de entorno:
   ```bash
   go run ./cmd/
   ```

---

## 💡 4. Conceptos Clave para Desarrolladores

### A. Bloqueo de Sesiones por Asignación (Ticket Assignee Lock)
Zoho Desk implementa una regla de seguridad estricta para canales de mensajería instantánea: **Solo el agente asignado actualmente al ticket tiene permitido enviar respuestas por chat**.
* Si el Ticket de WhatsApp está asignado al Agente A, e intentas responder usando las credenciales del Agente B, la API rechazará el mensaje.
* **Solución**: En Obertrack, antes de enviar un mensaje de WhatsApp, el agente debe **Tomar (Claim)** el ticket o el ticket debe ser **Reasignado** a su usuario. Al realizar esta reasignación en Obertrack, el backend automáticamente envía una petición `PATCH` a Zoho Desk sincronizando el `assigneeId` del ticket para desbloquear la sesión de chat para ese agente.

### B. Ventana de 24 Horas y Envío de Plantillas (WhatsApp Templates)
* **Regla de Meta**: No se pueden enviar mensajes de texto plano (free-text) a clientes a menos que el cliente haya enviado un mensaje en las últimas 24 horas.
* **Flujo en Obertrack**:
  * **Dentro de la ventana de 24h**: Los mensajes se envían de forma estándar y fluida (conversación en vivo).
  * **Fuera de la ventana de 24h**: El cuadro de texto se bloquea o requiere enviar una plantilla pre-aprobada por Meta. En este caso, el backend llama al endpoint de Zoho `/api/v1/im/channels/{channelId}/initiateSession` pasando el `cannedMessageId` (ID de plantilla). Esto reabre el canal de comunicación.
