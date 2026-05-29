# Documentación de Obertrack

¡Bienvenido/a a la carpeta de documentación de Obertrack! Aquí encontrarás la arquitectura detallada, el flujo de datos y las guías técnicas para cada una de las integraciones del sistema.

---

## 📂 Directorio de Funcionalidades

### 📱 [Integración de WhatsApp](features/whatsapp-integration.md)
* Flujo de emparejamiento con el servidor WAHA.
* Enlace dinámico de perfiles de usuario (Profesionales / Empresas) desde la base de datos PostgreSQL.
* Componentes frontend modularizados y responsivos (`ChatList`, `ChatWindow`, `EmptyState`).

### 📥 [Integración de Zoho Desk y WhatsApp Oficial](features/zoho-desk-integration.md)
* Gestión de tickets centralizada vía API oficial de Zoho Desk.
* Flujo de autorización OAuth 2.0 y renovación automática de tokens con el par Client ID y Client Secret provisto.
* Arquitectura técnica del backend transaccional en Node.js/Express.
* Listados interactivos de hilos (threads) de WhatsApp dentro de la UI.

### ✉️ [Notificaciones y Respuestas por Email (Brevo)](features/email-notifications.md)
* Configuración transaccional a través de Brevo (Sendinblue).
* Plantillas HTML con diseño premium y sanitización de inyecciones de código.
* Validaciones robustas de direcciones de correo electrónico en el backend.
* Gestión de alertas de error en tiempo real en la UI del agente.

---

## 🛠️ Guía de Buenas Prácticas de Desarrollo

1. **Documentación Proactiva**: Cada vez que se desarrolle una funcionalidad clave, se debe actualizar o crear el correspondiente archivo markdown en `docs/features/`.
2. **Modularidad**: Mantener lógica de componentes separada y tipada bajo TypeScript para garantizar mantenibilidad.
3. **Validación Temprana**: Validar siempre los datos sensibles de los contactos (teléfonos, emails) en el backend y retornar códigos de estado HTTP semánticos (400 Bad Request, etc.).
