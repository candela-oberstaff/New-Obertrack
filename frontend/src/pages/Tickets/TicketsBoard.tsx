import { useCallback, useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Ticket, ticketService } from '../../services/ticket.service';
import TicketColumn from './components/TicketColumn';
import styles from './Tickets.module.css';
import { RefreshCw, Ticket as TicketIcon, Filter, User as UserIcon } from 'lucide-react';
import { useAuth } from '../../context/AuthContext';

const STAGES = [
  { id: 'new',         title: 'Nuevo' },
  { id: 'in_progress', title: 'En Progreso' },
  { id: 'waiting',     title: 'Esperando' },
  { id: 'closed',      title: 'Cerrado' },
] as const;

export default function TicketsBoard() {
  const { user } = useAuth();
  const [tickets, setTickets]     = useState<Ticket[]>([]);
  const [loading, setLoading]     = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [lastRefresh, setLastRefresh] = useState<Date | null>(null);
  const [error, setError]         = useState<string | null>(null);
  const [filterOwn, setFilterOwn] = useState(false);
  const navigate = useNavigate();

  const fetchTickets = useCallback(async (silent = false) => {
    if (!silent) setLoading(true);
    else setRefreshing(true);
    setError(null);
    try {
      const data = await ticketService.getTickets();
      setTickets(data ?? []);
      setLastRefresh(new Date());
    } catch (err: any) {
      setError(err?.response?.data?.error ?? 'No se pudieron cargar los tickets.');
    } finally {
      setLoading(false);
      setRefreshing(false);
    }
  }, []);

  // Initial load
  useEffect(() => {
    fetchTickets();
  }, [fetchTickets]);

  // Auto-refresh every 60 seconds
  useEffect(() => {
    const interval = setInterval(() => fetchTickets(true), 60_000);
    return () => clearInterval(interval);
  }, [fetchTickets]);

  const openTicket = (zohoId: string) => navigate(`/tickets/${encodeURIComponent(zohoId)}`);

  const filteredTickets = tickets.filter(t => {
    if (filterOwn && user?.email) {
      return t.assignee_email?.toLowerCase() === user.email.toLowerCase();
    }
    return true;
  });

  const total = filteredTickets.length;

  return (
    <div className={styles.container}>
      {/* ── Header ── */}
      <div className={styles.header}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem' }}>
          <TicketIcon size={22} style={{ color: 'var(--primary)' }} />
          <h1>Tickets de Soporte</h1>
          {total > 0 && (
            <span style={{
              fontSize: '0.8rem', fontWeight: 600, padding: '0.25rem 0.65rem',
              borderRadius: '99px', background: 'rgba(204,51,204,0.12)',
              color: 'var(--primary)',
            }}>
              Total {total} 
            </span>
          )}
        </div>

        <div style={{ display: 'flex', alignItems: 'center', gap: '1rem' }}>
          {/* Filter Toggle */}
          {user?.email && (
            <div style={{
              display: 'flex',
              background: 'var(--bg-secondary)',
              border: '1px solid var(--border)',
              borderRadius: '8px',
              padding: '2px',
            }}>
              <button
                onClick={() => setFilterOwn(false)}
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: '6px',
                  border: 'none',
                  padding: '6px 12px',
                  borderRadius: '6px',
                  fontSize: '0.82rem',
                  fontWeight: 600,
                  cursor: 'pointer',
                  background: !filterOwn ? 'var(--bg-primary)' : 'transparent',
                  color: !filterOwn ? 'var(--text-primary)' : 'var(--text-secondary)',
                  boxShadow: !filterOwn ? '0 1px 3px rgba(0,0,0,0.1)' : 'none',
                  transition: 'all 0.2s',
                }}
              >
                <Filter size={13} />
                Todos
              </button>
              <button
                onClick={() => setFilterOwn(true)}
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: '6px',
                  border: 'none',
                  padding: '6px 12px',
                  borderRadius: '6px',
                  fontSize: '0.82rem',
                  fontWeight: 600,
                  cursor: 'pointer',
                  background: filterOwn ? 'var(--bg-primary)' : 'transparent',
                  color: filterOwn ? 'var(--text-primary)' : 'var(--text-secondary)',
                  boxShadow: filterOwn ? '0 1px 3px rgba(0,0,0,0.1)' : 'none',
                  transition: 'all 0.2s',
                }}
              >
                <UserIcon size={13} />
                Mis Tickets
              </button>
            </div>
          )}

          {lastRefresh && (
            <span style={{ fontSize: '0.75rem', color: 'var(--gray-400)' }}>
              Actualizado {lastRefresh.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
            </span>
          )}
          <button
            onClick={() => fetchTickets(true)}
            className={styles.channelBtn}
            disabled={refreshing}
            style={{ display: 'flex', alignItems: 'center', gap: '0.4rem', padding: '0.55rem 1rem' }}
          >
            <RefreshCw size={14} style={{ animation: refreshing ? 'spin 1s linear infinite' : 'none' }} />
            {refreshing ? 'Actualizando…' : 'Actualizar'}
          </button>
        </div>
      </div>

      {/* ── Error ── */}
      {error && (
        <div style={{
          padding: '0.85rem 1.25rem', borderRadius: 'var(--radius-sm)',
          background: 'rgba(239,68,68,0.08)', border: '1px solid rgba(239,68,68,0.2)',
          color: '#dc2626', fontSize: '0.9rem',
        }}>
          ⚠️ {error}
        </div>
      )}

      {/* ── Board ── */}
      {loading ? (
        <div style={{ display: 'flex', justifyContent: 'center', padding: '4rem', color: 'var(--gray-400)' }}>
          Cargando tickets desde Zoho Desk…
        </div>
      ) : (
        <div className={styles.board}>
          {STAGES.map((stage) => (
            <TicketColumn
              key={stage.id}
              title={stage.title}
              stage={stage.id}
              tickets={filteredTickets.filter(t => t.stage === stage.id)}
              onTicketClick={openTicket}
            />
          ))}
        </div>
      )}

      <style>{`
        @keyframes spin { from { transform: rotate(0deg); } to { transform: rotate(360deg); } }
      `}</style>
    </div>
  );
}
