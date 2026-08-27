package repository

// SurveyRepository manages surveys, questions and responses.
// NOTE: No tenant_id filtering — CRUD endpoints are behind RequireSuperadmin()
// middleware. The QuickResponse endpoint (public) is validated separately in the handler.

import (
	"fmt"

	"github.com/obertrack/backend/internal/models"
	"gorm.io/gorm"
)

type SurveyRepository interface {
	CreateSurvey(survey *models.Survey) error
	GetSurveys() ([]models.Survey, error)
	GetSurveyByID(id uint) (*models.Survey, error)
	UpdateSurvey(survey *models.Survey) error
	DeleteSurvey(id uint) error

	CreateResponse(response *models.SurveyResponse) error
	GetSurveyResponses(surveyID uint) ([]models.SurveyResponse, error)
}

type surveyRepository struct {
	db *gorm.DB
}

func NewSurveyRepository(db *gorm.DB) SurveyRepository {
	return &surveyRepository{db: db}
}

func (r *surveyRepository) CreateSurvey(survey *models.Survey) error {
	return r.db.Create(survey).Error
}

func (r *surveyRepository) GetSurveys() ([]models.Survey, error) {
	var surveys []models.Survey
	err := r.db.Preload("Questions").Preload("Responses").Find(&surveys).Error
	return surveys, err
}

func (r *surveyRepository) GetSurveyByID(id uint) (*models.Survey, error) {
	var survey models.Survey
	err := r.db.Preload("Questions").Preload("Responses.User").Preload("Responses.Answers").First(&survey, id).Error
	return &survey, err
}

// ErrPreguntaRespondida es quitar del formulario una pregunta que alguien ya
// contestó. No es un fallo técnico: es una decisión que le corresponde a quien
// edita, porque borrarla se llevaría por delante las respuestas dadas.
type ErrPreguntaRespondida struct {
	Preguntas  int
	Respuestas int64
	Enunciado  string
}

func (e ErrPreguntaRespondida) Error() string {
	if e.Preguntas == 1 {
		return fmt.Sprintf(
			"la pregunta %q ya la contestaron %d persona(s): si la quitas se borrarían sus respuestas. "+
				"Cámbiale el texto si te has equivocado, o deja la pregunta y crea una nueva.",
			e.Enunciado, e.Respuestas)
	}
	return fmt.Sprintf(
		"quitaste %d preguntas que ya tienen %d respuesta(s) entre todas: borrarlas se llevaría esas respuestas. "+
			"Cámbiales el texto si te has equivocado, o déjalas y crea preguntas nuevas.",
		e.Preguntas, e.Respuestas)
}

