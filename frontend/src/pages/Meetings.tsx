import { useMemo, useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
  Video,
  Plus,
  Calendar,
  Clock,
  Users,
  Link2,
  Copy,
  Check,
  X,
  Repeat,
  Trash2,
  Pencil,
  AlertTriangle,
} from 'lucide-react'
import {
  meetingService,
  parseMeetingError,
  type Meeting,
  type MeetingPayload,
} from '../services/meeting.service'
import { userService } from '../services/user.service'
import { useGoogleConnection } from '../hooks/useGoogleConnection'
import { useAuth } from '../context/AuthContext'
import { useConfirm } from '../components/ui/ConfirmProvider'
import styles from './Meetings.module.css'

/** Ruta a la que vuelve el navegador tras el consentimiento de Google. */
const RETURN_TO = '/sesiones'

/** Zona horaria del navegador: con la que se convoca por defecto. */
const BROWSER_TZ = Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC'

/** Frecuencias soportadas. El backend valida y normaliza la regla que se manda. */
const FREQ_OPTIONS = [
  { value: '', label: 'No se repite' },
  { value: 'FREQ=DAILY', label: 'Cada día' },
  { value: 'FREQ=WEEKLY', label: 'Cada semana' },
  { value: 'FREQ=WEEKLY;INTERVAL=2', label: 'Cada dos semanas' },
  { value: 'FREQ=MONTHLY', label: 'Cada mes' },
]

type SeriesEnd = 'never' | 'on' | 'after'

/**
 * buildRRULE junta frecuencia y final en la regla que entiende el backend. El
 * UNTIL del estándar va en UTC básico (20260930T235959Z) y marca el último día
 * completo, para que una reunión de ese mismo día siga contando.
 */
function buildRRULE(freq: string, end: SeriesEnd, until: string, count: number): string {
  if (!freq) return ''
  if (end === 'on' && until) {
    const d = new Date(`${until}T23:59:59`)
    const iso = d.toISOString().replace(/[-:]/g, '').replace(/\.\d{3}/, '')
    return `RRULE:${freq};UNTIL=${iso}`
  }
  if (end === 'after' && count > 0) return `RRULE:${freq};COUNT=${count}`
  return `RRULE:${freq}`
}

/** Descompone la regla guardada para poder reabrirla en el formulario. */
function parseRRULE(rule?: string): { freq: string; end: SeriesEnd; until: string; count: number } {
  if (!rule) return { freq: '', end: 'never', until: '', count: 10 }
  const body = rule.replace(/^RRULE:/i, '')
  const parts = body.split(';')
  const countPart = parts.find(p => p.startsWith('COUNT='))
  const untilPart = parts.find(p => p.startsWith('UNTIL='))
  const freq = parts.filter(p => !p.startsWith('COUNT=') && !p.startsWith('UNTIL=')).join(';')

  if (countPart) return { freq, end: 'after', until: '', count: Number(countPart.slice(6)) || 10 }
  if (untilPart) {
    // 20260930T235959Z → 2026-09-30, que es lo que espera <input type="date">.
    const raw = untilPart.slice(6)
    const iso = `${raw.slice(0, 4)}-${raw.slice(4, 6)}-${raw.slice(6, 8)}`
    return { freq, end: 'on', until: iso, count: 10 }
  }
  return { freq, end: 'never', until: '', count: 10 }
}

function recurrenceLabel(rule?: string) {
  const { freq, end, count } = parseRRULE(rule)
  const base = FREQ_OPTIONS.find(o => o.value === freq)?.label
  if (!base) return rule ? 'Se repite' : undefined
  if (end === 'after') return `${base} · ${count} veces`
  if (end === 'on') return `${base} · hasta fecha`
  return base
}

/**
 * occurrenceOf devuelve la reunión que toca mostrar. Para una serie, `start_at`
 * es solo la PRIMERA: usarlo dejaría una sesión diaria mostrando para siempre la
 * fecha del día que se creó.
 */
function occurrenceOf(meeting: Meeting) {
  return {
    start: new Date(meeting.next_start_at ?? meeting.start_at),
    end: new Date(meeting.next_end_at ?? meeting.end_at),
  }
}

