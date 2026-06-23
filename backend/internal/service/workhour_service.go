package service

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"strings"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
	"github.com/obertrack/backend/internal/utils"
)

type WorkHourService interface {
	GetAll(userID uint, role string, isSuperadmin, isManager bool, tenantID, companyFilter uint, userIDFilter, startDate, endDate string, offset, limit int) ([]models.WorkHour, int64, error)
	Create(userID uint, reqData map[string]interface{}) (*models.WorkHour, error)
	Update(id, tenantID, userID uint, role string, isManager, isSuperadmin bool, reqData map[string]interface{}) (*models.WorkHour, error)
	Approve(ids []uint, userID uint, role string, isSuperadmin bool, isManager bool, tenantID uint) error
	Reject(ids []uint, userID uint, role string, isSuperadmin bool, isManager bool, tenantID uint, reason string) error
	GetSummary(userID uint, role string, isSuperadmin, isManager bool, tenantID, companyFilter uint, userIDFilter string) (map[string]float64, error)
	GetPending(tenantID, userID uint, role string, isSuperadmin bool, isManager bool, companyFilter uint, userIDFilter string) ([]models.WorkHour, error)
	SendReportEmail(employerID uint, month int, year int, companyFilter uint) error
	GetPDFReportBytes(userID uint, month int, year int, companyFilter uint) ([]byte, string, error)
	GetExcelReportBytes(userID uint, month int, year int, companyFilter uint) ([]byte, string, error)
}

type workHourService struct {
	repo           repository.WorkHourRepository
	userRepo       repository.UserRepository
	notifSvc       NotificationService
	brevoSvc       *BrevoService
	ticketSvc      TicketService
	employmentRepo repository.EmploymentRepository
}

func NewWorkHourService(
	repo repository.WorkHourRepository,
	userRepo repository.UserRepository,
	notifSvc NotificationService,
	brevoSvc *BrevoService,
	ticketSvc TicketService,
	employmentRepo repository.EmploymentRepository,
) WorkHourService {
	return &workHourService{
		repo:           repo,
		userRepo:       userRepo,
		notifSvc:       notifSvc,
		brevoSvc:       brevoSvc,
		ticketSvc:      ticketSvc,
		employmentRepo: employmentRepo,
	}
}

func (s *workHourService) parseStringVal(val interface{}) string {
	if str, ok := val.(string); ok {
		return str
	}
	return ""
}

func (s *workHourService) parseFloatVal(val interface{}) float64 {
	if val == nil {
		return 0
	}
	strVal := fmt.Sprintf("%v", val)
	if f, err := strconv.ParseFloat(strVal, 64); err == nil {
		return f
	}
	return 0
}

func (s *workHourService) GetAll(userID uint, role string, isSuperadmin, isManager bool, tenantID, companyFilter uint, userIDFilter, startDate, endDate string, offset, limit int) ([]models.WorkHour, int64, error) {
	filters := make(map[string]interface{})

	if isSuperadmin {
		// Superadmin must scope to a company explicitly. Without it, no records are
		// returned so we never mix work hours from different tenants in the view.
		if companyFilter == 0 {
			return []models.WorkHour{}, 0, nil
		}
		filters["tenant_id"] = companyFilter
	} else if isManager {
		// Un manager ve solo su equipo (él + subordinados directos), igual que su
		// lista de pendientes y su resumen; no todas las horas del tenant. Así
		// "lo que ve" coincide con "lo que puede aprobar".
		if tenantID > 0 {
			filters["tenant_id"] = tenantID
		}
		if MultiManagerReadsEnabled() {
			filters["manager_or_user_links_id"] = userID
		} else {
			filters["manager_or_user_id"] = userID
		}
	} else if isEmployerRole(role) {
		// Los empleadores ven todas las horas de su tenant.
		if tenantID > 0 {
			filters["tenant_id"] = tenantID
		}
	} else {
		// Los profesionales solo ven sus propias horas.
		filters["user_id"] = userID
	}

	if userIDFilter != "" && (isSuperadmin || role == string(models.UserTypeEmployer) || role == "empleador") {
		uid, _ := strconv.ParseUint(userIDFilter, 10, 32)
		filters["user_id"] = uint(uid)
	}

	if startDate != "" {
		if t, err := time.Parse("2006-01-02", startDate); err == nil {
			filters["start_date"] = t
		}
	}

	if endDate != "" {
		if t, err := time.Parse("2006-01-02", endDate); err == nil {
			filters["end_date"] = t
		}
	}

	return s.repo.FindAll(filters, offset, limit)
}

