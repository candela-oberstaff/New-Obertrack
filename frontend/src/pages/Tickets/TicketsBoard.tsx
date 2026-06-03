import { useCallback, useEffect, useState } from 'react';
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
  const [tickets, setTickets] = useState<Ticket[]>([]);
  const [stages, setStages] = useState<TicketStageConfig[]>(getInitialStageOrder);
  const [draggedStageIdx, setDraggedStageIdx] = useState<number | null>(null);
  const [dragOverStageIdx, setDragOverStageIdx] = useState<number | null>(null);
  const navigate = useNavigate();

  useEffect(() => {
    fetchTickets();
  }, []);

  useEffect(() => {
    localStorage.setItem(STAGE_ORDER_STORAGE_KEY, JSON.stringify(stages.map(stage => stage.id)));
  }, [stages]);

  const fetchTickets = async () => {
    try {
      const data = await ticketService.getTickets();
      setTickets(data);
    } catch (error) {
      console.error('Error fetching tickets:', error);
    }
  };

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
      <div className={styles.header} data-tour="tickets-header">
        <h1>Tickets (WhatsApp & Email)</h1>
        <button
          onClick={simulateWebhook}
          className={styles.sendBtn}
          data-tour="tickets-action"
          style={{ padding: '0.6rem 1.2rem', boxShadow: 'none' }}
        >
          Simular Mensaje WhatsApp
        </button>
      </div>

      <div className={styles.board} data-tour="tickets-board">
        {stages.map((stage, idx) => (
          <TicketColumn
            key={stage.id}
            title={stage.title}
            stage={stage.id}
            tickets={tickets.filter(t => t.stage === stage.id)}
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
    </div>
  );
}