// UpdateSurvey guarda la encuesta CONSERVANDO la identidad de sus preguntas.
//
// Antes borraba todas las preguntas y las recreaba, y eso hacía dos daños. Las
// respuestas ya dadas apuntan a la pregunta por id, así que recrearlas las dejaba
// apuntando al vacío y la pantalla de resultados salía en blanco. Y desde que
// existe la clave foránea fk_survey_answers_question el borrado falla en seco: una
// encuesta contestada no se podía volver a editar nunca más, ni para corregir una
// falta de ortografía.
func (r *surveyRepository) UpdateSurvey(survey *models.Survey) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var actual models.Survey
		if err := tx.First(&actual, survey.ID).Error; err != nil {
			return err
		}

		var previas []models.SurveyQuestion
		if err := tx.Where("survey_id = ?", survey.ID).Find(&previas).Error; err != nil {
			return err
		}

		siguen := make(map[uint]bool, len(survey.Questions))
		for i := range survey.Questions {
			survey.Questions[i].SurveyID = survey.ID
			if id := survey.Questions[i].ID; id != 0 {
				siguen[id] = true
			}
		}

		// Lo que se quitó del formulario. Sólo se borra lo que nadie contestó; si
		// alguien la respondió, se para y se explica, porque perder respuestas de
		// verdad no es algo que deba decidir el guardado por su cuenta.
		var retiradas []models.SurveyQuestion
		for _, p := range previas {
			if !siguen[p.ID] {
				retiradas = append(retiradas, p)
			}
		}
		if len(retiradas) > 0 {
			ids := make([]uint, 0, len(retiradas))
			for _, p := range retiradas {
				ids = append(ids, p.ID)
			}
			// Se cuenta por pregunta y no en bloque: el mensaje tiene que nombrar la
			// que estorba. "Quitaste 3 preguntas con respuestas" cuando sólo una las
			// tiene manda a buscar a ciegas por todo el formulario.
			type conteo struct {
				QuestionID uint
				Total      int64
			}
			var conRespuesta []conteo
			if err := tx.Model(&models.SurveyAnswer{}).
				Select("question_id, COUNT(*) AS total").
				Where("question_id IN ?", ids).
				Group("question_id").Scan(&conRespuesta).Error; err != nil {
				return err
			}
			if len(conRespuesta) > 0 {
				textos := map[uint]string{}
				for _, p := range retiradas {
					textos[p.ID] = p.Text
				}
				var total int64
				for _, c := range conRespuesta {
					total += c.Total
				}
				return ErrPreguntaRespondida{
					Preguntas:  len(conRespuesta),
					Respuestas: total,
					Enunciado:  textos[conRespuesta[0].QuestionID],
				}
			}
			if err := tx.Where("id IN ?", ids).Delete(&models.SurveyQuestion{}).Error; err != nil {
				return err
			}
		}

		// La encuesta, campo por campo y nombrándolos POR EL MODELO, no por el nombre
		// de la columna: SendByInApp se guarda en send_by_in_app, y escribir la
		// columna a mano es equivocarse.
		//
		// Se enumeran en vez de hacer un Save a secas porque el editor de Encuestas
		// manda un cuerpo parcial: kind, passing_score y created_by llegan en cero y
		// un Save los pisaría. Abrir un cuestionario de inducción desde Encuestas y
		// pulsar Guardar bastaba para dejarlo sin tipo y con nota de corte 0.
		//
		// Select es lo que hace que un false se guarde: sin él, Updates con un struct
		// ignora los valores cero y "desmarcar el envío por correo" no se guardaría.
		// Una encuesta ya enviada no vuelve a borrador por guardarla. La pantalla
		// manda status "draft" en cada guardado, y eso bastaba para que corregir una
		// falta de ortografía la desactivara: quien todavía no había respondido se
		// encontraba con "esta encuesta ya no está activa" y se perdían respuestas.
		estado := survey.Status
		if actual.Status != models.SurveyStatusDraft && estado == models.SurveyStatusDraft {
			estado = actual.Status
		}

		cambios := models.Survey{
			Title:         survey.Title,
			Description:   survey.Description,
			Status:        estado,
			SendByEmail:   survey.SendByEmail,
			SendByInApp:   survey.SendByInApp,
			RecipientList: survey.RecipientList,
		}
		campos := []string{"Title", "Description", "Status", "SendByEmail", "SendByInApp", "RecipientList"}
		if survey.Kind != "" {
			cambios.Kind = survey.Kind
			campos = append(campos, "Kind")
		}
		if survey.PassingScore > 0 {
			cambios.PassingScore = survey.PassingScore
			campos = append(campos, "PassingScore")
		}
		if survey.CreatedBy > 0 {
			cambios.CreatedBy = survey.CreatedBy
			campos = append(campos, "CreatedBy")
		}
		if err := tx.Model(&models.Survey{}).Where("id = ?", survey.ID).
			Select(campos).Updates(cambios).Error; err != nil {
			return err
		}

		// Las que ya existían se actualizan en su sitio; las nuevas se insertan.
		for i := range survey.Questions {
			q := &survey.Questions[i]
			if q.ID == 0 {
				if err := tx.Create(q).Error; err != nil {
					return err
				}
				continue
			}
			if err := tx.Model(&models.SurveyQuestion{}).Where("id = ?", q.ID).
				Select("Text", "Type", "Options", "IsRequired", "OrderIndex", "CorrectAnswer", "Weight").
				Updates(*q).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *surveyRepository) DeleteSurvey(id uint) error {
	return r.db.Delete(&models.Survey{}, id).Error
}

func (r *surveyRepository) CreateResponse(response *models.SurveyResponse) error {
	return r.db.Create(response).Error
}

func (r *surveyRepository) GetSurveyResponses(surveyID uint) ([]models.SurveyResponse, error) {
	var responses []models.SurveyResponse
	err := r.db.Preload("Answers").Preload("User").Where("survey_id = ?", surveyID).Find(&responses).Error
	return responses, err
}