// Errores tipados de la creación/edición de jornadas. El handler los mapea a
// códigos HTTP con errors.Is en vez de comparar el texto del mensaje (frágil).
var (
	ErrInvalidDateFormat = errors.New("Invalid date format")
	ErrFutureWorkDate    = errors.New("No puedes registrar horas en fechas futuras")
	ErrDuplicateWorkDay  = errors.New("Ya existe un registro para esta fecha en esta empresa. Solo puedes registrar un máximo de una jornada por día.")
)

const standardWorkDay = 8.0

// clampHours acota las horas a un rango válido [0,24]. Evita valores negativos
// o desbordes de la columna decimal(5,2) y stats infladas por payloads del
// cliente sin validar.
func clampHours(h float64) float64 {
	if h < 0 {
		return 0
	}
	if h > 24 {
		return 24
	}
	return h
}

// clampAbsenceHours acota las horas de ausencia a una jornada laboral [0,8].
func clampAbsenceHours(h float64) float64 {
	if h < 0 {
		return 0
	}
	if h > standardWorkDay {
		return standardWorkDay
	}
	return h
}

func (s *workHourService) Create(userID uint, reqData map[string]interface{}) (*models.WorkHour, error) {
	workDateStr := s.parseStringVal(reqData["work_date"])
	workDate, err := time.Parse("2006-01-02", workDateStr)
	if err != nil {
		return nil, ErrInvalidDateFormat
	}

	today := time.Now().Truncate(24 * time.Hour)
	if workDate.After(today) {
		return nil, ErrFutureWorkDate
	}

	creator, _ := s.userRepo.GetByID(userID)
	tenantID := models.TenantForUser(creator)

	// El límite de una jornada por día es POR empresa activa: un profesional
	// multi-empresa puede registrar el mismo día en cada una.
	if _, err := s.repo.FindByUserAndDate(userID, workDate, tenantID); err == nil {
		return nil, ErrDuplicateWorkDay
	}

	// Las horas son autoritativas en el servidor: no se confía en lo que mande
	// el cliente para complete/absence; recover se acota a [0,24].
	workTypeStr := s.parseStringVal(reqData["work_type"])
	workType := models.WorkTypeComplete
	absenceHours := clampAbsenceHours(s.parseFloatVal(reqData["absence_hours"]))
	var hoursWorked float64

	switch workTypeStr {
	case "absence":
		workType = models.WorkTypeAbsence
		hoursWorked = standardWorkDay - absenceHours
	case "recover":
		workType = models.WorkTypeRecover
		absenceHours = 0
		hoursWorked = clampHours(s.parseFloatVal(reqData["hours_worked"]))
	default:
		workType = models.WorkTypeComplete
		absenceHours = 0
		hoursWorked = standardWorkDay
	}

	workHour := &models.WorkHour{
		UserID:        userID,
		TenantID:      tenantID,
		WorkDate:      workDate,
		WorkType:      workType,
		HoursWorked:   hoursWorked,
		Activities:    utils.SanitizeHTML(s.parseStringVal(reqData["activities"])),
		Comments:      utils.SanitizeHTML(s.parseStringVal(reqData["comments"])),
		AbsenceReason: s.parseStringVal(reqData["absence_reason"]),
		AbsenceHours:  absenceHours,
	}

	startTime := s.parseStringVal(reqData["start_time"])
	endTime := s.parseStringVal(reqData["end_time"])

	if startTime != "" {
		if t, err := time.Parse("15:04", startTime); err == nil {
			workHour.StartTime = &t
		}
	}
	if endTime != "" {
		if t, err := time.Parse("15:04", endTime); err == nil {
			workHour.EndTime = &t
		}
	}

	if err := s.repo.Create(workHour); err != nil {
		if strings.Contains(err.Error(), "duplicate") {
			return nil, ErrDuplicateWorkDay
		}
		return nil, errors.New("Failed to create work hour")
	}

	// Fetch with preload for response
	finalWH, err := s.repo.FindByID(workHour.ID)
	if err == nil && finalWH != nil {
		// Notificar al Manager y al Empleador internamente
		go func() {
			user, _ := s.userRepo.GetByID(finalWH.UserID)
			if user != nil {
				// Notificar al Manager per-empresa de la jornada
				if MultiManagerReadsEnabled() {
					// Con el flag ON notificamos a TODOS los managers del empleo.
					if managerIDs, err := s.employmentRepo.ListManagerIDs(finalWH.UserID, finalWH.TenantID); err == nil {
						for _, managerID := range managerIDs {
							_ = s.notifSvc.CreateNotification(managerID, "work_hour_created", "Nueva jornada registrada", fmt.Sprintf("%s registró una jornada para el %s", user.Name, finalWH.WorkDate.Format("02/01")), map[string]interface{}{"id": finalWH.ID, "link": "/work-hours"})
						}
					}
				} else if emp, err := s.employmentRepo.GetActive(finalWH.UserID, finalWH.TenantID); err == nil && emp != nil && emp.ManagerID != nil {
					_ = s.notifSvc.CreateNotification(*emp.ManagerID, "work_hour_created", "Nueva jornada registrada", fmt.Sprintf("%s registró una jornada para el %s", user.Name, finalWH.WorkDate.Format("02/01")), map[string]interface{}{"id": finalWH.ID, "link": "/work-hours"})
				}
				// Notificar al Empleador
				if user.EmpleadorID != nil {
					_ = s.notifSvc.CreateNotification(*user.EmpleadorID, "work_hour_created", "Nueva jornada registrada", fmt.Sprintf("%s registró una jornada para el %s", user.Name, finalWH.WorkDate.Format("02/01")), map[string]interface{}{"id": finalWH.ID, "link": "/work-hours"})
				}
			}
		}()
	}

	return finalWH, err
}

