package service

import (
	"errors"
	"strconv"
	"time"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
	"github.com/obertrack/backend/internal/utils"
)

type WorkHourService interface {
	GetAll(userID uint, role string, isSuperadmin bool, userIDFilter, startDate, endDate string, offset, limit int) ([]models.WorkHour, int64, error)
	Create(userID uint, reqData map[string]interface{}) (*models.WorkHour, error)
	Update(id uint, reqData map[string]interface{}) (*models.WorkHour, error)
	Approve(ids []uint, userID uint, role string, isSuperadmin bool, isManager bool) error
	GetSummary(userID uint, role string, isSuperadmin bool) (map[string]float64, error)
	GetPending(empleadorID, userID uint, role string, isSuperadmin bool) ([]models.WorkHour, error)
}

type workHourService struct {
	repo repository.WorkHourRepository
}

func NewWorkHourService(repo repository.WorkHourRepository) WorkHourService {
	return &workHourService{repo: repo}
}

func (s *workHourService) parseStringVal(val interface{}) string {
	if str, ok := val.(string); ok {
		return str
	}
	return ""
}

func (s *workHourService) parseFloatVal(val interface{}) float64 {
	if f, ok := val.(float64); ok {
		return f
	}
	return 0
}

func (s *workHourService) GetAll(userID uint, role string, isSuperadmin bool, userIDFilter, startDate, endDate string, offset, limit int) ([]models.WorkHour, int64, error) {
	filters := make(map[string]interface{})

	if !isSuperadmin {
		if role == string(models.UserTypeEmployer) || role == "empleador" {
			filters["employer_id"] = userID
		} else if role == string(models.UserTypeProfessional) || role == "profesional" {
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
	} else if hoursWorked == 0 {
		hoursWorked = 8
	}

	workHour := &models.WorkHour{
		UserID:        userID,
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
		return nil, errors.New("Failed to create work hour")
	}

	// Fetch with preload for response
	return s.repo.FindByID(workHour.ID)
}

func (s *workHourService) Update(id uint, reqData map[string]interface{}) (*models.WorkHour, error) {
	workHour, err := s.repo.FindByID(id)
	if err != nil {
		return nil, errors.New("Work hour not found")
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
		if workTypeStr == "absence" {
			workHour.HoursWorked = 0
		} else {
			workHour.HoursWorked = 8
		}
	}

	if act := s.parseStringVal(reqData["activities"]); act != "" {
		workHour.Activities = utils.SanitizeHTML(act)
	}
	if com := s.parseStringVal(reqData["comments"]); com != "" {
		workHour.Comments = utils.SanitizeHTML(com)
	}

	if err := s.repo.Update(workHour); err != nil {
		return nil, errors.New("Failed to update work hour")
	}

	return s.repo.FindByID(workHour.ID)
}

func (s *workHourService) Approve(ids []uint, userID uint, role string, isSuperadmin bool, isManager bool) error {
	workHours, err := s.repo.FindManyByIDs(ids)
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

	return s.repo.ApproveMultiple(ids, userID, time.Now())
}

func (s *workHourService) GetSummary(userID uint, role string, isSuperadmin bool) (map[string]float64, error) {
	filters := make(map[string]interface{})

	if !isSuperadmin {
		if role == string(models.UserTypeEmployer) || role == "empleador" {
			filters["employer_id"] = userID
		} else {
			filters["user_id"] = userID
		}
	}

	return s.repo.GetSummary(filters)
}

func (s *workHourService) GetPending(empleadorID, userID uint, role string, isSuperadmin bool) ([]models.WorkHour, error) {
	filters := make(map[string]interface{})
	filters["approved"] = false

	if isSuperadmin {
		res, _, err := s.repo.FindAll(filters, 0, 1000)
		return res, err
	}

	if role == "empresa" || role == "empleador" {
		empleadorID = userID
	}

	if empleadorID == 0 {
		return nil, errors.New("Only employers can access this resource")
	}

	filters["employer_id"] = empleadorID

	res, _, err := s.repo.FindAll(filters, 0, 1000)
	return res, err
}
