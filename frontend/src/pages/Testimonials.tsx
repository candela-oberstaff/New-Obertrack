import { useEffect, useMemo, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  BadgeCheck,
  Building2,
  Clock,
  Download,
  FileSignature,
  MessageSquareQuote,
  Plus,
  Search,
  Send,
  Star,
  Trash2,
  Undo2,
  User,
  X,
} from 'lucide-react'

import { adminService } from '../services/admin.service'
import {
  parseTestimonialAnswers,
  testimonialService,
  type Testimonial,
  type TestimonialAudience,
  type TestimonialStatus,
  type TestimonialBulkResult,
} from '../services/testimonial.service'
import { useAuth } from '../context/AuthContext'
import { useNotification } from '../context/NotificationContext'
import { Modal, Select, useConfirm } from '../components/ui'
import styles from './Testimonials.module.css'

/** Pestañas de la bandeja. El orden sigue el flujo de trabajo, no el alfabeto. */
const TABS: { value: string; label: string }[] = [
  { value: 'submitted', label: 'Por revisar' },
  { value: 'changes_requested', label: 'En corrección' },
  { value: 'pending', label: 'Esperando respuesta' },
  { value: 'approved', label: 'Aprobados' },
  { value: 'rejected', label: 'Descartados' },
  { value: '', label: 'Todos' },
]

const STATUS_LABEL: Record<TestimonialStatus, string> = {
  pending: 'Esperando respuesta',
  submitted: 'Por revisar',
  approved: 'Aprobado',
  rejected: 'Descartado',
  changes_requested: 'En corrección',
}

// Cómo se firmó. Las tres valen, pero no prueban lo mismo, así que el panel lo
// dice igual que la constancia en vez de dejarlo en una imagen sin contexto.
const SIGNATURE_MODE_LABEL: Record<string, string> = {
  drawn: 'Trazada a mano',
  uploaded: 'Imagen cargada por el firmante',
  typed: 'Nombre escrito con una tipografía',
}

const AUDIENCE_LABEL: Record<TestimonialAudience, string> = {
  professional: 'Profesional',
  company: 'Empresa',
}

const fmtDate = (iso?: string | null) =>
  iso
    ? new Date(iso).toLocaleDateString('es', { day: '2-digit', month: 'short', year: 'numeric' })
    : '—'

const fmtDateTime = (iso?: string | null) =>
  iso
    ? new Date(iso).toLocaleString('es', {
        day: '2-digit',
        month: 'short',
        year: 'numeric',
        hour: '2-digit',
        minute: '2-digit',
      })
    : '—'

function errorMessage(err: unknown, fallback: string): string {
  const responseError =
    err && typeof err === 'object' && 'response' in err
      ? (err as { response?: { data?: { error?: string } } }).response?.data?.error
      : undefined
  return responseError || fallback
}

/** Estrellas de solo lectura. */
function Stars({ value }: { value: number }) {
  if (!value) return null
  return (
    <span className={styles.stars} aria-label={`${value} de 5`}>
      {[1, 2, 3, 4, 5].map((n) => (
        <Star
          key={n}
          size={14}
          className={n <= value ? styles.starOn : styles.starOff}
          fill={n <= value ? 'currentColor' : 'none'}
        />
      ))}
    </span>
  )
}

/**
 * Bandeja de testimonios: se piden desde aquí, llegan firmados desde la página
 * pública y se aprueban antes de poder usarse.
 *
 * El alcance es el mismo que el de Admin y Empresas —superadmin y Customer
 * Success—, que es el equipo que trata a diario con profesionales y empresas.
 */
