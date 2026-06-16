import React, { useEffect } from 'react';
import { useQuery } from '@tanstack/react-query';
import { ArrowLeft, Users, FileText, CheckCircle } from 'lucide-react';
import styles from './SurveyResults.module.css';
import commonStyles from '../Tools.module.css';
import { surveyService } from '../../../../services/surveyService';

interface SurveyResultsProps {
  surveyId: number;
  onBack: () => void;
}

const SurveyResults: React.FC<SurveyResultsProps> = ({ surveyId, onBack }) => {
  const { data: survey = null, isLoading: loading } = useQuery<any>({
    queryKey: ['survey', surveyId],
    queryFn: () => surveyService.getSurvey(surveyId),
    enabled: !!surveyId,
  });

  useEffect(() => {
    // Scroll the outlet container back to top when results view mounts
    const outletContainer = document.querySelector('[class*="outlet-container"]') as HTMLElement;
    if (outletContainer) outletContainer.scrollTop = 0;
    else window.scrollTo(0, 0);
  }, [surveyId]);

  if (loading) return <div className={styles.builderContainer}><p style={{padding: 40}}>Cargando resultados...</p></div>;
  if (!survey) return <div className={styles.builderContainer}><p style={{padding: 40}}>No se encontró la encuesta.</p></div>;

  const responses = (survey as any).responses || [];

  return (
    <div className={styles.builderContainer}>
      <header className={styles.builderHeader}>
        <div className={styles.headerLeft}>
          <button className={commonStyles['btn-outline']} onClick={onBack}>
            <ArrowLeft size={16} /> Volver
          </button>
          <div className={styles.titleInfo}>
            <h2 style={{margin: 0, fontSize: 20}}>{survey.title} - Resultados</h2>
            <p style={{margin: '4px 0 0 0', color: '#64748b', fontSize: 13}}>Estado: {survey.status}</p>
          </div>
        </div>
      </header>

      <div className={styles.canvasArea} style={{flexDirection: 'column', alignItems: 'center', paddingTop: 24}}>
        
        {/* Metric Cards */}
        <div className={styles.metricsRow}>
          <div className={styles.metricCard}>
            <div className={styles.metricIcon}><Users size={24} color="#8b5cf6" /></div>
            <div className={styles.metricData}>
              <h4>Respuestas</h4>
              <p>{responses.length}</p>
            </div>
          </div>
          <div className={styles.metricCard}>
            <div className={styles.metricIcon}><FileText size={24} color="#10b981" /></div>
            <div className={styles.metricData}>
              <h4>Preguntas</h4>
              <p>{survey.questions?.length || 0}</p>
            </div>
          </div>
        </div>

        {/* Detailed Responses */}
        <div className={styles.surveyForm} style={{marginTop: 32}}>
          <h3>Detalle de Respuestas</h3>
          {responses.length === 0 ? (
            <div className={styles.emptyQuestions}>Nadie ha respondido a esta encuesta todavía.</div>
          ) : (
            <div className={styles.responsesList}>
              {responses.map((resp: any) => (
                <div key={resp.id} className={styles.responseCard}>
                  <div className={styles.responseHeader}>
                    <div className={styles.responderInfo}>
                      <div className={styles.avatarPlaceholder}>
                        {resp.user?.name?.charAt(0) || 'U'}
                      </div>
                      <div>
                        <strong>{resp.user?.name || 'Usuario'}</strong>
                        <span style={{fontSize: 12, color: '#64748b'}}>{resp.user?.email}</span>
                      </div>
                    </div>
                    <div className={styles.responseTime}>
                      <CheckCircle size={14} color="#10b981" />
                      {new Date(resp.completed_at).toLocaleDateString()}
                    </div>
                  </div>
                  <div className={styles.responseAnswers}>
                    {resp.answers?.sort((a: any, b: any) => {
                      const qA = survey.questions?.find((sq: any) => sq.id === a.question_id);
                      const qB = survey.questions?.find((sq: any) => sq.id === b.question_id);
                      return (qA?.order_index || 0) - (qB?.order_index || 0);
                    }).map((ans: any) => {
                      const q = survey.questions?.find((sq: any) => sq.id === ans.question_id);
                      return (
                        <div key={ans.id} className={styles.answerItem}>
                          <p className={styles.answerQuestion}>{q?.text}</p>
                          <div className={styles.answerValue}>
                            {(() => {
                              if (q?.type === 'rating') {
                                return <span className={styles.ratingBadge}>⭐ {ans.number_value} / 5</span>;
                              }
                              if (q?.type === 'linear_scale') {
                                return <span className={styles.ratingBadge}>📊 Escala: {ans.number_value}</span>;
                              }
                              // Parse checkbox arrays
                              if (q?.type === 'checkbox') {
                                try {
                                  const arr = JSON.parse(ans.text_value || '[]');
                                  if (Array.isArray(arr)) {
                                    return (
                                      <div style={{ display: 'flex', gap: '6px', flexWrap: 'wrap', marginTop: '4px' }}>
                                        {arr.map((val: string, vi: number) => (
                                          <span key={vi} style={{ background: '#f5f3ff', border: '1px solid #ddd6fe', color: '#7c3aed', padding: '2px 8px', borderRadius: '12px', fontSize: '12px' }}>
                                            {val}
                                          </span>
                                        ))}
                                      </div>
                                    );
                                  }
                                } catch {}
                              }
                              // Parse grid / checkbox_grid objects
                              if (q?.type === 'grid' || q?.type === 'checkbox_grid') {
                                try {
                                  const obj = JSON.parse(ans.text_value || '{}');
                                  return (
                                    <div style={{ marginTop: '8px', background: '#fafafa', border: '1px solid #f1f5f9', borderRadius: '8px', padding: '8px' }}>
                                      {Object.entries(obj).map(([row, val]: [string, any], oi: number) => (
                                        <div key={oi} style={{ display: 'flex', justifyContent: 'space-between', fontSize: '13px', padding: '4px 0', borderBottom: oi < Object.keys(obj).length - 1 ? '1px solid #f1f5f9' : 'none' }}>
                                          <span style={{ fontWeight: 500, color: '#475569' }}>{row}</span>
                                          <span style={{ color: '#1e293b' }}>
                                            {Array.isArray(val) ? val.join(', ') : String(val)}
                                          </span>
                                        </div>
                                      ))}
                                    </div>
                                  );
                                } catch {}
                              }
                              return ans.text_value;
                            })()}
                          </div>
                        </div>
                      );
                    })}
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  );
};

export default SurveyResults;
