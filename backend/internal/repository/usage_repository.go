package repository

import (
	"fmt"
	"strings"
	"time"

	"github.com/obertrack/backend/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// UsageOverview son los números de portada de la pestaña Uso.
type UsageOverview struct {
	EligibleUsers     int64   `json:"eligible_users"`
	ActiveUsers       int64   `json:"active_users"`
	NeverActive       int64   `json:"never_active"`
	AdoptionRate      float64 `json:"adoption_rate"`
	DAU               int64   `json:"dau"`
	WAU               int64   `json:"wau"`
	MAU               int64   `json:"mau"`
	Stickiness        float64 `json:"stickiness"`
	EligibleCompanies int64   `json:"eligible_companies"`
	ActiveCompanies   int64   `json:"active_companies"`
	CompanyRate       float64 `json:"company_rate"`
	AvgActiveDays     float64 `json:"avg_active_days"`

	// El mismo cálculo sobre el período INMEDIATAMENTE anterior, del mismo
	// largo. Es lo que distingue a una empresa que se está yendo de una que
	// nunca despegó: sin esto, las dos se pintan igual.
	PrevActiveUsers     int64   `json:"prev_active_users"`
	PrevAdoptionRate    float64 `json:"prev_adoption_rate"`
	AdoptionDelta       float64 `json:"adoption_delta"`
	PrevActiveCompanies int64   `json:"prev_active_companies"`
	PrevCompanyRate     float64 `json:"prev_company_rate"`
	CompanyDelta        float64 `json:"company_delta"`

	// TrackingSince es el primer día con datos. Sin él, un "12% de adopción"
	// recién activado el contador se lee como catástrofe en vez de como
	// "llevamos dos días midiendo".
	TrackingSince *time.Time `json:"tracking_since"`
	// Comparable es falso mientras el contador no cubra ENTERO el período
	// anterior. Con media ventana medida, cualquier variación sería un
	// artefacto de cuándo encendimos la medición, no un cambio de conducta, y
	// pintar flechas con eso sería peor que no pintar nada.
	Comparable bool `json:"comparable"`
}

// ModuleUsage es el "% de uso" de un módulo: cuánta gente distinta lo tocó en
// el período, sobre el total de gente que podría haberlo tocado.
type ModuleUsage struct {
	Module    string  `json:"module"`
	Users     int64   `json:"users"`
	Hits      int64   `json:"hits"`
	Rate      float64 `json:"rate"`
	PrevUsers int64   `json:"prev_users"`
	PrevRate  float64 `json:"prev_rate"`
	// Delta va en PUNTOS porcentuales, no en porcentaje de variación: pasar del
	// 2% al 4% es "+2 puntos", y decir "+100%" de una base de dos personas
	// alarmaría por nada.
	Delta float64 `json:"delta"`
}

type UsageDay struct {
	Day   string `json:"day"`
	Users int64  `json:"users"`
}

type CompanyUsage struct {
	CompanyID       uint       `json:"company_id"`
	CompanyName     string     `json:"company_name"`
	TotalUsers      int64      `json:"total_users"`
	ActiveUsers     int64      `json:"active_users"`
	Rate            float64    `json:"rate"`
	ChatUsers       int64      `json:"chat_users"`
	ChatRate        float64    `json:"chat_rate"`
	Hits            int64      `json:"hits"`
	LastActive      *time.Time `json:"last_active"`
	PrevActiveUsers int64      `json:"prev_active_users"`
	PrevRate        float64    `json:"prev_rate"`
	Delta           float64    `json:"delta"`
}

type PersonUsage struct {
	UserID      uint       `json:"user_id"`
	Name        string     `json:"name"`
	Email       string     `json:"email"`
	UserType    string     `json:"user_type"`
	CompanyID   uint       `json:"company_id"`
	CompanyName string     `json:"company_name"`
	ActiveDays  int64      `json:"active_days"`
	Hits        int64      `json:"hits"`
	LastActive  *time.Time `json:"last_active"`
	Modules     string     `json:"modules"`
	// Online lo rellena el handler desde el hub de WebSockets; la base de datos
	// no sabe quién tiene la pestaña abierta ahora mismo.
	Online bool `json:"online"`
}

// NeverActiveUser es una cuenta que JAMÁS ha aparecido en el contador de uso.
// Es el hueco de activación: alguien a quien se le dieron credenciales y nunca
// las usó, que es un problema distinto —y más barato de arreglar— que el de
// quien usaba la app y dejó de hacerlo.
type NeverActiveUser struct {
	UserID      uint      `json:"user_id"`
	Name        string    `json:"name"`
	Email       string    `json:"email"`
	UserType    string    `json:"user_type"`
	CompanyID   uint      `json:"company_id"`
	CompanyName string    `json:"company_name"`
	CreatedAt   time.Time `json:"created_at"`
	DaysSince   int64     `json:"days_since"`
	// Certain distingue el hecho de la laguna. El alta posterior al primer día
	// medido significa que esta cuenta ha existido SIEMPRE bajo observación y
	// nunca abrió nada: es un hecho. Con un alta anterior solo sabemos que no
	// ha entrado desde que medimos, que no es lo mismo, y presentarlos juntos
	// convertiría la lista en una acusación falsa contra medio padrón.
	Certain bool `json:"certain"`
}

// UsageScope acota QUÉ se mide. CompanyID en 0 significa toda la plataforma;
// con un id, las mismas consultas responden la misma pregunta pero para una
// sola empresa, que es lo que se pinta en su ficha.
//
// Va como struct y no como tres parámetros sueltos porque los tres viajan
// siempre juntos: con una firma posicional, añadir el alcance por empresa
// habría significado colar un `0` en cada llamada y confundir days con
// companyID sería un error que compila.
type UsageScope struct {
	Days        int
	ClientsOnly bool
	CompanyID   uint
}

// where arma el filtro de usuarios del alcance: elegibilidad más, si toca, la
// pertenencia a una empresa.
func (s UsageScope) where() string {
	w := eligibleUsers(s.ClientsOnly)
	if s.CompanyID > 0 {
		w += fmt.Sprintf(" AND (CASE WHEN u.user_type = 'empleador' THEN u.id ELSE u.empleador_id END) = %d", s.CompanyID)
	}
	return w
}

// PeopleFilter acota el listado de personas.
type PeopleFilter struct {
	Days        int
	ClientsOnly bool
	CompanyID   uint
	Search      string
	// Status: "" (todas), "active", "inactive" (sin una sola marca en el período).
	Status string
	Limit  int
	Offset int
}

type UsageRepository interface {
	UpsertActivity(entries []models.UserActivityDaily) error
	Overview(scope UsageScope) (UsageOverview, error)
	ModuleUsage(scope UsageScope) ([]ModuleUsage, error)
	DailyTrend(scope UsageScope) ([]UsageDay, error)
	CompanyUsage(days int) ([]CompanyUsage, error)
	PeopleUsage(f PeopleFilter) ([]PersonUsage, int64, error)
	NeverActive(scope UsageScope, limit, offset int) ([]NeverActiveUser, int64, error)
	// StaleCompanies lista las empresas vivas sin una sola señal de uso en los
	// últimos days días. Lo usa el vigía que avisa por correo.
	StaleCompanies(days int) ([]CompanyUsage, error)
}

type usageRepository struct {
	db *gorm.DB
}

func NewUsageRepository(db *gorm.DB) UsageRepository {
	return &usageRepository{db: db}
}

// eligibleUsers es el denominador de TODOS los porcentajes de la pestaña.
//
// clientsOnly deja fuera a superadmins, Customer Success y analistas de IT: son
// el equipo de casa, entran a diario por oficio, y contarlos como "usuarios
// activos" subiría la adopción sin que ningún cliente haya abierto nada.
func eligibleUsers(clientsOnly bool) string {
	base := `u.deleted_at IS NULL AND u.is_active = true AND u.is_system = false`
	if clientsOnly {
		return base + ` AND u.user_type IN ('profesional','empleador')`
	}
	return base
}

// usageWindows arma las tres ventanas de la comparación: el período pedido, el
// inmediatamente anterior del mismo largo, y los dos juntos. days=30 son los 29
// días anteriores más hoy, no 31.
//
// days entra como int de Go —el handler lo acota a 1..365 y los llamadores
// internos pasan constantes—, así que interpolarlo no puede inyectar nada. Con
// marcadores, cada consulta acabaría con seis "?" seguidos y reordenar un FILTER
// sería un error silencioso que ni siquiera rompe la compilación.
func usageWindows(days int) (cur, prev, both string) {
	if days < 1 {
		days = 1
	}
	cur = fmt.Sprintf("a.day >= CURRENT_DATE - %d", days-1)
	prev = fmt.Sprintf("a.day >= CURRENT_DATE - %d AND a.day < CURRENT_DATE - %d", 2*days-1, days-1)
	both = fmt.Sprintf("a.day >= CURRENT_DATE - %d", 2*days-1)
	return
}

func (r *usageRepository) UpsertActivity(entries []models.UserActivityDaily) error {
	if len(entries) == 0 {
		return nil
	}
	// SkipHooks: sin esto el hook de auditoría de datos escribiría una fila en
	// audit_logs por cada contador volcado —miles al día— y la propia medición
	// sería la mayor fuente de ruido de la auditoría.
	return r.db.Session(&gorm.Session{SkipHooks: true, NewDB: true}).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "user_id"}, {Name: "day"}, {Name: "module"}},
			DoUpdates: clause.Assignments(map[string]interface{}{
				"hits":      gorm.Expr("user_activity_daily.hits + excluded.hits"),
				"last_at":   gorm.Expr("GREATEST(user_activity_daily.last_at, excluded.last_at)"),
				"tenant_id": gorm.Expr("excluded.tenant_id"),
			}),
		}).
		CreateInBatches(&entries, 200).Error
}

