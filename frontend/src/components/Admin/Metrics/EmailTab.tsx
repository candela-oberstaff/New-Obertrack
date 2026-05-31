import React from 'react';
import { 
  Mail, 
  Users, 
  MousePointer2, 
  AlertCircle
} from 'lucide-react';
import { 
  AreaChart, 
  Area, 
  XAxis, 
  YAxis, 
  CartesianGrid, 
  Tooltip, 
  ResponsiveContainer, 
  Legend 
} from 'recharts';
import styles from './EmailTab.module.css';

interface EmailTabProps {
  data: any;
}

const EmailTab: React.FC<EmailTabProps> = ({ data }) => {
  // Transform evolution data for the chart
  const evolutionMap: { [key: string]: any } = {};
  if (data?.emails.evolution) {
    data.emails.evolution.forEach((item: any) => {
      const date = item.date.split('T')[0];
      if (!evolutionMap[date]) evolutionMap[date] = { name: date };
      evolutionMap[date][item.event] = item.count;
    });
  }
  const evolutionData = Object.values(evolutionMap).sort((a: any, b: any) => a.name.localeCompare(b.name));

  return (
    <div className={styles.metricsGrid}>
      <div className={styles.statsRow}>
        <div className={styles.statCard}>
          <div className={styles.statIcon} style={{ background: '#e0e7ff', color: '#4338ca' }}><Mail size={24} /></div>
          <div className={styles.statInfo}>
            <label>Enviados</label>
            <h3>{data?.emails.total_sent.toLocaleString() || '0'}</h3>
          </div>
        </div>
        <div className={styles.statCard}>
          <div className={styles.statIcon} style={{ background: '#dcfce7', color: '#15803d' }}><Users size={24} /></div>
          <div className={styles.statInfo}>
            <label>Apertura %</label>
            <h3>{data?.emails.open_rate.toFixed(1) || '0'}%</h3>
          </div>
        </div>
        <div className={styles.statCard}>
          <div className={styles.statIcon} style={{ background: '#fef9c3', color: '#a16207' }}><MousePointer2 size={24} /></div>
          <div className={styles.statInfo}>
            <label>CTR (Clics)</label>
            <h3>{data?.emails.click_rate.toFixed(1) || '0'}%</h3>
          </div>
        </div>
        <div className={styles.statCard}>
          <div className={styles.statIcon} style={{ background: '#fee2e2', color: '#b91c1c' }}><AlertCircle size={24} /></div>
          <div className={styles.statInfo}>
            <label>Rebotes</label>
            <h3>{data?.emails.total_bounced || '0'}</h3>
          </div>
        </div>
      </div>

      <div className={styles.mainChartsGrid}>
        <div className={styles.chartCardLarge}>
          <div className={styles.cardHeader}>
            <h4>Evolución Diaria</h4>
            <p>Interacción real basada en eventos de Brevo</p>
          </div>
          <div className={styles.chartWrapper}>
            {evolutionData.length > 0 ? (
              <ResponsiveContainer width="100%" height={320}>
                <AreaChart data={evolutionData}>
                  <defs>
                    <linearGradient id="colorOpened" x1="0" y1="0" x2="0" y2="1">
                      <stop offset="5%" stopColor="#8b5cf6" stopOpacity={0.1}/>
                      <stop offset="95%" stopColor="#8b5cf6" stopOpacity={0}/>
                    </linearGradient>
                  </defs>
                  <CartesianGrid strokeDasharray="3 3" vertical={false} stroke="#f1f5f9" />
                  <XAxis dataKey="name" axisLine={false} tickLine={false} tick={{fill: '#64748b', fontSize: 12}} />
                  <YAxis axisLine={false} tickLine={false} tick={{fill: '#64748b', fontSize: 12}} />
                  <Tooltip 
                    contentStyle={{ borderRadius: '12px', border: 'none', boxShadow: '0 10px 15px -3px rgba(0,0,0,0.1)' }}
                  />
                  <Legend verticalAlign="top" height={36} iconType="circle" />
                  <Area name="Abiertos" type="monotone" dataKey="opened" stroke="#8b5cf6" fillOpacity={1} fill="url(#colorOpened)" strokeWidth={3} />
                  <Area name="Clics" type="monotone" dataKey="click" stroke="#3b82f6" fillOpacity={0} strokeWidth={3} />
                </AreaChart>
              </ResponsiveContainer>
            ) : (
              <div className={styles.emptyChart}>Esperando eventos de Brevo...</div>
            )}
          </div>
        </div>

        <div className={styles.sideInfoGrid}>
          <div className={styles.chartCard}>
            <h4>Resumen de Interacción</h4>
            <div className={styles.bounceChart}>
              <div className={styles.bounceRow}>
                <span>Aperturas</span>
                <strong>{data?.emails.total_opened || 0}</strong>
              </div>
              <div className={styles.bounceRow}>
                <span>Clics</span>
                <strong>{data?.emails.total_clicked || 0}</strong>
              </div>
              <div className={styles.bounceRow}>
                <span>Rebotes</span>
                <strong>{data?.emails.total_bounced || 0}</strong>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
};

export default EmailTab;
