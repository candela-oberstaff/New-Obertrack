import { useEffect, useRef, useState } from 'react';
import { Ticket, Contact, TicketMessage } from '../../../services/ticket.service';
import { isWaMediaMessage, downloadWaMedia } from '../../../lib/waMedia';
import { useNotification } from '../../../context/NotificationContext';
import styles from '../Tickets.module.css';
import { MessageSquare, Mail, User, Download, Loader2 } from 'lucide-react';

interface MessageTimelineProps {
  ticket: Ticket;
  contact: Contact | undefined;
}

export default function MessageTimeline({ ticket, contact }: MessageTimelineProps) {
  const boxRef = useRef<HTMLDivElement>(null);
  const { error: showError } = useNotification();
  // Un adjunto a la vez: WAHA tarda y dos descargas seguidas confunden.
  const [downloadingId, setDownloadingId] = useState<number | null>(null);
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

  // Mismo criterio y misma descarga que la bandeja de WhatsApp: el archivo se
  // pide a WAHA por el id externo del mensaje.
  const handleDownload = async (msg: TicketMessage) => {
    if (downloadingId !== null) return;
    setDownloadingId(msg.id);
    try {
      await downloadWaMedia(ticket.id, msg);
    } catch (err) {
      console.error('Error downloading media:', err);
      showError('No se pudo descargar el archivo. Puede que WhatsApp ya no lo tenga disponible.');
    } finally {
      setDownloadingId(null);
    }
  };

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
          const hasMedia = msg.channel === 'whatsapp' && isWaMediaMessage(msg);
          const downloading = downloadingId === msg.id;
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

                {hasMedia && (
                  <button
                    type="button"
                    className={styles.mediaDownloadBtn}
                    onClick={() => handleDownload(msg)}
                    disabled={downloading}
                    title="Descargar archivo adjunto"
                  >
                    {downloading ? <Loader2 size={14} className={styles.spin} /> : <Download size={14} />}
                    <span>{downloading ? 'Descargando...' : 'Descargar'}</span>
                  </button>
                )}

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
