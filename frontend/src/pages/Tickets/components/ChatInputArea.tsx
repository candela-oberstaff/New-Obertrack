import { useState, KeyboardEvent } from 'react';
import styles from '../Tickets.module.css';
import { Send, MessageSquare, Mail } from 'lucide-react';

interface ChatInputAreaProps {
  onSend: (content: string, channel: 'whatsapp' | 'email') => void;
}

export default function ChatInputArea({ onSend }: ChatInputAreaProps) {
  const [content, setContent] = useState('');
  const [channel, setChannel] = useState<'whatsapp' | 'email'>('whatsapp');

  const handleSend = () => {
    if (!content.trim()) return;
    onSend(content, channel);
    setContent('');
  };

  const handleKeyDown = (e: KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  };

  return (
    <div className={styles.chatInput}>
      <div className={styles.channelSelector}>
        <button 
          className={`${styles.channelBtn} ${channel === 'whatsapp' ? styles.active : ''}`}
          onClick={() => setChannel('whatsapp')}
          aria-label="Responder por WhatsApp"
        >
          <MessageSquare size={14} />
          <span>Responder por WhatsApp</span>
        </button>
        <button 
          className={`${styles.channelBtn} ${channel === 'email' ? styles.active : ''}`}
          onClick={() => setChannel('email')}
          aria-label="Responder por Email"
        >
          <Mail size={14} />
          <span>Responder por Email</span>
        </button>
      </div>
      <div className={styles.inputRow}>
        <textarea 
          rows={2} 
          value={content} 
          onChange={(e) => setContent(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder={`Escribe tu respuesta vía ${channel === 'whatsapp' ? 'WhatsApp' : 'Email'}... (Presiona Enter para enviar)`}
          className={styles.textarea}
        />
        <button className={styles.sendBtn} onClick={handleSend} aria-label="Enviar mensaje">
          <Send size={18} />
        </button>
      </div>
    </div>
  );
}
