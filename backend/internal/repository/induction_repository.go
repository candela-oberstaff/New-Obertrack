package repository

import (
	"errors"
	"sort"

	"github.com/obertrack/backend/internal/models"
	"gorm.io/gorm"
)

// InductionRepository accede al interruptor global de inducción, a la
// biblioteca de bloques, a los programas y su asignación por empresa, y a las
// invitaciones personales con su progreso por bloque.
type InductionRepository interface {
	// GetConfig devuelve la fila única de configuración (id=1). Si no existe,
	// devuelve una configuración apagada en lugar de error: la inducción
	// simplemente no aplica.
	GetConfig() (*models.InductionConfig, error)
	SaveConfig(cfg *models.InductionConfig) error

	// --- Biblioteca de videos ---
	ListVideos() ([]models.InductionVideo, error)
	GetVideo(id uint) (*models.InductionVideo, error)
	CreateVideo(video *models.InductionVideo) error
	UpdateVideo(id uint, updates map[string]interface{}) error
	DeleteVideo(id uint) error
	// CountBlocksUsingVideo dice cuántos bloques vivos usan el video.
	CountBlocksUsingVideo(videoID uint) (int64, error)

	// --- Biblioteca de bloques ---
	ListBlocks() ([]models.InductionBlock, error)
	GetBlock(id uint) (*models.InductionBlock, error)
	CreateBlock(block *models.InductionBlock) error
	UpdateBlock(id uint, updates map[string]interface{}) error
	DeleteBlock(id uint) error
	// CountProgramsUsingBlock dice en cuántos programas vivos está el bloque.
	CountProgramsUsingBlock(blockID uint) (int64, error)

	// --- Programas ---
	ListPrograms() ([]models.InductionProgram, error)
	// GetProgram carga el programa con sus bloques en orden y sus empresas.
	GetProgram(id uint) (*models.InductionProgram, error)
	CreateProgram(program *models.InductionProgram) error
	UpdateProgram(id uint, updates map[string]interface{}) error
	DeleteProgram(id uint) error
	// SetDefaultProgram marca el programa como el por defecto y desmarca al
	// resto, en una sola transacción.
	SetDefaultProgram(id uint) error
	ReplaceProgramBlocks(programID uint, blockIDs []uint) error
	ReplaceProgramCompanies(programID uint, companyIDs []uint) error
	ReplaceProgramUsers(programID uint, userIDs []uint) error
	// GetDefaultProgram devuelve el programa por defecto con sus bloques, o
	// nil sin error si no hay ninguno.
	GetDefaultProgram() (*models.InductionProgram, error)
	// GetProgramForCompany devuelve el programa asignado a la empresa con sus
	// bloques, o nil sin error si no tiene.
	GetProgramForCompany(companyID uint) (*models.InductionProgram, error)
	// GetProgramForUser devuelve el programa asignado al profesional uno a
	// uno, o nil sin error si no tiene.
	GetProgramForUser(userID uint) (*models.InductionProgram, error)
	// ListProgramRecipients devuelve los profesionales activos que reciben el
	// programa: los asignados en persona, los de sus empresas (salvo quien
	// tenga otro programa en persona) y, si es el por defecto, los de empresas
	// sin programa. Con su última invitación, para mostrar su estado.
	ListProgramRecipients(programID uint, isDefault bool) ([]ProgramRecipient, error)

	// --- Invitaciones ---
	CreateInvite(invite *models.InductionInvite, blocks []models.InductionInviteBlock) error
	UpdateInvite(invite *models.InductionInvite, updates map[string]interface{}) error
	GetInviteByToken(token string) (*models.InductionInvite, error)
	// GetInviteByUser devuelve la invitación ACTUAL: la pendiente si la hay y,
	// si no, la más reciente (aprobada o bloqueada).
	GetInviteByUser(userID uint) (*models.InductionInvite, error)
	// GetPendingInviteByUser devuelve la pendiente, o nil sin error.
	GetPendingInviteByUser(userID uint) (*models.InductionInvite, error)
	// ListInvitesByUser devuelve todas, de la más reciente a la más antigua.
	ListInvitesByUser(userID uint) ([]models.InductionInvite, error)
	GetInviteByID(id uint) (*models.InductionInvite, error)
	// DeleteInvite borra una invitación y sus bloques.
	DeleteInvite(inviteID uint) error
	// ListInviteBlocks devuelve los bloques de la invitación en orden.
	ListInviteBlocks(inviteID uint) ([]models.InductionInviteBlock, error)
	UpdateInviteBlock(inviteID, blockID uint, updates map[string]interface{}) error
	// ResetInviteBlocks devuelve todos los bloques de la invitación a
	// pendiente con cero intentos.
	ResetInviteBlocks(inviteID uint) error

	CreateAttempt(attempt *models.InductionAttempt) error
	ListAttempts(inviteID uint) ([]models.InductionAttempt, error)

	// GetSurveyWithQuestions carga el cuestionario con sus preguntas ordenadas.
	// Vive aquí (y no en el repo de encuestas) para que el servicio de inducción
	// no dependa del módulo completo de encuestas.
	GetSurveyWithQuestions(surveyID uint) (*models.Survey, error)
	GetTutorial(tutorialID uint) (*models.Tutorial, error)
}

