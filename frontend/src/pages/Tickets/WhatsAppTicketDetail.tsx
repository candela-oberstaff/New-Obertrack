import { useCallback, useEffect, useRef, useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { Ticket, ticketService } from '../../services/ticket.service';
import MessageTimeline from './components/MessageTimeline';
import ChatInputArea from './components/ChatInputArea';
import ContactSidebar from './components/ContactSidebar';
import styles from './Tickets.module.css';
import { ArrowLeft, RefreshCw, MessageSquare } from 'lucide-react';
import { useAuth } from '../../context/AuthContext';
import { useNotification } from '../../context/NotificationContext';
import { canEditModule } from '../../lib/permissions';

const STAGE_LABELS: Record<string, { label: string; color: string }> = {
  new:         { label: 'Nuevo',       color: 'var(--color-azure)' },
  in_progress: { label: 'En Progreso', color: 'var(--primary)' },
  waiting:     { label: 'Esperando',   color: 'var(--warning)' },
  closed:      { label: 'Cerrado',     color: 'var(--gray-400)' },
};

const ACTION_LABELS: Record<'claim' | 'resolve' | 'reopen', string> = {
  claim: 'Tomaste el ticket.',
  resolve: 'Ticket resuelto.',
  reopen: 'Ticket reabierto.',
};

export default function WhatsAppTicketDetail() {
  const { id } = useParams<{ id: string }>();
  const numericId = Number(id);
  const navigate = useNavigate();
  const { user } = useAuth();
  const { error: showError, success: showSuccess } = useNotification();

  const [ticket, setTicket] = useState<Ticket | null>(null);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [acting, setActing] = useState(false);
  const [sendError, setSendError] = useState<string | null>(null);
  const pollRef = useRef<number | null>(null);

  const fetchTicket = useCallback(async (silent: boolean) => {
    if (!numericId) return;
    if (!silent) setLoading(true); else setRefreshing(true);
    try {
      const data = await ticketService.getWhatsAppTicket(numericId);
      setTicket(data);
    } catch (err) {
      console.error('Error fetching WhatsApp ticket:', err);
    } finally {
      setLoading(false);
      setRefreshing(false);
    }
  }, [numericId]);

  useEffect(() => { fetchTicket(false); }, [fetchTicket]);

  useEffect(() => {
    pollRef.current = window.setInterval(() => fetchTicket(true), 15_000);
    return () => { if (pollRef.current) window.clearInterval(pollRef.current); };
  }, [fetchTicket]);

  const handleSend = async (content: string) => {
    if (!ticket) return;
    setSendError(null);
    try {
      const msg = await ticketService.sendWhatsAppReply(ticket.id, content);
      setTicket(prev => prev ? { ...prev, messages: [...(prev.messages || []), msg] } : prev);
    } catch (err: any) {
      const m = err?.response?.data?.error || 'No se pudo enviar el mensaje por WhatsApp.';
      setSendError(m);
      throw err;
    }
  };

  const runAction = async (action: 'claim' | 'resolve' | 'reopen') => {
    if (!ticket) return;
    setActing(true);
    try {
      const updated = await ticketService.whatsAppAction(ticket.id, action);
      setTicket(prev => prev ? { ...prev, ...updated } : updated);
      showSuccess(ACTION_LABELS[action]);
    } catch (err: any) {
      showError(err?.response?.data?.error || 'No se pudo actualizar el ticket.');
    } finally {
      setActing(false);
    }
  };

  if (loading) {
    return (
      <div className={styles.container} style={{ justifyContent: 'center', alignItems: 'center', paddingTop: '4rem' }}>
        <p style={{ color: 'var(--gray-400)', fontSize: '1rem' }}>Cargando conversación…</p>
      </div>
    );
  }

  if (!ticket) {
    return (
      <div className={styles.container} style={{ justifyContent: 'center', alignItems: 'center', paddingTop: '4rem' }}>
        <p style={{ color: 'var(--danger)', fontSize: '1rem' }}>Conversación no encontrada.</p>
        <button onClick={() => navigate('/tickets/soporte')} className={styles.sendBtn} style={{ marginTop: '1rem' }}>
          Volver a Soporte
        </button>
      </div>
    );
  }

  const stageMeta = STAGE_LABELS[ticket.stage] ?? STAGE_LABELS['new'];
  const contactName = ticket.contact?.name || 'Contacto WhatsApp';
  const canEdit = canEditModule(user, 'tickets');

  return (
    <div className={styles.container}>
      <div className={styles.header} style={{ marginBottom: '0.5rem' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem', flexWrap: 'wrap' }}>
          <button
            onClick={() => navigate('/tickets/soporte')}
            className={styles.channelBtn}
            style={{ padding: '0.5rem', borderRadius: '50%', flexShrink: 0 }}
            aria-label="Volver"
          >
            <ArrowLeft size={17} />
          </button>

          <h1 style={{ margin: 0, fontSize: '1.35rem' }}>{contactName}</h1>

          <span style={{
            fontSize: '0.75rem', fontWeight: 700, padding: '0.25rem 0.7rem',
            borderRadius: '99px', border: `1px solid ${stageMeta.color}`,
            color: stageMeta.color, background: `${stageMeta.color}18`,
          }}>
            {stageMeta.label}
          </span>

          <span style={{
            display: 'inline-flex', alignItems: 'center', gap: 4,
            fontSize: '0.72rem', fontWeight: 700, padding: '0.2rem 0.6rem',
            borderRadius: '99px', background: 'rgba(37,211,102,0.12)',
            color: '#15803d', border: '1px solid rgba(37,211,102,0.25)',
          }}>
            <MessageSquare size={12} /> WhatsApp
          </span>
        </div>

        <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
          <button
            onClick={() => fetchTicket(true)}
            className={styles.channelBtn}
            disabled={refreshing}
            style={{ display: 'flex', alignItems: 'center', gap: '0.4rem', padding: '0.5rem 0.9rem' }}
          >
            <RefreshCw size={13} style={{ animation: refreshing ? 'spin 1s linear infinite' : 'none' }} />
            {refreshing ? 'Actualizando…' : 'Actualizar'}
          </button>
        </div>
      </div>

      {sendError && (
        <div style={{
          padding: '0.75rem 1.1rem', borderRadius: 'var(--radius-sm)',
          background: 'rgba(239,68,68,0.08)', border: '1px solid rgba(239,68,68,0.2)',
          color: '#dc2626', fontSize: '0.875rem', display: 'flex', justifyContent: 'space-between',
        }}>
          ⚠️ {sendError}
          <button onClick={() => setSendError(null)} style={{ background: 'none', border: 'none', cursor: 'pointer', color: '#dc2626', fontWeight: 700 }}>✕</button>
        </div>
      )}

      <div className={styles.detailContainer}>
        <ContactSidebar
          ticket={ticket}
          linkedUser={null}
          statusOptions={[]}
          onStatusChange={() => {}}
          onWhatsAppAction={runAction}
          actionBusy={acting}
        />

        <div className={styles.detailMain}>
          <MessageTimeline ticket={ticket} contact={ticket.contact} />
          {canEdit ? (
            <ChatInputArea onSend={(content) => handleSend(content)} />
          ) : (
            <div style={{ padding: '0.85rem 1.1rem', borderTop: '1px solid var(--glass-border, #e2e8f0)', fontSize: '0.85rem', color: 'var(--gray-400)', textAlign: 'center' }}>
              Tu rol tiene acceso de solo lectura en Tickets
            </div>
          )}
        </div>
      </div>

      <style>{`
        @keyframes spin { from { transform: rotate(0deg); } to { transform: rotate(360deg); } }
      `}</style>
    </div>
  );
}