func (s *workHourService) Update(id, tenantID, userID uint, role string, isManager, isSuperadmin bool, reqData map[string]interface{}) (*models.WorkHour, error) {
	var workHour *models.WorkHour
	var err error
	if !isSuperadmin && tenantID > 0 {
		workHour, err = s.repo.FindByIDAndTenant(id, tenantID)
	} else {
		workHour, err = s.repo.FindByID(id)
	}
	if err != nil {
		return nil, errors.New("Work hour not found")
	}

	if !isSuperadmin {
		if tenantID == 0 || workHour.TenantID != tenantID {
			return nil, errors.New("Access denied")
		}
		allowed := workHour.UserID == userID || isEmployerRole(role)
		if !allowed && isManager && workHour.UserID != userID {
			if MultiManagerReadsEnabled() {
				if ok, _ := s.employmentRepo.IsManagerOf(workHour.UserID, workHour.TenantID, userID); ok {
					allowed = true
				}
			} else {
				if emp, err := s.employmentRepo.GetActive(workHour.UserID, workHour.TenantID); err == nil && emp != nil && emp.ManagerID != nil && *emp.ManagerID == userID {
					allowed = true
				}
			}
		}
		if !allowed {
			return nil, errors.New("Access denied")
		}
	}

	workDateStr := s.parseStringVal(reqData["work_date"])
	if workDateStr != "" {
		if t, err := time.Parse("2006-01-02", workDateStr); err == nil {
			workHour.WorkDate = t
		}
	}

	workTypeStr := s.parseStringVal(reqData["work_type"])
	if workTypeStr != "" {
		workHour.WorkType = models.WorkType(workTypeStr)
	}

	if val, ok := reqData["absence_reason"]; ok {
		workHour.AbsenceReason = s.parseStringVal(val)
	}
	if val, ok := reqData["absence_hours"]; ok {
		workHour.AbsenceHours = clampAbsenceHours(s.parseFloatVal(val))
	}

	// Las horas se derivan del tipo de jornada en el servidor; solo recover
	// toma las horas del cliente (acotadas). No se confía en hours_worked para
	// complete/absence.
	switch workHour.WorkType {
	case models.WorkTypeAbsence:
		workHour.HoursWorked = standardWorkDay - workHour.AbsenceHours
	case models.WorkTypeRecover:
		workHour.AbsenceReason = ""
		workHour.AbsenceHours = 0
		if val, ok := reqData["hours_worked"]; ok {
			workHour.HoursWorked = clampHours(s.parseFloatVal(val))
		}
	default:
		workHour.HoursWorked = standardWorkDay
		workHour.AbsenceReason = ""
		workHour.AbsenceHours = 0
	}

	if act, ok := reqData["activities"]; ok {
		workHour.Activities = utils.SanitizeHTML(s.parseStringVal(act))
	}
	if com, ok := reqData["comments"]; ok {
		workHour.Comments = utils.SanitizeHTML(s.parseStringVal(com))
	}

	// Integridad de nómina: cualquier edición por un no-superadmin (incluidos
	// empleador y manager) devuelve la jornada a "pendiente" para forzar una
	// re-aprobación. Así nadie altera horas ya aprobadas sin re-revisión; el
	// superadmin queda exento para correcciones puntuales.
	//
	// Se CONSERVA el historial del rechazo previo (RejectedBy/RejectedAt/
	// RejectionReason) para auditoría: al re-someter sabemos por qué se había
	// rechazado. Una nueva aprobación lo limpia (ver repo.ApproveMultiple) y un
	// nuevo rechazo lo sobrescribe.
	if !isSuperadmin {
		workHour.Approved = false
		workHour.ApprovedBy = nil
		workHour.ApprovedAt = nil
		workHour.Rejected = false
	}

	if err := s.repo.Update(workHour); err != nil {
		return nil, errors.New("Failed to update work hour")
	}

	return s.repo.FindByID(workHour.ID)
}

