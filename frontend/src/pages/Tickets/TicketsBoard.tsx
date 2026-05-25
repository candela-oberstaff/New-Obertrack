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
      await fetch('http://localhost:8080/api/webhooks/waha', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          event: 'message',
          session: 'default',
          payload: {
            id: 'false_5491122334455@c.us_3A' + Date.now(),
            from: '5491122334455@c.us',
            to: 'me',
            body: 'Hola, necesito ayuda con mi cuenta (Mensaje de prueba)',
            type: 'chat',
            fromMe: false,
            timestamp: Date.now() / 1000
          }
        })
      });
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
