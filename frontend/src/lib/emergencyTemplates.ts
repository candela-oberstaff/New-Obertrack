import type { EmergencyTemplate } from '../services/admin.service'

export interface TemplateOption {
  value: string
  label: string
  subject: string
  body: string
}

export const BUILTIN_EMERGENCY_TEMPLATES: { label: string; subject: string; body: string }[] = [
  {
    label: 'Confirmá que estás bien',
    subject: 'Confirmación de seguridad',
    body: 'Hola, estamos verificando el estado de nuestro equipo en tu zona. Por favor confirmá que estás bien respondiendo a este correo. Gracias.',
  },
  {
    label: 'Sismo en tu zona',
    subject: 'Alerta: sismo reportado en tu zona',
    body: 'Hemos detectado actividad sísmica cerca de tu ubicación registrada. Por favor confirmá que vos y tu entorno se encuentran a salvo.',
  },
  {
    label: 'Reporte de situación',
    subject: 'Solicitud de reporte de situación',
    body: 'Necesitamos un breve reporte de tu situación actual ante el evento en curso. Indicanos tu estado y si requerís asistencia.',
  },
  {
    label: 'Instrucciones de emergencia',
    subject: 'Instrucciones ante la emergencia',
    body: 'Ante la situación de emergencia, te pedimos seguir los protocolos de seguridad y mantenerte en contacto. Respondé este correo para confirmar recepción.',
  },
]

export function buildTemplateOptions(dbTemplates: EmergencyTemplate[]): TemplateOption[] {
  const builtins: TemplateOption[] = BUILTIN_EMERGENCY_TEMPLATES.map((t, i) => ({
    value: `builtin:${i}`,
    label: t.label,
    subject: t.subject,
    body: t.body,
  }))
  const db: TemplateOption[] = dbTemplates.map((t) => ({
    value: `db:${t.id}`,
    label: t.title,
    subject: t.subject,
    body: t.body,
  }))
  return [...builtins, ...db]
}
