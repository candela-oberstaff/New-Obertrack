import React from 'react';
import { TrendingUp, Users, MousePointer2, Mail } from 'lucide-react';
import styles from './Metrics.module.css';
import commonStyles from '../Tools.module.css';

const Metrics: React.FC = () => {
  const stats = [
    { label: 'Emails Enviados', value: '1,284', trend: '+12%', icon: Mail, color: '#3b82f6' },
    { label: 'Tasa de Apertura', value: '24.8%', trend: '+5%', icon: Users, color: '#10b981' },
    { label: 'Click Rate', value: '3.2%', trend: '-2%', icon: MousePointer2, color: '#f59e0b' },
    { label: 'Feedback Positivo', value: '92%', trend: '+8%', icon: TrendingUp, color: '#8b5cf6' },
  ];

  return (
    <div className={styles['metrics-section']}>
      <div className={commonStyles['section-header']}>
        <h2>Métricas de Rendimiento (EN CONSTRUCCIÓN)</h2>
      </div>

      <div className={styles['metrics-overview']}>
        {stats.map((stat, idx) => (
          <div key={idx} className={styles['metric-card-wide']}>
            <div className={styles['metric-icon-box']} style={{ backgroundColor: `${stat.color}15`, color: stat.color }}>
              <stat.icon size={24} />
            </div>
            <div className={styles['metric-data']}>
              <span className={styles['metric-label']}>{stat.label}</span>
              <div className={styles['metric-value-row']}>
                <span className={styles['metric-value-main']}>{stat.value}</span>
                <span className={`${styles['metric-trend']} ${stat.trend.startsWith('+') ? styles.up : styles.down}`}>
                  {stat.trend}
                </span>
              </div>
            </div>
          </div>
        ))}
      </div>

      <div className={styles['metrics-charts-grid']}>
        <div className={styles['chart-placeholder-card']}>
          <h4>Crecimiento de Audiencia</h4>
          <div className={styles['mock-chart-container']}>
            {/* Aquí iría un gráfico real de Recharts o similar */}
            <div style={{ height: '100%', display: 'flex', alignItems: 'flex-end', gap: '8px', padding: '10px' }}>
              {[40, 60, 45, 80, 55, 90, 70].map((h, i) => (
                <div key={i} style={{ flex: 1, height: `${h}%`, background: 'var(--color-blue-violet-hex)', borderRadius: '4px', opacity: 0.2 + (i * 0.1) }}></div>
              ))}
            </div>
          </div>
        </div>
        <div className={styles['chart-placeholder-card']}>
          <h4>Engagement por Canal</h4>
          <div className={styles['mock-chart-container']}>
            <div style={{ display: 'flex', flexDirection: 'column', gap: '12px', padding: '20px' }}>
              <div style={{ height: '8px', width: '85%', background: '#10b981', borderRadius: '4px' }}></div>
              <div style={{ height: '8px', width: '60%', background: '#3b82f6', borderRadius: '4px' }}></div>
              <div style={{ height: '8px', width: '40%', background: '#f59e0b', borderRadius: '4px' }}></div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
};

export default Metrics;
