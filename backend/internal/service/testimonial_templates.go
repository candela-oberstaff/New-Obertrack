package service

import "github.com/obertrack/backend/internal/models"

// ConsentVersion es la versión de la redacción del consentimiento. Cambiarla
// cuando se toque cualquiera de los textos de abajo: cada testimonio guarda la
// versión con la que se firmó, así que los ya firmados siguen respaldados por
// el texto que su autor leyó de verdad.
const TestimonialConsentVersion = "v1"

// TestimonialTemplate es el molde de una solicitud: qué se le pregunta a quien
// escribe y qué autoriza al firmar.
//
// Vive en código y no en base de datos a propósito. El texto de consentimiento
// es la pieza legal del módulo: que se edite desde un panel invita a cambiarlo
// sin advertir que los testimonios viejos quedan firmados sobre otra redacción.
// Lo que sí se personaliza por solicitud es la nota de presentación
// (IntroMessage), que no compromete nada.
type TestimonialTemplate struct {
	Audience string `json:"audience"`
	Label    string `json:"label"`
	// Headline es el titular de la página pública.
	Headline string `json:"headline"`
	// Intro es la nota de presentación sugerida, editable al pedir.
	Intro string `json:"intro"`
	// Prompts son las preguntas guía que se muestran junto al campo de texto.
	Prompts []string `json:"prompts"`
	// ConsentText es la autorización que se firma.
	ConsentText string `json:"consent_text"`
}

// testimonialTemplates son las dos audiencias del módulo.
var testimonialTemplates = map[string]TestimonialTemplate{
	models.TestimonialFromProfessional: {
		Audience: models.TestimonialFromProfessional,
		Label:    "Profesional",
		Headline: "Cuéntanos cómo ha sido tu experiencia",
		Intro: "Nos encantaría conocer tu experiencia trabajando con nosotros. " +
			"Te tomará menos de cinco minutos y nos ayuda muchísimo.",
		Prompts: []string{
			"¿Cómo llegaste a trabajar con Oberstaff y qué te decidió?",
			"¿Qué es lo que más valoras del día a día?",
			"¿Qué le dirías a alguien que está pensando en dar el paso?",
		},
		ConsentText: "Autorizo a Oberstaff a publicar el testimonio que escribí, junto con " +
			"los datos que marqué a continuación, en sus canales de comunicación " +
			"(sitio web, redes sociales, presentaciones y materiales comerciales). " +
			"Declaro que el testimonio es mío, que lo escribí libremente y que es " +
			"veraz. Entiendo que puedo pedir que se retire en cualquier momento " +
			"escribiendo a Oberstaff, y que no recibo pago alguno por esta " +
			"autorización.",
	},
	models.TestimonialFromCompany: {
		Audience: models.TestimonialFromCompany,
		Label:    "Empresa",
		Headline: "Cuéntanos cómo te ha ido con Oberstaff",
		Intro: "Nos gustaría contar con tu opinión sobre el servicio. " +
			"Son unas pocas líneas y significan mucho para nosotros.",
		Prompts: []string{
			"¿Qué necesidad tenían antes de trabajar con Oberstaff?",
			"¿Qué resultados han visto desde entonces?",
			"¿Recomendarías el servicio a otra empresa? ¿Por qué?",
		},
		ConsentText: "En representación de la empresa, autorizo a Oberstaff a publicar el " +
			"testimonio que escribí, junto con los datos que marqué a continuación, " +
			"en sus canales de comunicación (sitio web, redes sociales, " +
			"presentaciones y materiales comerciales). Declaro que cuento con " +
			"facultades para otorgar esta autorización en nombre de la empresa y " +
			"que el testimonio es veraz. Entiendo que puedo pedir que se retire en " +
			"cualquier momento escribiendo a Oberstaff.",
	},
}

// TestimonialTemplates devuelve las plantillas disponibles, en orden estable.
func TestimonialTemplates() []TestimonialTemplate {
	return []TestimonialTemplate{
		testimonialTemplates[models.TestimonialFromProfessional],
		testimonialTemplates[models.TestimonialFromCompany],
	}
}

// testimonialTemplateFor resuelve la plantilla de una audiencia. El segundo
// valor es false si la audiencia no existe.
func testimonialTemplateFor(audience string) (TestimonialTemplate, bool) {
	tpl, ok := testimonialTemplates[audience]
	return tpl, ok
}