// ProgramRecipient es un profesional que recibe un programa, con el estado de
// su inducción para decidir si tiene sentido enviársela.
type ProgramRecipient struct {
	UserID  uint   `json:"user_id"`
	Name    string `json:"name"`
	Email   string `json:"email"`
	Company string `json:"company"`
	// Source: "persona" (asignado uno a uno), "empresa" o "por_defecto".
	Source string `json:"source"`
	// Status de su última invitación: "" (nunca), pending, passed, blocked.
	Status      string `json:"status"`
	ProgramName string `json:"program_name"`
	// PassedThis: ya aprobó ESTE programa alguna vez.
	PassedThis bool `json:"passed_this"`
}

type inductionRepository struct {
	db *gorm.DB
}

func NewInductionRepository(db *gorm.DB) InductionRepository {
	return &inductionRepository{db: db}
}

// --- Configuración -----------------------------------------------------------

func (r *inductionRepository) GetConfig() (*models.InductionConfig, error) {
	var cfg models.InductionConfig
	if err := r.db.First(&cfg, 1).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Sin configuración = inducción apagada (no es un error).
			return &models.InductionConfig{ID: 1, InviteTTLDays: 30}, nil
		}
		return nil, err
	}
	return &cfg, nil
}

func (r *inductionRepository) SaveConfig(cfg *models.InductionConfig) error {
	cfg.ID = 1
	return r.db.Save(cfg).Error
}

// --- Bloques -----------------------------------------------------------------

func (r *inductionRepository) ListBlocks() ([]models.InductionBlock, error) {
	var blocks []models.InductionBlock
	if err := r.db.Order("name ASC, id ASC").Find(&blocks).Error; err != nil {
		return nil, err
	}
	if err := r.enrichBlocks(blocks); err != nil {
		return nil, err
	}
	return blocks, nil
}

func (r *inductionRepository) GetBlock(id uint) (*models.InductionBlock, error) {
	var block models.InductionBlock
	if err := r.db.First(&block, id).Error; err != nil {
		return nil, err
	}
	blocks := []models.InductionBlock{block}
	if err := r.enrichBlocks(blocks); err != nil {
		return nil, err
	}
	return &blocks[0], nil
}

func (r *inductionRepository) CreateBlock(block *models.InductionBlock) error {
	return r.db.Create(block).Error
}

func (r *inductionRepository) UpdateBlock(id uint, updates map[string]interface{}) error {
	return r.db.Model(&models.InductionBlock{}).Where("id = ?", id).Updates(updates).Error
}

func (r *inductionRepository) DeleteBlock(id uint) error {
	return r.db.Delete(&models.InductionBlock{}, id).Error
}

func (r *inductionRepository) CountProgramsUsingBlock(blockID uint) (int64, error) {
	var count int64
	err := r.db.Table("induction_program_blocks pb").
		Joins("JOIN induction_programs p ON p.id = pb.program_id AND p.deleted_at IS NULL").
		Where("pb.block_id = ?", blockID).
		Count(&count).Error
	return count, err
}

