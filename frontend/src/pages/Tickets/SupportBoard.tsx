import { useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import { Ticket, ticketService } from '../../services/ticket.service';
import { channelService } from '../../services/channel.service';
import styles from './Tickets.module.css';
import {
  RefreshCw, LifeBuoy, Search, User as UserIcon, Filter, Hand, CheckCircle2,
  MessageSquare, AlertTriangle, Building2, Mail, Clock, UserX,
} from 'lucide-react';
import { useAuth } from '../../context/AuthContext';
import { useNotification } from '../../context/NotificationContext';

type SupportState = 'open' | 'assigned' | 'resolved';

const STAGES = [
  { id: 'new', title: 'Nuevo' },
  { id: 'in_progress', title: 'En Progreso' },
  { id: 'closed', title: 'Cerrado' },
] as const;

type StageId = typeof STAGES[number]['id'];

const STALE_MS = 24 * 60 * 60 * 1000;

function timeAgo(iso?: string): string {
  if (!iso) return '—';
  const diff = Date.now() - new Date(iso).getTime();
  if (diff < 0) return 'ahora';
  const mins = Math.floor(diff / 60000);
  if (mins < 1) return 'ahora';
  if (mins < 60) return `hace ${mins} min`;
  const hours = Math.floor(mins / 60);
  if (hours < 24) return `hace ${hours} h`;
  const days = Math.floor(hours / 24);
  return `hace ${days} d`;
}

function isToday(iso?: string): boolean {
  if (!iso) return false;
  const d = new Date(iso);
  const now = new Date();
  return d.getDate() === now.getDate() && d.getMonth() === now.getMonth() && d.getFullYear() === now.getFullYear();
}

function supportState(t: Ticket): SupportState {
  if (t.stage === 'closed') return 'resolved';
  if (t.assigned_to) return 'assigned';
  return 'open';
}

const STATE_META: Record<SupportState, { label: string; color: string; bg: string }> = {
  open: { label: 'Abierto', color: '#b45309', bg: 'rgba(245,158,11,0.14)' },
  assigned: { label: 'Asignado', color: '#6d28d9', bg: 'rgba(124,58,237,0.12)' },
  resolved: { label: 'Resuelto', color: '#15803d', bg: 'rgba(22,163,74,0.12)' },
};

export default function SupportBoard() {
  const { user } = useAuth();
  const { success: showSuccess, error: showError } = useNotification();
  const navigate = useNavigate();
  const queryClient = useQueryClient();

  const [search, setSearch] = useState('');
  const [stateFilter, setStateFilter] = useState<'all' | SupportState>('all');
  const [agentFilter, setAgentFilter] = useState<string>('all');
  const [onlyMine, setOnlyMine] = useState(false);
  const [onlyUnassigned, setOnlyUnassigned] = useState(false);

  const { data: tickets = [], isLoading: loading, isFetching, error: queryError, refetch, dataUpdatedAt } = useQuery({
    queryKey: ['tickets'],
    queryFn: async () => (await ticketService.getTickets()) ?? [],
    refetchInterval: 60_000,
  });
  const refreshing = isFetching && !loading;
  const lastRefresh = dataUpdatedAt ? new Date(dataUpdatedAt) : null;
  const error = queryError ? ((queryError as any)?.response?.data?.error ?? 'No se pudieron cargar los tickets.') : null;

  const supportTickets = useMemo(() => tickets.filter(t => t.origin === 'support'), [tickets]);

  const isMine = (t: Ticket): boolean => {
    if (!user) return false;
    const byId = !!(t.assigned_to && user.id && t.assigned_to === user.id);
    const byEmail = !!(t.assignee_email && user.email && t.assignee_email.toLowerCase() === user.email.toLowerCase());
    return byId || byEmail;
  };

  const isStale = (t: Ticket): boolean => {
    if (t.stage === 'closed') return false;
    if (!t.assigned_to || t.stage === 'waiting') {
      return Date.now() - new Date(t.created_at).getTime() > STALE_MS;
    }
    return false;
  };

  const agents = useMemo(() => {
    const map = new Map<number, string>();
    supportTickets.forEach(t => {
      if (t.assigned_to && t.assignee_name) map.set(t.assigned_to, t.assignee_name);
    });
    return Array.from(map.entries()).map(([id, name]) => ({ id, name }));
  }, [supportTickets]);

  const metrics = useMemo(() => {
    let open = 0, unassigned = 0, mine = 0, resolvedToday = 0;
    supportTickets.forEach(t => {
      const st = supportState(t);
      if (st !== 'resolved') open++;
      if (!t.assigned_to && st !== 'resolved') unassigned++;
      if (isMine(t) && st !== 'resolved') mine++;
      if (st === 'resolved' && isToday(t.updated_at)) resolvedToday++;
    });
    return { open, unassigned, mine, resolvedToday };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [supportTickets, user]);

  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase();
    return supportTickets.filter(t => {
      const st = supportState(t);
      if (stateFilter !== 'all' && st !== stateFilter) return false;
      if (agentFilter !== 'all' && String(t.assigned_to ?? '') !== agentFilter) return false;
      if (onlyMine && !isMine(t)) return false;
      if (onlyUnassigned && t.assigned_to) return false;
      if (q) {
        const haystack = [
          t.contact?.name, t.contact?.email, t.contact?.phone, t.company_name,
          t.professional_email, t.professional_phone, t.title,
        ].filter(Boolean).join(' ').toLowerCase();
        if (!haystack.includes(q)) return false;
      }
      return true;
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [supportTickets, search, stateFilter, agentFilter, onlyMine, onlyUnassigned, user]);

  const claimMutation = useMutation({
    mutationFn: (ticketId: number) => channelService.claimSupport(ticketId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['tickets'] });
      showSuccess('Tomaste el ticket.');
    },
    onError: (e: any) => showError(e?.response?.data?.error || 'No se pudo tomar el ticket.'),
  });

  const resolveMutation = useMutation({
    mutationFn: (ticketId: number) => channelService.resolveSupport(ticketId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['tickets'] });
      showSuccess('Ticket marcado como resuelto.');
    },
    onError: (e: any) => showError(e?.response?.data?.error || 'No se pudo resolver el ticket.'),
  });

  const reopenMutation = useMutation({
    mutationFn: (ticketId: number) => channelService.reopenSupport(ticketId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['tickets'] });
      showSuccess('Solicitud reabierta.');
    },
    onError: (e: any) => showError(e?.response?.data?.error || 'No se pudo reabrir la solicitud.'),
  });

  const busyTicketId = (claimMutation.isPending ? claimMutation.variables : undefined)
    ?? (resolveMutation.isPending ? resolveMutation.variables : undefined)
    ?? (reopenMutation.isPending ? reopenMutation.variables : undefined);

  const [dragTicket, setDragTicket] = useState<Ticket | null>(null);
  const [dragOverStage, setDragOverStage] = useState<StageId | null>(null);

  const moveTicket = (ticket: Ticket, target: StageId) => {
    const current = (ticket.stage ?? 'new') as StageId;
    if (current === target) return;
    if (target === 'in_progress') {
      claimMutation.mutate(ticket.id);
    } else if (target === 'closed') {
      resolveMutation.mutate(ticket.id);
    } else if (target === 'new') {
      if (supportState(ticket) === 'resolved') {
        reopenMutation.mutate(ticket.id);
      } else {
        showError('Un ticket asignado no se puede dejar sin asignar arrastrándolo. Usá "Reasignar" en el chat.');
      }
    }
  };

  const openChat = (t: Ticket) => {
    if (t.channel_id) navigate(`/chat?channel=${t.channel_id}`);
  };

  const metricCards = [
    { label: 'Abiertos', value: metrics.open, color: '#b45309', bg: 'rgba(245,158,11,0.12)', icon: <LifeBuoy size={18} /> },
    { label: 'Sin asignar', value: metrics.unassigned, color: '#dc2626', bg: 'rgba(239,68,68,0.1)', icon: <UserX size={18} /> },
    { label: 'Míos', value: metrics.mine, color: '#6d28d9', bg: 'rgba(124,58,237,0.12)', icon: <UserIcon size={18} /> },
    { label: 'Resueltos hoy', value: metrics.resolvedToday, color: '#15803d', bg: 'rgba(22,163,74,0.12)', icon: <CheckCircle2 size={18} /> },
  ];

  return (
    <div className={styles.container}>
      <div className={styles.header}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem' }}>
          <LifeBuoy size={22} style={{ color: '#7c3aed' }} />
          <h1>Soporte por Chat</h1>
        </div>
        <div style={{ display: 'flex', alignItems: 'center', gap: '1rem' }}>
          {lastRefresh && (
            <span style={{ fontSize: '0.75rem', color: 'var(--gray-400)' }}>
              Actualizado {lastRefresh.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
            </span>
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

      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(170px, 1fr))', gap: '1rem' }}>
        {metricCards.map(m => (
          <div
            key={m.label}
            style={{
              display: 'flex', alignItems: 'center', gap: '0.85rem',
              padding: '1rem 1.25rem', borderRadius: 'var(--radius)',
              background: 'var(--white)', border: '1px solid var(--gray-100)',
              boxShadow: 'var(--shadow-sm)',
            }}
          >
            <span style={{ display: 'inline-flex', padding: '0.6rem', borderRadius: '10px', color: m.color, background: m.bg }}>
              {m.icon}
            </span>
            <div style={{ display: 'flex', flexDirection: 'column' }}>
              <span style={{ fontSize: '1.6rem', fontWeight: 700, color: 'var(--black)', lineHeight: 1 }}>{m.value}</span>
              <span style={{ fontSize: '0.8rem', color: 'var(--gray-500)', fontWeight: 600 }}>{m.label}</span>
            </div>
          </div>
        ))}
      </div>

      <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem', flexWrap: 'wrap' }}>
        <div style={{ position: 'relative', flex: '1 1 240px', minWidth: 200 }}>
          <Search size={15} style={{ position: 'absolute', left: 12, top: '50%', transform: 'translateY(-50%)', color: 'var(--gray-400)' }} />
          <input
            value={search}
            onChange={e => setSearch(e.target.value)}
            placeholder="Buscar por solicitante, correo o empresa..."
            style={{
              width: '100%', padding: '0.6rem 0.85rem 0.6rem 2.2rem',
              borderRadius: 'var(--radius-sm)', border: '1px solid var(--gray-200)',
              background: 'var(--white)', fontSize: '0.88rem', outline: 'none',
            }}
          />
        </div>

        <select
          value={stateFilter}
          onChange={e => setStateFilter(e.target.value as any)}
          className={styles.select}
          style={{ padding: '0.6rem 0.85rem', minWidth: 150 }}
        >
          <option value="all">Todos los estados</option>
          <option value="open">Abiertos</option>
          <option value="assigned">Asignados</option>
          <option value="resolved">Resueltos</option>
        </select>

        <select
          value={agentFilter}
          onChange={e => setAgentFilter(e.target.value)}
          className={styles.select}
          style={{ padding: '0.6rem 0.85rem', minWidth: 160 }}
        >
          <option value="all">Todos los agentes</option>
          {agents.map(a => (
            <option key={a.id} value={String(a.id)}>{a.name}</option>
          ))}
        </select>

        <button
          onClick={() => setOnlyMine(v => !v)}
          className={styles.channelBtn}
          style={{ padding: '0.6rem 1rem', ...(onlyMine ? { background: 'var(--primary-dark)', color: 'var(--white)', borderColor: 'var(--primary-dark)' } : {}) }}
        >
          <UserIcon size={14} /> Míos
        </button>

        <button
          onClick={() => setOnlyUnassigned(v => !v)}
          className={styles.channelBtn}
          style={{ padding: '0.6rem 1rem', ...(onlyUnassigned ? { background: '#b45309', color: 'var(--white)', borderColor: '#b45309' } : {}) }}
        >
          <Filter size={14} /> Sin asignar
        </button>
      </div>

      {error && (
        <div style={{
          padding: '0.85rem 1.25rem', borderRadius: 'var(--radius-sm)',
          background: 'rgba(239,68,68,0.08)', border: '1px solid rgba(239,68,68,0.2)',
          color: '#dc2626', fontSize: '0.9rem',
        }}>
          {error}
        </div>
      )}

      {loading ? (
        <div style={{ display: 'flex', justifyContent: 'center', padding: '4rem', color: 'var(--gray-400)' }}>
          Cargando solicitudes de soporte...
        </div>
      ) : (
        <div className={styles.board}>
          {STAGES.map(stage => {
            const colTickets = filtered.filter(t => t.stage === stage.id);
            const isDropTarget = !!dragTicket && (dragTicket.stage ?? 'new') !== stage.id;
            const isOver = dragOverStage === stage.id && isDropTarget;
            return (
              <div
                key={stage.id}
                className={`${styles.column} ${isOver ? styles.columnDragOver : ''}`}
                style={{ flex: '1 1 0', minWidth: 300, width: 'auto' }}
                onDragOver={(e) => { if (isDropTarget) { e.preventDefault(); setDragOverStage(stage.id); } }}
                onDragLeave={() => setDragOverStage(prev => (prev === stage.id ? null : prev))}
                onDrop={(e) => {
                  e.preventDefault();
                  if (dragTicket) moveTicket(dragTicket, stage.id);
                  setDragTicket(null);
                  setDragOverStage(null);
                }}
              >
                <div className={`${styles.columnHeader} ${styles[`header-${stage.id}`]}`} style={{ cursor: 'default' }}>
                  <div className={styles.columnTitle}>
                    <h3>{stage.title}</h3>
                  </div>
                  <span className={styles.countBadge}>{colTickets.length}</span>
                </div>
                <div className={styles.ticketList}>
                  {colTickets.length === 0 ? (
                    <div className={styles.emptyColumn}><span>{isOver ? 'Soltá aquí' : 'Sin tickets'}</span></div>
                  ) : (
                    colTickets.map(t => (
                      <SupportCard
                        key={t.id}
                        ticket={t}
                        stale={isStale(t)}
                        mine={isMine(t)}
                        busy={busyTicketId === t.id}
                        dragging={dragTicket?.id === t.id}
                        onDragStart={() => setDragTicket(t)}
                        onDragEnd={() => { setDragTicket(null); setDragOverStage(null); }}
                        onClaim={() => claimMutation.mutate(t.id)}
                        onResolve={() => resolveMutation.mutate(t.id)}
                        onOpenChat={() => openChat(t)}
                      />
                    ))
                  )}
                </div>
              </div>
            );
          })}
        </div>
      )}

      <style>{`
        @keyframes spin { from { transform: rotate(0deg); } to { transform: rotate(360deg); } }
      `}</style>
    </div>
  );
}

interface SupportCardProps {
  ticket: Ticket;
  stale: boolean;
  mine: boolean;
  busy: boolean;
  dragging: boolean;
  onDragStart: () => void;
  onDragEnd: () => void;
  onClaim: () => void;
  onResolve: () => void;
  onOpenChat: () => void;
}

function SupportCard({ ticket, stale, mine, busy, dragging, onDragStart, onDragEnd, onClaim, onResolve, onOpenChat }: SupportCardProps) {
  const st = supportState(ticket);
  const meta = STATE_META[st];
  const unassigned = !ticket.assigned_to;
  const requester = ticket.contact?.name || ticket.professional_email || 'Desconocido';
  const email = ticket.contact?.email || ticket.professional_email;

  return (
    <div
      className={styles.ticketCard}
      draggable={!busy}
      onDragStart={onDragStart}
      onDragEnd={onDragEnd}
      style={{
        cursor: 'grab',
        opacity: dragging ? 0.5 : 1,
        borderColor: stale ? 'rgba(239,68,68,0.45)' : undefined,
        boxShadow: stale ? '0 0 0 1px rgba(239,68,68,0.25)' : undefined,
      }}
    >
      <div className={styles.cardHeader}>
        <span style={{ display: 'inline-flex', alignItems: 'center', gap: 4, padding: '0.25rem 0.5rem', borderRadius: 'var(--radius-sm)', fontSize: '0.72rem', fontWeight: 600, color: meta.color, background: meta.bg }}>
          {meta.label}
        </span>
        <span style={{ display: 'inline-flex', alignItems: 'center', gap: 4, fontSize: '0.72rem', fontWeight: 600, color: stale ? '#dc2626' : 'var(--gray-500)' }}>
          {stale && <AlertTriangle size={12} />}
          <Clock size={11} />
          {timeAgo(ticket.created_at)}
        </span>
      </div>

      <div style={{ display: 'flex', flexDirection: 'column', gap: '0.15rem', textAlign: 'left', minWidth: 0 }}>
        <span style={{ fontSize: '0.92rem', fontWeight: 600, color: 'var(--text-primary, #1e293b)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{requester}</span>
        {email && (
          <span style={{ display: 'inline-flex', alignItems: 'center', gap: 4, fontSize: '0.76rem', color: 'var(--text-secondary, #64748b)', minWidth: 0 }}>
            <Mail size={11} style={{ flexShrink: 0 }} />
            <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{email}</span>
          </span>
        )}
        {ticket.company_name && (
          <span style={{ display: 'inline-flex', alignItems: 'center', gap: 4, fontSize: '0.76rem', color: 'var(--text-secondary, #64748b)', minWidth: 0 }}>
            <Building2 size={11} style={{ flexShrink: 0 }} />
            <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{ticket.company_name}</span>
          </span>
        )}
      </div>

      <p style={{
        fontSize: '0.78rem', color: '#8e8e93', margin: 0, lineHeight: '1.35',
        wordBreak: 'break-word', display: '-webkit-box', WebkitLineClamp: 2,
        WebkitBoxOrient: 'vertical', overflow: 'hidden', textAlign: 'left',
      }}>
        {ticket.title}
      </p>

      <div style={{ display: 'flex', alignItems: 'center', gap: 6, flexWrap: 'wrap' }}>
        {unassigned ? (
          <span style={{ display: 'inline-flex', alignItems: 'center', gap: 4, fontSize: 10, fontWeight: 600, color: '#b45309', background: 'rgba(245,158,11,0.12)', padding: '2px 6px', borderRadius: 4, border: '1px solid rgba(245,158,11,0.2)' }}>
            <UserX size={10} /> Sin asignar
          </span>
        ) : (
          <span style={{ display: 'inline-flex', alignItems: 'center', gap: 4, fontSize: 10, fontWeight: 500, color: 'var(--text-secondary)', background: 'rgba(99,102,241,0.06)', padding: '2px 6px', borderRadius: 4, border: '1px solid rgba(99,102,241,0.12)' }}>
            <UserIcon size={10} style={{ color: '#6366f1' }} /> {ticket.assignee_name || 'Agente'}{mine ? ' (yo)' : ''}
          </span>
        )}
      </div>

      <div className={styles.cardFooter} style={{ borderTop: '1px solid var(--gray-100)', gap: 8, flexWrap: 'wrap' }}>
        {st !== 'resolved' && unassigned && (
          <button
            onClick={onClaim}
            disabled={busy || !ticket.channel_id}
            className={styles.channelBtn}
            style={{ padding: '0.4rem 0.6rem', fontSize: '0.78rem', flex: '1 1 auto', minWidth: 0, justifyContent: 'center' }}
          >
            <Hand size={13} style={{ flexShrink: 0 }} /> Tomar
          </button>
        )}
        {st !== 'resolved' && (
          <button
            onClick={onResolve}
            disabled={busy || !ticket.channel_id}
            className={styles.channelBtn}
            style={{ padding: '0.4rem 0.6rem', fontSize: '0.78rem', flex: '1 1 auto', minWidth: 0, justifyContent: 'center', color: '#15803d', borderColor: 'rgba(22,163,74,0.3)' }}
          >
            <CheckCircle2 size={13} style={{ flexShrink: 0 }} /> Resolver
          </button>
        )}
        <button
          onClick={onOpenChat}
          disabled={!ticket.channel_id}
          className={styles.channelBtn}
          style={{ padding: '0.4rem 0.6rem', fontSize: '0.78rem', flex: '1 1 auto', minWidth: 0, justifyContent: 'center', color: '#7c3aed', borderColor: 'rgba(124,58,237,0.3)' }}
        >
          <MessageSquare size={13} style={{ flexShrink: 0 }} /> Chat
        </button>
      </div>
    </div>
  );
}
