import { useEffect, useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { Search } from 'lucide-react'

import { Button, Modal } from '../ui'
import {
  inductionService,
  type InductionProgram,
  type ProgramSend,
  type ProgramSendStats,
} from '../../services/induction.service'
import ProgramSendsCharts from './ProgramSendsCharts'
import styles from './InductionSettings.module.css'

interface Props {
  program: InductionProgram
  onClose: () => void
}

type Filter = 'all' | ProgramSend['status']

const STATUS: Record<ProgramSend['status'], { label: string; className: string }> = {
  pending: { label: 'En curso', className: styles.tagWarn },
  expired: { label: 'Enlace vencido', className: styles.tag },
  passed: { label: 'Aprobada', className: styles.tagOk },
  blocked: { label: 'Bloqueado', className: styles.tagDanger },
}

const dateFmt = new Intl.DateTimeFormat('es', { day: 'numeric', month: 'short', year: 'numeric' })
const formatDate = (iso?: string) => (iso ? dateFmt.format(new Date(iso)) : '')

/**
 * Métricas de envío de un programa: a cuántos y a quiénes se les ha enviado,
 * y cómo va cada envío. Cuenta todas las invitaciones del programa, las del
 * botón «Enviar» y las de ingreso que se emiten solas.
 */
export default function ProgramSendsModal({ program, onClose }: Props) {
  const [data, setData] = useState<{ stats: ProgramSendStats; sends: ProgramSend[] } | null>(null)
  const [failed, setFailed] = useState(false)
  const [filter, setFilter] = useState<Filter>('all')
  const [query, setQuery] = useState('')

  useEffect(() => {
    let cancelled = false
    inductionService
      .programSends(program.id)
      .then((res) => {
        if (!cancelled) setData(res)
      })
      .catch(() => {
        if (!cancelled) setFailed(true)
      })
    return () => {
      cancelled = true
    }
  }, [program.id])

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase()
    return (data?.sends ?? []).filter(
      (s) =>
        (filter === 'all' || s.status === filter) &&
        (!q || `${s.name} ${s.email} ${s.company}`.toLowerCase().includes(q))
    )
  }, [data, filter, query])

  const stats = data?.stats
  const cards: { key: Filter; label: string; value: number; sub?: string }[] = stats
    ? [
        {
          key: 'all',
          label: 'Enviados',
          value: stats.sent,
          sub: `${stats.people} ${stats.people === 1 ? 'persona' : 'personas'}`,
        },
        { key: 'pending', label: 'En curso', value: stats.pending },
        {
          key: 'passed',
          label: 'Aprobadas',
          value: stats.passed,
          sub: stats.sent > 0 ? `${Math.round((stats.passed / stats.sent) * 100)}% de los envíos` : undefined,
        },
        { key: 'blocked', label: 'Bloqueados', value: stats.blocked },
        { key: 'expired', label: 'Enlace vencido', value: stats.expired },
      ]
    : []

  return (
    <Modal
      isOpen
      onClose={onClose}
      title={`Envíos de «${program.name}»`}
      size="lg"
      footer={<Button onClick={onClose}>Cerrar</Button>}
    >
      {failed ? (
        <div className={styles.empty}>No pudimos cargar los envíos. Inténtalo de nuevo.</div>
      ) : !data || !stats ? (
        <p className={styles.muted}>Cargando envíos...</p>
      ) : stats.sent === 0 ? (
        <div className={styles.empty}>Este programa todavía no se le ha enviado a nadie.</div>
      ) : (
        <>
          <div className={styles.sendStats}>
            {cards.map((c) => (
              <button
                key={c.key}
                type="button"
                className={filter === c.key ? styles.sendStatOn : styles.sendStat}
                aria-pressed={filter === c.key}
                onClick={() => setFilter(c.key)}
              >
                <span className={styles.sendStatValue}>{c.value}</span>
                <span className={styles.sendStatLabel}>{c.label}</span>
                {c.sub && <span className={styles.sendStatSub}>{c.sub}</span>}
              </button>
            ))}
          </div>
          <p className={styles.hint} style={{ margin: '0 0 12px' }}>
            Último envío: {formatDate(stats.last_sent_at)}. Una persona cuenta una vez por cada envío que recibió.
          </p>

          <ProgramSendsCharts stats={stats} sends={data.sends} />

          <div className={styles.pickToolbar}>
            <div className={styles.videoSearch} style={{ margin: 0, flex: 1, maxWidth: 360 }}>
              <Search size={15} />
              <input
                type="text"
                placeholder="Buscar por nombre, correo o empresa..."
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                aria-label="Buscar envío"
              />
            </div>
            <span className={styles.muted} style={{ fontSize: 13 }}>
              {filtered.length} de {stats.sent} envíos
            </span>
          </div>

          <div className={styles.sendList}>
            {filtered.map((s) => (
              <div key={s.invite_id} className={styles.sendItem}>
                <span className={styles.sendInfo}>
                  <span className={styles.pickName}>
                    {s.name ? (
                      <Link to={`/admin/users/${s.user_id}`} className={styles.sendLink}>
                        {s.name}
                      </Link>
                    ) : (
                      `Usuario ${s.user_id}`
                    )}
                  </span>
                  <span className={styles.pickSub}>
                    {[s.email, s.company || 'Sin empresa'].filter(Boolean).join(' · ')}
                  </span>
                  <span className={styles.pickSub}>
                    Enviado el {formatDate(s.sent_at)} · {s.gates_access ? 'Ingreso' : 'Capacitación'} ·{' '}
                    {s.blocks_passed}/{s.blocks_total} bloques
                    {s.completed_at && ` · terminó el ${formatDate(s.completed_at)}`}
                  </span>
                </span>
                <span className={STATUS[s.status].className}>{STATUS[s.status].label}</span>
              </div>
            ))}
            {filtered.length === 0 && <p className={styles.hint}>Ningún envío coincide con el filtro.</p>}
          </div>
        </>
      )}
    </Modal>
  )
}
