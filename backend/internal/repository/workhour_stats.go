package repository

import (
	"time"
)

// WorkHourStats son los totales de un período: lo que necesitan las tarjetas
// de resumen (Horas, Inicio de la empresa, Reportes) sin descargar cada
// jornada. Se calculan en la base con los mismos filtros de alcance que la
// lista, así que cuadran con ella sin importar el tamaño del equipo.
type WorkHourStats struct {
	TotalHours     float64 `json:"total_hours"`
	ApprovedHours  float64 `json:"approved_hours"`
	PendingHours   float64 `json:"pending_hours"`
	RejectedHours  float64 `json:"rejected_hours"`
	Entries        int64   `json:"entries"`
	PendingEntries int64   `json:"pending_entries"`
	DaysWorked     int64   `json:"days_worked"`
	People         int64   `json:"people"`

	// Horas por día del período, solo los días con jornadas.
	Daily []WorkHourDay `json:"daily"`
	// Horas por persona, de más a menos.
	ByPerson []WorkHourPerson `json:"by_person"`
	// Quiénes tienen jornada el día [today] (si se pidió).
	TodayUserIDs []uint `json:"today_user_ids"`
}

type WorkHourDay struct {
	Date  string  `json:"date"`
	Hours float64 `json:"hours"`
}

type WorkHourPerson struct {
	UserID  uint    `json:"user_id"`
	Name    string  `json:"name"`
	Hours   float64 `json:"hours"`
	Entries int64   `json:"entries"`
}

// WorkHourStatsRepository lo implementa el repositorio real. Va aparte de
// WorkHourRepository para no obligar a los dobles de los tests a implementarlo:
// el servicio, si no lo encuentra, suma sobre la lista.
type WorkHourStatsRepository interface {
	Stats(filters map[string]interface{}, today *time.Time) (*WorkHourStats, error)
}

func (r *workHourRepository) Stats(filters map[string]interface{}, today *time.Time) (*WorkHourStats, error) {
	out := &WorkHourStats{
		Daily:        []WorkHourDay{},
		ByPerson:     []WorkHourPerson{},
		TodayUserIDs: []uint{},
	}

	var totals struct {
		Total          float64
		Approved       float64
		Rejected       float64
		Entries        int64
		PendingEntries int64
		Days           int64
		People         int64
	}
	if err := r.scoped(filters).Select(`
		COALESCE(SUM(work_hours.hours_worked), 0) AS total,
		COALESCE(SUM(CASE WHEN work_hours.approved THEN work_hours.hours_worked ELSE 0 END), 0) AS approved,
		COALESCE(SUM(CASE WHEN work_hours.rejected THEN work_hours.hours_worked ELSE 0 END), 0) AS rejected,
		COUNT(*) AS entries,
		COALESCE(SUM(CASE WHEN NOT work_hours.approved AND NOT work_hours.rejected THEN 1 ELSE 0 END), 0) AS pending_entries,
		COUNT(DISTINCT work_hours.work_date) AS days,
		COUNT(DISTINCT work_hours.user_id) AS people`).
		Scan(&totals).Error; err != nil {
		return nil, err
	}
	out.TotalHours = totals.Total
	out.ApprovedHours = totals.Approved
	out.RejectedHours = totals.Rejected
	// Igual que el resumen de siempre: pendiente es lo que no se decidió.
	out.PendingHours = totals.Total - totals.Approved - totals.Rejected
	if out.PendingHours < 0 {
		out.PendingHours = 0
	}
	out.Entries = totals.Entries
	out.PendingEntries = totals.PendingEntries
	out.DaysWorked = totals.Days
	out.People = totals.People

	var days []struct {
		Day   time.Time
		Hours float64
	}
	if err := r.scoped(filters).
		Select("work_hours.work_date AS day, COALESCE(SUM(work_hours.hours_worked), 0) AS hours").
		Group("work_hours.work_date").
		Order("work_hours.work_date").
		Scan(&days).Error; err != nil {
		return nil, err
	}
	for _, d := range days {
		out.Daily = append(out.Daily, WorkHourDay{Date: d.Day.Format("2006-01-02"), Hours: d.Hours})
	}

	if err := r.scoped(filters).
		Joins("JOIN users stats_user ON stats_user.id = work_hours.user_id").
		Select(`work_hours.user_id AS user_id, stats_user.name AS name,
			COALESCE(SUM(work_hours.hours_worked), 0) AS hours, COUNT(*) AS entries`).
		Group("work_hours.user_id, stats_user.name").
		Order("hours DESC").
		Scan(&out.ByPerson).Error; err != nil {
		return nil, err
	}

	if today != nil {
		day := *today
		scope := make(map[string]interface{}, len(filters))
		for k, v := range filters {
			scope[k] = v
		}
		scope["start_date"] = day
		scope["end_date"] = day
		if err := r.scoped(scope).
			Distinct("work_hours.user_id").
			Pluck("work_hours.user_id", &out.TodayUserIDs).Error; err != nil {
			return nil, err
		}
	}

	return out, nil
}
