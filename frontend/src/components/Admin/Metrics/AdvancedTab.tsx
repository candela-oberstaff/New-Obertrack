import React from 'react';
import { 
  Monitor, 
  Smartphone 
} from 'lucide-react';
import { 
  PieChart, 
  Pie, 
  Cell, 
  Tooltip, 
  ResponsiveContainer 
} from 'recharts';
import styles from './AdvancedTab.module.css';

interface AdvancedTabProps {
  data: any;
}

const AdvancedTab: React.FC<AdvancedTabProps> = ({ data }) => {
  const deviceData = [
    { name: 'Desktop', value: (data?.advanced.devices.desktop || 0) * 100 },
    { name: 'Mobile', value: (data?.advanced.devices.mobile || 0) * 100 }
  ];

  return (
    <div className={styles.metricsGrid}>
      <div className={styles.mainChartsGrid}>
        <div className={styles.chartCardLarge}>
          <div className={styles.cardHeader}>
            <h4>Engagement por Segmento</h4>
            <p>Rendimiento según el tipo de usuario</p>
          </div>
          <div className={styles.segmentList} style={{ marginTop: '24px' }}>
            {data?.advanced.segments.map((seg: any, i: number) => (
              <div key={i} className={styles.segmentItem}>
                <span>{seg.name}</span>
                <div className={styles.segValueWrap}>
                  <div className={styles.miniBar}>
                    <div style={{ width: `${seg.engagement * 100}%`, background: '#8b5cf6' }}></div>
                  </div>
                  <strong>{Math.round(seg.engagement * 100)}%</strong>
                </div>
              </div>
            ))}
          </div>
        </div>

        <div className={styles.sideInfoGrid}>
          <div className={styles.chartCard}>
            <h4>Uso de Dispositivos</h4>
            <div className={styles.pieChartContainer}>
              <ResponsiveContainer width="100%" height={200}>
                <PieChart>
                  <Pie
                    data={deviceData}
                    innerRadius={60}
                    outerRadius={80}
                    paddingAngle={8}
                    dataKey="value"
                  >
                    <Cell fill="#8b5cf6" />
                    <Cell fill="#3b82f6" />
                  </Pie>
                  <Tooltip />
                </PieChart>
              </ResponsiveContainer>
              <div className={styles.deviceStats}>
                <div className={styles.devItem}>
                  <Monitor size={16} /> 
                  <span>Desktop <strong>{Math.round((data?.advanced.devices.desktop || 0) * 100)}%</strong></span>
                </div>
                <div className={styles.devItem}>
                  <Smartphone size={16} /> 
                  <span>Mobile <strong>{Math.round((data?.advanced.devices.mobile || 0) * 100)}%</strong></span>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
};

export default AdvancedTab;