function formatWhen(meeting: Meeting) {
  const { start, end } = occurrenceOf(meeting)
  const day = start.toLocaleDateString('es-ES', { weekday: 'short', day: 'numeric', month: 'short' })
  const from = start.toLocaleTimeString('es-ES', { hour: '2-digit', minute: '2-digit' })
  const to = end.toLocaleTimeString('es-ES', { hour: '2-digit', minute: '2-digit' })
  return { day, range: `${from} – ${to}` }
}

/** Una sesión "en curso o inminente" merece destacarse: es la que se va a usar. */
function startsSoon(meeting: Meeting) {
  const { start, end } = occurrenceOf(meeting)
  return start.getTime() - Date.now() < 15 * 60 * 1000 && end.getTime() > Date.now()
}

/** El "https://" no aporta nada al leer un enlace de Meet y se come el ancho. */
function displayMeetURL(url: string) {
  return url.replace(/^https?:\/\//, '')
}

/**
 * useCopyToClipboard devuelve un "copiar" que se marca solo durante un momento.
 * Lo usan la tarjeta y el campo del formulario, que copian el mismo enlace desde
 * sitios distintos.
 */
function useCopyToClipboard() {
  const [copied, setCopied] = useState(false)
  const copy = async (text: string) => {
    try {
      await navigator.clipboard.writeText(text)
      setCopied(true)
      setTimeout(() => setCopied(false), 1800)
    } catch {
      // El navegador puede bloquear el portapapeles si no hay gesto directo; el
      // enlace se ve escrito en pantalla, así que siempre queda copiarlo a mano.
    }
  }
  return { copied, copy }
}

/**
 * LivePresence muestra cuánta gente hay AHORA en la sala, vía la API de Meet.
 *
 * Solo consulta mientras la sesión está en curso o a punto: preguntar por una
 * reunión de la semana que viene sería gastar cuota para que siempre conteste
 * "sala vacía". React Query repite cada 20s mientras la tarjeta está montada.
 */
function LivePresence({ meeting, onNeedsReconnect }: { meeting: Meeting; onNeedsReconnect: () => void }) {
  const relevant = meeting.status === 'scheduled' && !!meeting.meet_url && startsSoon(meeting)

  const { data, error } = useQuery({
    queryKey: ['meeting-presence', meeting.id],
    queryFn: () => meetingService.presence(meeting.id),
    enabled: relevant,
    refetchInterval: 20_000,
    // Un contador que falla no debe reintentar en bucle contra Google.
    retry: false,
  })

  if (!relevant) return null

  // La falta de scope es accionable (reconectar) y por eso se muestra; el resto
  // de fallos se callan: es un contador, no vale llenar la tarjeta de rojo.
  if (error) {
    if (parseMeetingError(error).meetScopeMissing) {
      return (
        <button className={styles['presence-reconnect']} onClick={onNeedsReconnect}>
          <AlertTriangle size={12} /> Reconecta Google para ver quién está dentro
        </button>
      )
    }
    return null
  }

  if (!data?.live) {
    return <span className={styles['presence-empty']}><Users size={12} /> Sala vacía</span>
  }

  return (
    <span className={styles['presence-live']} title={data.names?.join(', ')}>
      <span className={styles['presence-dot']} />
      {data.active} {data.active === 1 ? 'persona conectada' : 'personas conectadas'}
    </span>
  )
}

/**
 * MeetLinkField muestra el enlace de la sala dentro del formulario. Es de solo
 * lectura: la sala la crea Google, no se escribe a mano. En una sesión nueva
 * todavía no existe, así que el campo explica cuándo aparecerá en vez de quedarse
 * vacío sin más.
 */
function MeetLinkField({ url }: { url?: string }) {
  const { copied, copy } = useCopyToClipboard()

  return (
    <>
      <label className={styles['field-label']}>Enlace de Meet</label>
      <div className={styles['link-field']}>
        <input
          className={styles['link-input']}
          value={url ?? ''}
          readOnly
          placeholder="Google generará la sala al crear la sesión"
          onFocus={e => e.currentTarget.select()}
        />
        {url && (
          <button className={styles['btn-ghost']} onClick={() => copy(url)} type="button">
            {copied ? <Check size={14} /> : <Copy size={14} />}
          </button>
        )}
      </div>
      {url && (
        <span className={styles['hint']}>
          El enlace no cambia aunque muevas la fecha o la hora.
        </span>
      )}
    </>
  )
}

/**
 * Convierte un valor de <input type="datetime-local"> (hora local sin zona) al
 * instante real. El navegador ya lo interpreta en la zona del equipo, que es la
 * misma que mandamos como time_zone, así que no hay conversión manual.
 */
function localToISO(value: string) {
  return value ? new Date(value).toISOString() : ''
}

/** Y la vuelta: instante → el formato que espera datetime-local. */
function isoToLocal(iso: string) {
  const d = new Date(iso)
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}

export default function Meetings() {
  const qc = useQueryClient()
  const { user } = useAuth()
  const google = useGoogleConnection(RETURN_TO)
  const [tab, setTab] = useState<'upcoming' | 'past'>('upcoming')
  const [formOpen, setFormOpen] = useState(false)
  const [editing, setEditing] = useState<Meeting | null>(null)

  const { data: meetings = [], isLoading, isError } = useQuery({
    queryKey: ['meetings', tab],
    queryFn: () => meetingService.list({ past: tab === 'past' }),
    // Sin cuenta conectada no hay nada que listar todavía, pero el backend
    // responde igual (una lista vacía), así que no hace falta condicionar.
    enabled: google.enabled,
  })

  const refresh = () => qc.invalidateQueries({ queryKey: ['meetings'] })

  // Con la integración apagada en el servidor el módulo no tiene sentido: no se
  // puede crear ninguna sesión y ofrecer el botón solo llevaría a un error.
  if (google.loading) {
    return <div className={styles['page']}><p className={styles['state-msg']}>Cargando…</p></div>
  }
  if (!google.enabled) {
    return (
      <div className={styles['page']}>
        <div className={styles['header']}><h1><Video size={22} /> Sesiones</h1></div>
        <p className={styles['state-msg']}>
          La integración con Google no está disponible en este entorno.
        </p>
      </div>
    )
  }

  return (
    <div className={styles['page']}>
      <div className={styles['header']}>
        <h1><Video size={22} /> Sesiones</h1>
        {google.connected && !google.needsReauth && (
          <button className={styles['btn']} onClick={() => { setEditing(null); setFormOpen(true) }}>
            <Plus size={16} /> Nueva sesión
          </button>
        )}
      </div>

      {google.banner && (
        <div className={`${styles['banner']} ${styles[`banner-${google.banner.type}`]}`}>
          {google.banner.text}
        </div>
      )}

      {!google.connected ? (
        <ConnectPrompt busy={google.busy} onConnect={google.connect} />
      ) : (
        <>
          {google.needsReauth && (
            <div className={`${styles['banner']} ${styles['banner-warning']}`}>
              <AlertTriangle size={14} style={{ verticalAlign: '-2px', marginRight: 6 }} />
              Perdimos el acceso a tu cuenta de Google, así que no puedes convocar sesiones.{' '}
              <button className={styles['btn-ghost']} style={{ padding: '2px 8px', marginLeft: 6 }} onClick={google.connect}>
                Reconectar
              </button>
            </div>
          )}

          <div className={styles['tabs']}>
            <button
              className={`${styles['tab']} ${tab === 'upcoming' ? styles['tab-active'] : ''}`}
              onClick={() => setTab('upcoming')}
            >
              Próximas
            </button>
            <button
              className={`${styles['tab']} ${tab === 'past' ? styles['tab-active'] : ''}`}
              onClick={() => setTab('past')}
            >
              Pasadas
            </button>
          </div>

          {isLoading && <p className={styles['state-msg']}>Cargando sesiones…</p>}
          {isError && <p className={styles['state-msg']}>No se pudieron cargar las sesiones.</p>}
          {!isLoading && !isError && meetings.length === 0 && (
            <p className={styles['state-msg']}>
              {tab === 'upcoming'
                ? 'No tienes sesiones próximas. Convoca la primera con “Nueva sesión”.'
                : 'Todavía no hay sesiones pasadas.'}
            </p>
          )}

          <div className={styles['grid']}>
            {meetings.map(meeting => (
              <MeetingCard
                key={meeting.id}
                meeting={meeting}
                isOrganizer={meeting.organizer_id === user?.id}
                onEdit={() => { setEditing(meeting); setFormOpen(true) }}
                onChanged={refresh}
                onNeedsReconnect={google.connect}
              />
            ))}
          </div>
        </>
      )}

      {formOpen && (
        <MeetingFormModal
          meeting={editing}
          onClose={() => { setFormOpen(false); setEditing(null) }}
          onSaved={() => { setFormOpen(false); setEditing(null); refresh() }}
          onNeedsGoogle={google.connect}
        />
      )}
    </div>
  )
}

/**
 * ConnectPrompt es el gate: lo que ve quien todavía no vinculó su Google. Se
 * explica el módulo en vez de mostrar un error, porque para la mayoría de la
 * gente esta pantalla es el primer sitio donde se entera de que la integración
 * existe (el panel de Perfil casi nadie lo visita).
 */
function ConnectPrompt({ busy, onConnect }: { busy: boolean; onConnect: () => void }) {
  return (
    <div className={styles['connect-card']}>
      <span className={styles['connect-icon']}><Video size={26} /></span>
      <h2>Convoca reuniones con Google Meet</h2>
      <p>
        Conecta tu cuenta de Google y podrás crear sesiones con sala de Meet desde
        Obertrack: se agendan en tu calendario, los invitados reciben el aviso aquí
        y por correo, y el enlace queda a un clic. Funciona con cualquier cuenta de
        Gmail o de dominio propio.
      </p>
      <button className={styles['btn']} onClick={onConnect} disabled={busy}>
        <Link2 size={16} /> {busy ? 'Conectando…' : 'Conectar con Google'}
      </button>
    </div>
  )
}

function MeetingCard({
  meeting,
  isOrganizer,
  onEdit,
  onChanged,
  onNeedsReconnect,
}: {
  meeting: Meeting
  isOrganizer: boolean
  onEdit: () => void
  onChanged: () => void
  onNeedsReconnect: () => void
}) {
  const confirm = useConfirm()
  const { copied, copy } = useCopyToClipboard()
  const { day, range } = formatWhen(meeting)
  const cancelled = meeting.status === 'cancelled'
  const attendees = meeting.attendees ?? []

  const cancelMutation = useMutation({
    mutationFn: () => meetingService.cancel(meeting.id),
    onSuccess: onChanged,
  })

  const handleCancel = async () => {
    const ok = await confirm({
      title: 'Cancelar sesión',
      message: 'Se borrará el evento del calendario y se avisará a los invitados.',
      confirmLabel: 'Cancelar sesión',
      cancelLabel: 'Volver',
      variant: 'danger',
    })
    if (ok) cancelMutation.mutate()
  }

  return (
    <div className={`${styles['card']} ${cancelled ? styles['card-cancelled'] : ''}`}>
      <div className={styles['card-head']}>
        <h3 className={styles['card-title']}>{meeting.title}</h3>
        {cancelled && <span className={`${styles['badge']} ${styles['badge-cancelled']}`}>Cancelada</span>}
        {!cancelled && startsSoon(meeting) && (
          <span className={`${styles['badge']} ${styles['badge-soon']}`}>Ahora</span>
        )}
      </div>

      <div className={styles['card-meta']}>
        <span><Calendar size={13} /> {day}</span>
        <span><Clock size={13} /> {range}</span>
        {meeting.recurrence_rule && (
          <span className={`${styles['badge']} ${styles['badge-recurring']}`}>
            <Repeat size={11} style={{ verticalAlign: '-1px' }} /> {recurrenceLabel(meeting.recurrence_rule) ?? 'Se repite'}
          </span>
        )}
      </div>

      {meeting.description && <p className={styles['hint']}>{meeting.description}</p>}

      <div className={styles['avatars']}>
        <Users size={13} />
        {attendees.length === 0
          ? 'Sin invitados'
          : `${attendees.length} invitado${attendees.length === 1 ? '' : 's'}`}
        {meeting.organizer && ` · organiza ${meeting.organizer.name}`}
      </div>

      {!cancelled && meeting.meet_url && (
        <div className={styles['meet-link']}>
          <Video size={13} />
          <a href={meeting.meet_url} target="_blank" rel="noopener noreferrer" title={meeting.meet_url}>
            {displayMeetURL(meeting.meet_url)}
          </a>
        </div>
      )}

      <LivePresence meeting={meeting} onNeedsReconnect={onNeedsReconnect} />

      {!cancelled && (
        <div className={styles['card-actions']}>
          {meeting.meet_url ? (
            <>
              <a
                className={`${styles['btn']} ${styles['btn-join']} ${styles['btn-sm']}`}
                href={meeting.meet_url}
                target="_blank"
                rel="noopener noreferrer"
              >
                <Video size={14} /> Unirse
              </a>
              <button
                className={`${styles['btn-ghost']} ${styles['btn-sm']}`}
                onClick={() => copy(meeting.meet_url)}
              >
                {copied ? <Check size={14} /> : <Copy size={14} />} {copied ? 'Copiado' : 'Copiar enlace'}
              </button>
            </>
          ) : (
            <span className={styles['hint']}>Google todavía está generando la sala…</span>
          )}
          {isOrganizer && (
            <>
              <button className={`${styles['btn-ghost']} ${styles['btn-sm']}`} onClick={onEdit}>
                <Pencil size={14} /> Editar
              </button>
              <button
                className={`${styles['btn']} ${styles['btn-danger']} ${styles['btn-sm']}`}
                onClick={handleCancel}
                disabled={cancelMutation.isPending}
              >
                <Trash2 size={14} /> Cancelar
              </button>
            </>
          )}
        </div>
      )}
    </div>
  )
}

function MeetingFormModal({
  meeting,
  onClose,
  onSaved,
  onNeedsGoogle,
}: {
  meeting: Meeting | null
  onClose: () => void
  onSaved: () => void
  onNeedsGoogle: () => void
}) {
  const { user } = useAuth()
  const isEdit = !!meeting

  const [title, setTitle] = useState(meeting?.title ?? '')
  const [description, setDescription] = useState(meeting?.description ?? '')
  const [startAt, setStartAt] = useState(() =>
    meeting ? isoToLocal(meeting.start_at) : defaultStart()
  )
  const [endAt, setEndAt] = useState(() => (meeting ? isoToLocal(meeting.end_at) : defaultEnd()))
  const initialRule = parseRRULE(meeting?.recurrence_rule)
  const [freq, setFreq] = useState(initialRule.freq)
  const [seriesEnd, setSeriesEnd] = useState<SeriesEnd>(initialRule.end)
  const [until, setUntil] = useState(initialRule.until)
  const [count, setCount] = useState(initialRule.count)
  const [search, setSearch] = useState('')
  const [externalEmail, setExternalEmail] = useState('')
  const [error, setError] = useState('')

  // Invitados ya elegidos, separados por origen: los internos viajan como ids
  // (así el backend los avisa por campanita y DM) y los externos como correos.
  const [internal, setInternal] = useState<{ id: number; name: string }[]>(
    () => (meeting?.attendees ?? [])
      .filter(a => a.user_id)
      .map(a => ({ id: a.user_id as number, name: a.name || a.email }))
  )
  const [external, setExternal] = useState<string[]>(
    () => (meeting?.attendees ?? []).filter(a => !a.user_id).map(a => a.email)
  )

  const { data: usersPage } = useQuery({
    queryKey: ['meeting-users', search],
    queryFn: () => userService.getAll({ q: search || undefined, limit: 20 }),
  })

  const candidates = useMemo(() => {
    const chosen = new Set(internal.map(i => i.id))
    return (usersPage?.data ?? []).filter(u => u.id !== user?.id && !chosen.has(u.id))
  }, [usersPage, internal, user?.id])

  const mutation = useMutation({
    mutationFn: (payload: MeetingPayload) =>
      isEdit ? meetingService.update(meeting!.id, payload) : meetingService.create(payload),
    onSuccess: onSaved,
    onError: (err) => {
      const parsed = parseMeetingError(err)
      setError(parsed.message)
      // Si el problema es la cuenta de Google, se lleva al usuario al flujo de
      // conexión en vez de dejarlo releyendo un mensaje que no puede accionar.
      if (parsed.googleNotConnected || parsed.needsReauth) onNeedsGoogle()
    },
  })

  const addExternal = () => {
    const email = externalEmail.trim().toLowerCase()
    if (!email) return
    if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) {
      setError(`El correo "${email}" no es válido.`)
      return
    }
    if (!external.includes(email)) setExternal([...external, email])
    setExternalEmail('')
    setError('')
  }

  const submit = () => {
    setError('')
    if (!title.trim()) return setError('Ponle un título a la sesión.')
    if (!startAt || !endAt) return setError('Indica cuándo empieza y cuándo termina.')
    if (new Date(endAt) <= new Date(startAt)) {
      return setError('La hora de fin debe ser posterior a la de inicio.')
    }
    if (internal.length === 0 && external.length === 0) {
      return setError('Invita al menos a una persona.')
    }
    if (freq && seriesEnd === 'on' && !until) {
      return setError('Indica hasta qué fecha se repite.')
    }
    if (freq && seriesEnd === 'on' && new Date(`${until}T23:59:59`) < new Date(startAt)) {
      return setError('La repetición no puede terminar antes de la primera reunión.')
    }
    if (freq && seriesEnd === 'after' && (!count || count < 1)) {
      return setError('Indica cuántas veces se repite.')
    }

    mutation.mutate({
      title: title.trim(),
      description: description.trim(),
      start_at: localToISO(startAt),
      end_at: localToISO(endAt),
      time_zone: BROWSER_TZ,
      attendee_user_ids: internal.map(i => i.id),
      attendee_emails: external,
      recurrence_rule: buildRRULE(freq, seriesEnd, until, count),
      task_id: meeting?.task_id,
    })
  }

  return (
    <div className={styles['modal-overlay']} onClick={onClose}>
      <div className={styles['modal']} onClick={e => e.stopPropagation()}>
        <div className={styles['modal-head']}>
          <h3><Video size={18} /> {isEdit ? 'Editar sesión' : 'Nueva sesión'}</h3>
          <button className={styles['icon-btn']} onClick={onClose}><X size={16} /></button>
        </div>

        <div className={styles['modal-body']}>
          <label className={styles['field-label']}>Título</label>
          <input
            className={styles['field-input']}
            value={title}
            onChange={e => setTitle(e.target.value)}
            placeholder="Seguimiento semanal"
          />

          <label className={styles['field-label']}>Descripción (opcional)</label>
          <textarea
            className={styles['field-textarea']}
            rows={2}
            value={description}
            onChange={e => setDescription(e.target.value)}
          />

          <div className={styles['field-grid']}>
            <div>
              <label className={styles['field-label']}>Empieza</label>
              <input
                type="datetime-local"
                className={styles['field-input']}
                value={startAt}
                onChange={e => setStartAt(e.target.value)}
              />
            </div>
            <div>
              <label className={styles['field-label']}>Termina</label>
              <input
                type="datetime-local"
                className={styles['field-input']}
                value={endAt}
                onChange={e => setEndAt(e.target.value)}
              />
            </div>
          </div>
          <span className={styles['hint']}>Zona horaria: {BROWSER_TZ}</span>

          <label className={styles['field-label']}>Repetición</label>
          <select
            className={styles['field-select']}
            value={freq}
            onChange={e => setFreq(e.target.value)}
          >
            {FREQ_OPTIONS.map(o => (
              <option key={o.value} value={o.value}>{o.label}</option>
            ))}
          </select>

          {freq && (
            <>
              <label className={styles['field-label']}>Termina</label>
              <div className={styles['field-grid']}>
                <select
                  className={styles['field-select']}
                  value={seriesEnd}
                  onChange={e => setSeriesEnd(e.target.value as SeriesEnd)}
                >
                  <option value="never">Nunca</option>
                  <option value="on">En una fecha</option>
                  <option value="after">Tras varias veces</option>
                </select>
                {seriesEnd === 'on' && (
                  <input
                    type="date"
                    className={styles['field-input']}
                    value={until}
                    onChange={e => setUntil(e.target.value)}
                  />
                )}
                {seriesEnd === 'after' && (
                  <input
                    type="number"
                    min={1}
                    max={365}
                    className={styles['field-input']}
                    value={count}
                    onChange={e => setCount(Number(e.target.value))}
                  />
                )}
              </div>
              <span className={styles['hint']}>
                Toda la serie comparte el mismo enlace de Meet. Al editar o cancelar se
                modifica la serie completa, no una fecha suelta.
              </span>
            </>
          )}

          <MeetLinkField url={meeting?.meet_url} />

          <label className={styles['field-label']}>Invitados</label>
          {(internal.length > 0 || external.length > 0) && (
            <div className={styles['chips']}>
              {internal.map(i => (
                <span key={`u${i.id}`} className={styles['chip']}>
                  {i.name}
                  <button onClick={() => setInternal(internal.filter(x => x.id !== i.id))}>
                    <X size={12} />
                  </button>
                </span>
              ))}
              {external.map(email => (
                <span key={email} className={`${styles['chip']} ${styles['chip-external']}`}>
                  {email}
                  <button onClick={() => setExternal(external.filter(x => x !== email))}>
                    <X size={12} />
                  </button>
                </span>
              ))}
            </div>
          )}

          <input
            className={styles['field-input']}
            placeholder="Buscar compañeros por nombre…"
            value={search}
            onChange={e => setSearch(e.target.value)}
          />
          {candidates.length > 0 && (
            <div className={styles['picker']}>
              {candidates.map(u => (
                <button
                  key={u.id}
                  className={styles['picker-item']}
                  onClick={() => {
                    setInternal([...internal, { id: u.id, name: u.name }])
                    setSearch('')
                  }}
                >
                  <span>{u.name}</span>
                  <span className={styles['picker-email']}>{u.email}</span>
                </button>
              ))}
            </div>
          )}

          <div className={styles['field-grid']}>
            <input
              className={styles['field-input']}
              placeholder="correo@externo.com"
              value={externalEmail}
              onChange={e => setExternalEmail(e.target.value)}
              onKeyDown={e => { if (e.key === 'Enter') { e.preventDefault(); addExternal() } }}
            />
            <button className={styles['btn-ghost']} onClick={addExternal}>
              <Plus size={14} /> Añadir externo
            </button>
          </div>
          {external.length > 0 && (
            <span className={styles['hint']}>
              Google enviará la invitación por correo a los invitados externos desde tu cuenta.
            </span>
          )}

          {error && <p className={styles['error-text']}>{error}</p>}
        </div>

        <div className={styles['modal-actions']}>
          <button className={styles['btn-ghost']} onClick={onClose}>Cancelar</button>
          <button className={styles['btn']} onClick={submit} disabled={mutation.isPending}>
            {mutation.isPending ? 'Guardando…' : isEdit ? 'Guardar cambios' : 'Crear sesión'}
          </button>
        </div>
      </div>
    </div>
  )
}

/** Por defecto, la próxima hora en punto: el hueco más probable. */
function defaultStart() {
  const d = new Date()
  d.setMinutes(0, 0, 0)
  d.setHours(d.getHours() + 1)
  return isoToLocal(d.toISOString())
}

function defaultEnd() {
  const d = new Date()
  d.setMinutes(0, 0, 0)
  d.setHours(d.getHours() + 2)
  return isoToLocal(d.toISOString())
}
