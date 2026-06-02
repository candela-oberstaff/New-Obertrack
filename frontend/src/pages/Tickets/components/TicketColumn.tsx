import { Ticket } from '../../../services/ticket.service';
import TicketCard from './TicketCard';
import styles from '../Tickets.module.css';

interface TicketColumnProps {
  title: string;
  stage: string;
  tickets: Ticket[];
  onTicketClick: (zohoId: string) => void;
}

export default function TicketColumn({ title, stage, tickets, onTicketClick }: TicketColumnProps) {
  return (
    <div className={styles.column}>
      <div className={`${styles.columnHeader} ${styles[`header-${stage}`]}`}>
        <h3>{title}</h3>
        <span className={styles.countBadge}>{tickets.length}</span>
      </div>
      <div className={styles.ticketList}>
        {tickets.length === 0 ? (
          <div className={styles.emptyColumn}>
            <span>Sin tickets</span>
          </div>
        ) : (
          tickets.map(ticket => (
            <TicketCard
              key={ticket.zoho_id || ticket.id}
              ticket={ticket}
              onClick={() => onTicketClick(ticket.zoho_id)}
            />
          ))
        )}
      </div>
    </div>
  );
}
