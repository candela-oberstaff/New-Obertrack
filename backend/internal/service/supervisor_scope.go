package service

import (
	"github.com/obertrack/backend/internal/repository"
)

// Alcance del rol supervisor.
//
// Un manager ve y aprueba a sus reportes DIRECTOS. Un supervisor ve y aprueba
// su ÁRBOL: sus managers y la gente que cuelga de esos managers, hacia abajo.
// La diferencia es solo de profundidad, así que todo el rol se reduce a decidir
// cuántos niveles baja el recorrido y a resolverlo una vez por petición.
//
// Las funciones de aquí devuelven un `applied bool` en vez de un alcance
// "por defecto": applied=false significa "este usuario no es supervisor, o el
// flag está apagado", y el llamador sigue por el camino de manager de siempre,
// intacto. Esa es la garantía de que encender el flag es lo único que cambia
// comportamiento.

// maxSupervisorDepth acota cuántos niveles baja el árbol de un supervisor.
// Cinco cubre cualquier organigrama real (supervisor → manager → profesional
// deja tres de sobra) y le pone techo al coste de la consulta recursiva.
const maxSupervisorDepth = 5

// supervisorScopeApplies indica si hay que usar el alcance de árbol para este
// actor. Solo un manager puede ser supervisor, así que isManager corta antes de
// ir a la base: con el flag apagado, o para cualquiera que no sea manager, esto
// no cuesta ni una consulta.
func supervisorScopeApplies(userRepo repository.UserRepository, actorID uint, isManager bool) bool {
	if !SupervisorScopeEnabled() || !isManager || actorID == 0 {
		return false
	}
	user, err := userRepo.GetByID(actorID)
	return err == nil && user != nil && user.IsSupervisor
}

// supervisorTeamIDs devuelve el árbol a cargo del actor en una empresa (sin
// incluirlo a él). applied=false = no es supervisor: usa el camino de manager.
//
// Un error de base NO degrada a "sin alcance": devolvería el camino de manager
// y el supervisor vería de menos, que es confuso pero inofensivo; lo que no
// puede pasar es lo contrario. Por eso se propaga y el llamador decide.
func supervisorTeamIDs(
	userRepo repository.UserRepository,
	empRepo repository.EmploymentRepository,
	actorID, companyID uint,
	isManager bool,
) (ids []uint, applied bool, err error) {
	if companyID == 0 || !supervisorScopeApplies(userRepo, actorID, isManager) {
		return nil, false, nil
	}
	ids, err = empRepo.DescendantIDs(actorID, companyID, maxSupervisorDepth)
	if err != nil {
		return nil, false, err
	}
	if ids == nil {
		// Un árbol vacío es un resultado legítimo (supervisor sin nadie debajo) y
		// tiene que llegar como lista vacía, no como nil: el filtro distingue
		// "sin nadie" de "sin filtro".
		ids = []uint{}
	}
	return ids, true, nil
}

// supervisorTeamAndSelfIDs es supervisorTeamIDs incluyendo al propio actor. Lo
// usan las vistas donde el manager se ve a sí mismo junto a su equipo (listado,
// resumen, reporte mensual), a diferencia de la bandeja de pendientes.
func supervisorTeamAndSelfIDs(
	userRepo repository.UserRepository,
	empRepo repository.EmploymentRepository,
	actorID, companyID uint,
	isManager bool,
) ([]uint, bool, error) {
	ids, applied, err := supervisorTeamIDs(userRepo, empRepo, actorID, companyID, isManager)
	if !applied || err != nil {
		return nil, applied, err
	}
	return append(ids, actorID), true, nil
}
