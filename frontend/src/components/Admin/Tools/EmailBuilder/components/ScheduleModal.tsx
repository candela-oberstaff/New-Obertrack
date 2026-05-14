import React, { useState } from 'react';
import { Clock, Send, X as CloseIcon } from 'lucide-react';
import styles from '../Builder.module.css';

interface ScheduleModalProps {
  onClose: () => void;
  onConfirm: (data: { date: string, recipientIds: number[] }) => void;
  availableRecipients: any[];
  initialScheduledAt?: string;
  initialRecipientIds?: number[];
}

const ScheduleModal: React.FC<ScheduleModalProps> = ({
  onClose,
  onConfirm,
  availableRecipients,
  initialScheduledAt,
  initialRecipientIds = []
}) => {
  const existingDate = initialScheduledAt
    ? new Date(initialScheduledAt).toISOString().split('T')[0]
    : '';
  const existingTime = initialScheduledAt
    ? new Date(initialScheduledAt).toISOString().split('T')[1].substring(0, 5)
    : '10:00';

  const [scheduleDate, setScheduleDate] = useState(existingDate);
  const [scheduleTime, setScheduleTime] = useState(existingTime);
  const [selectedRecipientIds, setSelectedRecipientIds] = useState<number[]>(initialRecipientIds);
  const [searchTerm, setSearchTerm] = useState('');
  const [isProcessing, setIsProcessing] = useState(false);

  const isEditing = !!initialScheduledAt;

  const handleConfirm = async () => {
    setIsProcessing(true);
    try {
      const dateString = scheduleDate ? `${scheduleDate}T${scheduleTime}:00Z` : '';
      await onConfirm({ date: dateString, recipientIds: selectedRecipientIds });
      onClose();
    } finally {
      setIsProcessing(false);
    }
  };

  const toggleRecipient = (id: number) => {
    setSelectedRecipientIds(prev =>
      prev.includes(id) ? prev.filter(r => r !== id) : [...prev, id]
    );
  };

  const filteredRecipients = Array.isArray(availableRecipients)
    ? availableRecipients.filter(u =>
      (u.full_name || u.name || '').toLowerCase().includes(searchTerm.toLowerCase()) ||
      (u.email || '').toLowerCase().includes(searchTerm.toLowerCase())
    )
    : [];

  return (
    <div className={styles['modal-overlay']}>
      <div className={styles['schedule-modal']}>

        {/* Header */}
        <div className={styles['modal-header']}>
          <h3 style={{ fontSize: '18px', fontWeight: 700, color: '#0f172a', margin: 0, display: 'flex', alignItems: 'center', gap: '8px' }}>
            <Clock size={18} color="var(--color-blue-violet-hex)" />
            {isEditing ? 'Editar Programación' : 'Programar Envío'}
          </h3>
          <button className={styles['close-preview']} onClick={onClose}>
            <CloseIcon size={20} />
          </button>
        </div>

        {/* Body */}
        <div style={{ padding: '24px', maxHeight: '55vh', overflowY: 'auto' }}>
          {isEditing && (
            <div style={{
              padding: '10px 14px',
              background: '#eff6ff',
              border: '1px solid #bfdbfe',
              borderRadius: '8px',
              marginBottom: '20px',
              fontSize: '13px',
              color: '#1d4ed8',
              display: 'flex',
              alignItems: 'center',
              gap: '8px'
            }}>
              <Clock size={14} />
              Campaña ya programada. Puedes actualizar la fecha, hora o destinatarios.
            </div>
          )}

          {/* Date/Time */}
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '16px', marginBottom: '24px' }}>
            <div className={styles['prop-control']}>
              <label>Fecha de envío</label>
              <input
                type="date"
                value={scheduleDate}
                min={new Date().toISOString().split('T')[0]}
                onChange={(e) => setScheduleDate(e.target.value)}
              />
            </div>
            <div className={styles['prop-control']}>
              <label>Hora de envío</label>
              <input
                type="time"
                value={scheduleTime}
                onChange={(e) => setScheduleTime(e.target.value)}
              />
            </div>
          </div>

          {/* Recipients */}
          <div className={styles['prop-control']}>
            <label style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
              <span>Destinatarios</span>
              <span style={{ fontSize: '12px', color: '#94a3b8', fontWeight: 400 }}>
                {selectedRecipientIds.length} seleccionados
              </span>
            </label>

            <input
              type="text"
              placeholder="Buscar por nombre o email..."
              value={searchTerm}
              onChange={(e) => setSearchTerm(e.target.value)}
              style={{ marginBottom: '8px' }}
            />

            <div style={{ border: '1px solid #e2e8f0', borderRadius: '10px', maxHeight: '200px', overflowY: 'auto', background: '#f8fafc' }}>
              {filteredRecipients.length === 0 ? (
                <div style={{ padding: '20px', textAlign: 'center', color: '#94a3b8', fontSize: '14px' }}>
                  No se encontraron usuarios.
                </div>
              ) : (
                filteredRecipients.map(user => (
                  <div
                    key={user.id}
                    onClick={() => toggleRecipient(user.id)}
                    style={{
                      display: 'flex', alignItems: 'center', gap: '12px',
                      padding: '10px 14px', borderBottom: '1px solid #e2e8f0',
                      cursor: 'pointer',
                      background: selectedRecipientIds.includes(user.id) ? 'rgba(139, 92, 246, 0.05)' : 'transparent',
                      transition: 'background 0.15s'
                    }}
                  >
                    <input
                      type="checkbox"
                      checked={selectedRecipientIds.includes(user.id)}
                      onChange={() => { }}
                      style={{ width: '16px', height: '16px', cursor: 'pointer', accentColor: 'var(--color-blue-violet-hex)' }}
                    />
                    <div>
                      <div style={{ fontSize: '14px', fontWeight: 500, color: '#1e293b' }}>
                        {user.full_name || user.name}
                      </div>
                      <div style={{ fontSize: '12px', color: '#64748b' }}>{user.email}</div>
                    </div>
                  </div>
                ))
              )}
            </div>

            {filteredRecipients.length > 0 && (
              <div style={{ marginTop: '8px', display: 'flex', gap: '12px' }}>
                <button
                  onClick={() => setSelectedRecipientIds(availableRecipients.map(u => u.id))}
                  style={{ fontSize: '12px', color: 'var(--color-blue-violet-hex)', background: 'none', border: 'none', cursor: 'pointer', padding: 0, fontWeight: 600 }}
                >
                  Seleccionar todos
                </button>
                <span style={{ color: '#e2e8f0' }}>|</span>
                <button
                  onClick={() => setSelectedRecipientIds([])}
                  style={{ fontSize: '12px', color: '#64748b', background: 'none', border: 'none', cursor: 'pointer', padding: 0 }}
                >
                  Desmarcar todos
                </button>
              </div>
            )}
          </div>
        </div>

        {/* Footer */}
        <div style={{
          padding: '16px 24px', background: '#f8fafc', borderTop: '1px solid #e2e8f0',
          display: 'flex', justifyContent: 'flex-end', gap: '12px',
          borderBottomLeftRadius: '16px', borderBottomRightRadius: '16px'
        }}>
          <button
            onClick={onClose}
            style={{
              padding: '10px 20px', borderRadius: '8px', border: '1px solid #e2e8f0',
              background: 'white', cursor: 'pointer', fontWeight: 600, color: '#64748b', fontSize: '14px'
            }}
          >
            Cancelar
          </button>
          <button
            className={styles['header-send-btn']}
            disabled={selectedRecipientIds.length === 0 || isProcessing}
            onClick={handleConfirm}
            style={{
              display: 'flex', alignItems: 'center', gap: '8px',
              opacity: selectedRecipientIds.length === 0 || isProcessing ? 0.5 : 1,
              cursor: selectedRecipientIds.length === 0 ? 'not-allowed' : 'pointer'
            }}
          >
            {scheduleDate ? <Clock size={16} /> : <Send size={16} />}
            {isProcessing
              ? 'Procesando...'
              : isEditing
                ? 'Actualizar Programación'
                : scheduleDate ? 'Confirmar Programación' : 'Enviar Ahora'}
          </button>
        </div>

      </div>
    </div>
  );
};

export default ScheduleModal;
