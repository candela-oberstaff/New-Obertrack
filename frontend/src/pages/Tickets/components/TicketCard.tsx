import { Ticket } from '../../../services/ticket.service';
import styles from '../Tickets.module.css';
import { MessageSquare, Mail, Calendar, Shield } from 'lucide-react';

interface TicketCardProps {
  ticket: Ticket;
  onClick: () => void;
}

export default function TicketCard({ ticket, onClick }: TicketCardProps) {
  const getLatestMessageChannel = () => {
    if (!ticket.messages || ticket.messages.length === 0) return null;
    const latest = ticket.messages[ticket.messages.length - 1];
    return latest.channel;
  };

  const channel = getLatestMessageChannel();

  return (
    <div className={styles.ticketCard} onClick={onClick}>
      <div className={styles.cardHeader}>
        <span className={`${styles.badge} ${styles[`stage-${ticket.stage}`]}`}>
          {ticket.stage === 'new' && 'Nuevo'}
          {ticket.stage === 'in_progress' && 'En Progreso'}
          {ticket.stage === 'waiting' && 'Esperando'}
          {ticket.stage === 'closed' && 'Cerrado'}
        </span>
        <span
          className={`${styles.badge} ${ticket.origin === 'internal' ? styles['badge-internal'] : styles['badge-zoho']}`}
          style={ticket.origin === 'support' ? { background: 'rgba(124,58,237,0.12)', color: '#6d28d9' } : undefined}
        >
          {ticket.origin === 'internal' ? 'Interno' : ticket.origin === 'support' ? 'Soporte' : 'Zoho'}
        </span>
        {channel === 'whatsapp' && <MessageSquare size={14} className={styles.waIcon} />}
        {channel === 'email' && <Mail size={14} className={styles.mailIcon} />}
      </div>
      
      <h3 className={styles.cardTitle}>{ticket.title}</h3>

      {ticket.assignee_name && (
        <div style={{
          fontSize: '10px',
          color: 'var(--text-secondary)',
          display: 'flex',
          alignItems: 'center',
          gap: '4px',
          marginTop: '6px',
          marginBottom: '6px',
          background: 'rgba(99,102,241,0.06)',
          padding: '2px 6px',
          borderRadius: '4px',
          border: '1px solid rgba(99,102,241,0.12)',
          width: 'fit-content',
          fontWeight: 500
        }}>
          <Shield size={10} style={{ color: '#6366f1' }} />
          <span>Agente: {ticket.assignee_name}</span>
        </div>
      )}
      
      <div className={styles.cardFooter}>
        <div className={styles.cardTime}>
          <Calendar size={12} />
          <span>{new Date(ticket.created_at).toLocaleDateString()}</span>
        </div>
      </div>
    </div>
  );
}
