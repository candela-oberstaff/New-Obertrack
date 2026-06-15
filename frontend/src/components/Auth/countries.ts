import { Country, State } from 'country-state-city'

// Lista completa de países (en español) usada por TODOS los roles que piden
// país (profesional, empresa, customer success, analista de IT). El superadmin
// no registra país. Antes la empresa usaba una lista reducida — se unificó.
// LATAM va primero para facilitar la selección, luego el resto en orden alfabético.
const LATAM_COUNTRIES = [
  'Argentina',
  'Bolivia',
  'Brasil',
  'Chile',
  'Colombia',
  'Costa Rica',
  'Cuba',
  'Ecuador',
  'El Salvador',
  'Guatemala',
  'Honduras',
  'México',
  'Nicaragua',
  'Panamá',
  'Paraguay',
  'Perú',
  'Puerto Rico',
  'República Dominicana',
  'Uruguay',
  'Venezuela',
]

const OTHER_COUNTRIES = [
  'Afganistán',
  'Albania',
  'Alemania',
  'Andorra',
  'Angola',
  'Antigua y Barbuda',
  'Arabia Saudita',
  'Argelia',
  'Armenia',
  'Australia',
  'Austria',
  'Azerbaiyán',
  'Bahamas',
  'Bangladés',
  'Barbados',
  'Baréin',
  'Bélgica',
  'Belice',
  'Benín',
  'Bielorrusia',
  'Birmania (Myanmar)',
  'Bosnia y Herzegovina',
  'Botsuana',
  'Brunéi',
  'Bulgaria',
  'Burkina Faso',
  'Burundi',
  'Bután',
  'Cabo Verde',
  'Camboya',
  'Camerún',
  'Canadá',
  'Catar',
  'Chad',
  'China',
  'Chipre',
  'Comoras',
  'Corea del Norte',
  'Corea del Sur',
  'Costa de Marfil',
  'Croacia',
  'Dinamarca',
  'Dominica',
  'Egipto',
  'Emiratos Árabes Unidos',
  'Eritrea',
  'Eslovaquia',
  'Eslovenia',
  'España',
  'Estados Unidos',
  'Estonia',
  'Esuatini',
  'Etiopía',
  'Filipinas',
  'Finlandia',
  'Fiyi',
  'Francia',
  'Gabón',
  'Gambia',
  'Georgia',
  'Ghana',
  'Granada',
  'Grecia',
  'Guinea',
  'Guinea Ecuatorial',
  'Guinea-Bisáu',
  'Guyana',
  'Haití',
  'Hungría',
  'India',
  'Indonesia',
  'Irak',
  'Irán',
  'Irlanda',
  'Islandia',
  'Islas Marshall',
  'Islas Salomón',
  'Israel',
  'Italia',
  'Jamaica',
  'Japón',
  'Jordania',
  'Kazajistán',
  'Kenia',
  'Kirguistán',
  'Kiribati',
  'Kuwait',
  'Laos',
  'Lesoto',
  'Letonia',
  'Líbano',
  'Liberia',
  'Libia',
  'Liechtenstein',
  'Lituania',
  'Luxemburgo',
  'Macedonia del Norte',
  'Madagascar',
  'Malasia',
  'Malaui',
  'Maldivas',
  'Malí',
  'Malta',
  'Marruecos',
  'Mauricio',
  'Mauritania',
  'Micronesia',
  'Moldavia',
  'Mónaco',
  'Mongolia',
  'Montenegro',
  'Mozambique',
  'Namibia',
  'Nauru',
  'Nepal',
  'Níger',
  'Nigeria',
  'Noruega',
  'Nueva Zelanda',
  'Omán',
  'Países Bajos',
  'Pakistán',
  'Palaos',
  'Palestina',
  'Papúa Nueva Guinea',
  'Polonia',
  'Portugal',
  'Reino Unido',
  'República Centroafricana',
  'República Checa',
  'República del Congo',
  'República Democrática del Congo',
  'Ruanda',
  'Rumanía',
  'Rusia',
  'Samoa',
  'San Cristóbal y Nieves',
  'San Marino',
  'San Vicente y las Granadinas',
  'Santa Lucía',
  'Santo Tomé y Príncipe',
  'Senegal',
  'Serbia',
  'Seychelles',
  'Sierra Leona',
  'Singapur',
  'Siria',
  'Somalia',
  'Sri Lanka',
  'Sudáfrica',
  'Sudán',
  'Sudán del Sur',
  'Suecia',
  'Suiza',
  'Surinam',
  'Tailandia',
  'Tanzania',
  'Tayikistán',
  'Timor Oriental',
  'Togo',
  'Tonga',
  'Trinidad y Tobago',
  'Túnez',
  'Turkmenistán',
  'Turquía',
  'Tuvalu',
  'Ucrania',
  'Uganda',
  'Uzbekistán',
  'Vanuatu',
  'Vaticano',
  'Vietnam',
  'Yemen',
  'Yibuti',
  'Zambia',
  'Zimbabue',
].sort((a, b) => a.localeCompare(b, 'es'))

