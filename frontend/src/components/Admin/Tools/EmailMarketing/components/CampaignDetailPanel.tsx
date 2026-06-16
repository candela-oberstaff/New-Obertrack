import React, { useMemo } from 'react';
import { useQuery } from '@tanstack/react-query';
import {
  X, Users, Eye, MousePointer2, AlertTriangle, Send, Calendar,
  TrendingUp, Activity, Mail, Clock, CheckCircle2, XCircle, Info,
  Pencil, Trash2
} from 'lucide-react';
import { emailService } from '../../../../../services/emailService';
import styles from './CampaignDetailPanel.module.css';

interface Props {
  campaign: any;
  onClose: () => void;
  onEdit: (campaign: any) => void;
  onDelete: (id: number) => void;
}

const EVENT_META: Record<string, { label: string; color: string; icon: React.ReactNode }> = {
  request:       { label: 'Solicitud',     color: '#94a3b8', icon: <Send size={12} /> },
  delivered:     { label: 'Entregado',     color: '#22c55e', icon: <CheckCircle2 size={12} /> },
  opened:        { label: 'Abierto',       color: '#8b5cf6', icon: <Eye size={12} /> },
  unique_opened: { label: 'Apertura única',color: '#a78bfa', icon: <Eye size={12} /> },
  click:         { label: 'Clic',          color: '#3b82f6', icon: <MousePointer2 size={12} /> },
  soft_bounce:   { label: 'Rebote suave',  color: '#f59e0b', icon: <AlertTriangle size={12} /> },
  hard_bounce:   { label: 'Rebote duro',   color: '#ef4444', icon: <XCircle size={12} /> },
  spam:          { label: 'Spam',          color: '#f97316', icon: <AlertTriangle size={12} /> },
  unsubscribed:  { label: 'Baja',          color: '#64748b', icon: <XCircle size={12} /> },
  deferred:      { label: 'Diferido',      color: '#eab308', icon: <Clock size={12} /> },
  blocked:       { label: 'Bloqueado',     color: '#dc2626', icon: <XCircle size={12} /> },
  error:         { label: 'Error',         color: '#ef4444', icon: <AlertTriangle size={12} /> },
};

const getEventMeta = (event: string) =>
  EVENT_META[event] ?? { label: event, color: '#94a3b8', icon: <Info size={12} /> };

const formatDate = (d: string) => new Date(d).toLocaleString('es-AR', {
  day: '2-digit', month: 'short', year: 'numeric', hour: '2-digit', minute: '2-digit'
});

const StatCard: React.FC<{ icon: React.ReactNode; label: string; value: string | number; sub?: string; color?: string }> =
  ({ icon, label, value, sub, color = '#8b5cf6' }) => (
  <div className={styles.statCard}>
    <div className={styles.statIcon} style={{ background: color + '18', color }}>
      {icon}
    </div>
    <div>
      <div className={styles.statValue}>{value}</div>
      <div className={styles.statLabel}>{label}</div>
      {sub && <div className={styles.statSub}>{sub}</div>}
    </div>
  </div>
);