// enrichBlocks llena los campos de solo lectura del panel (título del video,
// título y cantidad de preguntas del cuestionario, programas que lo usan) con
// tres consultas por lote, en lugar de una por bloque.
func (r *inductionRepository) enrichBlocks(blocks []models.InductionBlock) error {
	if len(blocks) == 0 {
		return nil
	}
	blockIDs := make([]uint, 0, len(blocks))
	videoIDs := make([]uint, 0, len(blocks))
	surveyIDs := make([]uint, 0, len(blocks))
	for _, b := range blocks {
		blockIDs = append(blockIDs, b.ID)
		surveyIDs = append(surveyIDs, b.SurveyID)
		if b.VideoID != nil && *b.VideoID > 0 {
			videoIDs = append(videoIDs, *b.VideoID)
		}
	}

	videos := map[uint]models.InductionVideo{}
	if len(videoIDs) > 0 {
		var rows []models.InductionVideo
		if err := r.db.Where("id IN ?", videoIDs).Find(&rows).Error; err != nil {
			return err
		}
		for _, row := range rows {
			videos[row.ID] = row
		}
	}

	type surveyRow struct {
		ID            uint
		Title         string
		QuestionCount int
	}
	surveys := map[uint]surveyRow{}
	var surveyRows []surveyRow
	if err := r.db.Table("surveys s").
		Select("s.id, s.title, (SELECT COUNT(*) FROM survey_questions q WHERE q.survey_id = s.id) AS question_count").
		Where("s.id IN ? AND s.deleted_at IS NULL", surveyIDs).Scan(&surveyRows).Error; err != nil {
		return err
	}
	for _, row := range surveyRows {
		surveys[row.ID] = row
	}

	var usage []struct {
		BlockID uint
		Name    string
	}
	if err := r.db.Table("induction_program_blocks pb").
		Select("pb.block_id, p.name").
		Joins("JOIN induction_programs p ON p.id = pb.program_id AND p.deleted_at IS NULL").
		Where("pb.block_id IN ?", blockIDs).
		Order("p.name ASC").Scan(&usage).Error; err != nil {
		return err
	}
	programNames := map[uint][]string{}
	for _, row := range usage {
		programNames[row.BlockID] = append(programNames[row.BlockID], row.Name)
	}

	for i := range blocks {
		b := &blocks[i]
		if b.VideoID != nil {
			if v, ok := videos[*b.VideoID]; ok {
				b.VideoTitle = v.Title
				b.VideoURL = v.VideoURL
			}
		}
		if s, ok := surveys[b.SurveyID]; ok {
			b.SurveyTitle = s.Title
			b.QuestionCount = s.QuestionCount
		}
		b.ProgramNames = programNames[b.ID]
		if b.ProgramNames == nil {
			b.ProgramNames = []string{}
		}
	}
	return nil
}

// --- Programas ---------------------------------------------------------------

func (r *inductionRepository) ListPrograms() ([]models.InductionProgram, error) {
	var programs []models.InductionProgram
	if err := r.db.Order("is_default DESC, name ASC, id ASC").Find(&programs).Error; err != nil {
		return nil, err
	}
	// El listado trae el detalle completo (bloques enriquecidos y empresas):
	// el panel lo usa para decir qué le falta a cada programa sin abrirlo.
	// Los programas son pocos, así que el costo de cargarlos uno a uno es bajo.
	for i := range programs {
		if err := r.loadProgramDetail(&programs[i]); err != nil {
			return nil, err
		}
	}
	return programs, nil
}

func (r *inductionRepository) GetProgram(id uint) (*models.InductionProgram, error) {
	var program models.InductionProgram
	if err := r.db.First(&program, id).Error; err != nil {
		return nil, err
	}
	if err := r.loadProgramDetail(&program); err != nil {
		return nil, err
	}
	return &program, nil
}

// loadProgramDetail carga los bloques en orden y las empresas asignadas.
func (r *inductionRepository) loadProgramDetail(program *models.InductionProgram) error {
	var links []models.InductionProgramBlock
	if err := r.db.Where("program_id = ?", program.ID).
		Order("order_index ASC, block_id ASC").Find(&links).Error; err != nil {
		return err
	}
	program.Blocks = []models.InductionBlock{}
	if len(links) > 0 {
		ids := make([]uint, 0, len(links))
		orderOf := map[uint]int{}
		for _, link := range links {
			ids = append(ids, link.BlockID)
			orderOf[link.BlockID] = link.OrderIndex
		}
		var blocks []models.InductionBlock
		if err := r.db.Where("id IN ?", ids).Find(&blocks).Error; err != nil {
			return err
		}
		if err := r.enrichBlocks(blocks); err != nil {
			return err
		}
		for i := range blocks {
			blocks[i].OrderIndex = orderOf[blocks[i].ID]
		}
		// Un bloque borrado desaparece del programa sin romper el orden del
		// resto.
		sort.SliceStable(blocks, func(i, j int) bool {
			return blocks[i].OrderIndex < blocks[j].OrderIndex
		})
		program.Blocks = blocks
	}

	var companies []models.InductionProgramCompany
	if err := r.db.Where("program_id = ?", program.ID).Order("company_id ASC").Find(&companies).Error; err != nil {
		return err
	}
	program.CompanyIDs = make([]uint, 0, len(companies))
	for _, c := range companies {
		program.CompanyIDs = append(program.CompanyIDs, c.CompanyID)
	}
	var users []models.InductionProgramUser
	if err := r.db.Where("program_id = ?", program.ID).Order("user_id ASC").Find(&users).Error; err != nil {
		return err
	}
	program.UserIDs = make([]uint, 0, len(users))
	for _, u := range users {
		program.UserIDs = append(program.UserIDs, u.UserID)
	}
	program.BlockCount = len(program.Blocks)
	program.CompanyCount = len(program.CompanyIDs)
	program.UserCount = len(program.UserIDs)
	return nil
}

