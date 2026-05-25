import { Ticket } from '../../../services/ticket.service';
import styles from '../Tickets.module.css';
import { User, Phone, Mail, Clock, AlertCircle } from 'lucide-react';

interface ContactSidebarProps {
  ticket: Ticket;
  onStageChange: (newStage: 'new' | 'in_progress' | 'waiting' | 'closed') => void;
}

export default function ContactSidebar({ ticket, onStageChange }: ContactSidebarProps) {
  return (
    <aside className={styles.detailSidebar}>
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
        </div>
      </div>

      <hr className={styles.divider} />

      <div className={styles.sidebarSection}>
        <h4 className={styles.sectionTitle}>Estado de Gestión</h4>
        <div className={styles.controlGroup}>
          <label className={styles.fieldLabel} htmlFor="stage-select">Etapa actual</label>
          <select 
            id="stage-select"
            value={ticket.stage} 
            onChange={(e) => onStageChange(e.target.value as any)}
            className={styles.select}
          >
            <option value="new">Nuevo</option>
            <option value="in_progress">En Progreso</option>
            <option value="waiting">Esperando respuesta</option>
            <option value="closed">Cerrado</option>
          </select>
        </div>
        
        <div className={styles.ticketMeta}>
          <div className={styles.metaRow}>
            <Clock size={14} />
            <span>Creado: {new Date(ticket.created_at).toLocaleDateString()}</span>
          </div>
          <div className={styles.metaRow}>
            <AlertCircle size={14} />
            <span>ID Ticket: #{ticket.id}</span>
          </div>
        </div>
      </div>
    </aside>
  );
}
