import { useEffect, useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { Ticket, LinkedUser, TicketStatusOption, ticketService } from '../../services/ticket.service';
import ContactSidebar from './components/ContactSidebar';
import MessageTimeline from './components/MessageTimeline';
import ChatInputArea from './components/ChatInputArea';
import styles from './Tickets.module.css';
import { ArrowLeft, ExternalLink, RefreshCw } from 'lucide-react';

const STAGE_LABELS: Record<string, { label: string; color: string }> = {
  new:         { label: 'Nuevo',        color: 'var(--color-azure)' },
  in_progress: { label: 'En Progreso',  color: 'var(--primary)' },
  waiting:     { label: 'Esperando',    color: 'var(--warning)' },
  closed:      { label: 'Cerrado',      color: 'var(--gray-400)' },
};

const DEFAULT_STATUS_OPTIONS: TicketStatusOption[] = [
  { value: 'Open', label: 'Open', status_type: 'Open', stage: 'in_progress' },
  { value: 'OnHold', label: 'On Hold', status_type: 'OnHold', stage: 'waiting' },
  { value: 'Escalated', label: 'Escalated', status_type: 'Open', stage: 'in_progress' },
  { value: 'Closed', label: 'Closed', status_type: 'Closed', stage: 'closed' },
];

function mergeStatusOptions(...groups: TicketStatusOption[][]): TicketStatusOption[] {
  const seen = new Set<string>();
  return groups.flat().filter(option => {
    const key = option.value.trim().toLowerCase();
    if (!key || seen.has(key)) return false;
    seen.add(key);
    return true;
  });
}

export default function TicketDetail() {
  // The route param is now the real Zoho Desk ticket ID string
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [ticket, setTicket]         = useState<Ticket | null>(null);
  const [linkedUser, setLinkedUser] = useState<LinkedUser | null>(null);
  const [loading, setLoading]       = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [sendError, setSendError]   = useState<string | null>(null);
  const [stageError, setStageError] = useState<string | null>(null);
  const [updatingStage, setUpdatingStage] = useState(false);
  const [statusOptions, setStatusOptions] = useState<TicketStatusOption[]>(DEFAULT_STATUS_OPTIONS);

  useEffect(() => {
    if (id) fetchTicket(id, false);
  }, [id]);

  useEffect(() => {
    const fetchStatuses = async () => {
      try {
        const statuses = await ticketService.getTicketStatuses();
        setStatusOptions(mergeStatusOptions(DEFAULT_STATUS_OPTIONS, statuses ?? []));
      } catch (error) {
        console.error('Error fetching ticket statuses:', error);
        setStatusOptions(DEFAULT_STATUS_OPTIONS);
      }
    };
    fetchStatuses();
  }, []);

  const fetchTicket = async (zohoId: string, silent: boolean) => {
    if (!silent) setLoading(true);
    else setRefreshing(true);
    try {
      const data = await ticketService.getTicket(decodeURIComponent(zohoId));
      setTicket(data.ticket);
      setLinkedUser(data.linked_user);
    } catch (error) {
      console.error('Error fetching ticket:', error);
    } finally {
      setLoading(false);
      setRefreshing(false);
    }
  };

  const handleSendResponse = async (content: string, channel: 'whatsapp' | 'email') => {
    if (!ticket) return;
    setSendError(null);
    try {
      const newMessage = await ticketService.sendMessage(ticket.zoho_id, content, channel);
      setTicket(prev => prev ? {
        ...prev,
        messages: [...(prev.messages || []), newMessage],
      } : null);
    } catch (error: any) {
      const msg = error?.response?.data?.error || 'No se pudo enviar el mensaje. Intenta de nuevo.';
      setSendError(msg);
      throw error;
    }
  };

  const handleStatusChange = async (newStatus: string) => {
    if (!ticket) return;
    const previousTicket = ticket;
    const selectedOption = statusOptions.find(option => option.value === newStatus);
    setStageError(null);
    setUpdatingStage(true);
    setTicket(prev => prev ? {
      ...prev,
      status: newStatus,
      stage: selectedOption?.stage ?? prev.stage,
    } : null);
    try {
      const updatedTicket = await ticketService.updateTicket(ticket.zoho_id, { status: newStatus });
      setTicket(prev => prev ? { ...prev, ...updatedTicket } : updatedTicket);
    } catch (error: any) {
      console.error('Error updating ticket status:', error);
      setTicket(previousTicket);
      setStageError(error?.response?.data?.error || 'No se pudo actualizar el estado en Zoho Desk.');
    } finally {
      setUpdatingStage(false);
    }
  };

  /* ── Loading ── */
  if (loading) {
    return (
      <div className={styles.container} style={{ justifyContent: 'center', alignItems: 'center', paddingTop: '4rem' }}>
        <p style={{ color: 'var(--gray-400)', fontSize: '1rem' }}>Cargando ticket desde Zoho Desk…</p>
      </div>
    );
  }

  /* ── Not Found ── */
  if (!ticket) {
    return (
      <div className={styles.container} style={{ justifyContent: 'center', alignItems: 'center', paddingTop: '4rem' }}>
        <p style={{ color: 'var(--danger)', fontSize: '1rem' }}>Ticket no encontrado.</p>
        <button onClick={() => navigate('/tickets')} className={styles.sendBtn} style={{ marginTop: '1rem' }}>
          Volver a Tickets
        </button>
      </div>
    );
  }

  const stageMeta = STAGE_LABELS[ticket.stage] ?? STAGE_LABELS['new'];
  const zohoId = ticket.zoho_id;

  return (
    <div className={styles.container}>
      {/* ── Header ── */}
      <div className={styles.header} style={{ marginBottom: '0.5rem' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem', flexWrap: 'wrap' }}>
          <button
            onClick={() => navigate('/tickets')}
            className={styles.channelBtn}
            style={{ padding: '0.5rem', borderRadius: '50%', flexShrink: 0 }}
            aria-label="Volver"
          >
            <ArrowLeft size={17} />
          </button>

          <h1 style={{ margin: 0, fontSize: '1.35rem' }}>
            {ticket.title}
          </h1>

          {/* Stage badge */}
          <span style={{
            fontSize: '0.75rem', fontWeight: 700, padding: '0.25rem 0.7rem',
            borderRadius: '99px', border: `1px solid ${stageMeta.color}`,
            color: stageMeta.color, background: `${stageMeta.color}18`,
          }}>
            {stageMeta.label}
          </span>

          {/* Channel badge */}
          {ticket.channel && (
            <span style={{
              fontSize: '0.72rem', fontWeight: 600, padding: '0.2rem 0.6rem',
              borderRadius: '99px', background: 'rgba(99,102,241,0.1)',
              color: 'var(--primary)', border: '1px solid rgba(99,102,241,0.2)',
            }}>
              {ticket.channel}
            </span>
          )}

          {/* Zoho ID link */}
          {(ticket.web_url || zohoId) && (
            <a
              href={ticket.web_url || `https://desk.zoho.com/support/oberstaff/ShowHomePage.do#Cases/dv/${zohoId}`}
              target="_blank"
              rel="noopener noreferrer"
              style={{
                fontSize: '0.75rem', color: 'var(--gray-400)', display: 'flex',
                alignItems: 'center', gap: '0.25rem', textDecoration: 'none',
              }}
              title="Abrir en Zoho Desk"
            >
              <ExternalLink size={12} />
              Ver en Zoho Desk
            </a>
          )}
        </div>

        {/* Refresh */}
        <button
          onClick={() => id && fetchTicket(id, true)}
          className={styles.channelBtn}
          disabled={refreshing}
          style={{ display: 'flex', alignItems: 'center', gap: '0.4rem', padding: '0.5rem 0.9rem' }}
        >
          <RefreshCw size={13} style={{ animation: refreshing ? 'spin 1s linear infinite' : 'none' }} />
          {refreshing ? 'Actualizando…' : 'Actualizar'}
        </button>
      </div>

      {/* ── Send error banner ── */}
      {sendError && (
        <div style={{
          padding: '0.75rem 1.1rem', borderRadius: 'var(--radius-sm)',
          background: 'rgba(239,68,68,0.08)', border: '1px solid rgba(239,68,68,0.2)',
          color: '#dc2626', fontSize: '0.875rem', display: 'flex', justifyContent: 'space-between',
        }}>
          ⚠️ {sendError}
          <button
            onClick={() => setSendError(null)}
            style={{ background: 'none', border: 'none', cursor: 'pointer', color: '#dc2626', fontWeight: 700 }}
          >✕</button>
        </div>
      )}

      {/* ── Detail body ── */}
      <div className={styles.detailContainer}>
        {/* Sidebar */}
        <ContactSidebar
          ticket={ticket}
          linkedUser={linkedUser}
          statusOptions={statusOptions}
          onStatusChange={handleStatusChange}
          stageError={stageError}
          updatingStage={updatingStage}
        />

        {/* Chat area */}
        <div className={styles.detailMain}>
          <MessageTimeline ticket={ticket} contact={ticket.contact} />
          <ChatInputArea onSend={handleSendResponse} />
        </div>
      </div>

      <style>{`
        @keyframes spin { from { transform: rotate(0deg); } to { transform: rotate(360deg); } }
      `}</style>
    </div>
  );
}
