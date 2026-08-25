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

// resolveManagersFor devuelve los managers de un usuario DENTRO de una empresa.
//
// Existe porque hay tres fuentes del mismo dato —users.manager_id (relación
// canónica), employments.manager_id (espejo por empresa) y employment_managers
// (tabla N-a-N)— y cuál manda depende del flag MultiManagerReads, que hoy está
// encendido en desarrollo y apagado en producción. Cualquier consumidor que elija
// una fuente por su cuenta acaba resolviendo personas distintas en cada entorno, y
// esa divergencia se descubre tarde y mal: en un aviso que llegó a quien no era.
//
// Misma disciplina de flag que countManagerReports, y misma semántica de unión
// cuando está apagado: se combinan las dos fuentes del puntero en vez de elegir
// una, porque un empleo recién creado puede tener aún sólo una de las dos escrita.
func resolveManagersFor(
	userRepo repository.UserRepository,
	empRepo repository.EmploymentRepository,
	userID, companyID uint,
) []uint {
	if userID == 0 {
		return nil
	}

	seen := make(map[uint]bool)
	out := []uint{}
	add := func(id uint) {
		// Nadie es manager de sí mismo, y un 0 es "sin manager", no un usuario.
		if id == 0 || id == userID || seen[id] {
			return
		}
		seen[id] = true
		out = append(out, id)
	}

	if MultiManagerReadsEnabled() && companyID > 0 {
		ids, err := empRepo.ListManagerIDs(userID, companyID)
		if err == nil {
			for _, id := range ids {
				add(id)
			}
		}
		// Con el flag encendido la tabla N-a-N es la fuente; si viniera vacía por
		// un empleo aún sin espejar, el puntero de abajo la completa.
	}

	if companyID > 0 {
		if emp, err := empRepo.GetActive(userID, companyID); err == nil && emp != nil && emp.ManagerID != nil {
			add(*emp.ManagerID)
		}
	}

	if u, err := userRepo.GetByID(userID); err == nil && u != nil && u.ManagerID != nil {
		add(*u.ManagerID)
	}

	return out
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
// de manager, esté activo, (si se indica empresa) pertenezca a ella y que la
// asignación no cierre un ciclo en la cadena de mando. Devuelve errores con el
// prefijo "Manager inválido:" para que los handlers los mapeen a 400 Bad Request.
//
// subordinateID es el profesional que va a reportar a manager; pasar 0 cuando
// todavía no existe (alta) y por lo tanto no puede formar parte de ninguna
// cadena.
func ensureValidManager(userRepo repository.UserRepository, empRepo repository.EmploymentRepository, manager *models.User, subordinateID, companyID uint) error {
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
	return ensureNoManagerCycle(userRepo, empRepo, manager.ID, subordinateID, companyID)
}

// maxManagerChainDepth acota el recorrido hacia arriba de la cadena de mando.
// Es una red de seguridad: si datos previos ya contienen un ciclo, la búsqueda
// termina igual en vez de colgarse.
const maxManagerChainDepth = 64

// ensureNoManagerCycle rechaza asignar managerID a subordinateID cuando eso
// cerraría un círculo (A reporta a B y B a A, directa o indirectamente).
// Sustituye a la vieja defensa "un manager no puede tener manager", que evitaba
// los ciclos a costa de impedir cualquier organigrama de más de dos niveles.
//
// Sube por la cadena desde managerID: si en el camino aparece subordinateID, la
// asignación lo pondría por encima de sí mismo. Recorre en anchura porque con
// multi-manager (employment_managers) un empleo puede tener varios superiores y
// el ciclo puede estar en cualquiera de las ramas.
func ensureNoManagerCycle(userRepo repository.UserRepository, empRepo repository.EmploymentRepository, managerID, subordinateID, companyID uint) error {
	if subordinateID == 0 || managerID == 0 {
		return nil
	}
	if managerID == subordinateID {
		return errors.New("Manager inválido: un profesional no puede ser su propio manager")
	}

	visited := map[uint]bool{managerID: true}
	frontier := []uint{managerID}

	for depth := 0; depth < maxManagerChainDepth && len(frontier) > 0; depth++ {
		next := []uint{}
		for _, id := range frontier {
			for _, sup := range managersOf(userRepo, empRepo, id, companyID) {
				if sup == subordinateID {
					return errors.New("Manager inválido: la asignación crearía un ciclo en la cadena de mando")
				}
				if !visited[sup] {
					visited[sup] = true
					next = append(next, sup)
				}
			}
		}
		frontier = next
	}
	return nil
}

// managersOf devuelve los superiores directos de userID. Dentro de una empresa
// usa la membresía de esa empresa (los vínculos N-a-N si las lecturas
// multi-manager están activas, si no el puntero principal); sin empresa cae al
// canónico users.manager_id. Best-effort: ante un error devuelve lo que pudo
// resolver, porque el guard que lo llama ya validó lo demás.
func managersOf(userRepo repository.UserRepository, empRepo repository.EmploymentRepository, userID, companyID uint) []uint {
	if companyID > 0 {
		if emp, err := empRepo.GetActive(userID, companyID); err == nil && emp != nil {
			if MultiManagerReadsEnabled() {
				if links, err := empRepo.ListEmploymentManagers(emp.ID); err == nil {
					out := make([]uint, 0, len(links))
					for _, l := range links {
						if l.ManagerID > 0 {
							out = append(out, l.ManagerID)
						}
					}
					return out
				}
			}
			if emp.ManagerID != nil && *emp.ManagerID > 0 {
				return []uint{*emp.ManagerID}
			}
			// Sin manager en la membresía: puede ser un espejo aún no
			// sincronizado, así que se comprueba igual el canónico de abajo.
		}
	}
	if u, err := userRepo.GetByID(userID); err == nil && u.ManagerID != nil && *u.ManagerID > 0 {
		return []uint{*u.ManagerID}
	}
	return nil
}
