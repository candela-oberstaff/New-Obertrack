import { useNavigate } from 'react-router-dom';
import { Ticket, LinkedUser, TicketStatusOption } from '../../../services/ticket.service';
import { Select } from '../../../components/ui/Select';
import styles from '../Tickets.module.css';
import {
  User, Phone, Mail, Clock, AlertCircle, Building2,
  Briefcase, MapPin, MessageCircle, Shield, Brain, TrendingUp, ArrowRightLeft,
  Hand, CheckCircle2, RotateCcw
} from 'lucide-react';

interface ContactSidebarProps {
  ticket: Ticket;
  linkedUser: LinkedUser | null;
  statusOptions: TicketStatusOption[];
  onStatusChange: (newStatus: string) => void;
  stageError?: string | null;
  updatingStage?: boolean;
  onTransfer?: () => void;
  onWhatsAppAction?: (action: 'claim' | 'resolve' | 'reopen') => void;
  actionBusy?: boolean;
}

const STAGE_META: Record<string, { label: string; color: string }> = {
  new:         { label: 'Nuevo',       color: 'var(--color-azure)' },
  in_progress: { label: 'En Progreso', color: 'var(--primary)' },
  waiting:     { label: 'Esperando',   color: 'var(--warning)' },
  closed:      { label: 'Cerrado',     color: 'var(--gray-400)' },
};