const CampaignDetailPanel: React.FC<Props> = ({ campaign, onClose, onEdit, onDelete }) => {
  const { data: events = [], isLoading } = useQuery<any[]>({
    queryKey: ['campaign-events', campaign.id],
    queryFn: () => emailService.getCampaignEvents(campaign.id),
    enabled: campaign.status === 'sent',
  });

  // Group events by type for summary
  const eventSummary = useMemo(() => {
    const map: Record<string, number> = {};
    events.forEach(ev => { map[ev.event] = (map[ev.event] || 0) + 1; });
    return map;
  }, [events]);

  const openCount   = (eventSummary['opened'] || 0) + (eventSummary['unique_opened'] || 0);
  const clickCount  = eventSummary['click'] || 0;
  const bounceCount = (eventSummary['hard_bounce'] || 0) + (eventSummary['soft_bounce'] || 0);
  const total       = campaign.recipients || 0;

  const openRate  = total > 0 ? ((openCount  / total) * 100).toFixed(1) : '0.0';
  const clickRate = total > 0 ? ((clickCount / total) * 100).toFixed(1) : '0.0';
  const bounceRate= total > 0 ? ((bounceCount/ total) * 100).toFixed(1) : '0.0';

  const statusLabel = campaign.status === 'sent' ? 'Enviada' :
                      campaign.status === 'scheduled' ? 'Programada' : 'Borrador';
  const statusClass = campaign.status === 'sent' ? styles.statusSent :
                      campaign.status === 'scheduled' ? styles.statusScheduled : styles.statusDraft;

  return (
    <div className={styles.overlay} onClick={onClose}>
      <div className={styles.panel} onClick={e => e.stopPropagation()}>
        {/* Header */}
        <div className={styles.panelHeader}>
          <div className={styles.headerLeft}>
            <span className={`${styles.statusBadge} ${statusClass}`}>{statusLabel}</span>
            <h2 className={styles.title}>{campaign.title}</h2>
            {campaign.subject && <p className={styles.subject}>{campaign.subject}</p>}
          </div>
          <div className={styles.headerActions}>
            {campaign.status !== 'sent' && (
              <button className={styles.actionBtn} onClick={() => onEdit(campaign)} title="Editar">
                <Pencil size={16} />
              </button>
            )}
            <button className={`${styles.actionBtn} ${styles.danger}`} onClick={() => onDelete(campaign.id)} title="Eliminar">
              <Trash2 size={16} />
            </button>
            <button className={styles.closeBtn} onClick={onClose}>
              <X size={20} />
            </button>
          </div>
        </div>

        {/* Meta row */}
        <div className={styles.metaRow}>
          {campaign.sent_at && (
            <span className={styles.metaItem}>
              <Send size={12} /> Enviado {formatDate(campaign.sent_at)}
            </span>
          )}
          {campaign.scheduled_at && campaign.status === 'scheduled' && (
            <span className={styles.metaItem}>
              <Calendar size={12} /> Programado para {formatDate(campaign.scheduled_at)}
            </span>
          )}
          {campaign.created_at && (
            <span className={styles.metaItem}>
              <Clock size={12} /> Creado {formatDate(campaign.created_at)}
            </span>
          )}
        </div>

        {/* Stats grid */}
        {campaign.status === 'sent' && (
          <>
            <div className={styles.statsGrid}>
              <StatCard icon={<Users size={18} />}    label="Enviados"   value={total}        color="#3b82f6" />
              <StatCard icon={<Eye size={18} />}       label="Aperturas"  value={`${openRate}%`}  sub={`${openCount} únicos`} color="#8b5cf6" />
              <StatCard icon={<MousePointer2 size={18} />} label="Clics" value={`${clickRate}%`} sub={`${clickCount} clics`}  color="#22c55e" />
              <StatCard icon={<AlertTriangle size={18} />} label="Rebotes" value={`${bounceRate}%`} sub={`${bounceCount} emails`} color="#f59e0b" />
            </div>

            {/* Funnel bar */}
            <div className={styles.funnelSection}>
              <div className={styles.sectionTitle}><TrendingUp size={14} /> Embudo de rendimiento</div>
              <div className={styles.funnelBar}>
                {[
                  { label: 'Enviados', pct: 100,                   color: '#3b82f6' },
                  { label: 'Abiertos', pct: parseFloat(openRate),  color: '#8b5cf6' },
                  { label: 'Clics',    pct: parseFloat(clickRate), color: '#22c55e' },
                ].map(seg => (
                  <div key={seg.label} className={styles.funnelSegment}>
                    <div className={styles.funnelTrack}>
                      <div
                        className={styles.funnelFill}
                        style={{ width: `${seg.pct}%`, background: seg.color }}
                      />
                    </div>
                    <div className={styles.funnelLabel}>
                      <span style={{ color: seg.color, fontWeight: 700 }}>{seg.pct.toFixed(1)}%</span>
                      <span>{seg.label}</span>
                    </div>
                  </div>
                ))}
              </div>
            </div>

            {/* Event type summary pills */}
            {Object.keys(eventSummary).length > 0 && (
              <div className={styles.pillsSection}>
                <div className={styles.sectionTitle}><Activity size={14} /> Resumen por tipo de evento</div>
                <div className={styles.pills}>
                  {Object.entries(eventSummary).map(([evType, count]) => {
                    const meta = getEventMeta(evType);
                    return (
                      <div key={evType} className={styles.pill} style={{ borderColor: meta.color + '44', background: meta.color + '12' }}>
                        <span style={{ color: meta.color }}>{meta.icon}</span>
                        <span style={{ color: meta.color, fontWeight: 600 }}>{count}</span>
                        <span className={styles.pillLabel}>{meta.label}</span>
                      </div>
                    );
                  })}
                </div>
              </div>
            )}

            {/* Event log */}
            <div className={styles.eventLogSection}>
              <div className={styles.sectionTitle}><Mail size={14} /> Registro de eventos</div>
              {isLoading ? (
                <div className={styles.logEmpty}>Cargando eventos...</div>
              ) : events.length === 0 ? (
                <div className={styles.logEmpty}>No hay eventos registrados aún. Los eventos llegan vía webhook de Brevo.</div>
              ) : (
                <div className={styles.eventLog}>
                  {[...events].sort((a, b) =>
                    new Date(b.timestamp).getTime() - new Date(a.timestamp).getTime()
                  ).map((ev, i) => {
                    const meta = getEventMeta(ev.event);
                    return (
                      <div key={ev.id ?? i} className={styles.eventRow}>
                        <div className={styles.eventDot} style={{ background: meta.color }} />
                        <div className={styles.eventBody}>
                          <div className={styles.eventTop}>
                            <span className={styles.eventType} style={{ color: meta.color }}>{meta.label}</span>
                            <span className={styles.eventEmail}>{ev.email}</span>
                          </div>
                          <div className={styles.eventBottom}>
                            <span className={styles.eventTime}>{formatDate(ev.timestamp)}</span>
                            {ev.ip && <span className={styles.eventMeta}>IP: {ev.ip}</span>}
                          </div>
                        </div>
                      </div>
                    );
                  })}
                </div>
              )}
            </div>
          </>
        )}

        {/* Draft / Scheduled state */}
        {campaign.status !== 'sent' && (
          <div className={styles.draftState}>
            <div className={styles.draftIcon}>
              {campaign.status === 'scheduled' ? <Calendar size={32} /> : <Mail size={32} />}
            </div>
            <h3>{campaign.status === 'scheduled' ? 'Campaña programada' : 'Campaña en borrador'}</h3>
            <p>
              {campaign.status === 'scheduled'
                ? `Esta campaña está programada para enviarse el ${formatDate(campaign.scheduled_at)}.`
                : 'Esta campaña es un borrador. Editala y envíala para ver las métricas de rendimiento aquí.'}
            </p>
            <button className={styles.editCta} onClick={() => onEdit(campaign)}>
              <Pencil size={14} /> Editar campaña
            </button>
          </div>
        )}
      </div>
    </div>
  );
};

export default CampaignDetailPanel;
