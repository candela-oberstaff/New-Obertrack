import React, { useState, useEffect } from 'react';
import {
  Mail,
  BarChart3,
  CheckCircle2,
  Calendar,
  Activity
} from 'lucide-react';
import styles from './Metrics.module.css';
import { metricsService, MetricsData } from '../services/metrics.service';
import EmailTab from '../components/Admin/Metrics/EmailTab';
import SurveyTab from '../components/Admin/Metrics/SurveyTab';
import AdvancedTab from '../components/Admin/Metrics/AdvancedTab';

const MetricsPage: React.FC = () => {
  const [activeTab, setActiveTab] = useState<'emails' | 'surveys' | 'advanced'>('emails');
  const [data, setData] = useState<MetricsData | null>(null);
  const [loading, setLoading] = useState(true);
  const [days, setDays] = useState(30);

  useEffect(() => {
    const fetchData = async () => {
      setLoading(true);
      try {
        const res = await metricsService.getGlobalMetrics(days);
        setData(res);
      } catch (error) {
        console.error("Error fetching metrics:", error);
      } finally {
        setLoading(false);
      }
    };
    fetchData();
  }, [days]);

  if (loading && !data) return (
    <div className={styles.loading}>
      <Activity size={48} className={styles.spin} />
      <p>Analizando datos de rendimiento...</p>
    </div>
  );

  return (
    <div className={styles.metricsContainer}>
      <header className={styles.metricsHeader}>
        <div>
          <h1>Métricas</h1>
          <p>Visualización real del engagement basada en eventos registrados</p>
        </div>
        <div className={styles.headerActions}>
          <div className={styles.periodSelector}>
            <Calendar size={16} />
            <select
              value={days}
              onChange={(e) => setDays(Number(e.target.value))}
              className={styles.selectFilter}
            >
              <option value={7}>Últimos 7 días</option>
              <option value={30}>Últimos 30 días</option>
              <option value={90}>Últimos 90 días</option>
              <option value={365}>Último año</option>
            </select>
          </div>
        </div>
      </header>

      <nav className={styles.metricsTabs}>
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
        <select 
          value={activeTab} 
          onChange={(e) => setActiveTab(e.target.value as any)}
        >
          <option value="emails">Emails</option>
          <option value="surveys">Encuestas</option>
          <option value="advanced">Avanzado</option>
        </select>
      </div>

      <div className={styles.tabContent}>
        {activeTab === 'emails' && <EmailTab data={data} />}
        {activeTab === 'surveys' && <SurveyTab data={data} />}
        {activeTab === 'advanced' && <AdvancedTab data={data} />}
      </div>
    </div>
  );
};

export default MetricsPage;
