import { useEffect, useRef } from 'react';
import { Ticket, Contact } from '../../../services/ticket.service';
import styles from '../Tickets.module.css';
import { MessageSquare, Mail, User } from 'lucide-react';

interface MessageTimelineProps {
  ticket: Ticket;
  contact: Contact | undefined;
}

export default function MessageTimeline({ ticket, contact }: MessageTimelineProps) {
  const boxRef = useRef<HTMLDivElement>(null);
  // La primera colocación es instantánea: animar un salto que el usuario no pidió
  // solo distrae. A partir de ahí, los mensajes nuevos entran con desplazamiento.
  const firstScroll = useRef(true);

  useEffect(() => {
    const box = boxRef.current;
    if (!box) return;
    // Se desplaza el contenedor a mano en vez de usar scrollIntoView sobre un
    // elemento centinela: scrollIntoView arrastra a TODOS los ancestros
    // desplazables, la ventana incluida, así que al abrir un ticket la página
    // entera saltaba al final y el encabezado quedaba fuera de vista.
    box.scrollTo({ top: box.scrollHeight, behavior: firstScroll.current ? 'auto' : 'smooth' });
    firstScroll.current = false;
  }, [ticket.messages]);

  return (
    <div className={styles.chatMessages} ref={boxRef}>
      {(!ticket.messages || ticket.messages.length === 0) ? (
        <div className={styles.emptyMessages}>
          <User size={48} className={styles.emptyIcon} />
          <p>No hay mensajes en este ticket aún.</p>
        </div>
      ) : (
        ticket.messages.map(msg => {
          const isAgent = msg.sender_type === 'agent';
          return (
            <div 
              key={msg.id} 
              className={`${styles.messageWrapper} ${isAgent ? styles.messageWrapperAgent : styles.messageWrapperContact}`}
            >
              <div 
                className={`${styles.message} ${isAgent ? styles.messageAgent : styles.messageContact}`}
              >
                <div className={styles.messageMeta}>
                  <span className={styles.senderName}>
                    {isAgent ? 'Tú (Agente)' : (contact?.name || 'Contacto')}
                  </span>
                  <span className={styles.messageTime}>
                    {new Date(msg.created_at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
                  </span>
                </div>
                <div className={styles.messageContent}>{msg.content}</div>
                <div className={styles.messageChannelFooter}>
                  {msg.channel === 'whatsapp' ? (
                    <>
                      <MessageSquare size={12} />
                      <span>WhatsApp</span>
                    </>
                  ) : (
                    <>
                      <Mail size={12} />
                      <span>Email</span>
                    </>
                  )}
                </div>
              </div>
            </div>
          );
        })
      )}
    </div>
  );
}