export const ALL_COUNTRY_OPTIONS = Array.from(
  new Set([...LATAM_COUNTRIES, ...OTHER_COUNTRIES])
)
  .sort((a, b) => a.localeCompare(b, 'es'))
  .map((name) => ({ value: name, label: name }))

// COUNTRY_OPTIONS es alias de la lista completa: empresa, perfil y edición de
// empresa importan este nombre y ahora reciben el mismo catálogo extenso.
export const COUNTRY_OPTIONS = ALL_COUNTRY_OPTIONS

// --- Estados / provincias por país (offline, sin API) -------------------

// Mapea el nombre de país (en español, como lo guardamos) a su código ISO2,
// usando Intl.DisplayNames sobre el catálogo de la librería. Así no hay que
// mantener a mano una tabla de ~250 códigos.
const NAME_TO_ISO: Record<string, string> = (() => {
  const map: Record<string, string> = {}
  try {
    const dn = new Intl.DisplayNames(['es'], { type: 'region' })
    for (const c of Country.getAllCountries()) {
      const es = dn.of(c.isoCode)
      if (es) map[es.toLowerCase()] = c.isoCode
    }
  } catch {
    // Intl.DisplayNames no disponible: getStatesForCountry devolverá [].
  }
  // Overrides para nuestras etiquetas que no coinciden con Intl.
  map['usa'] = 'US'
  map['birmania (myanmar)'] = 'MM'
  return map
})()

// Nombres en español donde la librería los devuelve en inglés.
const STATE_OVERRIDES: Record<string, string[]> = {
  ES: [
    'Andalucía', 'Aragón', 'Asturias', 'Islas Baleares', 'Canarias',
    'Cantabria', 'Castilla-La Mancha', 'Castilla y León', 'Cataluña',
    'Comunidad Valenciana', 'Extremadura', 'Galicia', 'La Rioja',
    'Comunidad de Madrid', 'Región de Murcia', 'Navarra', 'País Vasco',
    'Ceuta', 'Melilla',
  ],
}

export interface SelectOptionLike {
  value: string
  label: string
}

// Devuelve los estados/provincias del país dado (por su nombre en español).
// Si el país no tiene datos, devuelve [] y el formulario oculta el campo.
export function getStatesForCountry(countryName: string): SelectOptionLike[] {
  if (!countryName) return []
  const iso = NAME_TO_ISO[countryName.trim().toLowerCase()]
  if (!iso) return []
  const names = STATE_OVERRIDES[iso] ?? State.getStatesOfCountry(iso).map((s) => s.name)
  return names
    .slice()
    .sort((a, b) => a.localeCompare(b, 'es'))
    .map((name) => ({ value: name, label: name }))
}
