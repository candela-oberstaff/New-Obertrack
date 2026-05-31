import { ResponsiveContainer, BarChart, CartesianGrid, XAxis, YAxis, Tooltip, Bar } from 'recharts'
import { FileText } from 'lucide-react'
import type { WorkHour, User } from '../../types'
import CommonTooltip from '../Common/Tooltip'
import styles from '../../pages/Reports.module.css'

interface ReportsChartsProps {
  dailyData: any[]
  workHours: WorkHour[]
  user: User | null
}

export function ReportsCharts({ dailyData, workHours, user }: ReportsChartsProps) {
  const isProf = user?.user_type === 'profesional'

  return (
    <div className={styles['charts-row'] || 'charts-row'}>
      {workHours.some(wh => wh.activities) && (
        <div className={styles['activities-section']}>
          <h3><FileText size={20} style={{ verticalAlign: 'middle', marginRight: '8px' }} /> Actividades Registradas</h3>
          <div className={styles['activities-list']}>
            {workHours.filter(wh => wh.activities).slice(0, 10).map((wh) => (
              <div key={wh.id} className={styles['activity-card']}>
                <div className={styles['activity-date']}>
                  <span className={styles['activity-day']}>{new Date(wh.work_date).getDate()}</span>
                  <span className={styles['activity-month']}>{new Date(wh.work_date).toLocaleDateString('es-ES', { month: 'short' })}</span>
                </div>
                <div className={styles['activity-content']}>
                  <p className={styles['activity-text']}>{wh.activities?.replace(/<[^>]*>/g, '') || ''}</p>
                  <span className={`${styles['activity-type']} ${styles[wh.work_type] || wh.work_type}`}>
                    {wh.work_type === 'complete' ? 'Jornada Completa' : 'Ausencia'}
                  </span>
                </div>
              </div>
            ))}
          </div>
        </div>
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
