export const ABSENCE_REASONS: { value: string; label: string }[] = [
  { value: 'enfermedad', label: 'Enfermedad' },
  { value: 'cita_medica', label: 'Cita Médica' },
  { value: 'emergencia_familiar', label: 'Emergencia Familiar' },
  { value: 'vacaciones', label: 'Vacaciones' },
  { value: 'permiso_personal', label: 'Permiso Personal' },
  { value: 'corte_internet', label: 'Corte de Internet' },
  { value: 'corte_electricidad', label: 'Corte de Electricidad' },
  { value: 'corte_servicios', label: 'Caída de Servicios (otros)' },
  { value: 'otro', label: 'Otro' },
]

export const REASONS_REQUIRING_DETAIL = ['otro', 'corte_servicios']

export function absenceReasonLabel(value: string | null | undefined): string {
  if (!value) return ''
  const found = ABSENCE_REASONS.find((r) => r.value === value)
  if (found) return found.label
  const s = value.replace(/_/g, ' ').trim()
  return s ? s.charAt(0).toUpperCase() + s.slice(1) : s
}
