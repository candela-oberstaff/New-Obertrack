import { useState, KeyboardEvent } from 'react';
import styles from '../Tickets.module.css';
import { Send, MessageSquare, Mail } from 'lucide-react';

interface ChatInputAreaProps {
  onSend: (content: string, channel: 'whatsapp' | 'email') => void;
}

export default function ChatInputArea({ onSend }: ChatInputAreaProps) {
  const [content, setContent] = useState('');
  const [channel, setChannel] = useState<'whatsapp' | 'email'>('email');
  const [sending, setSending] = useState(false);

  const handleSend = async () => {
    if (!content.trim() || sending) return;
    setSending(true);
    try {
      await onSend(content.trim(), channel);
      setContent('');
    } finally {
      setSending(false);
    }
  };

  const handleKeyDown = (e: KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  };

  const placeholder =
    channel === 'whatsapp'
      ? 'Escribe tu respuesta por WhatsApp… (Enter para enviar)'
      : 'Escribe tu respuesta por Email… (Enter para enviar)';

  return (
    <div className={styles.chatInput}>
      {/* Channel selector */}
      <div className={styles.channelSelector}>
        <button
          id="channel-email-btn"
          className={`${styles.channelBtn} ${channel === 'email' ? styles.active : ''}`}
          onClick={() => setChannel('email')}
          aria-label="Responder por Email"
        >
          <Mail size={14} />
          <span>Email</span>
        </button>
        <button
          id="channel-whatsapp-btn"
          className={`${styles.channelBtn} ${channel === 'whatsapp' ? styles.active : ''}`}
          onClick={() => setChannel('whatsapp')}
          aria-label="Responder por WhatsApp"
          style={channel === 'whatsapp' ? {
            background: '#128C7E',
            color: 'white',
            borderColor: '#128C7E',
            boxShadow: '0 4px 10px rgba(18,140,126,0.25)',
          } : {}}
        >
          <MessageSquare size={14} />
          <span>WhatsApp</span>
        </button>

        {/* Active channel badge */}
        <span style={{
          marginLeft: 'auto',
          fontSize: '0.72rem',
          color: 'var(--gray-400)',
          display: 'flex',
          alignItems: 'center',
          gap: '0.3rem',
        }}>
          Enviando vía{' '}
          <strong style={{ color: channel === 'whatsapp' ? '#128C7E' : 'var(--primary)' }}>
            {channel === 'whatsapp' ? 'WhatsApp' : 'Email'}
          </strong>
        </span>
      </div>

      {/* Textarea + Send */}
      <div className={styles.inputRow}>
        <textarea
          id="ticket-reply-input"
          rows={3}
          value={content}
          onChange={(e) => setContent(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder={placeholder}
          className={styles.textarea}
          disabled={sending}
        />
        <button
          id="ticket-send-btn"
          className={styles.sendBtn}
          onClick={handleSend}
          disabled={sending || !content.trim()}
          aria-label="Enviar mensaje"
          style={sending || !content.trim() ? { opacity: 0.5, cursor: 'not-allowed' } : {}}
        >
          <Send size={18} />
        </button>
      </div>
    </div>
  );
}
