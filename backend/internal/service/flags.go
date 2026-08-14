package service

// multiManagerReads es el feature flag (Fase 2) que conmuta las LECTURAS de
// manager del puntero único (employments.manager_id) a la tabla N-a-N
// employment_managers con semántica "CUALQUIER manager".
//
// Vive como variable de paquete (no en config) para que los services lo
// consulten sin crear un ciclo de imports service->config. El wiring
// (routes/deps.go) llama SetMultiManagerReads(cfg.MultiManagerReads) una vez al
// arrancar. Default OFF: comportamiento actual intacto. Como el dual-write
// mantiene la tabla en espejo del puntero, con el flag ON y sin managers
// adicionales el resultado es idéntico al actual.
var multiManagerReads bool

// SetMultiManagerReads fija el estado del flag. Se llama una sola vez en el
// wiring; no es thread-safe para escrituras concurrentes en runtime.
func SetMultiManagerReads(v bool) { multiManagerReads = v }

// MultiManagerReadsEnabled indica si las lecturas via-links están activas.
func MultiManagerReadsEnabled() bool { return multiManagerReads }

// supervisorScope es el feature flag que le da al supervisor su alcance propio:
// el ÁRBOL completo bajo su mando (sus managers y la gente de esos managers) en
// vez de solo sus reportes directos.
//
// Mismo patrón y mismas reglas que multiManagerReads: variable de paquete para
// que los services la consulten sin ciclo de imports, fijada una sola vez desde
// routes/deps.go al arrancar. Default OFF: con el flag apagado un supervisor ve
// y aprueba exactamente lo mismo que el manager que ya es, así que encenderlo es
// lo único que cambia comportamiento.
var supervisorScope bool

// SetSupervisorScope fija el estado del flag (una sola vez, en el wiring).
func SetSupervisorScope(v bool) { supervisorScope = v }

// SupervisorScopeEnabled indica si el alcance transitivo del supervisor está
// activo.
func SupervisorScopeEnabled() bool { return supervisorScope }