export default function Testimonials() {
  const qc = useQueryClient()
  const notify = useNotification()
  const confirm = useConfirm()
  const { user } = useAuth()

  const [tab, setTab] = useState('submitted')
  const [audience, setAudience] = useState('')
  const [search, setSearch] = useState('')
  const [debouncedSearch, setDebouncedSearch] = useState('')
  const [detail, setDetail] = useState<Testimonial | null>(null)
  const [requestOpen, setRequestOpen] = useState(false)
  const [searchParams, setSearchParams] = useSearchParams()

  // El buscador va contra el servidor; sin rebote dispararía una consulta por
  // cada tecla.
  useEffect(() => {
    const t = setTimeout(() => setDebouncedSearch(search.trim()), 300)
    return () => clearTimeout(t)
  }, [search])

  // Enlace profundo desde el expediente ("Ver testimonio"): se abre el detalle
  // aunque el testimonio no esté en la pestaña actual, así que se pide por id en
  // lugar de buscarlo en la lista cargada. El parámetro se consume al abrir para
  // que recargar o cerrar no lo vuelva a disparar.
  useEffect(() => {
    const openId = Number(searchParams.get('open'))
    if (!openId) return
    let alive = true
    testimonialService
      .get(openId)
      .then((t) => {
        if (alive) setDetail(t)
      })
      .catch(() => notify.error('No encontramos ese testimonio'))
      .finally(() => setSearchParams({}, { replace: true }))
    return () => {
      alive = false
    }
    // notify y setSearchParams son estables; depender de ellos re-dispararía
    // la apertura en cada render.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [searchParams])

  const { data, isLoading, isError } = useQuery({
    queryKey: ['testimonials', tab, audience, debouncedSearch],
    queryFn: () =>
      testimonialService.list({ status: tab, audience, search: debouncedSearch }),
  })

  const items = data?.items ?? []
  const counts = data?.counts ?? {}

  const invalidate = () => qc.invalidateQueries({ queryKey: ['testimonials'] })

  const resend = useMutation({
    mutationFn: (id: number) => testimonialService.resend(id),
    onSuccess: () => {
      notify.success('Solicitud reenviada')
      invalidate()
    },
    onError: (err) => notify.error(errorMessage(err, 'No pudimos reenviar la solicitud')),
  })

  const remove = useMutation({
    mutationFn: (id: number) => testimonialService.remove(id),
    onSuccess: () => {
      notify.success('Testimonio eliminado')
      setDetail(null)
      invalidate()
    },
    onError: (err) => notify.error(errorMessage(err, 'No pudimos eliminar el testimonio')),
  })

  const askDelete = async (t: Testimonial) => {
    const ok = await confirm({
      title: '¿Eliminar este testimonio?',
      message: t.signed_at
        ? 'Está firmado: al eliminarlo se pierde la evidencia del consentimiento. Descarga antes la constancia si la necesitas.'
        : 'Se eliminará la solicitud y su enlace dejará de funcionar.',
      confirmLabel: 'Eliminar',
      variant: 'danger',
    })
    if (ok) remove.mutate(t.id)
  }

  return (
    <div className={styles.page}>
      <header className={styles.header}>
        <h1>
          <MessageSquareQuote size={22} /> Testimonios
        </h1>
        <button type="button" className={styles.primaryBtn} onClick={() => setRequestOpen(true)}>
          <Plus size={16} /> Pedir testimonio
        </button>
      </header>

      <p className={styles.lead}>
        Pide un testimonio por correo. Quien lo recibe escribe su experiencia y firma la
        autorización de uso en la misma página, sin necesidad de entrar a la plataforma.
      </p>

      {/* --- Filtros --- */}
      <div className={styles.toolbar}>
        <div className={styles.tabs} role="tablist">
          {TABS.map((t) => {
            const n = t.value ? counts[t.value] : Object.values(counts).reduce((a, b) => a + b, 0)
            return (
              <button
                key={t.value || 'all'}
                type="button"
                role="tab"
                aria-selected={tab === t.value}
                className={`${styles.tab} ${tab === t.value ? styles.tabOn : ''}`}
                onClick={() => setTab(t.value)}
              >
                {t.label}
                {n ? <span className={styles.tabCount}>{n}</span> : null}
              </button>
            )
          })}
        </div>

        <div className={styles.filters}>
          <div className={styles.searchBox}>
            <Search size={15} />
            <input
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="Buscar por nombre, correo o empresa"
              aria-label="Buscar testimonios"
            />
            {search && (
              <button type="button" onClick={() => setSearch('')} aria-label="Limpiar búsqueda">
                <X size={14} />
              </button>
            )}
          </div>
          <Select
            options={[
              { value: 'professional', label: 'Profesionales' },
              { value: 'company', label: 'Empresas' },
            ]}
            value={audience}
            onChange={(v) => setAudience(String(v))}
            placeholder="Toda audiencia"
            clearable
            ariaLabel="Filtrar por audiencia"
          />
        </div>
      </div>

      {/* --- Listado --- */}
      {isLoading ? (
        <div className={styles.empty}>Cargando testimonios...</div>
      ) : isError ? (
        <div className={styles.empty}>No pudimos cargar los testimonios.</div>
      ) : items.length === 0 ? (
        <div className={styles.empty}>
          <MessageSquareQuote size={30} />
          <p>
            {tab === 'submitted'
              ? 'No hay testimonios esperando revisión.'
              : 'Todavía no hay testimonios aquí.'}
          </p>
        </div>
      ) : (
        <div className={styles.grid}>
          {items.map((t) => (
            <article key={t.id} className={styles.card} onClick={() => setDetail(t)}>
              <div className={styles.cardHead}>
                <span className={`${styles.avatar} ${styles[`av_${t.audience}`]}`}>
                  {t.audience === 'company' ? <Building2 size={16} /> : <User size={16} />}
                </span>
                <div className={styles.cardWho}>
                  <p className={styles.cardName}>{t.recipient_name}</p>
                  <p className={styles.cardMeta}>
                    {[t.recipient_role, t.recipient_company].filter(Boolean).join(' · ') ||
                      t.recipient_email}
                  </p>
                </div>
                <span className={`${styles.badge} ${styles[`st_${t.status}`]}`}>
                  {STATUS_LABEL[t.status]}
                </span>
              </div>

              {t.quote ? (
                <p className={styles.cardQuote}>{t.published_quote || t.quote}</p>
              ) : (
                <p className={styles.cardPending}>
                  <Clock size={13} /> Enviado el {fmtDate(t.created_at)} · vence el{' '}
                  {fmtDate(t.expires_at)}
                </p>
              )}

              <div className={styles.cardFoot}>
                <Stars value={t.rating} />
                <span className={styles.cardTag}>{AUDIENCE_LABEL[t.audience]}</span>
                {t.signed_at && (
                  <span className={styles.signedTag}>
                    <FileSignature size={12} /> Firmado
                  </span>
                )}
              </div>
            </article>
          ))}
        </div>
      )}

      {detail && (
        <DetailModal
          testimonial={detail}
          onClose={() => setDetail(null)}
          onResend={() => resend.mutate(detail.id)}
          onDelete={() => void askDelete(detail)}
          canDelete={!!user?.is_superadmin}
          onReviewed={() => {
            setDetail(null)
            invalidate()
          }}
        />
      )}

      {requestOpen && (
        <RequestModal
          onClose={() => setRequestOpen(false)}
          onCreated={() => {
            setRequestOpen(false)
            invalidate()
          }}
        />
      )}
    </div>
  )
}

/* ===================== Detalle ===================== */

interface DetailModalProps {
  testimonial: Testimonial
  onClose: () => void
  onResend: () => void
  onDelete: () => void
  onReviewed: () => void
  canDelete: boolean
}

function DetailModal({
  testimonial: t,
  onClose,
  onResend,
  onDelete,
  onReviewed,
  canDelete,
}: DetailModalProps) {
  const notify = useNotification()
  const [publishedQuote, setPublishedQuote] = useState(t.published_quote || t.quote)
  const [note, setNote] = useState(t.review_note)
  const [signatureURL, setSignatureURL] = useState('')

  const answers = useMemo(() => parseTestimonialAnswers(t.answers), [t.answers])

  // El trazo se pide por XHR autenticado (el endpoint exige sesión) y se pinta
  // desde un object URL, que hay que revocar al cerrar para no filtrar memoria.
  useEffect(() => {
    if (!t.signed_at) return
    let url = ''
    let alive = true
    testimonialService
      .signatureURL(t.id)
      .then((u) => {
        if (alive) {
          url = u
          setSignatureURL(u)
        } else {
          URL.revokeObjectURL(u)
        }
      })
      .catch(() => {})
    return () => {
      alive = false
      if (url) URL.revokeObjectURL(url)
    }
  }, [t.id, t.signed_at])

  const [changesOpen, setChangesOpen] = useState(false)

  const review = useMutation({
    mutationFn: (approve: boolean) =>
      testimonialService.review(t.id, {
        approve,
        note,
        // Solo se guarda la versión editada si de verdad difiere del original.
        published_quote: publishedQuote.trim() === t.quote.trim() ? '' : publishedQuote,
      }),
    onSuccess: (data, approve) => {
      notify.success(approve ? 'Testimonio aprobado' : 'Testimonio descartado')
      // El aviso se cuenta aparte y dura más: es justo lo que se perdería si se
      // mezclara con el "aprobado" y desapareciera en dos segundos.
      if (data.warning) notify.warning(data.warning)
      onReviewed()
    },
    onError: (err) => notify.error(errorMessage(err, 'No pudimos guardar la revisión')),
  })

  const download = async () => {
    try {
      await testimonialService.downloadConsent(t.id, `consentimiento-testimonio-${t.id}.pdf`)
    } catch (err) {
      notify.error(errorMessage(err, 'No pudimos generar la constancia'))
    }
  }

  const answered = t.status !== 'pending'

  return (
    <Modal isOpen onClose={onClose} title={t.recipient_name} size="lg">
      <div className={styles.detail}>
        <div className={styles.detailMeta}>
          <span className={`${styles.badge} ${styles[`st_${t.status}`]}`}>
            {STATUS_LABEL[t.status]}
          </span>
          <span className={styles.cardTag}>{AUDIENCE_LABEL[t.audience]}</span>
          <span className={styles.detailMuted}>{t.recipient_email}</span>
          {t.revisions > 0 && (
            <span className={styles.cardTag}>
              {t.revisions} {t.revisions === 1 ? 'corrección pedida' : 'correcciones pedidas'}
            </span>
          )}
        </div>

        {/* Mientras está en corrección, lo que el equipo necesita ver es qué se
            le pidió: sin esto no se sabe qué se está esperando. */}
        {t.status === 'changes_requested' && t.change_reason && (
          <div className={styles.changeBox}>
            <p className={styles.changeTitle}>
              <Undo2 size={14} /> Esperando corrección desde el{' '}
              {fmtDate(t.change_requested_at)}
            </p>
            <p className={styles.changeReason}>{t.change_reason}</p>
            <p className={styles.changeHint}>
              Abajo sigue el testimonio tal como lo envió: es lo que hay hasta que lo corrija.
            </p>
          </div>
        )}

        {!answered ? (
          <>
            <div className={styles.detailBox}>
              <p className={styles.detailNote}>{t.intro_message}</p>
            </div>
            <dl className={styles.kv}>
              <div>
                <dt>Enviado</dt>
                <dd>{fmtDate(t.created_at)}</dd>
              </div>
              <div>
                <dt>Vence</dt>
                <dd>{fmtDate(t.expires_at)}</dd>
              </div>
              <div>
                <dt>Último recordatorio</dt>
                <dd>{fmtDate(t.reminded_at)}</dd>
              </div>
            </dl>
          </>
        ) : (
          <>
            {/* El original, tal como llegó. Nunca se edita: es evidencia. */}
            <section className={styles.section}>
              <h3 className={styles.sectionTitle}>
                Testimonio recibido <Stars value={t.rating} />
              </h3>
              <blockquote className={styles.quote}>{t.quote}</blockquote>
            </section>

            {answers.length > 0 && (
              <section className={styles.section}>
                <h3 className={styles.sectionTitle}>Respuestas a las preguntas guía</h3>
                {answers.map((a, i) => (
                  <div key={i} className={styles.answer}>
                    <p className={styles.answerPrompt}>{a.prompt}</p>
                    <p className={styles.answerText}>{a.answer}</p>
                  </div>
                ))}
              </section>
            )}

            {/* Versión para publicar. Editarla no toca el original. */}
            <section className={styles.section}>
              <h3 className={styles.sectionTitle}>Versión para publicar</h3>
              <textarea
                className={styles.textArea}
                rows={4}
                value={publishedQuote}
                onChange={(e) => setPublishedQuote(e.target.value)}
              />
              <p className={styles.hint}>
                Puedes recortar o corregir la puntuación. El texto original se conserva intacto.
              </p>
            </section>

            <section className={styles.section}>
              <h3 className={styles.sectionTitle}>Permiso otorgado</h3>
              <ul className={styles.permList}>
                {[
                  ['Nombre', t.allow_public_name],
                  ['Cargo y empresa', t.allow_role],
                  ['Fotografía', t.allow_photo],
                  ['Logo de la empresa', t.allow_logo],
                ].map(([label, granted]) => (
                  <li key={String(label)} className={granted ? styles.permYes : styles.permNo}>
                    {granted ? <BadgeCheck size={14} /> : <X size={14} />} {String(label)}
                  </li>
                ))}
              </ul>
            </section>

            <section className={styles.section}>
              <h3 className={styles.sectionTitle}>Firma y evidencia</h3>
              {signatureURL && (
                <div className={styles.signatureBox}>
                  <img src={signatureURL} alt="Firma" className={styles.signatureImg} />
                </div>
              )}
              <dl className={styles.kv}>
                <div>
                  <dt>Firmado por</dt>
                  <dd>{t.signature_name || '—'}</dd>
                </div>
                <div>
                  <dt>Modalidad</dt>
                  <dd>{SIGNATURE_MODE_LABEL[t.signature_mode || 'drawn']}</dd>
                </div>
                <div>
                  <dt>Fecha y hora</dt>
                  <dd>{fmtDateTime(t.signed_at)}</dd>
                </div>
                <div>
                  <dt>Dirección IP</dt>
                  <dd>{t.signer_ip || '—'}</dd>
                </div>
                <div>
                  <dt>Redacción firmada</dt>
                  <dd>{t.consent_version || '—'}</dd>
                </div>
              </dl>
              <button type="button" className={styles.ghostBtn} onClick={() => void download()}>
                <Download size={15} /> Descargar constancia firmada
              </button>
            </section>

            <section className={styles.section}>
              <h3 className={styles.sectionTitle}>Nota interna</h3>
              <textarea
                className={styles.textArea}
                rows={2}
                value={note}
                onChange={(e) => setNote(e.target.value)}
                placeholder="Dónde se va a usar, con quién se coordinó..."
              />
            </section>
          </>
        )}

        <footer className={styles.detailActions}>
          {canDelete && (
            <button type="button" className={styles.dangerBtn} onClick={onDelete}>
              <Trash2 size={15} /> Eliminar
            </button>
          )}
          <span className={styles.spacer} />
          {!answered && (
            <button type="button" className={styles.ghostBtn} onClick={onResend}>
              <Send size={15} /> Reenviar solicitud
            </button>
          )}
          {answered && (
            <>
              {/* Devolver es distinto de descartar: deja el testimonio abierto
                  esperando a su autor en vez de cerrar el asunto. Solo tiene
                  sentido sobre lo que espera revisión. */}
              {t.status === 'submitted' && (
                <button
                  type="button"
                  className={styles.ghostBtn}
                  onClick={() => setChangesOpen(true)}
                  disabled={review.isPending}
                >
                  <Undo2 size={15} /> Pedir corrección
                </button>
              )}
              <button
                type="button"
                className={styles.ghostBtn}
                onClick={() => review.mutate(false)}
                disabled={review.isPending}
              >
                Descartar
              </button>
              <button
                type="button"
                className={styles.primaryBtn}
                onClick={() => review.mutate(true)}
                disabled={review.isPending}
              >
                <BadgeCheck size={16} /> Aprobar
              </button>
            </>
          )}
        </footer>
      </div>

      {changesOpen && (
        <ChangesModal
          testimonial={t}
          onClose={() => setChangesOpen(false)}
          onDone={onReviewed}
        />
      )}
    </Modal>
  )
}

/* ===================== Pedir corrección ===================== */

// Motivos frecuentes. Rellenan el campo en lugar de sustituirlo: casi siempre
// hay que concretar ("tu nombre aparece como 'Laura M.'"), y un desplegable de
// causas cerradas produce mensajes que no dicen qué arreglar.
const CHANGE_PRESETS = [
  {
    label: 'Firma ilegible',
    text: 'La firma no se ve bien, quedó cortada o casi en blanco. ¿Puedes volver a trazarla con más calma?',
  },
  {
    label: 'Nombre incompleto',
    text: 'Necesitamos tu nombre completo tal como aparece en tu documento; el que escribiste está abreviado.',
  },
  {
    label: 'Errata en el texto',
    text: 'Hay una errata en el testimonio que preferimos que corrijas tú antes de publicarlo: ',
  },
  {
    label: 'Falta el cargo',
    text: 'Para poder publicarlo necesitamos que autorices que aparezca tu cargo y tu empresa.',
  },
]

function ChangesModal({
  testimonial: t,
  onClose,
  onDone,
}: {
  testimonial: Testimonial
  onClose: () => void
  onDone: () => void
}) {
  const notify = useNotification()
  const [reason, setReason] = useState('')

  const send = useMutation({
    mutationFn: () => testimonialService.requestChanges(t.id, reason.trim()),
    onSuccess: () => {
      notify.success('Se lo devolvimos para que lo corrija')
      onDone()
    },
    onError: (err) => notify.error(errorMessage(err, 'No pudimos devolver el testimonio')),
  })

  return (
    <Modal
      isOpen
      onClose={onClose}
      title="Pedir una corrección"
      size="md"
      isDirty={reason.trim() !== ''}
    >
      <div className={styles.form}>
        <p className={styles.hint}>
          {t.recipient_name} recibirá este mensaje por correo y en la app, con el enlace reabierto y
          lo que ya escribió precargado. Tendrá que volver a firmar: su firma actual autoriza el
          texto sin corregir.
        </p>

        <div className={styles.field}>
          <span className={styles.label}>Motivos frecuentes</span>
          <div className={styles.presets}>
            {CHANGE_PRESETS.map((p) => (
              <button
                key={p.label}
                type="button"
                className={styles.preset}
                onClick={() => setReason((prev) => (prev ? `${prev.trim()} ${p.text}` : p.text))}
              >
                {p.label}
              </button>
            ))}
          </div>
        </div>

        <div className={styles.field}>
          <label className={styles.label} htmlFor="reason">
            Qué debe corregir
          </label>
          <textarea
            id="reason"
            className={styles.textArea}
            rows={4}
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            placeholder="Escríbelo como se lo dirías a esa persona..."
          />
          <p className={styles.hint}>
            Es lo único que va a leer, así que conviene ser concreto y amable.
          </p>
        </div>

        <footer className={styles.detailActions}>
          <span className={styles.spacer} />
          <button type="button" className={styles.ghostBtn} onClick={onClose}>
            Cancelar
          </button>
          <button
            type="button"
            className={styles.primaryBtn}
            disabled={reason.trim() === '' || send.isPending}
            onClick={() => send.mutate()}
          >
            <Undo2 size={16} /> {send.isPending ? 'Enviando...' : 'Devolver para corregir'}
          </button>
        </footer>
      </div>
    </Modal>
  )
}

/* ===================== Pedir testimonio ===================== */

interface Candidate {
  id: number
  name: string
  email: string
  user_type: string
  job_title?: string
  company_name?: string
  is_superadmin?: boolean
}

function RequestModal({ onClose, onCreated }: { onClose: () => void; onCreated: () => void }) {
  const notify = useNotification()

  const [audience, setAudience] = useState<TestimonialAudience>('professional')
  const [query, setQuery] = useState('')
  const [candidates, setCandidates] = useState<Candidate[]>([])
  const [selected, setSelected] = useState<Candidate[]>([])
  const [intro, setIntro] = useState('')
  const [introTouched, setIntroTouched] = useState(false)
  // Resultado del último envío. Mientras existe, el modal deja de ser un
  // formulario y pasa a ser un informe: hay que poder leer quién quedó fuera.
  const [result, setResult] = useState<TestimonialBulkResult | null>(null)

  const { data: templates } = useQuery({
    queryKey: ['testimonial-templates'],
    queryFn: () => testimonialService.getTemplates(),
  })

  const template = templates?.find((t) => t.audience === audience)

  // La nota arranca con la sugerida por la plantilla y se sigue sincronizando
  // al cambiar de audiencia, salvo que ya se haya escrito algo a mano.
  useEffect(() => {
    if (!introTouched && template) setIntro(template.intro)
  }, [template, introTouched])

  // Buscador contra el servidor, con rebote. Se piden más de los que se muestran
  // porque después se filtra por tipo de cuenta.
  useEffect(() => {
    const term = query.trim()
    if (term.length < 2) {
      setCandidates([])
      return
    }
    let alive = true
    const t = setTimeout(() => {
      adminService
        .getUsers({ q: term, limit: 20 })
        .then((res: { data?: Candidate[] } | Candidate[]) => {
          const arr = Array.isArray(res) ? res : (res?.data ?? [])
          if (alive) setCandidates(arr)
        })
        .catch(() => {})
    }, 280)
    return () => {
      alive = false
      clearTimeout(t)
    }
  }, [query])

  // El backend rechaza una audiencia que no case con el tipo de cuenta, así que
  // aquí ni se ofrece: es más claro que dejar elegir y luego fallar. Los ya
  // elegidos tampoco vuelven a salir en las sugerencias.
  const wantedType = audience === 'company' ? 'empleador' : 'profesional'
  const chosen = new Set(selected.map((c) => c.id))
  const suggestions = candidates
    .filter((c) => c.user_type === wantedType && !chosen.has(c.id))
    .slice(0, 6)

  const add = (c: Candidate) => {
    setSelected((prev) => [...prev, c])
    setQuery('')
  }
  const removeAt = (id: number) => setSelected((prev) => prev.filter((c) => c.id !== id))

  const send = useMutation({
    mutationFn: () =>
      testimonialService.requestMany(
        selected.map((c) => c.id),
        { audience, intro_message: intro.trim() }
      ),
    onSuccess: (res) => {
      if (res.sent > 0) {
        notify.success(res.sent === 1 ? 'Solicitud enviada' : `${res.sent} solicitudes enviadas`)
      }
      // Con omisiones el modal se queda abierto enseñando el detalle: cerrarlo
      // y dejar solo un aviso pasajero escondería justo lo que hay que leer.
      if (res.skipped > 0) {
        setResult(res)
      } else {
        onCreated()
      }
    },
    onError: (err) => notify.error(errorMessage(err, 'No pudimos enviar las solicitudes')),
  })

  // --- Informe posterior al envío ---
  if (result) {
    return (
      <Modal isOpen onClose={onCreated} title="Resultado del envío" size="md">
        <div className={styles.form}>
          <p className={styles.hint}>
            Se {result.sent === 1 ? 'envió' : 'enviaron'} <strong>{result.sent}</strong>{' '}
            {result.sent === 1 ? 'solicitud' : 'solicitudes'}.{' '}
            {result.skipped === 1 ? 'Esta quedó fuera' : `Estas ${result.skipped} quedaron fuera`}:
          </p>

          <ul className={styles.outcomeList}>
            {result.outcomes
              .filter((o) => !o.sent)
              .map((o) => (
                <li key={o.user_id} className={styles.outcome}>
                  <span className={styles.selectedName}>{o.name}</span>
                  <span className={styles.selectedMeta}>{o.reason}</span>
                </li>
              ))}
          </ul>

          <footer className={styles.detailActions}>
            <span className={styles.spacer} />
            <button type="button" className={styles.primaryBtn} onClick={onCreated}>
              Entendido
            </button>
          </footer>
        </div>
      </Modal>
    )
  }

  const many = selected.length > 1

  return (
    <Modal
      isOpen
      onClose={onClose}
      title={many ? `Pedir ${selected.length} testimonios` : 'Pedir un testimonio'}
      size="md"
      isDirty={selected.length > 0 || introTouched}
    >
      <div className={styles.form}>
        <div className={styles.field}>
          <span className={styles.label}>¿A quién se lo pedimos?</span>
          <div className={styles.audiencePick}>
            {(['professional', 'company'] as TestimonialAudience[]).map((a) => (
              <button
                key={a}
                type="button"
                className={`${styles.audienceBtn} ${audience === a ? styles.audienceOn : ''}`}
                onClick={() => {
                  setAudience(a)
                  // Cambiar de audiencia invalida lo elegido: son tipos de cuenta
                  // distintos y el servidor los rechazaría uno a uno.
                  setSelected([])
                }}
              >
                {a === 'company' ? <Building2 size={16} /> : <User size={16} />}
                {a === 'company' ? 'Empresas' : 'Profesionales'}
              </button>
            ))}
          </div>
        </div>

        <div className={styles.field}>
          <label className={styles.label} htmlFor="who">
            {audience === 'company' ? 'Empresas' : 'Profesionales'}
            {selected.length > 0 && (
              <span className={styles.countTag}>{selected.length} seleccionadas</span>
            )}
          </label>

          {selected.length > 0 && (
            <ul className={styles.chips}>
              {selected.map((c) => (
                <li key={c.id} className={styles.chip}>
                  <span>{c.name}</span>
                  <button
                    type="button"
                    onClick={() => removeAt(c.id)}
                    aria-label={`Quitar a ${c.name}`}
                  >
                    <X size={13} />
                  </button>
                </li>
              ))}
            </ul>
          )}

          <div className={styles.searchBox}>
            <Search size={15} />
            <input
              id="who"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder={
                selected.length > 0 ? 'Añadir a alguien más...' : 'Escribe un nombre o correo'
              }
              autoComplete="off"
            />
          </div>

          {suggestions.length > 0 && (
            <ul className={styles.suggestions}>
              {suggestions.map((c) => (
                <li key={c.id}>
                  <button type="button" onClick={() => add(c)}>
                    <span className={styles.selectedName}>{c.name}</span>
                    <span className={styles.selectedMeta}>
                      {[c.job_title, c.company_name].filter(Boolean).join(' · ') || c.email}
                    </span>
                  </button>
                </li>
              ))}
            </ul>
          )}

          {query.trim().length >= 2 && suggestions.length === 0 && (
            <p className={styles.hint}>
              Sin resultados nuevos entre las cuentas de tipo{' '}
              {audience === 'company' ? 'empresa' : 'profesional'}.
            </p>
          )}
        </div>

        <div className={styles.field}>
          <label className={styles.label} htmlFor="intro">
            Nota de presentación
          </label>
          <textarea
            id="intro"
            className={styles.textArea}
            rows={3}
            value={intro}
            onChange={(e) => {
              setIntro(e.target.value)
              setIntroTouched(true)
            }}
          />
          <p className={styles.hint}>
            {many
              ? 'La misma nota va a todas. Si quieres personalizar el mensaje, pídelo de una en una.'
              : 'Es lo primero que lee en el correo. Personalízala si puedes.'}
          </p>
        </div>

        {template && (
          <div className={styles.field}>
            <span className={styles.label}>
              {many ? 'Preguntas guía que verán' : 'Preguntas guía que verá'}
            </span>
            <ul className={styles.promptList}>
              {template.prompts.map((p, i) => (
                <li key={i}>{p}</li>
              ))}
            </ul>
          </div>
        )}

        <footer className={styles.detailActions}>
          <span className={styles.spacer} />
          <button type="button" className={styles.ghostBtn} onClick={onClose}>
            Cancelar
          </button>
          <button
            type="button"
            className={styles.primaryBtn}
            disabled={selected.length === 0 || send.isPending}
            onClick={() => send.mutate()}
          >
            <Send size={16} />{' '}
            {send.isPending
              ? 'Enviando...'
              : many
                ? `Enviar ${selected.length} solicitudes`
                : 'Enviar solicitud'}
          </button>
        </footer>
      </div>
    </Modal>
  )
}
