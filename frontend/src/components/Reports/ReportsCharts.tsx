import { useState } from 'react'
import { ResponsiveContainer, BarChart, CartesianGrid, XAxis, YAxis, Tooltip, Bar } from 'recharts'
import { FileText, Clock, CheckCircle2, AlertCircle, XCircle } from 'lucide-react'
import type { WorkHour, User } from '../../types'
import CommonTooltip from '../Common/Tooltip'
import { Modal } from '../ui'
import styles from '../../pages/Reports.module.css'

interface ReportsChartsProps {
  dailyData: any[]
  workHours: WorkHour[]
  user: User | null
}

function decodeEntities(s: string) {
  return s
    .replace(/&nbsp;/gi, ' ')
    .replace(/&amp;/gi, '&')
    .replace(/&lt;/gi, '<')
    .replace(/&gt;/gi, '>')
    .replace(/&quot;/gi, '"')
    .replace(/&#0?39;|&apos;/gi, "'")
}

function toPlain(html?: string) {
  if (!html) return ''
  const withBreaks = html
    .replace(/<br\s*\/?>/gi, '\n')
    .replace(/<li[^>]*>/gi, '\n• ')
    .replace(/<\/(p|div|li|h[1-6]|tr|ul|ol|blockquote)>/gi, '\n')
    .replace(/<[^>]+>/g, '')
  return decodeEntities(withBreaks)
    .replace(/([a-záéíóúñ])([A-ZÁÉÍÓÚÑ])/g, '$1 $2')
    .replace(/[ \t\f\v]+/g, ' ')
    .replace(/[ \t]*\n[ \t]*/g, '\n')
    .replace(/\n{3,}/g, '\n\n')
    .trim()
}

const typeLabel = (t: string) => (t === 'complete' ? 'Jornada Completa' : t === 'recover' ? 'Recuperación' : 'Ausencia')

function approvalStatus(wh: WorkHour) {
  if (wh.approved) return { label: 'Aprobada', cls: 'approved', Icon: CheckCircle2 }
  if (wh.rejected) return { label: 'Rechazada', cls: 'rejected', Icon: XCircle }
  return { label: 'Pendiente', cls: 'pending', Icon: AlertCircle }
}

export function ReportsCharts({ dailyData, workHours, user }: ReportsChartsProps) {
  const isProf = user?.user_type === 'profesional'
  const [selected, setSelected] = useState<WorkHour | null>(null)

  return (
    <div className={styles['charts-row'] || 'charts-row'}>
      {workHours.some(wh => wh.activities) && (
        <div className={styles['activities-section']}>
          <h3><FileText size={20} style={{ verticalAlign: 'middle', marginRight: '8px' }} /> Actividades Registradas</h3>
          <div className={styles['activities-list']}>
            {workHours.filter(wh => wh.activities).slice(0, 10).map((wh) => {
              const plain = toPlain(wh.activities)
              const isLong = plain.length > 180
              return (
                <div key={wh.id} className={styles['activity-card']}>
                  <div className={styles['activity-date']}>
                    <span className={styles['activity-day']}>{new Date(wh.work_date).getDate()}</span>
                    <span className={styles['activity-month']}>{new Date(wh.work_date).toLocaleDateString('es-ES', { month: 'short' })}</span>
                  </div>
                  <div className={styles['activity-content']}>
                    <p className={styles['activity-text']}>{plain}</p>
                    <div className={styles['activity-footer']}>
                      <span className={`${styles['activity-type']} ${styles[wh.work_type] || wh.work_type}`}>
                        {typeLabel(wh.work_type)}
                      </span>
                      {isLong && (
                        <button type="button" className={styles['activity-more']} onClick={() => setSelected(wh)}>
                          Ver detalle
                        </button>
                      )}
                    </div>
                  </div>
                </div>
              )
            })}
          </div>
        </div>
      )}

      {selected && (
        <Modal
          isOpen
          onClose={() => setSelected(null)}
          size="lg"
          title={
            <span style={{ display: 'inline-flex', alignItems: 'center', gap: 8 }}>
              <FileText size={18} />
              {new Date(selected.work_date).toLocaleDateString('es-ES', { weekday: 'long', day: 'numeric', month: 'long', year: 'numeric' })}
            </span>
          }
        >
          <div className={styles['activity-modal']}>
            <div className={styles['activity-modal-meta']}>
              <span className={`${styles['activity-type']} ${styles[selected.work_type] || selected.work_type}`}>
                {typeLabel(selected.work_type)}
              </span>
              <span className={styles['activity-chip']}>
                <Clock size={14} />
                {selected.work_type === 'absence'
                  ? `${selected.absence_hours ?? 0}h de ausencia`
                  : `${selected.hours_worked}h trabajadas`}
              </span>
              {(() => {
                const st = approvalStatus(selected)
                return (
                  <span className={`${styles['activity-chip']} ${styles[`status-${st.cls}`]}`}>
                    <st.Icon size={14} />
                    {st.label}
                  </span>
                )
              })()}
            </div>

            {selected.work_type === 'absence' && selected.absence_reason && (
              <p className={styles['activity-modal-reason']}>
                <strong>Motivo de ausencia:</strong> {selected.absence_reason}
              </p>
            )}

            <div className={styles['activity-modal-body']}>
              <p className={styles['activity-modal-text']}>{toPlain(selected.activities)}</p>
            </div>
          </div>
        </Modal>
      )}

      <div className={`${styles['chart-card'] || 'chart-card'} ${styles['large'] || 'large'}`}>
        <div className={styles['chart-header'] || 'chart-header'}>
          <h3>
            {isProf ? 'Días Trabajados' : 'Horas Diarias'}{' '}
            {!isProf && (
              <CommonTooltip content="Gráfico de horas diarias que el profesional ha registrado en el mes" size={14} />
            )}
          </h3>
        </div>
        <div className={styles['chart-body'] || 'chart-body'}>
          <ResponsiveContainer width="100%" height={260}>
            <BarChart data={dailyData}>
              <CartesianGrid strokeDasharray="3 3" stroke="#f1f5f9" vertical={false} />
              <XAxis dataKey="day" stroke="#94a3b8" fontSize={12} tickLine={false} axisLine={false} />
              <YAxis stroke="#94a3b8" fontSize={12} tickLine={false} axisLine={false} />
              <Tooltip
                cursor={{ fill: '#f8fafc' }}
                contentStyle={{ borderRadius: 12, border: 'none', boxShadow: '0 10px 15px -3px rgba(0,0,0,0.1)' }}
                formatter={(val) => isProf ? [`${val} Día`, 'Actividad'] : [`${val}h`, 'Horas']}
              />
              <Bar dataKey="value" fill="var(--primary)" radius={[4, 4, 0, 0]} name={isProf ? 'Días' : 'Horas'} />
              <Bar dataKey="target" fill="#e2e8f0" radius={[4, 4, 0, 0]} name="Meta" />
            </BarChart>
          </ResponsiveContainer>
        </div>
      </div>
    </div>
  )
}
