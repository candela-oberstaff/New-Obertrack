import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { LifeBuoy, CheckCircle2, MessageCircle, Clock, Plus, Inbox, RotateCcw } from 'lucide-react'
import { channelService } from '../services/api'
import { useNotification } from '../context/NotificationContext'
import { Select } from '../components/ui/Select'
import type { MySupportTicket, SupportStatus } from '../types/chat'
import styles from './Soporte.module.css'

const PRIORITY_OPTIONS = [
  { value: 'Baja', label: 'Baja' },
  { value: 'Media', label: 'Media' },
  { value: 'Alta', label: 'Alta' },
]

const MODULE_OPTIONS = [
  { value: 'Tareas', label: 'Tareas' },
  { value: 'Horas', label: 'Horas' },
  { value: 'Reportes', label: 'Reportes' },
  { value: 'Chat', label: 'Chat' },
  { value: 'Profesionales/Empresas', label: 'Profesionales / Empresas' },
  { value: 'Perfil', label: 'Perfil' },
  { value: 'Tutoriales', label: 'Tutoriales' },
  { value: 'Otro', label: 'Otro' },
]

const STATUS_META: Record<SupportStatus, { label: string; className: string }> = {
  open: { label: 'Abierto', className: styles.badgeOpen },
  assigned: { label: 'En atención', className: styles.badgeAssigned },
  resolved: { label: 'Resuelto', className: styles.badgeResolved },
}

const PRIORITY_META: Record<string, string> = {
  Alta: styles.prioHigh,
  Media: styles.prioMed,
  Baja: styles.prioLow,
}

const STEPS: { key: SupportStatus; label: string }[] = [
  { key: 'open', label: 'Enviada' },
  { key: 'assigned', label: 'En atención' },
  { key: 'resolved', label: 'Resuelta' },
]

function timeAgo(iso?: string): string {
  if (!iso) return ''
  const diff = Date.now() - new Date(iso).getTime()
  if (diff < 0) return 'ahora'
  const mins = Math.floor(diff / 60000)
  if (mins < 1) return 'ahora'
  if (mins < 60) return `hace ${mins} min`
  const hours = Math.floor(mins / 60)
  if (hours < 24) return `hace ${hours} h`
  const days = Math.floor(hours / 24)
  return `hace ${days} d`
}

export default function Soporte() {
  const navigate = useNavigate()
  const { error: showError, success: showSuccess } = useNotification()
  const queryClient = useQueryClient()
  const [subject, setSubject] = useState('')
  const [description, setDescription] = useState('')
  const [priority, setPriority] = useState<string>('Media')
  const [module, setModule] = useState<string>('')
  const [moduleOther, setModuleOther] = useState('')
  const [showForm, setShowForm] = useState(false)

  const resolvedModule = module === 'Otro' ? moduleOther.trim() : module
  const moduleValid = module !== '' && (module !== 'Otro' || moduleOther.trim() !== '')

  const { data: tickets = [], isLoading } = useQuery({
    queryKey: ['my-support-tickets'],
    queryFn: () => channelService.getMySupportTickets(),
    refetchInterval: 30_000,
  })

  const mutation = useMutation({
    mutationFn: () =>
      channelService.contactSupport({ subject: subject.trim(), message: description.trim(), priority, module: resolvedModule, new: true }),
    onSuccess: (channel) => {
      setSubject('')
      setDescription('')
      setPriority('Media')
      setModule('')
      setModuleOther('')
      setShowForm(false)
      queryClient.invalidateQueries({ queryKey: ['my-support-tickets'] })
      showSuccess('Tu solicitud llegó al equipo de soporte.')
      navigate(`/chat?channel=${channel.id}`)
    },
    onError: (e: any) => {
      showError(e?.response?.data?.error || 'No se pudo enviar tu solicitud. Intenta de nuevo.')
    },
  })

  const reopenMutation = useMutation({
    mutationFn: (ticketId: number) => channelService.reopenSupport(ticketId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['my-support-tickets'] })
      showSuccess('Reabrimos tu solicitud.')
    },
    onError: (e: any) => showError(e?.response?.data?.error || 'No se pudo reabrir la solicitud.'),
  })

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (mutation.isPending) return
    if (!subject.trim() || !description.trim() || !moduleValid) return
    mutation.mutate()
  }

  const hasTickets = tickets.length > 0
  const openCount = tickets.filter((t) => t.status !== 'resolved').length
  const resolvedCount = tickets.filter((t) => t.status === 'resolved').length

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <h1>
          <LifeBuoy size={22} />
          Soporte
        </h1>
        {hasTickets && !showForm && (
          <button type="button" className={styles.newBtn} onClick={() => setShowForm(true)}>
            <Plus size={16} />
            Nueva solicitud
          </button>
        )}
      </div>

      {(showForm || !hasTickets) && (
        <>
          <p className={styles.helper}>
            Tu solicitud llega al equipo de soporte y podés seguirla por el chat.
          </p>
          <form className={styles.form} onSubmit={handleSubmit}>
            <label className={styles.field}>
              <span className={styles.label}>Asunto</span>
              <input
                type="text"
                className={styles.input}
                value={subject}
                onChange={(e) => setSubject(e.target.value)}
                placeholder="Resumí tu consulta"
                maxLength={120}
                required
              />
            </label>

            <label className={styles.field}>
              <span className={styles.label}>Descripción</span>
              <textarea
                className={styles.textarea}
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                placeholder="Contanos qué necesitás con el mayor detalle posible"
                rows={6}
                required
              />
            </label>

            <div className={styles.field}>
              <span className={styles.label}>Módulo</span>
              <Select
                options={MODULE_OPTIONS}
                value={module}
                onChange={(v) => setModule(String(v))}
                placeholder="¿Dónde encontraste el problema?"
                fullWidth
              />
              {module === 'Otro' && (
                <input
                  type="text"
                  className={styles.input}
                  value={moduleOther}
                  onChange={(e) => setModuleOther(e.target.value)}
                  placeholder="Especificá el módulo"
                  maxLength={60}
                  style={{ marginTop: 8 }}
                  required
                />
              )}
            </div>

            <div className={styles.field}>
              <span className={styles.label}>Prioridad</span>
              <Select
                options={PRIORITY_OPTIONS}
                value={priority}
                onChange={(v) => setPriority(String(v))}
                fullWidth
              />
            </div>

            <div className={styles.formActions}>
              <button
                type="submit"
                className={styles.primaryBtn}
                disabled={mutation.isPending || !subject.trim() || !description.trim() || !moduleValid}
              >
                {mutation.isPending ? 'Enviando...' : 'Enviar solicitud'}
              </button>
              {hasTickets && (
                <button
                  type="button"
                  className={styles.secondaryBtn}
                  onClick={() => setShowForm(false)}
                  disabled={mutation.isPending}
                >
                  Cancelar
                </button>
              )}
            </div>
          </form>
        </>
      )}

      {hasTickets && (
        <div className={styles.ticketsSection}>
          <div className={styles.sectionHead}>
            <h2 className={styles.sectionTitle}>
              <Inbox size={18} />
              Mis solicitudes
            </h2>
            <span className={styles.summary}>
              {openCount} abierta{openCount === 1 ? '' : 's'} · {resolvedCount} resuelta{resolvedCount === 1 ? '' : 's'}
            </span>
          </div>
          <div className={styles.ticketList}>
            {tickets.map((t) => (
              <TicketRow
                key={t.id}
                ticket={t}
                onOpen={() => navigate(`/chat?channel=${t.channel_id}`)}
                onReopen={() => reopenMutation.mutate(t.id)}
                reopening={reopenMutation.isPending && reopenMutation.variables === t.id}
              />
            ))}
          </div>
        </div>
      )}

      {!hasTickets && !isLoading && !showForm && (
        <div className={styles.empty}>
          <Inbox size={40} className={styles.emptyIcon} />
          <p>Aún no abriste solicitudes de soporte.</p>
        </div>
      )}

      {isLoading && !hasTickets && <p className={styles.helper}>Cargando tus solicitudes...</p>}
    </div>
  )
}

