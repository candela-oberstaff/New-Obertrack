import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Ticket, ticketService } from '../../services/ticket.service';
import TicketColumn from './components/TicketColumn';
import styles from './Tickets.module.css';

const STAGES = [
  { id: 'new', title: 'Nuevo' },
  { id: 'in_progress', title: 'En Progreso' },
  { id: 'waiting', title: 'Esperando' },
  { id: 'closed', title: 'Cerrado' }
];

export default function TicketsBoard() {
  const [tickets, setTickets] = useState<Ticket[]>([]);
  const navigate = useNavigate();

  useEffect(() => {
    fetchTickets();
  }, []);

  const fetchTickets = async () => {
    try {
      const data = await ticketService.getTickets();
      setTickets(data);
    } catch (error) {
      console.error('Error fetching tickets:', error);
    }
  };

  const openTicket = (id: number) => {
    navigate(`/tickets/${id}`);
  };

  const simulateWebhook = async () => {
    try {
      await ticketService.simulateWahaMessage(
        '5491122334455',
        'Hola, necesito ayuda con mi cuenta (Mensaje de prueba)'
      );
      fetchTickets();
    } catch (error) {
      console.error('Error simulating webhook', error);
    }
  };

  return (
    <div className={styles.container}>
      <div className={styles.header}>
        <h1>Tickets (WhatsApp & Email)</h1>
        <button 
          onClick={simulateWebhook}
          className={styles.sendBtn}
          style={{ padding: '0.6rem 1.2rem', boxShadow: 'none' }}
        >
          Simular Mensaje WhatsApp
        </button>
      </div>

      <div className={styles.board}>
        {STAGES.map((stage) => (
          <TicketColumn
            key={stage.id}
            title={stage.title}
            stage={stage.id}
            tickets={tickets.filter(t => t.stage === stage.id)}
            onTicketClick={openTicket}
          />
        ))}
      </div>
    </div>
  );
}