func (s *workHourService) Approve(ids []uint, userID uint, role string, isSuperadmin bool, isManager bool, tenantID uint) error {
	// Use tenant-scoped query for defense-in-depth
	var workHours []models.WorkHour
	var err error
	if !isSuperadmin && tenantID > 0 {
		workHours, err = s.repo.FindManyByIDsAndTenant(ids, tenantID)
	} else {
		workHours, err = s.repo.FindManyByIDs(ids)
	}
	if err != nil {
		return errors.New("Failed to fetch work hours")
	}

	if len(workHours) == 0 {
		return errors.New("No work hours found")
	}

	for _, wh := range workHours {
		canApprove := false

		if isSuperadmin {
			canApprove = true
		} else if role == string(models.UserTypeEmployer) || role == "empleador" {
			if wh.User.EmpleadorID != nil && *wh.User.EmpleadorID == userID {
				canApprove = true
			}
		} else if isManager {
			// Separación de funciones: un manager NO puede aprobar sus propias
			// jornadas, solo las de sus subordinados directos (per-empresa).
			if wh.UserID != userID {
				if MultiManagerReadsEnabled() {
					if ok, _ := s.employmentRepo.IsManagerOf(wh.UserID, wh.TenantID, userID); ok {
						canApprove = true
					}
				} else {
					if emp, err := s.employmentRepo.GetActive(wh.UserID, wh.TenantID); err == nil && emp != nil && emp.ManagerID != nil && *emp.ManagerID == userID {
						canApprove = true
					}
				}
			}
		}

		if !canApprove {
			return errors.New("No tienes permiso para aprobar estas horas.")
		}
	}

	if !isSuperadmin && tenantID > 0 {
		err = s.repo.ApproveMultipleAndTenant(ids, userID, time.Now(), tenantID)
	} else {
		err = s.repo.ApproveMultiple(ids, userID, time.Now())
	}
	if err == nil {
		// Notificaciones de aprobación
		go func() {
			// Agrupar por usuario para enviar un solo mensaje
			userHours := make(map[uint][]models.WorkHour)
			approver, _ := s.userRepo.GetByID(userID)
			var approvedNames []string
			uniqueNames := make(map[string]bool)

			for _, wh := range workHours {
				userHours[wh.UserID] = append(userHours[wh.UserID], wh)
				if !uniqueNames[wh.User.Name] {
					uniqueNames[wh.User.Name] = true
					approvedNames = append(approvedNames, wh.User.Name)
				}
			}

			// 1. Notificar a cada profesional
			for _, hours := range userHours {
				if len(hours) > 0 {
					professional := hours[0].User
					dates := ""
					for i, h := range hours {
						if i > 0 {
							dates += ", "
						}
						dates += h.WorkDate.Format("02/01")
					}
					profMsg := fmt.Sprintf("✅ Tus horas de los días *%s* han sido aprobadas.", dates)
					_ = s.notifSvc.CreateNotification(professional.ID, "work_hour_approved", "Jornadas aprobadas", profMsg, map[string]interface{}{"dates": dates, "link": "/work-hours"})
				}
			}

			// 2. Notificar al aprobador (Resumen masivo) internamente
			if approver != nil {
				summary := "📢 *Resumen de Aprobación de Jornadas*\nSe han aprobado las jornadas de los siguientes profesionales:\n"
				for _, name := range approvedNames {
					summary += fmt.Sprintf("• %s\n", name)
				}
				_ = s.notifSvc.CreateNotification(approver.ID, "work_hour_approved_summary", "Resumen de aprobación", summary, map[string]interface{}{"link": "/work-hours"})
			}
		}()
	}
	return err
}

