import { useQuery, useMutation, useQueryClient, keepPreviousData } from '@tanstack/react-query'
import { adminService } from '../services/api'
import type { TenantContactChannel, EventThread } from '../services/admin.service'

/** Categorías del expediente, en el mismo orden en que se ofrecen como filtro. */
export const ACTIVITY_CATEGORIES = [
  // { value: '', label: 'Todo' },
  { value: 'lifecycle', label: 'Eventos' },
  { value: 'staff', label: 'Altas / Bajas de personal' },
  { value: 'work', label: 'Jornadas' },
  { value: 'management', label: 'Customer Success' },
  // Contactos antes que notas: es lo primero que se mira al abrir una ficha
  // ("¿ya hablamos con ellos?"), y las notas son el detalle de eso.
  { value: 'contact', label: 'Comunicaciones' },
  { value: 'note', label: 'Notas' },
  // Los testimonios tienen filtro propio: al preparar material comercial se
  // busca qué dijo este cliente, no se rebusca entre las notas internas.
  { value: 'testimonial', label: 'Testimonios' },
] as const

export interface TenantActivity {
  type: string
  category: string
  user: string
  user_id: number
  company: string
  details: string
  timestamp: string
  /** Solo las entradas de company_events lo traen; 0 en las derivadas. */
  event_id: number
  /** Solo en notas: fijada en la cabecera y fecha de la última corrección. */
  pinned?: boolean
  edited_at?: string
  /** Solo en contactos: por qué vía se contactó con la empresa. */
  channel?: TenantContactChannel
  /**
   * Registro del módulo que originó la entrada (hoy solo el testimonio). Sirve
   * para volver al original desde el expediente. 0 = no lleva a ningún sitio.
   */
  ref_id?: number
}

export interface TenantActivityPerson {
  user_id: number
  name: string
}

interface UseTenantActivityReturn {
  activity: TenantActivity[]
  total: number
  /** Movimientos por categoría ('' = total), con el filtro de persona aplicado. */
  counts: Record<string, number>
  /** Personas que aparecen en el expediente, para el desplegable de filtro. */
  people: TenantActivityPerson[]
  /** Notas fijadas, que van arriba y fuera de la cronología. */
  pinnedNotes: TenantActivity[]
  /**
   * Comentarios y archivos de las entradas de ESTA página, indexados por
   * event_id. Solo las entradas que son filas de company_events (notas,
   * contactos, suspensiones) pueden tener hilo; las derivadas de otras tablas
   * llegan con event_id 0 y no existen como registro al que colgar nada.
   */
  threads: Record<number, EventThread>
  isLoading: boolean
  /** Hay una página nueva en camino (se sigue mostrando la anterior). */
  isFetching: boolean
  error: string | null
  /** Crea la nota y devuelve el id de la entrada, para poder adjuntarle archivos. */
  addNote: (detail: string) => Promise<number>
  /** Deja constancia de un contacto con la empresa en su expediente. */
  logContact: (channel: TenantContactChannel, detail?: string) => Promise<void>
  updateNote: (eventId: number, detail: string) => Promise<void>
  addComment: (eventId: number, content: string) => Promise<number>
  updateComment: (commentId: number, content: string) => Promise<void>
  deleteComment: (commentId: number) => Promise<void>
  /** Sube el archivo y lo cuelga de la entrada (o de uno de sus comentarios). */
  addAttachment: (eventId: number, file: File, commentId?: number) => Promise<void>
  deleteAttachment: (attachmentId: number) => Promise<void>
  setNotePinned: (eventId: number, pinned: boolean) => Promise<void>
  deleteNote: (eventId: number) => Promise<void>
}

/** Clave de una página del expediente. Compartida por el expediente completo y
 *  por la vista acotada a una persona, para que ambas lean de la misma caché y
 *  una nota nueva no aparezca en una y falte en la otra. */
const activityKey = (tenantId: number, category: string, userId: number, page: number, pageSize: number) =>
  ['tenant-activity', tenantId, category, userId, page, pageSize]

/**
 * Una página del expediente de la empresa. La paginación y los filtros son de
 * servidor: el expediente de una empresa con recorrido son miles de líneas y
 * traerlas todas para enseñar veinte no tiene sentido.
 */
