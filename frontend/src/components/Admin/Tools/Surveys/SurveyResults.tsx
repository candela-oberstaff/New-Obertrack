import React, { useEffect, useState, useMemo } from 'react';
import { useQuery } from '@tanstack/react-query';
import { ArrowLeft, Users, FileText, CheckCircle, Search } from 'lucide-react';
import styles from './SurveyResults.module.css';
import commonStyles from '../Tools.module.css';
import { surveyService } from '../../../../services/surveyService';
import Pagination from '../Common/Pagination';

interface SurveyResultsProps {
  surveyId: number;
  onBack: () => void;
}

const PAGE_SIZE = 10;

const USER_TYPE_LABELS: Record<string, string> = {
  profesional: 'Profesional',
  empleador: 'Empresa',
  customer_success: 'Customer Success',
  superadmin: 'Superadmin',
  manager: 'Manager',
};

const SurveyResults: React.FC<SurveyResultsProps> = ({ surveyId, onBack }) => {
  const [search, setSearch] = useState('');
  const [userTypeFilter, setUserTypeFilter] = useState('all');
  const [page, setPage] = useState(1);

  const { data: survey = null, isLoading: loading } = useQuery<any>({
    queryKey: ['survey', surveyId],
    queryFn: () => surveyService.getSurvey(surveyId),
    enabled: !!surveyId,
  });

  useEffect(() => {
    const outletContainer = document.querySelector('[class*="outlet-container"]') as HTMLElement;
    if (outletContainer) outletContainer.scrollTop = 0;
    else window.scrollTo(0, 0);
  }, [surveyId]);

  useEffect(() => { setPage(1); }, [search, userTypeFilter]);

  const allResponses = survey ? (survey as any).responses || [] : [];

  const filtered = useMemo(() => {
    return allResponses.filter((resp: any) => {
      const name = (resp.user?.name || '').toLowerCase();
      const email = (resp.user?.email || '').toLowerCase();
      const q = search.toLowerCase();
      if (q && !name.includes(q) && !email.includes(q)) return false;
      if (userTypeFilter !== 'all' && resp.user?.user_type !== userTypeFilter) return false;
      return true;
    });
  }, [allResponses, search, userTypeFilter]);

  const totalPages = Math.max(1, Math.ceil(filtered.length / PAGE_SIZE));
  const safePage = Math.min(page, totalPages);
  const paginated = filtered.slice((safePage - 1) * PAGE_SIZE, safePage * PAGE_SIZE);

  if (loading) return <div className={styles.builderContainer}><p style={{padding: 40}}>Cargando resultados...</p></div>;
  if (!survey) return <div className={styles.builderContainer}><p style={{padding: 40}}>No se encontró la encuesta.</p></div>;

  const barStyle: React.CSSProperties = {
    display: 'flex', gap: 10, alignItems: 'center', marginBottom: 16, flexWrap: 'wrap',
  };
  const searchInputStyle: React.CSSProperties = {
    flex: 1, minWidth: 180, border: '1px solid #e2e8f0', borderRadius: 8, padding: '8px 12px 8px 34px',
    fontSize: 13, outline: 'none', color: '#1e293b', background: '#fff',
  };
  const searchWrapStyle: React.CSSProperties = {
    position: 'relative', flex: 1, minWidth: 180,
  };
  const selectStyle: React.CSSProperties = {
    border: '1px solid #e2e8f0', borderRadius: 8, padding: '8px 12px', fontSize: 13,
    outline: 'none', background: '#fff', cursor: 'pointer', color: '#1e293b', minWidth: 160,
  };

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
              <p>{allResponses.length}</p>
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
          <h3 style={{ marginBottom: 12 }}>Detalle de Respuestas</h3>

          {/* Search + Filter */}
          <div style={barStyle}>
            <div style={searchWrapStyle}>
              <Search size={15} style={{ position: 'absolute', left: 10, top: '50%', transform: 'translateY(-50%)', color: '#94a3b8', pointerEvents: 'none' }} />
              <input
                type="text"
                placeholder="Buscar por nombre o email..."
                value={search}
                onChange={e => setSearch(e.target.value)}
                style={searchInputStyle}
              />
            </div>
            <select
              value={userTypeFilter}
              onChange={e => setUserTypeFilter(e.target.value)}
              style={selectStyle}
            >
              <option value="all">Todos los tipos</option>
              <option value="profesional">Profesionales</option>
              <option value="empleador">Empresas</option>
              <option value="customer_success">Customer Success</option>
              <option value="manager">Managers</option>
              <option value="superadmin">Superadmins</option>
            </select>
            <span style={{ fontSize: 12, color: '#94a3b8', whiteSpace: 'nowrap' }}>
              {filtered.length} de {allResponses.length}
            </span>
          </div>

          {filtered.length === 0 ? (
            <div className={styles.emptyQuestions}>
              {search || userTypeFilter !== 'all' ? 'No se encontraron respuestas con los filtros aplicados.' : 'Nadie ha respondido a esta encuesta todavía.'}
            </div>
          ) : (
            <>
              <div className={styles.responsesList}>
                {paginated.map((resp: any) => (
                  <div key={resp.id} className={styles.responseCard}>
                    <div className={styles.responseHeader}>
                      <div className={styles.responderInfo}>
                        <div className={styles.avatarPlaceholder}>
                          {resp.user?.name?.charAt(0) || 'U'}
                        </div>
                        <div>
                          <strong>{resp.user?.name || 'Usuario'}</strong>
                          <span style={{fontSize: 12, color: '#64748b'}}>
                            {resp.user?.email}
                            {resp.user?.user_type && (
                              <span style={{ marginLeft: 8, fontSize: 10, fontWeight: 600, color: '#7c3aed', background: '#f5f3ff', padding: '1px 6px', borderRadius: 4 }}>
                                {USER_TYPE_LABELS[resp.user.user_type] || resp.user.user_type}
                              </span>
                            )}
                          </span>
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
              <Pagination currentPage={safePage} totalPages={totalPages} onPageChange={setPage} />
            </>
          )}
        </div>
      </div>
    </div>
  );
};

export default SurveyResults;
