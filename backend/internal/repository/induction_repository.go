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
	// GetDefaultProgram devuelve el programa por defecto con sus bloques, o
	// nil sin error si no hay ninguno.
	GetDefaultProgram() (*models.InductionProgram, error)
	// GetProgramForCompany devuelve el programa asignado a la empresa con sus
	// bloques, o nil sin error si no tiene.
	GetProgramForCompany(companyID uint) (*models.InductionProgram, error)

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
	tutorialIDs := make([]uint, 0, len(blocks))
	surveyIDs := make([]uint, 0, len(blocks))
	for _, b := range blocks {
		blockIDs = append(blockIDs, b.ID)
		surveyIDs = append(surveyIDs, b.SurveyID)
		if b.TutorialID != nil && *b.TutorialID > 0 {
			tutorialIDs = append(tutorialIDs, *b.TutorialID)
		}
	}

	tutorialTitles := map[uint]string{}
	if len(tutorialIDs) > 0 {
		var rows []struct {
			ID    uint
			Title string
		}
		if err := r.db.Table("tutorials").Select("id, title").
			Where("id IN ? AND deleted_at IS NULL", tutorialIDs).Scan(&rows).Error; err != nil {
			return err
		}
		for _, row := range rows {
			tutorialTitles[row.ID] = row.Title
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
		if b.TutorialID != nil {
			b.TutorialTitle = tutorialTitles[*b.TutorialID]
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
	if len(programs) == 0 {
		return programs, nil
	}
	ids := make([]uint, 0, len(programs))
	for _, p := range programs {
		ids = append(ids, p.ID)
	}
	var blockCounts []struct {
		ProgramID uint
		Count     int
	}
	if err := r.db.Table("induction_program_blocks pb").
		Select("pb.program_id, COUNT(*) AS count").
		Joins("JOIN induction_blocks b ON b.id = pb.block_id AND b.deleted_at IS NULL").
		Where("pb.program_id IN ?", ids).Group("pb.program_id").
		Scan(&blockCounts).Error; err != nil {
		return nil, err
	}
	var companyCounts []struct {
		ProgramID uint
		Count     int
	}
	if err := r.db.Table("induction_program_companies").
		Select("program_id, COUNT(*) AS count").
		Where("program_id IN ?", ids).Group("program_id").
		Scan(&companyCounts).Error; err != nil {
		return nil, err
	}
	blocksBy := map[uint]int{}
	for _, row := range blockCounts {
		blocksBy[row.ProgramID] = row.Count
	}
	companiesBy := map[uint]int{}
	for _, row := range companyCounts {
		companiesBy[row.ProgramID] = row.Count
	}
	for i := range programs {
		programs[i].BlockCount = blocksBy[programs[i].ID]
		programs[i].CompanyCount = companiesBy[programs[i].ID]
		programs[i].Blocks = []models.InductionBlock{}
		programs[i].CompanyIDs = []uint{}
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
	program.BlockCount = len(program.Blocks)
	program.CompanyCount = len(program.CompanyIDs)
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

func (r *inductionRepository) GetTutorial(tutorialID uint) (*models.Tutorial, error) {
	var tutorial models.Tutorial
	if err := r.db.First(&tutorial, tutorialID).Error; err != nil {
		return nil, err
	}
	return &tutorial, nil
}
