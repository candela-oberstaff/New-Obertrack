package service

import (
	"testing"

	"github.com/obertrack/backend/internal/models"
)

// El expediente de un empleo lo abren TRES públicos distintos por la misma
// pantalla, y no deben ver lo mismo:
//
//   - Obertrack (superadmin / CS) ve todo, incluido lo que nace de módulos
//     nuestros, como un testimonio sobre Oberstaff.
//   - La empresa cliente ve la relación laboral, pero NO ese material nuestro:
//     no le corresponde leer lo que su empleado escribió sobre nosotros.
//   - El profesional solo ve lo que se le compartió expresamente.
//
// Esto se comprobó contra la base real y la empresa cliente SÍ lo veía, porque
// "private" en este código significa "lo ve RR.HH. del cliente", no "solo
// Obertrack". La prueba fija el reparto para que no se vuelva a colar.
func TestExpediente_LaEmpresaClienteNoVeLosTestimonios(t *testing.T) {
	casos := []struct {
		audiencia string
		kind      string
		visible   string
		quiero    bool
	}{
		// El testimonio: solo para nosotros.
		{AudiencePlatform, models.NoteKindTestimonial, models.ExpedientePrivate, true},
		{AudienceCompany, models.NoteKindTestimonial, models.ExpedientePrivate, false},
		{AudienceProfessional, models.NoteKindTestimonial, models.ExpedientePrivate, false},
		// Marcarlo como compartido tampoco se lo enseña al cliente: lo que lo
		// aparta no es la visibilidad, es de qué módulo viene.
		{AudienceCompany, models.NoteKindTestimonial, models.ExpedienteShared, false},

		// Las notas de siempre no cambian de comportamiento.
		{AudiencePlatform, models.NoteKindNote, models.ExpedientePrivate, true},
		{AudienceCompany, models.NoteKindNote, models.ExpedientePrivate, true},
		{AudienceProfessional, models.NoteKindNote, models.ExpedientePrivate, false},
		{AudienceProfessional, models.NoteKindNote, models.ExpedienteShared, true},
		{AudienceCompany, models.NoteKindEvaluation, models.ExpedientePrivate, true},
		{AudienceProfessional, models.NoteKindEvaluation, models.ExpedienteShared, true},
	}

	for _, c := range casos {
		got := !hidesNote(c.audiencia, c.kind, c.visible)
		if got != c.quiero {
			t.Errorf("audiencia=%q kind=%q visibilidad=%q: la ve=%v, se esperaba %v",
				c.audiencia, c.kind, c.visible, got, c.quiero)
		}
	}
}
