import { useMemo } from 'react'
import { Bar, BarChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts'

import type { ProgramSend, ProgramSendStats } from '../../services/induction.service'
import styles from './InductionSettings.module.css'

interface Props {
  stats: ProgramSendStats
  sends: ProgramSend[]
}

// Mismos tonos que las etiquetas de estado de la lista: el color acompaña a la
// etiqueta y a la cifra, nunca va solo.
const STATUS_PARTS: { key: keyof ProgramSendStats; label: string; color: string }[] = [
  { key: 'passed', label: 'Aprobadas', color: '#16a34a' },
  { key: 'pending', label: 'En curso', color: '#f59e0b' },
  { key: 'blocked', label: 'Bloqueados', color: '#dc2626' },
  { key: 'expired', label: 'Enlace vencido', color: '#94a3b8' },
]

const BRAND = '#a21caf'
const DAY = 24 * 60 * 60 * 1000

const shortFmt = new Intl.DateTimeFormat('es', { day: 'numeric', month: 'short' })
const monthFmt = new Intl.DateTimeFormat('es', { month: 'short', year: '2-digit' })

/** Lunes (local) de la semana de la fecha. */
function weekStart(d: Date) {
  const start = new Date(d.getFullYear(), d.getMonth(), d.getDate())
  start.setDate(start.getDate() - ((start.getDay() + 6) % 7))
  return start
}

interface Bucket {
  label: string
  title: string
  sent: number
  passed: number
}

/**
 * Envíos por semana desde el primero hasta hoy, con las semanas vacías en 0
 * para que el ritmo se vea tal cual. Pasadas 26 semanas se agrupa por mes.
 */
function buildTimeline(sends: ProgramSend[]): Bucket[] {
  if (sends.length === 0) return []
  const dates = sends.map((s) => new Date(s.sent_at))
  const first = new Date(Math.min(...dates.map((d) => d.getTime())))
  const now = new Date()
  const byMonth = (now.getTime() - first.getTime()) / DAY > 26 * 7

  const buckets: Bucket[] = []
  const index = new Map<string, Bucket>()
  const keyOf = (d: Date) =>
    byMonth ? `${d.getFullYear()}-${d.getMonth()}` : weekStart(d).toISOString().slice(0, 10)

  if (byMonth) {
    for (let d = new Date(first.getFullYear(), first.getMonth(), 1); d <= now; d.setMonth(d.getMonth() + 1)) {
      const b = { label: monthFmt.format(d), title: monthFmt.format(d), sent: 0, passed: 0 }
      buckets.push(b)
      index.set(keyOf(d), b)
    }
  } else {
    for (let d = weekStart(first); d <= now; d = new Date(d.getTime() + 7 * DAY)) {
      const b = { label: shortFmt.format(d), title: `Semana del ${shortFmt.format(d)}`, sent: 0, passed: 0 }
      buckets.push(b)
      index.set(keyOf(d), b)
    }
  }
  sends.forEach((s, i) => {
    const b = index.get(keyOf(dates[i]))
    if (!b) return
    b.sent++
    if (s.status === 'passed') b.passed++
  })
  return buckets
}

function TimelineTooltip({ active, payload }: { active?: boolean; payload?: { payload: Bucket }[] }) {
  if (!active || !payload?.length) return null
  const b = payload[0].payload
  return (
    <div className={styles.chartTooltip}>
      <strong>{b.title}</strong>
      <span>
        {b.sent} {b.sent === 1 ? 'envío' : 'envíos'}
        {b.sent > 0 && ` · ${b.passed} ${b.passed === 1 ? 'aprobado' : 'aprobados'} hasta hoy`}
      </span>
    </div>
  )
}

/**
 * Gráficos de los envíos de un programa: cómo se reparten por estado, cuántos
 * avanzan (enviados → empezaron → aprobaron) y el ritmo de envíos en el tiempo.
 * Siempre sobre todos los envíos, no sobre el filtro de la lista.
 */
export default function ProgramSendsCharts({ stats, sends }: Props) {
  const timeline = useMemo(() => buildTimeline(sends), [sends])
  const started = useMemo(
    () => sends.filter((s) => s.blocks_passed > 0 || s.status === 'passed' || s.status === 'blocked').length,
    [sends]
  )
  const funnel = [
    { label: 'Enviados', value: stats.sent },
    { label: 'Empezaron', value: started, hint: 'aprobaron al menos un bloque o agotaron sus intentos' },
    { label: 'Aprobaron', value: stats.passed },
  ]
  const pct = (n: number) => (stats.sent > 0 ? Math.round((n / stats.sent) * 100) : 0)
  const parts = STATUS_PARTS.map((p) => ({ ...p, value: stats[p.key] as number })).filter((p) => p.value > 0)

  return (
    <div className={styles.chartGrid}>
      <section className={styles.chartCard}>
        <h4 className={styles.chartTitle}>Estado de los envíos</h4>
        <div className={styles.stackBar} role="img" aria-label={parts.map((p) => `${p.label}: ${p.value}`).join(', ')}>
          {parts.map((p) => (
            <span
              key={p.key}
              className={styles.stackSeg}
              style={{ flexGrow: p.value, background: p.color }}
              title={`${p.label}: ${p.value} (${pct(p.value)}%)`}
            />
          ))}
        </div>
        <ul className={styles.chartLegend}>
          {STATUS_PARTS.map((p) => (
            <li key={p.key}>
              <span className={styles.legendDot} style={{ background: p.color }} />
              {p.label}
              <strong>{stats[p.key] as number}</strong>
              <span className={styles.legendPct}>{pct(stats[p.key] as number)}%</span>
            </li>
          ))}
        </ul>
      </section>

      <section className={styles.chartCard}>
        <h4 className={styles.chartTitle}>Avance</h4>
        <div className={styles.funnel}>
          {funnel.map((f) => (
            <div key={f.label} className={styles.funnelRow} title={f.hint}>
              <span className={styles.funnelLabel}>{f.label}</span>
              <span className={styles.funnelTrack}>
                <span className={styles.funnelBar} style={{ width: `${Math.max(pct(f.value), f.value > 0 ? 2 : 0)}%` }} />
              </span>
              <span className={styles.funnelValue}>
                {f.value} <small>{pct(f.value)}%</small>
              </span>
            </div>
          ))}
        </div>
      </section>

      <section className={`${styles.chartCard} ${styles.chartWide}`}>
        <h4 className={styles.chartTitle}>Envíos en el tiempo</h4>
        <ResponsiveContainer width="100%" height={180}>
          <BarChart data={timeline} margin={{ top: 8, right: 4, bottom: 0, left: -24 }} barCategoryGap={2}>
            <CartesianGrid strokeDasharray="3 3" vertical={false} stroke="#f1f5f9" />
            <XAxis
              dataKey="label"
              axisLine={false}
              tickLine={false}
              tick={{ fill: '#64748b', fontSize: 11 }}
              interval="preserveStartEnd"
              minTickGap={12}
            />
            <YAxis allowDecimals={false} axisLine={false} tickLine={false} tick={{ fill: '#64748b', fontSize: 11 }} />
            <Tooltip content={<TimelineTooltip />} cursor={{ fill: 'rgba(162, 28, 175, 0.06)' }} />
            <Bar dataKey="sent" name="Envíos" fill={BRAND} radius={[4, 4, 0, 0]} maxBarSize={28} />
          </BarChart>
        </ResponsiveContainer>
      </section>
    </div>
  )
}
