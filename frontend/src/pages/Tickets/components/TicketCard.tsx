import { Ticket } from '../../../services/ticket.service';
import styles from '../Tickets.module.css';
import { MessageSquare, Mail, Calendar, User } from 'lucide-react';

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
        {channel === 'whatsapp' && <MessageSquare size={14} className={styles.waIcon} />}
        {channel === 'email' && <Mail size={14} className={styles.mailIcon} />}
      </div>
      
      <h3 className={styles.cardTitle}>{ticket.title}</h3>
      
      <div className={styles.cardFooter}>
        <div className={styles.contactInfo}>
          <User size={12} />
          <span>{ticket.contact?.name || 'Contacto sin nombre'}</span>
        </div>
        <div className={styles.cardTime}>
          <Calendar size={12} />
          <span>{new Date(ticket.updated_at).toLocaleDateString()}</span>
        </div>
      </div>
    </div>
  );
}