func (r *inductionRepository) CreateProgram(program *models.InductionProgram) error {
	return r.db.Create(program).Error
}

func (r *inductionRepository) UpdateProgram(id uint, updates map[string]interface{}) error {
	return r.db.Model(&models.InductionProgram{}).Where("id = ?", id).Updates(updates).Error
}

func (r *inductionRepository) DeleteProgram(id uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("program_id = ?", id).Delete(&models.InductionProgramBlock{}).Error; err != nil {
			return err
		}
		if err := tx.Where("program_id = ?", id).Delete(&models.InductionProgramCompany{}).Error; err != nil {
			return err
		}
		if err := tx.Where("program_id = ?", id).Delete(&models.InductionProgramUser{}).Error; err != nil {
			return err
		}
		return tx.Delete(&models.InductionProgram{}, id).Error
	})
}

func (r *inductionRepository) SetDefaultProgram(id uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// Primero se desmarca al resto: el índice parcial único no deja dos
		// marcados ni por un instante.
		if err := tx.Model(&models.InductionProgram{}).
			Where("is_default = ? AND id <> ?", true, id).
			Update("is_default", false).Error; err != nil {
			return err
		}
		return tx.Model(&models.InductionProgram{}).Where("id = ?", id).
			Update("is_default", true).Error
	})
}

