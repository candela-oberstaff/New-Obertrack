import type { DragEvent } from 'react';
import { Ticket } from '../../../services/ticket.service';
import TicketCard from './TicketCard';
import styles from '../Tickets.module.css';
import { GripVertical } from 'lucide-react';

interface TicketColumnProps {
  title: string;
  stage: string;
  tickets: Ticket[];
  onTicketClick: (ticket: Ticket) => void;
  index?: number;
  draggable?: boolean;
  isDragging?: boolean;
  isDragOver?: boolean;
  onDragStart?: (index: number) => void;
  onDragEnter?: (index: number) => void;
  onDrop?: (index: number) => void;
  onDragEnd?: () => void;
}

export default function TicketColumn({
  title,
  stage,
  tickets,
  onTicketClick,
  index,
  draggable = false,
  isDragging = false,
  isDragOver = false,
  onDragStart,
  onDragEnter,
  onDrop,
  onDragEnd,
}: TicketColumnProps) {
  const dragHandlers = draggable && typeof index === 'number'
    ? {
        onDragOver: (e: DragEvent) => {
          e.preventDefault();
          onDragEnter?.(index);
        },
        onDrop: (e: DragEvent) => {
          e.preventDefault();
          onDrop?.(index);
        },
      }
    : {};

  return (
    <div
      className={`${styles.column} ${isDragging ? styles.columnDragging : ''} ${isDragOver ? styles.columnDragOver : ''}`}
      {...dragHandlers}
    >
      <div
        className={`${styles.columnHeader} ${styles[`header-${stage}`]}`}
        draggable={draggable}
        onDragStart={draggable && typeof index === 'number' ? () => onDragStart?.(index) : undefined}
        onDragEnd={draggable ? onDragEnd : undefined}
      >
        <div className={styles.columnTitle}>
          {draggable && (
            <span className={styles.columnGrip} aria-hidden="true">
              <GripVertical size={16} />
            </span>
          )}
          <h3>{title}</h3>
        </div>
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
              key={`${ticket.origin || 'zoho'}-${ticket.zoho_id || ticket.id}`}
              ticket={ticket}
              onClick={() => onTicketClick(ticket)}
            />
          ))
        )}
      </div>
    </div>
  );
}