func (r *usageRepository) Overview(scope UsageScope) (UsageOverview, error) {
	var out UsageOverview
	days := scope.Days
	elig := scope.where()
	cur, prev, both := usageWindows(days)

	if err := r.db.Raw(`SELECT COUNT(*) FROM users u WHERE ` + elig).Scan(&out.EligibleUsers).Error; err != nil {
		return out, err
	}

	// Un solo recorrido para las dos ventanas más DAU/WAU/MAU: son seis cortes
	// del mismo dato y separarlos serían seis escaneos de la misma tabla.
	var counts struct {
		Active     int64
		PrevActive int64
		DAU        int64
		WAU        int64
		MAU        int64
		Days       float64
	}
	if err := r.db.Raw(`
		SELECT
		  COUNT(DISTINCT a.user_id) FILTER (WHERE ` + cur + `) AS active,
		  COUNT(DISTINCT a.user_id) FILTER (WHERE ` + prev + `) AS prev_active,
		  COUNT(DISTINCT a.user_id) FILTER (WHERE a.day = CURRENT_DATE) AS dau,
		  COUNT(DISTINCT a.user_id) FILTER (WHERE a.day >= CURRENT_DATE - 6) AS wau,
		  COUNT(DISTINCT a.user_id) FILTER (WHERE a.day >= CURRENT_DATE - 29) AS mau,
		  COALESCE(
		    COUNT(*) FILTER (WHERE ` + cur + `)::float
		    / NULLIF(COUNT(DISTINCT a.user_id) FILTER (WHERE ` + cur + `), 0), 0) AS days
		FROM user_activity_daily a
		JOIN users u ON u.id = a.user_id
		WHERE a.module = '` + models.ModuleApp + `' AND ` + elig + ` AND ` + both,
	).Scan(&counts).Error; err != nil {
		return out, err
	}
	out.ActiveUsers, out.PrevActiveUsers = counts.Active, counts.PrevActive
	out.DAU, out.WAU, out.MAU = counts.DAU, counts.WAU, counts.MAU
	out.AvgActiveDays = counts.Days
	out.NeverActive = out.EligibleUsers - out.ActiveUsers
	if out.NeverActive < 0 {
		out.NeverActive = 0
	}

	// Empresas: el denominador son las cuentas empleador vivas; el numerador,
	// aquellas donde ALGUIEN (la cuenta empresa o cualquiera de su gente) abrió
	// la app. Una empresa cuya plantilla usa la herramienta la está usando
	// aunque el titular de la cuenta no entre nunca.
	//
	// Mirando UNA empresa este recuento no se calcula: saldría el número de toda
	// la plataforma junto a los datos de una sola cuenta, y en una ficha eso se
	// lee como si fueran suyos. Se quedan en cero y la ficha no los pinta.
	if scope.CompanyID > 0 {
		out.AdoptionRate = pct(out.ActiveUsers, out.EligibleUsers)
		out.PrevAdoptionRate = pct(out.PrevActiveUsers, out.EligibleUsers)
		out.AdoptionDelta = out.AdoptionRate - out.PrevAdoptionRate
		out.Stickiness = pct(out.DAU, out.MAU)
		var since *time.Time
		if err := r.db.Raw(`SELECT MIN(day) FROM user_activity_daily`).Scan(&since).Error; err != nil {
			return out, err
		}
		out.TrackingSince = since
		out.Comparable = comparable(since, days)
		return out, nil
	}

	if err := r.db.Raw(`
		SELECT COUNT(*) FROM users u
		WHERE u.user_type = 'empleador' AND u.deleted_at IS NULL AND u.is_active = true AND u.is_system = false
	`).Scan(&out.EligibleCompanies).Error; err != nil {
		return out, err
	}
	var companies struct {
		Active     int64
		PrevActive int64
	}
	if err := r.db.Raw(`
		SELECT
		  COUNT(DISTINCT CASE WHEN u.user_type = 'empleador' THEN u.id ELSE u.empleador_id END)
		    FILTER (WHERE ` + cur + `) AS active,
		  COUNT(DISTINCT CASE WHEN u.user_type = 'empleador' THEN u.id ELSE u.empleador_id END)
		    FILTER (WHERE ` + prev + `) AS prev_active
		FROM user_activity_daily a
		JOIN users u ON u.id = a.user_id
		WHERE a.module = '` + models.ModuleApp + `'
		  AND u.deleted_at IS NULL AND u.is_system = false
		  AND (u.user_type = 'empleador' OR u.empleador_id IS NOT NULL)
		  AND ` + both,
	).Scan(&companies).Error; err != nil {
		return out, err
	}
	out.ActiveCompanies, out.PrevActiveCompanies = companies.Active, companies.PrevActive

	var since *time.Time
	if err := r.db.Raw(`SELECT MIN(day) FROM user_activity_daily`).Scan(&since).Error; err != nil {
		return out, err
	}
	out.TrackingSince = since

	out.AdoptionRate = pct(out.ActiveUsers, out.EligibleUsers)
	out.PrevAdoptionRate = pct(out.PrevActiveUsers, out.EligibleUsers)
	out.AdoptionDelta = out.AdoptionRate - out.PrevAdoptionRate
	out.CompanyRate = pct(out.ActiveCompanies, out.EligibleCompanies)
	out.PrevCompanyRate = pct(out.PrevActiveCompanies, out.EligibleCompanies)
	out.CompanyDelta = out.CompanyRate - out.PrevCompanyRate
	// Stickiness = DAU/MAU: de la gente que usa la app en el mes, qué porción
	// la usa un día cualquiera. Es la que distingue "entraron una vez" de
	// "viven aquí", que es la pregunta real detrás de "¿la usan?".
	out.Stickiness = pct(out.DAU, out.MAU)
	out.Comparable = comparable(since, days)
	return out, nil
}

