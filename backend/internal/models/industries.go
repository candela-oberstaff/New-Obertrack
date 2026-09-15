package models

// Industries es la lista cerrada de rubros de una empresa. Tiene que decir lo
// MISMO que INDUSTRY_OPTIONS en frontend/src/components/Auth/industries.ts:
// es la que ofrece nuestro formulario, y la que se le manda a Obersuite para
// que su formulario ofrezca lo mismo. Hasta hoy el backend no la validaba —solo
// la pantalla la acotaba—, y con un segundo cliente escribiendo empresas eso ya
// no basta: el filtro por rubro del listado se llenaría de variantes. Lo
// protege una prueba que lee el archivo del frontend.
var Industries = []string{
	"Tecnología / Software",
	"Telecomunicaciones",
	"Salud / Medicina",
	"Educación",
	"Finanzas / Banca",
	"Seguros",
	"Comercio / Retail",
	"E-commerce",
	"Manufactura / Industria",
	"Construcción",
	"Inmobiliaria",
	"Turismo / Hostelería",
	"Gastronomía / Restaurantes",
	"Transporte / Logística",
	"Marketing / Publicidad",
	"Medios / Comunicación",
	"Arte / Diseño / Entretenimiento",
	"Consultoría / Servicios profesionales",
	"Legal / Jurídico",
	"Recursos Humanos",
	"Agricultura / Agroindustria",
	"Energía / Minería",
	"Automotriz",
	"Alimentos y Bebidas",
	"Organizaciones sin fines de lucro",
	"Gobierno / Sector público",
	"Otro",
}

// IsValidIndustry dice si el rubro está en la lista. Vacío vale: el rubro es
// opcional.
func IsValidIndustry(v string) bool {
	if v == "" {
		return true
	}
	for _, i := range Industries {
		if i == v {
			return true
		}
	}
	return false
}
