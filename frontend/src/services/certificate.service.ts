import api from './client'

export type CertificateFieldKey = 'name' | 'program' | 'date' | 'code' | 'text'

/** Un texto colocado sobre el diseño. X e Y en porcentaje de la página. */
export interface CertificateField {
  key: CertificateFieldKey
  text?: string
  x: number
  y: number
  /** Puntos. */
  size: number
  /** #rrggbb */
  color: string
  align: 'L' | 'C' | 'R'
  bold: boolean
  font: 'Helvetica' | 'Times' | 'Courier'
}

/** Diseño subido por el equipo más la posición de los campos. */
export interface CertificateTemplate {
  id: number
  name: string
  image_filename: string
  /** L = horizontal, P = vertical. Se deduce del tamaño de la imagen. */
  orientation: 'L' | 'P'
  fields: CertificateField[]
  created_by: number
  created_at: string
  updated_at: string
  program_names: string[]
}

export interface CertificateTemplateInput {
  name: string
  image_filename: string
  fields: CertificateField[]
}

/** Certificado emitido: PDF inmutable con código de verificación. */
export interface Certificate {
  id: number
  user_id: number
  invite_id: number
  program_id: number
  program_name: string
  template_id: number
  template_name: string
  code: string
  issued_at: string
  reissued_at?: string
}

/** Resumen que viaja con la landing y el historial. */
export interface CertificateSummary {
  id: number
  code: string
  program_name: string
  issued_at: string
  download_url: string
}

/** Emitido en un programa, con el nombre de la persona. */
export interface IssuedCertificate {
  id: number
  user_id: number
  user_name: string
  code: string
  program_name: string
  issued_at: string
  reissued_at?: string
}

export interface ProgramCertificates {
  total: number
  recent: IssuedCertificate[]
}

export interface CertificateVerification {
  valid: boolean
  code: string
  professional_name: string
  program_name: string
  issued_at: string
  download_url: string
}

/** Ruta pública del diseño, para pintarlo en el editor. */
export function templateImageUrl(filename: string) {
  return `/api/public/uploads/${filename}`
}

/** Descarga pública por código: es lo que imprime el certificado. */
export function certificateDownloadUrl(code: string) {
  return `/api/public/certificates/${encodeURIComponent(code)}/pdf`
}

export const certificateService = {
  // --- Plantillas (panel) ---
  listTemplates: async () => {
    const { data } = await api.get<{ data: CertificateTemplate[] }>('/inductions/certificate-templates')
    return data.data
  },

  createTemplate: async (input: CertificateTemplateInput) => {
    const { data } = await api.post<CertificateTemplate>('/inductions/certificate-templates', input)
    return data
  },

  updateTemplate: async (id: number, input: CertificateTemplateInput) => {
    const { data } = await api.put<CertificateTemplate>(`/inductions/certificate-templates/${id}`, input)
    return data
  },

  deleteTemplate: async (id: number) => {
    const { data } = await api.delete<{ message: string }>(`/inductions/certificate-templates/${id}`)
    return data
  },

  /** PDF de vista previa con datos de ejemplo (plantilla guardada o no). */
  previewTemplate: async (input: CertificateTemplateInput) => {
    const res = await api.post('/inductions/certificate-templates/preview', input, { responseType: 'blob' })
    return res.data as Blob
  },

  // --- Certificados ---
  mine: async () => {
    const { data } = await api.get<{ data: Certificate[] }>('/me/certificates')
    return data.data
  },

  forUser: async (userId: number) => {
    const { data } = await api.get<{ data: Certificate[] }>(`/users/${userId}/certificates`)
    return data.data
  },

  /** Total emitido en un programa y los más recientes. */
  forProgram: async (programId: number) => {
    const { data } = await api.get<ProgramCertificates>(`/inductions/programs/${programId}/certificates`)
    return data
  },

  /** Emite (Soporte) el certificado de una capacitación aprobada sin él. */
  issueForInvite: async (inviteId: number) => {
    const { data } = await api.post<Certificate>(`/inductions/invites/${inviteId}/certificate`, {})
    return data
  },

  /** Vuelve a generar el PDF con la plantilla actual, mismo código. */
  reissue: async (certificateId: number) => {
    const { data } = await api.post<Certificate>(`/inductions/certificates/${certificateId}/reissue`, {})
    return data
  },

  // --- Público ---
  verify: async (code: string) => {
    const { data } = await api.get<CertificateVerification>(`/public/certificates/${encodeURIComponent(code)}`)
    return data
  },
}