// comparable responde si el contador cubre entero el período anterior. Con la
// medición encendida a mitad de esa ventana, la "caída" que saldría sería el
// hueco de datos, no una caída.
func comparable(trackingSince *time.Time, days int) bool {
	if trackingSince == nil {
		return false
	}
	start := time.Now().AddDate(0, 0, -(2*days - 1))
	y, m, d := start.Date()
	startDay := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	ty, tm, td := trackingSince.Date()
	sinceDay := time.Date(ty, tm, td, 0, 0, 0, 0, time.UTC)
	return !sinceDay.After(startDay)
}

func (r *usageRepository) ModuleUsage(scope UsageScope) ([]ModuleUsage, error) {
	elig := scope.where()
	cur, prev, both := usageWindows(scope.Days)

	var eligible int64
	if err := r.db.Raw(`SELECT COUNT(*) FROM users u WHERE ` + elig).Scan(&eligible).Error; err != nil {
		return nil, err
	}

	var rows []ModuleUsage
	// HAVING sobre las DOS ventanas: un módulo que cayó a cero sigue en la
	// lista, con su barra vacía y su flecha roja. Filtrarlo por no tener uso
	// hoy escondería justo el abandono que la comparación existe para enseñar.
	if err := r.db.Raw(`
		SELECT a.module,
		       COUNT(DISTINCT a.user_id) FILTER (WHERE ` + cur + `) AS users,
		       COUNT(DISTINCT a.user_id) FILTER (WHERE ` + prev + `) AS prev_users,
		       COALESCE(SUM(a.hits) FILTER (WHERE ` + cur + `), 0) AS hits
		FROM user_activity_daily a
		JOIN users u ON u.id = a.user_id
		WHERE ` + elig + ` AND ` + both + `
		GROUP BY a.module
		HAVING COUNT(*) FILTER (WHERE ` + cur + `) > 0 OR COUNT(*) FILTER (WHERE ` + prev + `) > 0
		ORDER BY users DESC, hits DESC
	`).Scan(&rows).Error; err != nil {
		return nil, err
	}
	for i := range rows {
		rows[i].Rate = pct(rows[i].Users, eligible)
		rows[i].PrevRate = pct(rows[i].PrevUsers, eligible)
		rows[i].Delta = rows[i].Rate - rows[i].PrevRate
	}
	return rows, nil
}

