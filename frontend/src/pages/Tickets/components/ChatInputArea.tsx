import { useState, KeyboardEvent, useEffect } from 'react';
import styles from '../Tickets.module.css';
import { Send } from 'lucide-react';
import { ticketService } from '../../../services/ticket.service';

interface ChatInputAreaProps {
  onSend: (content: string, channel: 'whatsapp' | 'email', templateId?: string) => void;
  departmentId?: string;
}

interface TemplateMessage {
  id: string;
  title: string;
  message: string;
  displayMessage: string;
  status: string;
}

export default function ChatInputArea({ onSend, departmentId }: ChatInputAreaProps) {
  const [content, setContent] = useState('');
  const [sending, setSending] = useState(false);
  const [templates, setTemplates] = useState<TemplateMessage[]>([]);
  const [selectedTemplateId, setSelectedTemplateId] = useState<string>('');

  useEffect(() => {
    ticketService.getWhatsAppTemplates(departmentId)
      .then(data => {
        setTemplates(data || []);
      })
      .catch(err => {
        console.error('Error fetching templates:', err);
      });
  }, [departmentId]);

  const handleSend = async () => {
    if (!content.trim() || sending) return;
    setSending(true);
    try {
      await onSend(content.trim(), 'whatsapp', selectedTemplateId || undefined);
      setContent('');
      setSelectedTemplateId('');
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

  const handleTemplateChange = (templateId: string) => {
    setSelectedTemplateId(templateId);
    if (templateId) {
      const template = templates.find(t => t.id === templateId);
      if (template) {
        setContent(template.message || template.displayMessage);
      }
    } else {
      setContent('');
    }
  };

  const placeholder = 'Escribe tu respuesta por WhatsApp… (Enter para enviar)';

  return (
    <div className={styles.chatInput}>
      {/* Control bar */}
      <div className={styles.channelSelector}>
        {templates.length > 0 ? (
          <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
            <span style={{ fontSize: '0.8rem', color: 'var(--text-secondary, #64748b)', fontWeight: 600 }}>Plantilla:</span>
            <select
              value={selectedTemplateId}
              onChange={(e) => handleTemplateChange(e.target.value)}
              style={{
                fontSize: '0.8rem',
                padding: '0.35rem 0.75rem',
                borderRadius: '6px',
                border: '1px solid var(--glass-border, #e2e8f0)',
                background: 'var(--bg-secondary, #fff)',
                color: 'var(--text-primary, #333)',
                outline: 'none',
                cursor: 'pointer',
              }}
            >
              <option value="">-- Escribir texto libre --</option>
              {templates.map(t => (
                <option key={t.id} value={t.id}>
                  {t.title}
                </option>
              ))}
            </select>
          </div>
        ) : (
          <span style={{ fontSize: '0.8rem', color: 'var(--gray-400)' }}>No hay plantillas de WhatsApp configuradas</span>
        )}

        {/* Active channel badge */}
        <span style={{
          marginLeft: 'auto',
          fontSize: '0.75rem',
          color: 'var(--gray-400)',
          display: 'flex',
          alignItems: 'center',
          gap: '0.3rem',
        }}>
          Enviando vía{' '}
          <strong style={{ color: '#128C7E' }}>
            WhatsApp
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
