import { useCallback, useEffect, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import { Ticket, ticketService } from '../../services/ticket.service';
import TicketColumn from './components/TicketColumn';
import styles from './Tickets.module.css';
import { RefreshCw, Ticket as TicketIcon, Filter, User as UserIcon, FileText } from 'lucide-react';
import { useAuth } from '../../context/AuthContext';
import { isSupportManager } from '../../lib/permissions';

type OriginFilter = 'all' | 'zoho' | 'internal';

const STAGES = [
  { id: 'new', title: 'Nuevo' },
  { id: 'in_progress', title: 'En Progreso' },
  { id: 'waiting', title: 'Esperando' },
  { id: 'closed', title: 'Cerrado' },
] as const;

const STAGE_ORDER_STORAGE_KEY = 'ticketsStageOrder';

type TicketStageConfig = typeof STAGES[number];

function getInitialStageOrder(): TicketStageConfig[] {
  try {
    const saved = localStorage.getItem(STAGE_ORDER_STORAGE_KEY);
    if (!saved) return [...STAGES];

    const savedIds = JSON.parse(saved);
    if (!Array.isArray(savedIds)) return [...STAGES];

    const ordered = savedIds
      .map((id: unknown) => STAGES.find(stage => stage.id === id))
      .filter((stage): stage is TicketStageConfig => Boolean(stage));
    const missing = STAGES.filter(stage => !ordered.some(item => item.id === stage.id));

    return [...ordered, ...missing];
  } catch {
    return [...STAGES];
  }
}

export default function TicketsBoard() {
  const { user } = useAuth();
  const [stages, setStages] = useState<TicketStageConfig[]>(getInitialStageOrder);
  const [draggedStageIdx, setDraggedStageIdx] = useState<number | null>(null);
  const [dragOverStageIdx, setDragOverStageIdx] = useState<number | null>(null);
  const [filterOwn, setFilterOwn] = useState(false);
  const [filterOrigin, setFilterOrigin] = useState<OriginFilter>('all');
  const navigate = useNavigate();

  // Tickets auto-refresh every 60s via React Query's polling.
  const { data: tickets = [], isLoading: loading, isFetching, error: queryError, refetch, dataUpdatedAt } = useQuery({
    queryKey: ['tickets'],
    queryFn: async () => (await ticketService.getTickets()) ?? [],
    refetchInterval: 60_000,
  });
  const refreshing = isFetching && !loading;
  const lastRefresh = dataUpdatedAt ? new Date(dataUpdatedAt) : null;
  const error = queryError ? ((queryError as any)?.response?.data?.error ?? 'No se pudieron cargar los tickets.') : null;

  useEffect(() => {
    localStorage.setItem(STAGE_ORDER_STORAGE_KEY, JSON.stringify(stages.map(stage => stage.id)));
  }, [stages]);

  const handleStageDragStart = useCallback((idx: number) => {
    setDraggedStageIdx(idx);
  }, []);

  const handleStageDragEnter = useCallback((idx: number) => {
    setDragOverStageIdx(prev => (prev === idx ? prev : idx));
  }, []);

  const handleStageDrop = useCallback((idx: number) => {
    if (draggedStageIdx !== null && draggedStageIdx !== idx) {
      setStages(current => {
        const next = [...current];
        const [dragged] = next.splice(draggedStageIdx, 1);
        next.splice(idx, 0, dragged);
        return next;
      });
    }
    setDraggedStageIdx(null);
    setDragOverStageIdx(null);
  }, [draggedStageIdx]);

  const handleStageDragEnd = useCallback(() => {
    setDraggedStageIdx(null);
    setDragOverStageIdx(null);
  }, []);

  const openTicket = (ticket: Ticket) => {
    if (ticket.origin === 'internal') {
      navigate(`/tickets/internal/${ticket.id}`);
      return;
    }
    navigate(`/tickets/${encodeURIComponent(ticket.zoho_id)}`);
  };

  const filteredTickets = tickets.filter(t => {
    if (filterOrigin !== 'all' && (t.origin ?? 'zoho') !== filterOrigin) {
      return false;
    }
    if (filterOwn && user?.email) {
      return t.assignee_email?.toLowerCase() === user.email.toLowerCase();
    }
    return true;
  });

  const total = filteredTickets.length;

  return (
    <div className={styles.container}>
      <div className={styles.header}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem' }}>
          <TicketIcon size={22} style={{ color: 'var(--primary)' }} />
          <h1>Tickets de Soporte</h1>
          {total > 0 && (
            <span style={{
              fontSize: '0.8rem',
              fontWeight: 600,
              padding: '0.25rem 0.65rem',
              borderRadius: '99px',
              background: 'rgba(204,51,204,0.12)',
              color: 'var(--primary)',
            }}>
              Total {total}
            </span>
          )}
        </div>

        <div style={{ display: 'flex', alignItems: 'center', gap: '1rem' }}>
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

          <div style={{
            display: 'flex',
            background: 'var(--bg-secondary)',
            border: '1px solid var(--border)',
            borderRadius: '8px',
            padding: '2px',
          }}>
            {([
              { id: 'all', label: 'Todos' },
              { id: 'zoho', label: 'Zoho' },
              { id: 'internal', label: 'Internos' },
            ] as { id: OriginFilter; label: string }[]).map(opt => (
              <button
                key={opt.id}
                onClick={() => setFilterOrigin(opt.id)}
                style={{
                  border: 'none',
                  padding: '6px 12px',
                  borderRadius: '6px',
                  fontSize: '0.82rem',
                  fontWeight: 600,
                  cursor: 'pointer',
                  background: filterOrigin === opt.id ? 'var(--bg-primary)' : 'transparent',
                  color: filterOrigin === opt.id ? 'var(--text-primary)' : 'var(--text-secondary)',
                  boxShadow: filterOrigin === opt.id ? '0 1px 3px rgba(0,0,0,0.1)' : 'none',
                  transition: 'all 0.2s',
                }}
              >
                {opt.label}
              </button>
            ))}
          </div>

          {lastRefresh && (
            <span style={{ fontSize: '0.75rem', color: 'var(--gray-400)' }}>
              Actualizado {lastRefresh.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
            </span>
          )}
          {isSupportManager(user) && (
            <button
              onClick={() => navigate('/tickets/report')}
              className={styles.channelBtn}
              style={{ display: 'flex', alignItems: 'center', gap: '0.4rem', padding: '0.55rem 1rem' }}
            >
              <FileText size={14} />
              Informe de rechazos
            </button>
          )}
          <button
            onClick={() => refetch()}
            className={styles.channelBtn}
            disabled={refreshing}
            style={{ display: 'flex', alignItems: 'center', gap: '0.4rem', padding: '0.55rem 1rem' }}
          >
            <RefreshCw size={14} style={{ animation: refreshing ? 'spin 1s linear infinite' : 'none' }} />
            {refreshing ? 'Actualizando...' : 'Actualizar'}
          </button>
        </div>
      </div>

      {error && (
        <div style={{
          padding: '0.85rem 1.25rem',
          borderRadius: 'var(--radius-sm)',
          background: 'rgba(239,68,68,0.08)',
          border: '1px solid rgba(239,68,68,0.2)',
          color: '#dc2626',
          fontSize: '0.9rem',
        }}>
          {error}
        </div>
      )}

      {loading ? (
        <div style={{ display: 'flex', justifyContent: 'center', padding: '4rem', color: 'var(--gray-400)' }}>
          Cargando tickets desde Zoho Desk...
        </div>
      ) : (
        <div className={styles.board}>
          {stages.map((stage, idx) => (
            <TicketColumn
              key={stage.id}
              title={stage.title}
              stage={stage.id}
              tickets={filteredTickets.filter(t => t.stage === stage.id)}
              onTicketClick={openTicket}
              index={idx}
              draggable
              isDragging={draggedStageIdx === idx}
              isDragOver={dragOverStageIdx === idx && draggedStageIdx !== idx}
              onDragStart={handleStageDragStart}
              onDragEnter={handleStageDragEnter}
              onDrop={handleStageDrop}
              onDragEnd={handleStageDragEnd}
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
