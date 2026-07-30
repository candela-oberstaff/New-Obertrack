// Feature flags de producto (frontend).
//
// WALLET_ENABLED: módulo Wallet (integración Ontop). Escondido por decisión de
// producto (2026-07-22) — NO borrado. Todo el código (página, servicio, rutas)
// permanece intacto. Poner en `true` para reactivarlo: reaparece en el sidebar
// y su ruta /wallet vuelve a estar accesible.
export const WALLET_ENABLED = false

// GOOGLE_INTEGRATIONS_ENABLED: todo lo que depende del consentimiento de Google
// (vincular la cuenta desde el perfil y el módulo de Sesiones con salas de
// Meet). Escondido el 2026-07-30 — NO borrado.
//
// El motivo no es técnico: la app sigue en modo de prueba de Google, y hasta
// pasar la verificación el permiso de cada usuario caduca a los 7 días, así que
// la integración se rompería sola cada semana y lo reportarían como fallo.
// La verificación requiere trabajo previo en la web pública (portada de
// producto, política de privacidad propia con la cláusula de Uso Limitado y
// dominio verificado), que está pendiente.
//
// Poner en `true` para reactivarlo: vuelven el panel de Integraciones en el
// perfil y la sección de Sesiones (sidebar + ruta /sesiones). El backend tiene
// su propio interruptor independiente, GOOGLE_CALENDAR_ENABLED.
export const GOOGLE_INTEGRATIONS_ENABLED = false