func (r *usageRepository) DailyTrend(scope UsageScope) ([]UsageDay, error) {
	var rows []UsageDay
	// generate_series rellena los días sin actividad con un 0. Sin eso el
	// gráfico une el viernes con el lunes y un fin de semana muerto parece una
	// meseta de uso constante.
	err := r.db.Raw(`
		SELECT TO_CHAR(d.day, 'YYYY-MM-DD') AS day, COALESCE(x.users, 0) AS users
		FROM generate_series(
		       CURRENT_DATE - ((? - 1) * INTERVAL '1 day'), CURRENT_DATE, INTERVAL '1 day'
		     ) AS d(day)
		LEFT JOIN (
		  SELECT a.day, COUNT(DISTINCT a.user_id) AS users
		  FROM user_activity_daily a
		  JOIN users u ON u.id = a.user_id
		  WHERE a.module = ? AND `+scope.where()+`
		  GROUP BY a.day
		) x ON x.day = d.day::date
		ORDER BY d.day ASC
	`, scope.Days, models.ModuleApp).Scan(&rows).Error
	return rows, err
}

// companyUsageSQL es la consulta de uso por empresa. Se comparte con
// StaleCompanies para que el vigía que manda el correo y la tabla que mira
// Customer Success no puedan discrepar sobre qué es una empresa sin uso.
func companyUsageSQL(days int) string {
	cur, prev, both := usageWindows(days)
	return `
		WITH plantilla AS (
		  SELECT CASE WHEN u.user_type = 'empleador' THEN u.id ELSE u.empleador_id END AS company_id,
		         u.id AS user_id
		  FROM users u
		  WHERE u.deleted_at IS NULL AND u.is_active = true AND u.is_system = false
		    AND (u.user_type = 'empleador' OR u.empleador_id IS NOT NULL)
		),
		uso AS (
		  SELECT p.company_id,
		         COUNT(DISTINCT a.user_id) FILTER (WHERE a.module = '` + models.ModuleApp + `' AND ` + cur + `) AS active_users,
		         COUNT(DISTINCT a.user_id) FILTER (WHERE a.module = '` + models.ModuleApp + `' AND ` + prev + `) AS prev_active_users,
		         COUNT(DISTINCT a.user_id) FILTER (WHERE a.module = '` + models.ModuleChat + `' AND ` + cur + `) AS chat_users,
		         COALESCE(SUM(a.hits) FILTER (WHERE a.module = '` + models.ModuleApp + `' AND ` + cur + `), 0) AS hits
		  FROM user_activity_daily a
		  JOIN plantilla p ON p.user_id = a.user_id
		  WHERE ` + both + `
		  GROUP BY p.company_id
		),
		-- La última señal de vida se busca sin ventana: si la empresa lleva seis
		-- meses sin entrar, la respuesta es "hace seis meses", no "nunca".
		ultima AS (
		  SELECT p.company_id, MAX(a.last_at) AS last_active
		  FROM user_activity_daily a
		  JOIN plantilla p ON p.user_id = a.user_id
		  GROUP BY p.company_id
		)
		SELECT c.id AS company_id,
		       COALESCE(NULLIF(c.company_name, ''), c.name) AS company_name,
		       (SELECT COUNT(*) FROM plantilla p WHERE p.company_id = c.id) AS total_users,
		       COALESCE(uso.active_users, 0) AS active_users,
		       COALESCE(uso.prev_active_users, 0) AS prev_active_users,
		       COALESCE(uso.chat_users, 0) AS chat_users,
		       COALESCE(uso.hits, 0) AS hits,
		       ultima.last_active
		FROM users c
		LEFT JOIN uso ON uso.company_id = c.id
		LEFT JOIN ultima ON ultima.company_id = c.id
		WHERE c.user_type = 'empleador' AND c.deleted_at IS NULL AND c.is_active = true AND c.is_system = false`
}