func (s *workHourService) Reject(ids []uint, userID uint, role string, isSuperadmin bool, isManager bool, tenantID uint, reason string) error {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return errors.New("Rejection reason is required")
	}

	var workHours []models.WorkHour
	var err error
	if !isSuperadmin && tenantID > 0 {
		workHours, err = s.repo.FindManyByIDsAndTenant(ids, tenantID)
	} else {
		workHours, err = s.repo.FindManyByIDs(ids)
	}
	if err != nil {
		return errors.New("Failed to fetch work hours")
	}

	if len(workHours) == 0 {
		return errors.New("No work hours found")
	}

	for _, wh := range workHours {
		canReject := false

		if isSuperadmin {
			canReject = true
		} else if role == string(models.UserTypeEmployer) || role == "empleador" {
			if wh.User.EmpleadorID != nil && *wh.User.EmpleadorID == userID {
				canReject = true
			}
		} else if isManager {
			// Separación de funciones: un manager NO puede rechazar sus propias
			// jornadas, solo las de sus subordinados directos (per-empresa).
			if wh.UserID != userID {
				if MultiManagerReadsEnabled() {
					if ok, _ := s.employmentRepo.IsManagerOf(wh.UserID, wh.TenantID, userID); ok {
						canReject = true
					}
				} else {
					if emp, err := s.employmentRepo.GetActive(wh.UserID, wh.TenantID); err == nil && emp != nil && emp.ManagerID != nil && *emp.ManagerID == userID {
						canReject = true
					}
				}
			}
		}

		if !canReject {
			return errors.New("No tienes permiso para rechazar estas horas.")
		}
	}

	if !isSuperadmin && tenantID > 0 {
		err = s.repo.RejectMultipleAndTenant(ids, userID, time.Now(), utils.SanitizeHTML(reason), tenantID)
	} else {
		err = s.repo.RejectMultiple(ids, userID, time.Now(), utils.SanitizeHTML(reason))
	}
	if err == nil {
		go func() {
			// Resolve who rejected once (same approver for the whole batch).
			rejectedByName := ""
			if approver, err := s.userRepo.GetByID(userID); err == nil && approver != nil {
				rejectedByName = approver.Name
			}

			userHours := make(map[uint][]models.WorkHour)
			for _, wh := range workHours {
				userHours[wh.UserID] = append(userHours[wh.UserID], wh)
			}

			for _, hours := range userHours {
				if len(hours) == 0 {
					continue
				}
				professional := hours[0].User
				dates := ""
				for i, h := range hours {
					if i > 0 {
						dates += ", "
					}
					dates += h.WorkDate.Format("02/01")
				}
				msg := fmt.Sprintf("Tus horas de los dÃ­as %s fueron rechazadas. Motivo: %s", dates, reason)
				_ = s.notifSvc.CreateNotification(professional.ID, "work_hour_rejected", "Jornadas rechazadas", msg, map[string]interface{}{"dates": dates, "reason": reason, "link": "/work-hours"})

				// Surface the rejection as an internal alert in the support tickets area.
				if s.ticketSvc != nil {
					companyName := ""
					if professional.EmpleadorID != nil {
						if employer, err := s.userRepo.GetByID(*professional.EmpleadorID); err == nil && employer != nil {
							companyName = employer.CompanyName
						}
					}
					_ = s.ticketSvc.CreateWorkHourRejectionAlert(RejectionAlertInput{
						ProfessionalID:    professional.ID,
						ProfessionalName:  professional.Name,
						ProfessionalEmail: professional.Email,
						ProfessionalPhone: professional.PhoneNumber,
						CompanyName:       companyName,
						RejectedByName:    rejectedByName,
						Dates:             dates,
						Reason:            reason,
					})
				}
			}
		}()
	}
	return err
}