export function useTenantActivity(
  tenantId: number,
  category: string,
  userId: number,
  page: number,
  pageSize = 20,
): UseTenantActivityReturn {
  const qc = useQueryClient()
  const queryKey = activityKey(tenantId, category, userId, page, pageSize)

  const { data, isLoading, isFetching, error } = useQuery({
    queryKey,
    queryFn: () => adminService.getTenantActivity(tenantId, { category, user_id: userId, page, limit: pageSize }),
    enabled: !!tenantId,
    // Al cambiar de página o de filtro se mantiene lo anterior en pantalla
    // hasta que llega lo nuevo, en vez de parpadear a vacío.
    placeholderData: keepPreviousData,
  })

  // La lista de personas no depende de la página ni del filtro: se pide una vez
  // por empresa y se reutiliza mientras se navega el expediente.
  const peopleQ = useQuery({
    queryKey: ['tenant-activity-people', tenantId],
    queryFn: () => adminService.getTenantActivityPeople(tenantId),
    enabled: !!tenantId,
  })

  // Las fijadas son pocas y siempre visibles: van aparte de la paginación para
  // que no dependan de en qué página estés ni de qué filtro tengas puesto.
  const pinnedQ = useQuery({
    queryKey: ['tenant-activity-pinned', tenantId],
    queryFn: () => adminService.getTenantPinnedNotes(tenantId),
    enabled: !!tenantId,
  })

  // Tras tocar una nota queda obsoleto todo el expediente de esta empresa: sus
  // páginas, sus contadores, las fijadas, y la lista de personas (quien firma
  // su primera nota pasa a aparecer en el filtro).
  const invalidate = () =>
    qc.invalidateQueries({
      predicate: q =>
        typeof q.queryKey[0] === 'string' &&
        (q.queryKey[0] as string).startsWith('tenant-activity') &&
        q.queryKey[1] === tenantId,
    })

  const addMut = useMutation({
    mutationFn: (detail: string) => adminService.addTenantNote(tenantId, detail),
    onSuccess: invalidate,
  })

  // Comentarios y adjuntos invalidan lo mismo que las notas: el hilo viaja
  // dentro de la página del expediente, así que tocar uno deja obsoleta la
  // página entera.
  const addCommentMut = useMutation({
    mutationFn: ({ eventId, content }: { eventId: number; content: string }) =>
      adminService.addTenantComment(tenantId, eventId, content),
    onSuccess: invalidate,
  })

  const updateCommentMut = useMutation({
    mutationFn: ({ commentId, content }: { commentId: number; content: string }) =>
      adminService.updateTenantComment(tenantId, commentId, content),
    onSuccess: invalidate,
  })

  const deleteCommentMut = useMutation({
    mutationFn: (commentId: number) => adminService.deleteTenantComment(tenantId, commentId),
    onSuccess: invalidate,
  })

  const addAttachmentMut = useMutation({
    mutationFn: ({ eventId, file, commentId }: { eventId: number; file: File; commentId?: number }) =>
      adminService.addTenantAttachment(tenantId, eventId, file, commentId),
    onSuccess: invalidate,
  })

  const deleteAttachmentMut = useMutation({
    mutationFn: (attachmentId: number) => adminService.deleteTenantAttachment(tenantId, attachmentId),
    onSuccess: invalidate,
  })

  const contactMut = useMutation({
    mutationFn: ({ channel, detail }: { channel: TenantContactChannel; detail?: string }) =>
      adminService.logTenantContact(tenantId, channel, detail),
    onSuccess: invalidate,
  })

  const updateMut = useMutation({
    mutationFn: ({ eventId, detail }: { eventId: number; detail: string }) =>
      adminService.updateTenantNote(tenantId, eventId, detail),
    onSuccess: invalidate,
  })

  const pinMut = useMutation({
    mutationFn: ({ eventId, pinned }: { eventId: number; pinned: boolean }) =>
      adminService.setTenantNotePinned(tenantId, eventId, pinned),
    onSuccess: invalidate,
  })

  const deleteMut = useMutation({
    mutationFn: (eventId: number) => adminService.deleteTenantNote(tenantId, eventId),
    onSuccess: invalidate,
  })

  return {
    activity: (data?.data ?? []) as TenantActivity[],
    total: data?.total ?? 0,
    counts: (data?.counts ?? {}) as Record<string, number>,
    people: peopleQ.data ?? [],
    pinnedNotes: (pinnedQ.data ?? []) as TenantActivity[],
    threads: (data?.threads ?? {}) as Record<number, EventThread>,
    isLoading,
    isFetching,
    error: error ? 'No se pudo cargar el expediente' : null,
    // Devuelve el id de la entrada creada, por el mismo motivo que addComment:
    // los archivos adjuntos necesitan algo a lo que colgarse.
    addNote: async (detail) => (await addMut.mutateAsync(detail)).id,
    logContact: async (channel, detail) => { await contactMut.mutateAsync({ channel, detail }) },
    updateNote: async (eventId, detail) => { await updateMut.mutateAsync({ eventId, detail }) },
    // Devuelve el id para poder colgarle los archivos que se pegaron mientras
    // se escribía: el comentario tiene que existir antes de que nada apunte a él.
    addComment: async (eventId, content) => (await addCommentMut.mutateAsync({ eventId, content })).id,
    updateComment: async (commentId, content) => { await updateCommentMut.mutateAsync({ commentId, content }) },
    deleteComment: async (commentId) => { await deleteCommentMut.mutateAsync(commentId) },
    addAttachment: async (eventId, file, commentId) => { await addAttachmentMut.mutateAsync({ eventId, file, commentId }) },
    deleteAttachment: async (attachmentId) => { await deleteAttachmentMut.mutateAsync(attachmentId) },
    setNotePinned: async (eventId, pinned) => { await pinMut.mutateAsync({ eventId, pinned }) },
    deleteNote: async (eventId) => { await deleteMut.mutateAsync(eventId) },
  }
}

/**
 * Los movimientos de UNA persona dentro del expediente de su empresa, para la
 * ficha del profesional. Es de solo lectura a propósito: las notas y contactos
 * se escriben sobre la empresa, no sobre el individuo, y se gestionan desde su
 * expediente. Por eso no arrastra las consultas de personas ni de notas fijadas
 * (ambas son de la empresa entera y aquí no se enseñan).
 */
export function usePersonActivity(tenantId: number, userId: number, page: number, pageSize = 20) {
  const { data, isLoading, isFetching, error } = useQuery({
    queryKey: activityKey(tenantId, '', userId, page, pageSize),
    queryFn: () => adminService.getTenantActivity(tenantId, { category: '', user_id: userId, page, limit: pageSize }),
    enabled: !!tenantId && !!userId,
    placeholderData: keepPreviousData,
  })

  return {
    activity: (data?.data ?? []) as TenantActivity[],
    total: data?.total ?? 0,
    isLoading,
    isFetching,
    error: error ? 'No se pudo cargar la actividad de este profesional' : null,
  }
}