function StatusStepper({ status }: { status: SupportStatus }) {
  const activeIdx = STEPS.findIndex((s) => s.key === status)
  return (
    <div className={styles.stepper}>
      {STEPS.map((step, i) => {
        const done = i <= activeIdx
        return (
          <div key={step.key} className={styles.step}>
            <span className={`${styles.dot} ${done ? styles.dotDone : ''}`} />
            <span className={`${styles.stepLabel} ${i === activeIdx ? styles.stepCurrent : ''}`}>{step.label}</span>
            {i < STEPS.length - 1 && <span className={`${styles.line} ${i < activeIdx ? styles.lineDone : ''}`} />}
          </div>
        )
      })}
    </div>
  )
}

interface TicketRowProps {
  ticket: MySupportTicket
  onOpen: () => void
  onReopen: () => void
  reopening: boolean
}

function TicketRow({ ticket, onOpen, onReopen, reopening }: TicketRowProps) {
  const meta = STATUS_META[ticket.status] ?? STATUS_META.open
  const activity = ticket.last_message_at || ticket.updated_at
  const title = ticket.subject || ticket.last_message || 'Solicitud de soporte'

  return (
    <div className={styles.ticketCard} onClick={onOpen} role="button" tabIndex={0}>
      <div className={styles.ticketTop}>
        <span className={`${styles.badge} ${meta.className}`}>{meta.label}</span>
        {ticket.priority && (
          <span className={`${styles.prio} ${PRIORITY_META[ticket.priority] ?? ''}`}>{ticket.priority}</span>
        )}
        {ticket.module && <span className={styles.moduleChip}>{ticket.module}</span>}
        {ticket.status === 'assigned' && ticket.assignee_name && (
          <span className={styles.assignee}>Atiende {ticket.assignee_name}</span>
        )}
        {ticket.unread_count > 0 && (
          <span className={styles.unread}>
            {ticket.unread_count} nuevo{ticket.unread_count > 1 ? 's' : ''}
          </span>
        )}
      </div>

      <p className={styles.title}>{title}</p>

      <StatusStepper status={ticket.status} />

      <div className={styles.ticketFooter}>
        <span className={styles.meta}>
          <Clock size={12} />
          {activity ? `Actualizado ${timeAgo(activity)}` : ''}
        </span>
        <div className={styles.cardActions} onClick={(e) => e.stopPropagation()}>
          {ticket.status === 'resolved' && (
            <button type="button" className={styles.reopenBtn} onClick={onReopen} disabled={reopening}>
              <RotateCcw size={14} />
              {reopening ? 'Reabriendo...' : 'Reabrir'}
            </button>
          )}
          <button type="button" className={styles.chatBtn} onClick={onOpen}>
            {ticket.status === 'resolved' ? <CheckCircle2 size={14} /> : <MessageCircle size={14} />}
            {ticket.status === 'resolved' ? 'Ver conversación' : 'Continuar en el chat'}
          </button>
        </div>
      </div>
    </div>
  )
}
