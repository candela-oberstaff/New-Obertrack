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
	GetAll(userID uint, role string, isSuperadmin bool, tenantID uint, userIDFilter, startDate, endDate string, offset, limit int) ([]models.WorkHour, int64, error)
	Create(userID uint, reqData map[string]interface{}) (*models.WorkHour, error)
	Update(id, tenantID, userID uint, role string, isManager, isSuperadmin bool, reqData map[string]interface{}) (*models.WorkHour, error)
	Approve(ids []uint, userID uint, role string, isSuperadmin bool, isManager bool, tenantID uint) error
	Reject(ids []uint, userID uint, role string, isSuperadmin bool, isManager bool, tenantID uint, reason string) error
	GetSummary(userID uint, role string, isSuperadmin bool, tenantID uint) (map[string]float64, error)
	GetPending(tenantID, userID uint, role string, isSuperadmin bool) ([]models.WorkHour, error)
	SendReportEmail(employerID uint, month int, year int) error
	GetPDFReportBytes(userID uint, month int, year int) ([]byte, string, error)
	GetExcelReportBytes(userID uint, month int, year int) ([]byte, string, error)
}

type workHourService struct {
	repo      repository.WorkHourRepository
	userRepo  repository.UserRepository
	notifSvc  NotificationService
	brevoSvc  *BrevoService
	ticketSvc TicketService
}

