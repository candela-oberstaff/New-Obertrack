import { useEffect, useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { Ticket, ticketService } from '../../services/ticket.service';
import ContactSidebar from './components/ContactSidebar';
import MessageTimeline from './components/MessageTimeline';
import ChatInputArea from './components/ChatInputArea';
import styles from './Tickets.module.css';
import { ArrowLeft } from 'lucide-react';

export default function TicketDetail() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [ticket, setTicket] = useState<Ticket | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (id) fetchTicket(Number(id));
  }, [id]);

  const fetchTicket = async (ticketId: number) => {
    setLoading(true);
    try {
      const data = await ticketService.getTicket(ticketId);
      setTicket(data);
    } catch (error) {
      console.error('Error fetching ticket:', error);
    } finally {
      setLoading(false);
    }
  };

  const handleSendResponse = async (content: string, channel: 'whatsapp' | 'email') => {
    if (!ticket) return;
    try {
      const newMessage = await ticketService.sendMessage(ticket.id, content, channel);
      setTicket(prev => prev ? {
        ...prev,
        messages: [...(prev.messages || []), newMessage]
      } : null);
    } catch (error) {
      console.error('Error sending message:', error);
    }
  };

  const handleStageChange = async (newStage: 'new' | 'in_progress' | 'waiting' | 'closed') => {
    if (!ticket) return;
    try {
      const updated = await ticketService.updateTicket(ticket.id, { stage: newStage });
      setTicket(prev => prev ? { ...prev, stage: updated.stage } : null);
    } catch (error) {
      console.error('Error updating ticket stage:', error);
    }
  };

  if (loading) {
    return (
      <div className={styles.container} style={{ justifyContent: 'center', alignItems: 'center' }}>
        <p style={{ color: 'var(--gray-500)', fontSize: '1.1rem' }}>Cargando ticket...</p>
      </div>
    );
  }

  if (!ticket) {
    return (
      <div className={styles.container} style={{ justifyContent: 'center', alignItems: 'center' }}>
        <p style={{ color: 'var(--danger)', fontSize: '1.1rem' }}>Ticket no encontrado.</p>
        <button onClick={() => navigate('/tickets')} className={styles.sendBtn} style={{ marginTop: '1rem' }}>
          Volver a Tickets
        </button>
      </div>
    );
  }

  return (
    <div className={styles.container}>
      <div className={styles.header} style={{ marginBottom: '1.5rem' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '1rem' }}>
          <button 
            onClick={() => navigate('/tickets')} 
            className={styles.channelBtn} 
            style={{ padding: '0.5rem', borderRadius: '50%' }}
            aria-label="Volver"
          >
            <ArrowLeft size={18} />
          </button>
          <h1 style={{ margin: 0 }}>Ticket #{ticket.id} - {ticket.title}</h1>
        </div>
      </div>

      <div className={styles.detailContainer}>
        {/* Contact Details & Ticket Controls */}
        <ContactSidebar 
          ticket={ticket} 
          onStageChange={handleStageChange} 
        />

        {/* Message Flow & Response Area */}
        <div className={styles.detailMain}>
          <MessageTimeline 
            ticket={ticket} 
            contact={ticket.contact} 
          />
          
          <ChatInputArea 
            onSend={handleSendResponse} 
          />
        </div>
      </div>
    </div>
  );
}