func (r *usageRepository) CompanyUsage(days int) ([]CompanyUsage, error) {
	var rows []CompanyUsage
	// LEFT JOIN a propósito: las empresas con CERO actividad son justo las que
	// Customer Success necesita ver primero, y un INNER JOIN las escondería.
	err := r.db.Raw(companyUsageSQL(days) + `
		ORDER BY active_users DESC, total_users DESC
	`).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	fillCompanyRates(rows)
	return rows, nil
}

func (r *usageRepository) StaleCompanies(days int) ([]CompanyUsage, error) {
	var rows []CompanyUsage
	// Solo empresas CON gente: una cuenta recién creada y todavía vacía no está
	// abandonada, está a medio montar, y avisar por ella sería ruido en el
	// primer correo que reciba el equipo.
	err := r.db.Raw(companyUsageSQL(days) + `
		  AND COALESCE(uso.active_users, 0) = 0
		  AND (SELECT COUNT(*) FROM plantilla p WHERE p.company_id = c.id) > 0
		ORDER BY ultima.last_active DESC NULLS LAST
	`).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	fillCompanyRates(rows)
	return rows, nil
}

func fillCompanyRates(rows []CompanyUsage) {
	for i := range rows {
		rows[i].Rate = pct(rows[i].ActiveUsers, rows[i].TotalUsers)
		rows[i].PrevRate = pct(rows[i].PrevActiveUsers, rows[i].TotalUsers)
		rows[i].Delta = rows[i].Rate - rows[i].PrevRate
		rows[i].ChatRate = pct(rows[i].ChatUsers, rows[i].TotalUsers)
	}
}