export default function ContactSidebar({ ticket, linkedUser, statusOptions, onStatusChange, stageError, updatingStage = false, onTransfer, onWhatsAppAction, actionBusy = false }: ContactSidebarProps) {
  const navigate = useNavigate();

  const isWa = ticket.origin === 'whatsapp';
  const isProfessional = linkedUser?.user_type === 'profesional';
  const currentStatusOption = {
    value: ticket.status,
    label: ticket.status === 'OnHold' ? 'On Hold' : ticket.status,
    stage: ticket.stage,
  };
  const options = statusOptions.some(option => option.value === ticket.status)
    ? statusOptions
    : [
        ...statusOptions,
        currentStatusOption,
      ].filter(option => option.value);
  const statusColor = (stage: Ticket['stage']) => {
    switch (stage) {
      case 'closed':
        return 'var(--gray-400)';
      case 'waiting':
        return 'var(--warning)';
      case 'in_progress':
        return 'var(--primary)';
      default:
        return 'var(--color-azure)';
    }
  };

  return (
    <aside className={styles.detailSidebar}>
      {/* ── Zia AI Insights Card ─────────────────────────── */}
      {(() => {
        const hasSentiment = !!ticket.sentiment;
        const hasTone = !!ticket.customer_tone;
        const hasAnyZia = hasSentiment || hasTone || ticket.is_escalated;

        const sentimentColor = (s?: string) => {
          if (!s) return 'var(--text-secondary)';
          const l = s.toLowerCase();
          if (l === 'positive') return '#10b981';
          if (l === 'negative') return '#ef4444';
          return '#f59e0b';
        };

        const sentimentEmoji = (s?: string) => {
          if (!s) return '😐';
          const l = s.toLowerCase();
          if (l === 'positive') return '😊';
          if (l === 'negative') return '😠';
          return '😐';
        };

        const toneEmoji = (t?: string) => {
          if (!t) return '💬';
          const l = t.toLowerCase();
          if (l.includes('positive') || l.includes('happy') || l.includes('satisfied')) return '😊';
          if (l.includes('negative') || l.includes('angry') || l.includes('frustrated')) return '😠';
          if (l.includes('neutral')) return '😐';
          if (l.includes('anxious') || l.includes('worried')) return '😟';
          return '💬';
        };

        return (
          <div style={{
            background: 'linear-gradient(135deg, rgba(99,102,241,0.06), rgba(139,92,246,0.04))',
            border: '1px solid rgba(99,102,241,0.15)',
            borderRadius: '10px',
            marginBottom: '12px',
            overflow: 'hidden',
          }}>
            {/* Header */}
            <div style={{
              display: 'flex', alignItems: 'center', gap: '8px',
              padding: '8px 12px',
              background: 'rgba(99,102,241,0.08)',
              borderBottom: '1px solid rgba(99,102,241,0.12)',
            }}>
              <Brain size={14} style={{ color: '#6366f1' }} />
              <span style={{ fontSize: '11px', fontWeight: 700, color: '#6366f1', textTransform: 'uppercase', letterSpacing: '0.5px' }}>
                Zia Insights
              </span>
            </div>

            <div style={{ padding: '10px 12px', display: 'flex', flexDirection: 'column', gap: '8px' }}>

              {/* Escalation alert inside Zia card */}
              {ticket.is_escalated && (
                <div style={{
                  display: 'flex', alignItems: 'center', gap: '6px',
                  background: 'rgba(239,68,68,0.1)', border: '1px solid rgba(239,68,68,0.25)',
                  borderRadius: '6px', padding: '5px 8px',
                }}>
                  <AlertCircle size={13} style={{ color: '#ef4444', flexShrink: 0 }} />
                  <span style={{ fontSize: '11px', fontWeight: 700, color: '#ef4444' }}>⚠️ Ticket Escalado</span>
                </div>
              )}

              {/* Sentiment row */}
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                <span style={{ fontSize: '11px', color: 'var(--text-secondary)', fontWeight: 600, display: 'flex', alignItems: 'center', gap: '5px' }}>
                  <TrendingUp size={12} />
                  Sentimiento
                </span>
                {hasSentiment ? (
                  <span style={{
                    fontSize: '11px', fontWeight: 700,
                    color: sentimentColor(ticket.sentiment),
                    background: `${sentimentColor(ticket.sentiment)}18`,
                    padding: '2px 8px', borderRadius: '99px',
                    border: `1px solid ${sentimentColor(ticket.sentiment)}33`,
                  }}>
                    {sentimentEmoji(ticket.sentiment)} {ticket.sentiment}
                  </span>
                ) : (
                  <span style={{ fontSize: '11px', color: 'var(--text-secondary)', fontStyle: 'italic' }}>Sin datos</span>
                )}
              </div>

              {/* Customer Tone row */}
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                <span style={{ fontSize: '11px', color: 'var(--text-secondary)', fontWeight: 600, display: 'flex', alignItems: 'center', gap: '5px' }}>
                  <MessageCircle size={12} />
                  Tono del cliente
                </span>
                {hasTone ? (
                  <span style={{
                    fontSize: '11px', fontWeight: 700,
                    color: '#8b5cf6',
                    background: 'rgba(139,92,246,0.1)',
                    padding: '2px 8px', borderRadius: '99px',
                    border: '1px solid rgba(139,92,246,0.25)',
                  }}>
                    {toneEmoji(ticket.customer_tone)} {ticket.customer_tone}
                  </span>
                ) : (
                  <span style={{ fontSize: '11px', color: 'var(--text-secondary)', fontStyle: 'italic' }}>Sin datos</span>
                )}
              </div>

              {!hasAnyZia && (
                <div style={{ fontSize: '11px', color: 'var(--text-secondary)', fontStyle: 'italic', textAlign: 'center', padding: '2px 0' }}>
                  Zia no analizó este ticket
                </div>
              )}
            </div>
          </div>
        );
      })()}

      {/* ── Linked User Profile Card ────────────────────────── */}
      {linkedUser ? (
        <div className={styles.sidebarSection}>
          <div style={{
            display: 'flex', alignItems: 'center', gap: '10px',
            padding: '10px 12px',
            marginBottom: '12px',
            background: isProfessional
              ? 'linear-gradient(135deg, rgba(99,102,241,0.08), rgba(139,92,246,0.08))'
              : 'linear-gradient(135deg, rgba(16,185,129,0.08), rgba(5,150,105,0.08))',
            borderRadius: '10px',
            border: `1px solid ${isProfessional ? 'rgba(99,102,241,0.2)' : 'rgba(16,185,129,0.2)'}`
          }}>
            <div style={{
              width: '40px', height: '40px', borderRadius: '50%', flexShrink: 0,
              background: isProfessional
                ? 'linear-gradient(135deg, #6366f1, #8b5cf6)'
                : 'linear-gradient(135deg, #10b981, #059669)',
              display: 'flex', alignItems: 'center', justifyContent: 'center',
              color: 'white', fontWeight: 700, fontSize: '15px'
            }}>
              {linkedUser.name.split(' ').map(w => w[0]).join('').toUpperCase().slice(0, 2)}
            </div>
            <div>
              <div style={{ fontWeight: 700, fontSize: '14px', color: 'var(--text-primary)' }}>
                {linkedUser.name}
              </div>
              <div style={{
                fontSize: '11px', fontWeight: 600,
                color: isProfessional ? '#6366f1' : '#10b981',
                textTransform: 'uppercase', letterSpacing: '0.5px'
              }}>
                {isProfessional ? '✦ Profesional' : '✦ Empresa'}
              </div>
            </div>
          </div>

          <div className={styles.contactDetails}>
            <div className={styles.contactItem}>
              <Phone size={15} />
              <div>
                <span className={styles.label}>Teléfono</span>
                <span className={styles.value}>{linkedUser.phone_number || ticket.contact?.phone || 'N/A'}</span>
              </div>
            </div>

            <div className={styles.contactItem}>
              <Mail size={15} />
              <div>
                <span className={styles.label}>Email</span>
                <span className={styles.value}>{linkedUser.email || 'N/A'}</span>
              </div>
            </div>

            {isProfessional && linkedUser.job_title && (
              <div className={styles.contactItem}>
                <Briefcase size={15} />
                <div>
                  <span className={styles.label}>Cargo</span>
                  <span className={styles.value}>{linkedUser.job_title}</span>
                </div>
              </div>
            )}

            {linkedUser.company_name && (
              <div className={styles.contactItem}>
                <Building2 size={15} />
                <div>
                  <span className={styles.label}>{isProfessional ? 'Empresa' : 'Sector'}</span>
                  <span className={styles.value}>{linkedUser.company_name}</span>
                </div>
              </div>
            )}

            {(linkedUser.city || linkedUser.country) && (
              <div className={styles.contactItem}>
                <MapPin size={15} />
                <div>
                  <span className={styles.label}>Ubicación</span>
                  <span className={styles.value}>
                    {[linkedUser.city, linkedUser.country].filter(Boolean).join(', ')}
                  </span>
                </div>
              </div>
            )}
          </div>

          {/* WhatsApp chat button */}
          <button
            onClick={() => navigate(`/whatsapp?ticketId=${encodeURIComponent(ticket.zoho_id)}`)}
            style={{
              display: 'flex', alignItems: 'center', gap: '8px',
              width: '100%', marginTop: '12px', padding: '9px 14px',
              borderRadius: '8px', border: 'none', cursor: 'pointer', fontWeight: 600,
              fontSize: '13px', justifyContent: 'center',
              background: 'linear-gradient(135deg, #25D366, #128C7E)',
              color: 'white', transition: 'opacity 0.2s'
            }}
            onMouseEnter={e => (e.currentTarget.style.opacity = '0.85')}
            onMouseLeave={e => (e.currentTarget.style.opacity = '1')}
          >
            <MessageCircle size={16} />
            Ir al chat de WhatsApp
          </button>
        </div>
      ) : (
        /* Fallback: basic contact info */
        <div className={styles.sidebarSection}>
          <h4 className={styles.sectionTitle}>Contacto</h4>
          <div className={styles.contactDetails}>
            <div className={styles.contactItem}>
              <User size={16} />
              <div>
                <span className={styles.label}>Nombre</span>
                <span className={styles.value}>{ticket.contact?.name || 'Desconocido'}</span>
              </div>
            </div>
            <div className={styles.contactItem}>
              <Phone size={16} />
              <div>
                <span className={styles.label}>Teléfono</span>
                <span className={styles.value}>{ticket.contact?.phone || 'N/A'}</span>
              </div>
            </div>
            <div className={styles.contactItem}>
              <Mail size={16} />
              <div>
                <span className={styles.label}>Email</span>
                <span className={styles.value}>{ticket.contact?.email || 'N/A'}</span>
              </div>
            </div>

            {ticket.contact?.company_name && (
              <div className={styles.contactItem}>
                <Building2 size={16} />
                <div>
                  <span className={styles.label}>Empresa</span>
                  <span className={styles.value}>{ticket.contact.company_name}</span>
                </div>
              </div>
            )}

            {!isWa && (
              <button
                onClick={() => navigate(`/whatsapp?ticketId=${encodeURIComponent(ticket.zoho_id)}`)}
                style={{
                  display: 'flex', alignItems: 'center', gap: '8px',
                  width: '100%', marginTop: '12px', padding: '9px 14px',
                  borderRadius: '8px', border: 'none', cursor: 'pointer', fontWeight: 600,
                  fontSize: '13px', justifyContent: 'center',
                  background: 'linear-gradient(135deg, #25D366, #128C7E)',
                  color: 'white', transition: 'opacity 0.2s'
                }}
                onMouseEnter={e => (e.currentTarget.style.opacity = '0.85')}
                onMouseLeave={e => (e.currentTarget.style.opacity = '1')}
              >
                <MessageCircle size={16} />
                Ir al chat de WhatsApp
              </button>
            )}
          </div>
        </div>
      )}

      <hr className={styles.divider} />

      {/* ── Ticket Owner (Assignee) ──────────────────────────── */}
      {ticket.assignee_name && (
        <div className={styles.sidebarSection}>
          <h4 className={styles.sectionTitle} style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
            <Shield size={14} style={{ color: 'var(--primary)' }} />
            Propietario del Ticket
          </h4>
          <div style={{
            background: 'linear-gradient(135deg, rgba(99,102,241,0.06), rgba(139,92,246,0.06))',
            border: '1px solid rgba(99,102,241,0.18)',
            borderRadius: '10px', padding: '10px 12px',
          }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '10px', marginBottom: '8px' }}>
              <div style={{
                width: 32, height: 32, borderRadius: '50%', flexShrink: 0,
                background: 'linear-gradient(135deg, #6366f1, #8b5cf6)',
                display: 'flex', alignItems: 'center', justifyContent: 'center',
                color: 'white', fontWeight: 700, fontSize: '13px'
              }}>
                {ticket.assignee_name.split(' ').map((w: string) => w[0]).join('').toUpperCase().slice(0, 2)}
              </div>
              <div>
                <div style={{ fontWeight: 700, fontSize: '13px', color: 'var(--text-primary)' }}>
                  {ticket.assignee_name}
                </div>
                <div style={{ fontSize: '11px', color: 'var(--primary)', fontWeight: 600 }}>
                  {isWa ? 'Agente de soporte' : 'Agente Zoho Desk'}
                </div>
              </div>
            </div>
            {ticket.assignee_email && (
              <div className={styles.contactItem} style={{ marginBottom: 0 }}>
                <Mail size={13} style={{ color: 'var(--primary)', flexShrink: 0 }} />
                <div>
                  <span className={styles.label}>Email del agente</span>
                  <a
                    href={`mailto:${ticket.assignee_email}`}
                    style={{ fontSize: '0.78rem', color: 'var(--primary)', textDecoration: 'none', fontWeight: 600, display: 'block' }}
                  >
                    {ticket.assignee_email}
                  </a>
                </div>
              </div>
            )}
          </div>
        </div>
      )}

      <hr className={styles.divider} />

      {/* ── Stage & Meta ────────────────────────────────────── */}
      <div className={styles.sidebarSection}>
        <h4 className={styles.sectionTitle}>Estado de Gestión</h4>
        <div className={styles.controlGroup}>
          <label className={styles.fieldLabel} htmlFor="stage-select">Etapa actual</label>
          {isWa ? (
            <>
              <span style={{
                display: 'inline-flex', alignItems: 'center', alignSelf: 'flex-start',
                fontSize: '0.78rem', fontWeight: 700, padding: '0.28rem 0.75rem',
                borderRadius: '99px',
                border: `1px solid ${STAGE_META[ticket.stage]?.color ?? 'var(--color-azure)'}`,
                color: STAGE_META[ticket.stage]?.color ?? 'var(--color-azure)',
                background: `${STAGE_META[ticket.stage]?.color ?? 'var(--color-azure)'}18`,
              }}>
                {STAGE_META[ticket.stage]?.label ?? 'Nuevo'}
              </span>

              <div style={{ display: 'flex', flexDirection: 'column', gap: 8, marginTop: 12 }}>
                {ticket.stage !== 'closed' && !ticket.assigned_to && (
                  <button
                    onClick={() => onWhatsAppAction?.('claim')}
                    disabled={actionBusy}
                    style={{
                      display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 8,
                      width: '100%', padding: '9px 14px', borderRadius: '8px',
                      border: '1px solid var(--primary)', cursor: 'pointer', fontWeight: 600,
                      fontSize: '13px', background: 'transparent', color: 'var(--primary)',
                    }}
                  >
                    <Hand size={15} /> Tomar
                  </button>
                )}
                {ticket.stage !== 'closed' && (
                  <button
                    onClick={() => onWhatsAppAction?.('resolve')}
                    disabled={actionBusy}
                    style={{
                      display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 8,
                      width: '100%', padding: '9px 14px', borderRadius: '8px',
                      border: '1px solid rgba(22,163,74,0.4)', cursor: 'pointer', fontWeight: 600,
                      fontSize: '13px', background: 'transparent', color: '#15803d',
                    }}
                  >
                    <CheckCircle2 size={15} /> Resolver
                  </button>
                )}
                {ticket.stage === 'closed' && (
                  <button
                    onClick={() => onWhatsAppAction?.('reopen')}
                    disabled={actionBusy}
                    style={{
                      display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 8,
                      width: '100%', padding: '9px 14px', borderRadius: '8px',
                      border: '1px solid var(--primary)', cursor: 'pointer', fontWeight: 600,
                      fontSize: '13px', background: 'transparent', color: 'var(--primary)',
                    }}
                  >
                    <RotateCcw size={15} /> Reabrir
                  </button>
                )}
              </div>
            </>
          ) : (
            <>
              <Select
                id="stage-select"
                value={ticket.status || ''}
                onChange={(value) => onStatusChange(String(value))}
                options={options.map(option => ({
                  value: option.value,
                  label: option.label,
                  color: statusColor(option.stage),
                }))}
                placeholder="Seleccionar estado"
                fullWidth
                disabled={updatingStage}
              />
              {updatingStage && (
                <span className={styles.label} style={{ marginTop: 6, display: 'block' }}>
                  Actualizando en Zoho Desk...
                </span>
              )}
              {stageError && (
                <span style={{ marginTop: 6, display: 'block', color: '#dc2626', fontSize: '0.78rem', fontWeight: 600 }}>
                  {stageError}
                </span>
              )}
              {onTransfer && (
                <button
                  onClick={onTransfer}
                  style={{
                    display: 'flex', alignItems: 'center', justifyContent: 'center', gap: '8px',
                    width: '100%', marginTop: '12px', padding: '9px 14px',
                    borderRadius: '8px', border: '1px solid var(--primary)', cursor: 'pointer', fontWeight: 600,
                    fontSize: '13px', background: 'transparent', color: 'var(--primary)',
                  }}
                >
                  <ArrowRightLeft size={15} />
                  Traspasar ticket
                </button>
              )}
            </>
          )}
        </div>

        <div className={styles.ticketMeta}>
          {ticket.ticket_number && (
            <div className={styles.metaRow}>
              <AlertCircle size={14} />
              <span>Ticket #{ticket.ticket_number}</span>
            </div>
          )}
          <div className={styles.metaRow}>
            <Clock size={14} />
            <span>Creado: {new Date(ticket.created_at).toLocaleDateString('es-AR', { day: '2-digit', month: '2-digit', year: 'numeric' })}</span>
          </div>
          {ticket.status && (
            <div className={styles.metaRow}>
              <span style={{ width: 14 }}>●</span>
              <span>{isWa ? 'Estado' : 'Estado Zoho'}: <strong>{isWa ? (STAGE_META[ticket.stage]?.label ?? ticket.status) : ticket.status}</strong></span>
            </div>
          )}
          {ticket.priority && (
            <div className={styles.metaRow}>
              <span style={{
                width: 14, height: 14, borderRadius: '50%', display: 'inline-block', flexShrink: 0,
                background: ticket.priority === 'High' ? '#ef4444'
                  : ticket.priority === 'Medium' ? '#f59e0b'
                  : ticket.priority === 'Low' ? '#10b981'
                  : 'var(--gray-400)',
              }} />
              <span>Prioridad: <strong>{ticket.priority}</strong></span>
            </div>
          )}
          {ticket.category && (
            <div className={styles.metaRow}>
              <span style={{ width: 14 }}>📂</span>
              <span>Categoría: {ticket.category}</span>
            </div>
          )}
        </div>

        {ticket.description && (
          <div style={{ marginTop: '10px' }}>
            <span className={styles.fieldLabel}>Descripción</span>
            <p style={{
              margin: '4px 0 0', fontSize: '0.8rem', color: 'var(--text-secondary)',
              lineHeight: 1.5, maxHeight: '80px', overflowY: 'auto',
              padding: '6px 8px', background: 'var(--bg-secondary)',
              borderRadius: '6px', border: '1px solid var(--border)',
            }}>
              {ticket.description}
            </p>
          </div>
        )}
      </div>
    </aside>
  );
}