func (r *inductionRepository) ReplaceProgramBlocks(programID uint, blockIDs []uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("program_id = ?", programID).Delete(&models.InductionProgramBlock{}).Error; err != nil {
			return err
		}
		for i, blockID := range blockIDs {
			link := models.InductionProgramBlock{ProgramID: programID, BlockID: blockID, OrderIndex: i}
			if err := tx.Create(&link).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *inductionRepository) ReplaceProgramCompanies(programID uint, companyIDs []uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("program_id = ?", programID).Delete(&models.InductionProgramCompany{}).Error; err != nil {
			return err
		}
		for _, companyID := range companyIDs {
			// Una empresa tiene un solo programa: asignarla aquí la quita del
			// que tuviera.
			if err := tx.Where("company_id = ?", companyID).Delete(&models.InductionProgramCompany{}).Error; err != nil {
				return err
			}
			link := models.InductionProgramCompany{CompanyID: companyID, ProgramID: programID}
			if err := tx.Create(&link).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *inductionRepository) ReplaceProgramUsers(programID uint, userIDs []uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("program_id = ?", programID).Delete(&models.InductionProgramUser{}).Error; err != nil {
			return err
		}
		for _, userID := range userIDs {
			// Un profesional tiene un solo programa: asignarlo aquí lo quita
			// del que tuviera.
			if err := tx.Where("user_id = ?", userID).Delete(&models.InductionProgramUser{}).Error; err != nil {
				return err
			}
			link := models.InductionProgramUser{UserID: userID, ProgramID: programID}
			if err := tx.Create(&link).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *inductionRepository) GetProgramForUser(userID uint) (*models.InductionProgram, error) {
	var link models.InductionProgramUser
	err := r.db.Where("user_id = ?", userID).First(&link).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	var program models.InductionProgram
	if err := r.db.First(&program, link.ProgramID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	if err := r.loadProgramDetail(&program); err != nil {
		return nil, err
	}
	return &program, nil
}

func (r *inductionRepository) ListProgramRecipients(programID uint, isDefault bool) ([]ProgramRecipient, error) {
	recipients := []ProgramRecipient{}
	err := r.db.Raw(`
		SELECT u.id AS user_id, u.name, u.email,
			COALESCE(NULLIF(c.company_name, ''), c.name, '') AS company,
			CASE WHEN au.program_id = ? THEN 'persona'
			     WHEN ac.program_id = ? THEN 'empresa'
			     ELSE 'por_defecto' END AS source
		FROM users u
		LEFT JOIN users c ON c.id = u.empleador_id AND c.deleted_at IS NULL
		LEFT JOIN induction_program_users au ON au.user_id = u.id
		LEFT JOIN induction_program_companies ac ON ac.company_id = u.empleador_id
		WHERE u.user_type = 'profesional' AND u.deleted_at IS NULL AND u.is_active
		  AND (
		    au.program_id = ?
		    OR (au.user_id IS NULL AND ac.program_id = ?)
		    OR (? AND au.user_id IS NULL AND ac.company_id IS NULL)
		  )
		ORDER BY u.name ASC, u.id ASC`, programID, programID, programID, programID, isDefault).
		Scan(&recipients).Error
	if err != nil || len(recipients) == 0 {
		return recipients, err
	}

	ids := make([]uint, 0, len(recipients))
	for _, rcp := range recipients {
		ids = append(ids, rcp.UserID)
	}
	var invites []models.InductionInvite
	if err := r.db.Where("user_id IN ?", ids).Order("created_at DESC, id DESC").Find(&invites).Error; err != nil {
		return nil, err
	}
	latest := map[uint]models.InductionInvite{}
	passed := map[uint]bool{}
	for _, inv := range invites {
		if _, ok := latest[inv.UserID]; !ok {
			latest[inv.UserID] = inv
		}
		if inv.Status == models.InductionPassed && inv.ProgramID != nil && *inv.ProgramID == programID {
			passed[inv.UserID] = true
		}
	}
	for i := range recipients {
		if inv, ok := latest[recipients[i].UserID]; ok {
			recipients[i].Status = inv.Status
			recipients[i].ProgramName = inv.ProgramName
		}
		recipients[i].PassedThis = passed[recipients[i].UserID]
	}
	return recipients, nil
}

func (r *inductionRepository) GetDefaultProgram() (*models.InductionProgram, error) {
	var program models.InductionProgram
	err := r.db.Where("is_default = ?", true).First(&program).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	if err := r.loadProgramDetail(&program); err != nil {
		return nil, err
	}
	return &program, nil
}

func (r *inductionRepository) GetProgramForCompany(companyID uint) (*models.InductionProgram, error) {
	var link models.InductionProgramCompany
	err := r.db.Where("company_id = ?", companyID).First(&link).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	var program models.InductionProgram
	if err := r.db.First(&program, link.ProgramID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	if err := r.loadProgramDetail(&program); err != nil {
		return nil, err
	}
	return &program, nil
}

// --- Invitaciones ------------------------------------------------------------

func (r *inductionRepository) CreateInvite(invite *models.InductionInvite, blocks []models.InductionInviteBlock) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(invite).Error; err != nil {
			return err
		}
		for i := range blocks {
			blocks[i].InviteID = invite.ID
			if err := tx.Create(&blocks[i]).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *inductionRepository) UpdateInvite(invite *models.InductionInvite, updates map[string]interface{}) error {
	return r.db.Model(invite).Updates(updates).Error
}

func (r *inductionRepository) GetInviteByToken(token string) (*models.InductionInvite, error) {
	var invite models.InductionInvite
	if err := r.db.Where("token = ?", token).First(&invite).Error; err != nil {
		return nil, err
	}
	return &invite, nil
}

func (r *inductionRepository) GetInviteByUser(userID uint) (*models.InductionInvite, error) {
	var invite models.InductionInvite
	err := r.db.Where("user_id = ?", userID).
		Order("CASE WHEN status = 'pending' THEN 0 ELSE 1 END, created_at DESC, id DESC").
		First(&invite).Error
	if err != nil {
		return nil, err
	}
	return &invite, nil
}

func (r *inductionRepository) GetPendingInviteByUser(userID uint) (*models.InductionInvite, error) {
	var invite models.InductionInvite
	err := r.db.Where("user_id = ? AND status = ?", userID, models.InductionPending).
		Order("created_at DESC").First(&invite).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &invite, nil
}

func (r *inductionRepository) ListInvitesByUser(userID uint) ([]models.InductionInvite, error) {
	var invites []models.InductionInvite
	err := r.db.Where("user_id = ?", userID).Order("created_at DESC, id DESC").Find(&invites).Error
	return invites, err
}

func (r *inductionRepository) GetInviteByID(id uint) (*models.InductionInvite, error) {
	var invite models.InductionInvite
	if err := r.db.First(&invite, id).Error; err != nil {
		return nil, err
	}
	return &invite, nil
}

func (r *inductionRepository) DeleteInvite(inviteID uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("invite_id = ?", inviteID).Delete(&models.InductionInviteBlock{}).Error; err != nil {
			return err
		}
		return tx.Unscoped().Delete(&models.InductionInvite{}, inviteID).Error
	})
}

func (r *inductionRepository) ListInviteBlocks(inviteID uint) ([]models.InductionInviteBlock, error) {
	var blocks []models.InductionInviteBlock
	err := r.db.Where("invite_id = ?", inviteID).
		Order("order_index ASC, block_id ASC").Find(&blocks).Error
	return blocks, err
}

func (r *inductionRepository) UpdateInviteBlock(inviteID, blockID uint, updates map[string]interface{}) error {
	return r.db.Model(&models.InductionInviteBlock{}).
		Where("invite_id = ? AND block_id = ?", inviteID, blockID).
		Updates(updates).Error
}

func (r *inductionRepository) ResetInviteBlocks(inviteID uint) error {
	return r.db.Model(&models.InductionInviteBlock{}).
		Where("invite_id = ?", inviteID).
		Updates(map[string]interface{}{
			"status":       models.InductionPending,
			"attempts":     0,
			"best_score":   0,
			"completed_at": nil,
		}).Error
}

// --- Intentos ----------------------------------------------------------------

func (r *inductionRepository) CreateAttempt(attempt *models.InductionAttempt) error {
	return r.db.Create(attempt).Error
}

func (r *inductionRepository) ListAttempts(inviteID uint) ([]models.InductionAttempt, error) {
	var attempts []models.InductionAttempt
	err := r.db.Where("invite_id = ?", inviteID).Order("created_at ASC").Find(&attempts).Error
	return attempts, err
}

// --- Contenido de otros módulos ----------------------------------------------

func (r *inductionRepository) GetSurveyWithQuestions(surveyID uint) (*models.Survey, error) {
	var survey models.Survey
	err := r.db.
		Preload("Questions", func(db *gorm.DB) *gorm.DB {
			return db.Order("order_index ASC, id ASC")
		}).
		First(&survey, surveyID).Error
	if err != nil {
		return nil, err
	}
	return &survey, nil
}

// --- Biblioteca de videos ----------------------------------------------------

func (r *inductionRepository) ListVideos() ([]models.InductionVideo, error) {
	var videos []models.InductionVideo
	if err := r.db.Order("title ASC, id ASC").Find(&videos).Error; err != nil {
		return nil, err
	}
	if len(videos) == 0 {
		return videos, nil
	}
	ids := make([]uint, 0, len(videos))
	for _, v := range videos {
		ids = append(ids, v.ID)
	}
	var usage []struct {
		VideoID uint
		Name    string
	}
	if err := r.db.Table("induction_blocks").Select("video_id, name").
		Where("video_id IN ? AND deleted_at IS NULL", ids).
		Order("name ASC").Scan(&usage).Error; err != nil {
		return nil, err
	}
	names := map[uint][]string{}
	for _, row := range usage {
		names[row.VideoID] = append(names[row.VideoID], row.Name)
	}
	for i := range videos {
		videos[i].BlockNames = names[videos[i].ID]
		if videos[i].BlockNames == nil {
			videos[i].BlockNames = []string{}
		}
	}
	return videos, nil
}

func (r *inductionRepository) GetVideo(id uint) (*models.InductionVideo, error) {
	var video models.InductionVideo
	if err := r.db.First(&video, id).Error; err != nil {
		return nil, err
	}
	video.BlockNames = []string{}
	return &video, nil
}

func (r *inductionRepository) CreateVideo(video *models.InductionVideo) error {
	return r.db.Create(video).Error
}

func (r *inductionRepository) UpdateVideo(id uint, updates map[string]interface{}) error {
	return r.db.Model(&models.InductionVideo{}).Where("id = ?", id).Updates(updates).Error
}

func (r *inductionRepository) DeleteVideo(id uint) error {
	return r.db.Delete(&models.InductionVideo{}, id).Error
}

func (r *inductionRepository) CountBlocksUsingVideo(videoID uint) (int64, error) {
	var count int64
	err := r.db.Model(&models.InductionBlock{}).Where("video_id = ?", videoID).Count(&count).Error
	return count, err
}

func (r *inductionRepository) GetTutorial(tutorialID uint) (*models.Tutorial, error) {
	var tutorial models.Tutorial
	if err := r.db.First(&tutorial, tutorialID).Error; err != nil {
		return nil, err
	}
	return &tutorial, nil
}
