import React, { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import {
  Mail,
  BarChart3,
  CheckCircle2,
  Calendar,
  Activity
} from 'lucide-react';
import styles from './Metrics.module.css';
import { metricsService } from '../services/metrics.service';
import EmailTab from '../components/Admin/Metrics/EmailTab';
import SurveyTab from '../components/Admin/Metrics/SurveyTab';
import AdvancedTab from '../components/Admin/Metrics/AdvancedTab';
import { Select } from '../components/ui/Select';

const MetricsPage: React.FC = () => {
  const [activeTab, setActiveTab] = useState<'emails' | 'surveys' | 'advanced'>('emails');
  const [days, setDays] = useState(30);

  const { data = null, isLoading: loading } = useQuery({
    queryKey: ['metrics', days],
    queryFn: () => metricsService.getGlobalMetrics(days),
  });

  if (loading && !data) return (
    <div className={styles.loading}>
      <Activity size={48} className={styles.spin} />
      <p>Analizando datos de rendimiento...</p>
    </div>
  );

  return (
    <div className={styles.metricsContainer}>
      <header className={styles.metricsHeader} data-tour="metrics-header">
        <div>
          <h1>Métricas</h1>
          <p>Visualización real del engagement basada en eventos registrados</p>
        </div>
        <div className={styles.headerActions} data-tour="metrics-period">
          <Select
            value={days}
            onChange={(v) => setDays(Number(v))}
            leftIcon={<Calendar size={16} />}
            options={[
              { value: 7, label: 'Últimos 7 días' },
              { value: 30, label: 'Últimos 30 días' },
              { value: 90, label: 'Últimos 90 días' },
              { value: 365, label: 'Último año' },
            ]}
          />
        </div>
      </header>

      <nav className={styles.metricsTabs} data-tour="metrics-tabs">
        <button
          className={activeTab === 'emails' ? styles.active : ''}
          onClick={() => setActiveTab('emails')}
        >
          <Mail size={18} /> Emails
        </button>
        <button
          className={activeTab === 'surveys' ? styles.active : ''}
          onClick={() => setActiveTab('surveys')}
        >
          <CheckCircle2 size={18} /> Encuestas
        </button>
        <button
          className={activeTab === 'advanced' ? styles.active : ''}
          onClick={() => setActiveTab('advanced')}
        >
          <BarChart3 size={18} /> Avanzado
        </button>
      </nav>

      <div className={styles.mobileTabs}>
        <Select
          fullWidth
          value={activeTab}
          onChange={(v) => setActiveTab(v as any)}
          options={[
            { value: 'emails', label: 'Emails' },
            { value: 'surveys', label: 'Encuestas' },
            { value: 'advanced', label: 'Avanzado' },
          ]}
        />
      </div>

      <div className={styles.tabContent} data-tour="metrics-content">
        {activeTab === 'emails' && <EmailTab data={data} />}
        {activeTab === 'surveys' && <SurveyTab data={data} />}
        {activeTab === 'advanced' && <AdvancedTab data={data} />}
      </div>
    </div>
  );
};

export default MetricsPage;