func (r *usageRepository) PeopleUsage(f PeopleFilter) ([]PersonUsage, int64, error) {
	cur, _, _ := usageWindows(f.Days)

	where := []string{eligibleUsers(f.ClientsOnly)}
	var args []interface{}

	if f.CompanyID > 0 {
		where = append(where, `(CASE WHEN u.user_type = 'empleador' THEN u.id ELSE u.empleador_id END) = ?`)
		args = append(args, f.CompanyID)
	}
	if s := strings.TrimSpace(f.Search); s != "" {
		where = append(where, `(u.name ILIKE ? OR u.email ILIKE ?)`)
		args = append(args, "%"+s+"%", "%"+s+"%")
	}
	switch f.Status {
	case "active":
		where = append(where, `act.active_days > 0`)
	case "inactive":
		where = append(where, `COALESCE(act.active_days, 0) = 0`)
	}

	// act: días y última marca de "app". mods: qué módulos tocó, para la
	// columna de módulos. Van como subconsultas laterales y no como dos JOIN
	// sueltos para que el FILTER de app no multiplique las filas de módulos.
	//
	// last_active se calcula SIN ventana (por eso el FILTER solo cubre los
	// contadores): quien entró por última vez hace tres meses debe leerse "hace
	// tres meses", no "nunca".
	base := `
		FROM users u
		LEFT JOIN LATERAL (
		  SELECT COUNT(*) FILTER (WHERE ` + cur + `) AS active_days,
		         COALESCE(SUM(a.hits) FILTER (WHERE ` + cur + `), 0) AS hits,
		         MAX(a.last_at) AS last_active
		  FROM user_activity_daily a
		  WHERE a.user_id = u.id AND a.module = '` + models.ModuleApp + `'
		) act ON TRUE
		LEFT JOIN LATERAL (
		  SELECT STRING_AGG(DISTINCT a.module, ',') AS modules
		  FROM user_activity_daily a
		  WHERE a.user_id = u.id AND a.module <> '` + models.ModuleApp + `' AND ` + cur + `
		) mods ON TRUE
		LEFT JOIN users e ON e.id = u.empleador_id
		WHERE ` + strings.Join(where, " AND ")

	var total int64
	if err := r.db.Raw(`SELECT COUNT(*) `+base, args...).Scan(&total).Error; err != nil {
		return nil, 0, err
	}

	limit := f.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	listArgs := append(append([]interface{}{}, args...), limit, f.Offset)

	var rows []PersonUsage
	// NULLS LAST: quien nunca entró es el hallazgo, no el relleno del final.
	if err := r.db.Raw(`
		SELECT u.id AS user_id, u.name, u.email, u.user_type,
		       COALESCE(CASE WHEN u.user_type = 'empleador' THEN u.id ELSE u.empleador_id END, 0) AS company_id,
		       `+companyNameSQL+` AS company_name,
		       COALESCE(act.active_days, 0) AS active_days,
		       COALESCE(act.hits, 0) AS hits,
		       act.last_active,
		       COALESCE(mods.modules, '') AS modules
		`+base+`
		ORDER BY act.last_active DESC NULLS LAST, u.name ASC
		LIMIT ? OFFSET ?
	`, listArgs...).Scan(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// companyNameSQL resuelve el nombre de la empresa de una persona: la cuenta
// empleador es su propia empresa, el resto la heredan de su empleador.
const companyNameSQL = `CASE WHEN u.user_type = 'empleador'
	THEN COALESCE(NULLIF(u.company_name, ''), u.name)
	ELSE COALESCE(NULLIF(e.company_name, ''), e.name, '') END`

func (r *usageRepository) NeverActive(scope UsageScope, limit, offset int) ([]NeverActiveUser, int64, error) {
	elig := scope.where()

	// NOT EXISTS sin ventana: "nunca" es nunca, no "no en los últimos 30 días"
	// —para eso ya está el filtro "sin actividad" del listado de personas—.
	base := `
		FROM users u
		LEFT JOIN users e ON e.id = u.empleador_id
		WHERE ` + elig + `
		  AND NOT EXISTS (SELECT 1 FROM user_activity_daily a WHERE a.user_id = u.id)`

	var total int64
	if err := r.db.Raw(`SELECT COUNT(*) ` + base).Scan(&total).Error; err != nil {
		return nil, 0, err
	}

	if limit <= 0 || limit > 200 {
		limit = 50
	}

	var rows []NeverActiveUser
	// Primero las certeras y, dentro de ellas, las más antiguas: una cuenta de
	// hace seis meses que nunca se estrenó es peor noticia que la de ayer.
	if err := r.db.Raw(`
		SELECT u.id AS user_id, u.name, u.email, u.user_type,
		       COALESCE(CASE WHEN u.user_type = 'empleador' THEN u.id ELSE u.empleador_id END, 0) AS company_id,
		       `+companyNameSQL+` AS company_name,
		       u.created_at,
		       (CURRENT_DATE - u.created_at::date) AS days_since,
		       COALESCE(u.created_at::date >= (SELECT MIN(day) FROM user_activity_daily), false) AS certain
		`+base+`
		ORDER BY certain DESC, u.created_at ASC
		LIMIT ? OFFSET ?
	`, limit, offset).Scan(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func pct(part, whole int64) float64 {
	if whole <= 0 {
		return 0
	}
	return (float64(part) / float64(whole)) * 100
}
