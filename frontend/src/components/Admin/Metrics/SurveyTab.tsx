import React from 'react';
import { 
  CheckCircle2, 
  TrendingUp, 
  Award, 
  UserX, 
  Mail 
} from 'lucide-react';
import styles from './SurveyTab.module.css';

interface SurveyTabProps {
  data: any;
}

const SurveyTab: React.FC<SurveyTabProps> = ({ data }) => {
  return (
    <div className={styles.metricsGrid}>
      <div className={styles.statsRow}>
        <div className={styles.statCard}>
          <div className={styles.statIcon} style={{ background: '#f5f3ff', color: '#8b5cf6' }}><Mail size={24} /></div>
          <div className={styles.statInfo}>
            <label>Campañas</label>
            <h3>{data?.surveys.total_surveys || '0'}</h3>
          </div>
        </div>
        <div className={styles.statCard}>
          <div className={styles.statIcon} style={{ background: '#ecfeff', color: '#0891b2' }}><CheckCircle2 size={24} /></div>
          <div className={styles.statInfo}>
            <label>Respondidas</label>
            <h3>{data?.surveys.total_responses || '0'}</h3>
          </div>
        </div>
        <div className={styles.statCard}>
          <div className={styles.statIcon} style={{ background: '#fff7ed', color: '#ea580c' }}><Award size={24} /></div>
          <div className={styles.statInfo}>
            <label>Satisfacción</label>
            <h3>{data?.surveys.avg_satisfaction.toFixed(1) || '0'}/5</h3>
          </div>
        </div>
        <div className={styles.statCard}>
          <div className={styles.statIcon} style={{ background: '#fef2f2', color: '#dc2626' }}><UserX size={24} /></div>
          <div className={styles.statInfo}>
            <label>Falta Feedback</label>
            <h3>---</h3>
            <span className={styles.subText}>Sin datos históricos</span>
          </div>
        </div>
      </div>

      <div className={styles.mainChartsGrid}>
        <div className={styles.chartCardLarge}>
          <div className={styles.cardHeader}>
            <h4>Resumen de Satisfacción</h4>
            <p>Datos reales agregados desde la base de datos de respuestas</p>
          </div>
          <div className={styles.emptyChart}>
             <TrendingUp size={48} opacity={0.2} />
             <p>Se requiere mayor volumen de respuestas para generar tendencias temporales.</p>
          </div>
        </div>

        <div className={styles.sideInfoGrid}>
          <div className={styles.chartCard}>
            <h4>Preguntas Críticas</h4>
            <div className={styles.criticalQuestions}>
              <div className={styles.criticalItem}>
                <div className={styles.critHeader}>
                  <p>Puntaje Promedio General</p>
                  <span className={styles.critValue}>{data?.surveys.avg_satisfaction.toFixed(1) || '0'}</span>
                </div>
                <div className={styles.progressBar}>
                  <div style={{ 
                    width: `${(data?.surveys.avg_satisfaction / 5) * 100}%`, 
                    background: data?.surveys.avg_satisfaction > 4 ? '#10b981' : '#f59e0b' 
                  }}></div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
};

export default SurveyTab;