func NewWorkHourService(
	repo repository.WorkHourRepository,
	userRepo repository.UserRepository,
	notifSvc NotificationService,
	brevoSvc *BrevoService,
	ticketSvc TicketService,
) WorkHourService {
	return &workHourService{
		repo:      repo,
		userRepo:  userRepo,
		notifSvc:  notifSvc,
		brevoSvc:  brevoSvc,
		ticketSvc: ticketSvc,
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

func (s *workHourService) GetAll(userID uint, role string, isSuperadmin bool, tenantID uint, userIDFilter, startDate, endDate string, offset, limit int) ([]models.WorkHour, int64, error) {
	filters := make(map[string]interface{})

	if !isSuperadmin {
		if isEmployerRole(role) || role == "manager" {
			// Employers and managers see all work hours within their tenant
			if tenantID > 0 {
				filters["tenant_id"] = tenantID
			}
		} else {
			// Professionals only ever see their own work hours
			filters["user_id"] = userID
		}
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

func (s *workHourService) Create(userID uint, reqData map[string]interface{}) (*models.WorkHour, error) {
	workDateStr := s.parseStringVal(reqData["work_date"])
	workDate, err := time.Parse("2006-01-02", workDateStr)
	if err != nil {
		return nil, errors.New("Invalid date format")
	}

	today := time.Now().Truncate(24 * time.Hour)
	if workDate.After(today) {
		return nil, errors.New("No puedes registrar horas en fechas futuras")
	}

	if _, err := s.repo.FindByUserAndDate(userID, workDate); err == nil {
		return nil, errors.New("Ya existe un registro para esta fecha. Solo puedes registrar un máximo de una jornada por día.")
	}

	hoursWorked := s.parseFloatVal(reqData["hours_worked"])
	workTypeStr := s.parseStringVal(reqData["work_type"])
	workType := models.WorkTypeComplete
	absenceHours := s.parseFloatVal(reqData["absence_hours"])

	if workTypeStr == "absence" {
		workType = models.WorkTypeAbsence
		if hoursWorked == 0 {
			hoursWorked = 8 - absenceHours
			if hoursWorked < 0 {
				hoursWorked = 0
			}
		}
	} else if workTypeStr == "recover" {
		workType = models.WorkTypeRecover
	} else if hoursWorked == 0 {
		hoursWorked = 8
	}

	creator, _ := s.userRepo.GetByID(userID)

	workHour := &models.WorkHour{
		UserID:        userID,
		TenantID:      models.TenantForUser(creator),
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
			return nil, errors.New("Ya existe un registro para esta fecha. Solo puedes registrar un máximo de una jornada por día.")
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
				// Notificar al Manager
				if user.ManagerID != nil {
					_ = s.notifSvc.CreateNotification(*user.ManagerID, "work_hour_created", "Nueva jornada registrada", fmt.Sprintf("%s registró una jornada para el %s", user.Name, finalWH.WorkDate.Format("02/01")), map[string]interface{}{"id": finalWH.ID})
				}
				// Notificar al Empleador
				if user.EmpleadorID != nil {
					_ = s.notifSvc.CreateNotification(*user.EmpleadorID, "work_hour_created", "Nueva jornada registrada", fmt.Sprintf("%s registró una jornada para el %s", user.Name, finalWH.WorkDate.Format("02/01")), map[string]interface{}{"id": finalWH.ID})
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
		if !allowed && isManager {
			if owner, err := s.userRepo.GetByID(workHour.UserID); err == nil && owner.ManagerID != nil && *owner.ManagerID == userID {
				allowed = true
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
		workHour.AbsenceHours = s.parseFloatVal(val)
	}

	switch workHour.WorkType {
	case models.WorkTypeAbsence:
		hoursWorked := 8.0 - workHour.AbsenceHours
		if hoursWorked < 0 {
			hoursWorked = 0
		}
		workHour.HoursWorked = hoursWorked
	case models.WorkTypeRecover:
		workHour.AbsenceReason = ""
		workHour.AbsenceHours = 0
	default:
		workHour.HoursWorked = 8.0
		workHour.AbsenceReason = ""
		workHour.AbsenceHours = 0
	}

	if val, ok := reqData["hours_worked"]; ok {
		workHour.HoursWorked = s.parseFloatVal(val)
	}

	if act, ok := reqData["activities"]; ok {
		workHour.Activities = utils.SanitizeHTML(s.parseStringVal(act))
	}
	if com, ok := reqData["comments"]; ok {
		workHour.Comments = utils.SanitizeHTML(s.parseStringVal(com))
	}

	if workHour.UserID == userID && !isEmployerRole(role) && !isManager && !isSuperadmin {
		workHour.Approved = false
		workHour.ApprovedBy = nil
		workHour.ApprovedAt = nil
		workHour.Rejected = false
		workHour.RejectedBy = nil
		workHour.RejectedAt = nil
		workHour.RejectionReason = ""
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
		} else if role == "manager" || isManager {
			if wh.User.ManagerID != nil && *wh.User.ManagerID == userID {
				canApprove = true
			}
		}

		if !canApprove {
			return errors.New("Not authorized to approve work hours for user")
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
					_ = s.notifSvc.CreateNotification(professional.ID, "work_hour_approved", "Jornadas aprobadas", profMsg, map[string]interface{}{"dates": dates})
				}
			}

			// 2. Notificar al aprobador (Resumen masivo) internamente
			if approver != nil {
				summary := "📢 *Resumen de Aprobación de Jornadas*\nSe han aprobado las jornadas de los siguientes profesionales:\n"
				for _, name := range approvedNames {
					summary += fmt.Sprintf("• %s\n", name)
				}
				_ = s.notifSvc.CreateNotification(approver.ID, "work_hour_approved_summary", "Resumen de aprobación", summary, nil)
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
		} else if role == "manager" || isManager {
			if wh.User.ManagerID != nil && *wh.User.ManagerID == userID {
				canReject = true
			}
		}

		if !canReject {
			return errors.New("Not authorized to reject work hours for user")
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
				_ = s.notifSvc.CreateNotification(professional.ID, "work_hour_rejected", "Jornadas rechazadas", msg, map[string]interface{}{"dates": dates, "reason": reason})

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

func (s *workHourService) GetSummary(userID uint, role string, isSuperadmin bool, tenantID uint) (map[string]float64, error) {
	filters := make(map[string]interface{})

	// Filter for the current month
	now := time.Now()
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	filters["start_date"] = startOfMonth
	filters["end_date"] = now

	if !isSuperadmin {
		if (role == string(models.UserTypeEmployer) || role == "empleador" || role == "manager") && tenantID > 0 {
			filters["tenant_id"] = tenantID
		} else {
			filters["user_id"] = userID
		}
	}

	return s.repo.GetSummary(filters)
}

func (s *workHourService) GetPending(tenantID, userID uint, role string, isSuperadmin bool) ([]models.WorkHour, error) {
	filters := make(map[string]interface{})
	filters["approved"] = false
	filters["rejected"] = false

	// Filter to current month
	now := time.Now()
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	filters["start_date"] = startOfMonth
	filters["end_date"] = now

	if isSuperadmin {
		res, _, err := s.repo.FindAll(filters, 0, 1000)
		return res, err
	}

	if role == "empresa" || role == "empleador" {
		tenantID = userID
	}

	if tenantID == 0 {
		return nil, errors.New("Only employers can access this resource")
	}

	filters["tenant_id"] = tenantID

	res, _, err := s.repo.FindAll(filters, 0, 1000)
	return res, err
}
