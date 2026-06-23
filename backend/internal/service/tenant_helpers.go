package service

import (
	"errors"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
)

func tenantForUser(user *models.User) uint {
	return models.TenantForUser(user)
}

func isEmployerRole(role string) bool {
	return role == string(models.UserTypeEmployer) || role == "empleador"
}

// countManagerReports devuelve cuántos profesionales están a cargo de managerID,
// combinando la relación canónica users.manager_id (la que escribe toda
// asignación) con employments.manager_id (espejo per-empresa). Toma el mayor de
// ambos para no perder reportes cuyo employment aún no fue sincronizado (p.ej.
// subordinados que no han iniciado sesión tras crearse/asignarse). Propaga el
// error para que el guard falle cerrado (no degradar si no se pudo contar).
func countManagerReports(userRepo repository.UserRepository, empRepo repository.EmploymentRepository, managerID uint) (int64, error) {
	if MultiManagerReadsEnabled() {
		// Vía tabla N-a-N (semántica "cualquier manager"). Fail-closed por error.
		n, err := empRepo.CountActiveByManagerViaLinks(managerID)
		if err != nil {
			return 0, err
		}
		return n, nil
	}
	empN, err := empRepo.CountActiveByManager(managerID)
	if err != nil {
		return 0, err
	}
	userN, err := userRepo.CountReportsByManager(managerID)
	if err != nil {
		return 0, err
	}
	if userN > empN {
		return userN, nil
	}
	return empN, nil
}

// syncPrimaryManager mantiene employment_managers en línea con el principal
// escrito en employments.manager_id (dual-write Fase 1). Best-effort: un fallo
// no rompe la operación porque las lecturas siguen usando el puntero. Si no hay
// manager, limpia todos los vínculos del empleo; si lo hay, lo marca principal.
func syncPrimaryManager(empRepo repository.EmploymentRepository, employmentID uint, managerID *uint) {
	if managerID == nil || *managerID == 0 {
		_ = empRepo.ClearManagers(employmentID)
		return
	}
	_ = empRepo.SetPrimaryManager(employmentID, *managerID)
}

// ensureValidManager valida que el manager destino sea apto: que tenga el flag
// de manager, esté activo y (si se indica empresa) pertenezca a ella. Devuelve
// errores con el prefijo "Manager inválido:" para que los handlers los mapeen
// a 400 Bad Request.
func ensureValidManager(empRepo repository.EmploymentRepository, manager *models.User, companyID uint) error {
	if !manager.IsManager {
		return errors.New("Manager inválido: el usuario seleccionado no es manager")
	}
	if !manager.IsActive {
		return errors.New("Manager inválido: el manager seleccionado está inactivo")
	}
	if companyID > 0 {
		if _, err := empRepo.GetActive(manager.ID, companyID); err != nil {
			return errors.New("Manager inválido: el manager no pertenece a la empresa del profesional")
		}
	}
	return nil
}
