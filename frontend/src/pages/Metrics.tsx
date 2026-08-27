import React, { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import {
  Mail,
  CheckCircle2,
  Calendar,
  Activity,
  Gauge
} from 'lucide-react';
import styles from './Metrics.module.css';
import { metricsService } from '../services/metrics.service';
import EmailTab from '../components/Admin/Metrics/EmailTab';
import UsageTab from '../components/Admin/Metrics/UsageTab';
import SurveyTab from '../components/Admin/Metrics/SurveyTab';
import { Select } from '../components/ui/Select';

const MetricsPage: React.FC = () => {
  // Uso es la pestaña de entrada: la pregunta con la que se abre esta pantalla
  // —¿quién usa la app?— pesa más que el rendimiento de una campaña de correo.
  const [activeTab, setActiveTab] = useState<'usage' | 'emails' | 'surveys'>('usage');
  const [days, setDays] = useState(30);

  const { data = null, isLoading: loading } = useQuery({
    queryKey: ['metrics', days],
    queryFn: () => metricsService.getGlobalMetrics(days),
    // Solo hace falta para Emails y Encuestas. La pestaña Uso trae lo suyo, y
    // cargar esto al entrar retrasaría la pantalla por dos pestañas que puede
    // que nadie abra.
    enabled: activeTab !== 'usage',
  });

  if (activeTab !== 'usage' && loading && !data) return (
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
          <p>Uso real de la app, empresa por empresa y persona por persona</p>
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
          className={activeTab === 'usage' ? styles.active : ''}
          onClick={() => setActiveTab('usage')}
        >
          <Gauge size={18} /> Uso
        </button>
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
      </nav>

      <div className={styles.mobileTabs}>
        <Select
          fullWidth
          value={activeTab}
          onChange={(v) => setActiveTab(v as any)}
          options={[
            { value: 'usage', label: 'Uso' },
            { value: 'emails', label: 'Emails' },
            { value: 'surveys', label: 'Encuestas' },
          ]}
        />
      </div>

      <div className={styles.tabContent} data-tour="metrics-content">
        {activeTab === 'usage' && <UsageTab days={days} />}
        {activeTab === 'emails' && <EmailTab data={data} />}
        {activeTab === 'surveys' && <SurveyTab data={data} />}
      </div>
    </div>
  );
};

export default MetricsPage;