func (s *workHourService) GetSummary(userID uint, role string, isSuperadmin, isManager bool, tenantID, companyFilter uint, userIDFilter string) (map[string]float64, error) {
	filters := make(map[string]interface{})

	// Filter for the current month
	now := time.Now()
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	filters["start_date"] = startOfMonth
	filters["end_date"] = now

	if isSuperadmin {
		// Superadmin must scope to a company; otherwise return an empty summary.
		if companyFilter == 0 {
			return map[string]float64{"total_hours": 0, "approved_hours": 0, "pending_hours": 0, "rejected_hours": 0}, nil
		}
		filters["tenant_id"] = companyFilter
	} else if isManager {
		// Un manager solo ve el resumen de su equipo (él + subordinados), igual
		// que su lista de pendientes; no el total de toda la empresa.
		if tenantID > 0 {
			filters["tenant_id"] = tenantID
		}
		if MultiManagerReadsEnabled() {
			filters["manager_or_user_links_id"] = userID
		} else {
			filters["manager_or_user_id"] = userID
		}
	} else if (role == string(models.UserTypeEmployer) || role == "empleador") && tenantID > 0 {
		filters["tenant_id"] = tenantID
	} else {
		filters["user_id"] = userID
	}

	// Optional per-employee scope (superadmin or employer).
	if userIDFilter != "" && (isSuperadmin || role == string(models.UserTypeEmployer) || role == "empleador") {
		if uid, err := strconv.ParseUint(userIDFilter, 10, 32); err == nil {
			filters["user_id"] = uint(uid)
		}
	}

	return s.repo.GetSummary(filters)
}

func (s *workHourService) GetPending(tenantID, userID uint, role string, isSuperadmin bool, isManager bool, companyFilter uint, userIDFilter string) ([]models.WorkHour, error) {
	filters := make(map[string]interface{})
	filters["approved"] = false
	filters["rejected"] = false

	// Filter to current month
	now := time.Now()
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	filters["start_date"] = startOfMonth
	filters["end_date"] = now

	if isSuperadmin {
		// Superadmin must scope to a company; otherwise return nothing.
		if companyFilter == 0 {
			return []models.WorkHour{}, nil
		}
		filters["tenant_id"] = companyFilter
		if userIDFilter != "" {
			if uid, err := strconv.ParseUint(userIDFilter, 10, 32); err == nil {
				filters["user_id"] = uint(uid)
			}
		}
		res, _, err := s.repo.FindAll(filters, 0, 1000)
		return res, err
	}

	if isManager {
		if tenantID > 0 {
			filters["tenant_id"] = tenantID
		}
		// solo subordinados: el manager no aprueba sus propias horas
		if MultiManagerReadsEnabled() {
			filters["manager_links_id"] = userID
		} else {
			filters["manager_id"] = userID
		}
		res, _, err := s.repo.FindAll(filters, 0, 1000)
		return res, err
	}

	if isEmployerRole(role) {
		tenantID = userID
	}

	if tenantID == 0 {
		return nil, errors.New("Only employers can access this resource")
	}

	filters["tenant_id"] = tenantID

	res, _, err := s.repo.FindAll(filters, 0, 1000)
	return res, err
}
