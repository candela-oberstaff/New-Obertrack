import { useState } from 'react';
import { Mail } from 'lucide-react';
import { Modal, Button } from '../../../components/ui';

interface SendEmailModalProps {
  contactEmail?: string;
  onClose: () => void;
  onSend: (content: string) => Promise<void>;
}

export default function SendEmailModal({ contactEmail, onClose, onSend }: SendEmailModalProps) {
  const [content, setContent] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleSubmit = async () => {
    if (!content.trim()) return;
    setSubmitting(true);
    setError(null);
    try {
      await onSend(content.trim());
      onClose();
    } catch (err: any) {
      setError(err?.response?.data?.error ?? 'No se pudo enviar el correo.');
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal
      isOpen
      onClose={onClose}
      size="sm"
      title={
        <span style={{ display: 'inline-flex', alignItems: 'center', gap: '0.5rem' }}>
          <Mail size={19} style={{ color: 'var(--primary)' }} />
          Responder por Email
        </span>
      }
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={submitting}>
            Cancelar
          </Button>
          <Button
            onClick={handleSubmit}
            loading={submitting}
            disabled={!content.trim()}
            leftIcon={<Mail size={15} />}
          >
            Enviar
          </Button>
        </>
      }
    >
      <div style={{ marginBottom: '1rem' }}>
        <span style={{ fontSize: '0.82rem', color: 'var(--text-secondary)' }}>
          Enviar respuesta por correo a:{' '}
          <strong style={{ color: 'var(--text-primary)' }}>{contactEmail || 'el cliente (sin email configurado)'}</strong>
        </span>
      </div>

      <label style={{ display: 'block', fontSize: '0.8rem', fontWeight: 600, color: 'var(--text-secondary)', marginBottom: '0.3rem' }}>
        Mensaje
      </label>
      <textarea
        value={content}
        onChange={(e) => setContent(e.target.value)}
        rows={6}
        placeholder="Escribe el cuerpo del correo aquí..."
        disabled={submitting}
        style={{
          width: '100%',
          padding: '0.65rem',
          borderRadius: '8px',
          border: '1px solid var(--border, #cbd5e1)',
          background: 'var(--bg-primary, #fff)',
          color: 'var(--text-primary, #333)',
          fontSize: '0.9rem',
          fontFamily: 'inherit',
          resize: 'vertical',
          outline: 'none',
        }}
      />

      {error && <div style={{ color: '#dc2626', fontSize: '0.85rem', marginTop: '0.5rem' }}>{error}</div>}
    </Modal>
  );
}
