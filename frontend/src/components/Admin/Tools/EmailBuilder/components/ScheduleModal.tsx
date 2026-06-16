import React, { useState } from 'react';
import { Clock, Send } from 'lucide-react';
import { Modal, Button } from '../../../../ui';
import styles from '../Builder.module.css';
import AudienceSelector, { AudienceSelection } from '../../Common/AudienceSelector';

interface ScheduleModalProps {
  onClose: () => void;
  onConfirm: (data: { date: string, recipientIds: AudienceSelection }) => void;
  availableRecipients: any[];
  initialScheduledAt?: string;
  initialRecipientIds?: any;
}

const ScheduleModal: React.FC<ScheduleModalProps> = ({
  onClose,
  onConfirm,
  initialScheduledAt,
  initialRecipientIds
}) => {
  const existingDate = initialScheduledAt
    ? new Date(initialScheduledAt).toISOString().split('T')[0]
    : '';
  const existingTime = initialScheduledAt
    ? new Date(initialScheduledAt).toISOString().split('T')[1].substring(0, 5)
    : '10:00';

  const [scheduleDate, setScheduleDate] = useState(existingDate);
  const [scheduleTime, setScheduleTime] = useState(existingTime);
  const [selectedAudience, setSelectedAudience] = useState<AudienceSelection>(
    initialRecipientIds || { groupIds: [], userIds: [], expressContacts: [] }
  );
  const [isProcessing, setIsProcessing] = useState(false);

  const isEditing = !!initialScheduledAt;

  const handleConfirm = async () => {
    setIsProcessing(true);
    try {
      const dateString = scheduleDate ? `${scheduleDate}T${scheduleTime}:00Z` : '';
      await onConfirm({ date: dateString, recipientIds: selectedAudience });
      onClose();
    } finally {
      setIsProcessing(false);
    }
  };

  const hasSelection = 
    selectedAudience.groupIds.length > 0 || 
    selectedAudience.userIds.length > 0 || 
    selectedAudience.expressContacts.length > 0;

  return (
    <Modal
      isOpen
      onClose={onClose}
      size="md"
      title={
        <span style={{ display: 'inline-flex', alignItems: 'center', gap: '8px' }}>
          <Clock size={18} color="var(--color-blue-violet-hex)" />
          {isEditing ? 'Editar Programación' : 'Programar Envío'}
        </span>
      }
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>Cancelar</Button>
          <Button
            onClick={handleConfirm}
            loading={isProcessing}
            disabled={!hasSelection}
            leftIcon={scheduleDate ? <Clock size={16} /> : <Send size={16} />}
          >
            {isEditing
              ? 'Actualizar Programación'
              : scheduleDate ? 'Confirmar Programación' : 'Enviar Ahora'}
          </Button>
        </>
      }
    >
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
        <label style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '8px' }}>
          <span>Destinatarios</span>
        </label>
        <AudienceSelector value={selectedAudience} onChange={setSelectedAudience} />
      </div>
    </Modal>
  );
};

export default ScheduleModal;
