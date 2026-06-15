import React, { useState } from 'react';
import { ChevronLeft, Save, Eye, Send, Clock } from 'lucide-react';
import styles from './Builder.module.css';

// Types and Utils
import { Block, BlockType, EmailBuilderProps } from './types';
import { getDefaultContent, getDefaultStyle } from './utils';

// Sub-components
import Toolbar from './components/Toolbar';
import Canvas from './components/Canvas';
import PropertiesPanel from './components/PropertiesPanel';
import PreviewModal from './components/PreviewModal';
import ScheduleModal from './components/ScheduleModal';

const EmailBuilder: React.FC<EmailBuilderProps> = ({ 
  onBack, 
  onSave, 
  onSend, 
  onSchedule, 
  availableRecipients = [], 
  initialBlocks, 
  initialTitle,
  initialScheduledAt,
  initialRecipientIds = []
}) => {
  const [blocks, setBlocks] = useState<Block[]>(initialBlocks || []);
  const [title, setTitle] = useState(initialTitle || 'Nueva Campaña');
  const [selectedBlockId, setSelectedBlockId] = useState<string | null>(null);
  const [showPreview, setShowPreview] = useState(false);
  const [showScheduleModal, setShowScheduleModal] = useState(false);

  const addBlock = (type: BlockType) => {
    const newBlock: Block = {
      id: Math.random().toString(36).substr(2, 9),
      type,
      content: getDefaultContent(type),
      style: getDefaultStyle(type)
    };
    setBlocks([...blocks, newBlock]);
    setSelectedBlockId(newBlock.id);
  };

  const deleteBlock = (id: string, e: React.MouseEvent) => {
    e.stopPropagation();
    setBlocks(blocks.filter(b => b.id !== id));
    if (selectedBlockId === id) setSelectedBlockId(null);
  };

  const updateBlock = (id: string, updates: Partial<Block>) => {
    setBlocks(blocks.map(b => b.id === id ? { ...b, ...updates } : b));
  };

  const selectedBlock = blocks.find(b => b.id === selectedBlockId);

  const handleScheduleConfirm = async (data: { date: string, recipientIds: any }) => {
    if (data.date) {
      await onSchedule?.({ 
        title, 
        blocks, 
        date: data.date,
        recipientIds: data.recipientIds
      });
    } else {
      await onSend?.({
        title,
        blocks,
        recipientIds: data.recipientIds
      });
    }
  };

  return (
    <div className={styles['builder-container']}>
      <header className={styles['builder-header']}>
        <div className={styles['header-left']}>
          <button className={styles['back-btn']} onClick={onBack}>
            <ChevronLeft size={20} />
          </button>
          <div className={styles['builder-title']}>
            <input 
              type="text" 
              value={title} 
              onChange={(e) => setTitle(e.target.value)}
              className={styles['editable-title']}
              placeholder="Nombre de la campaña..."
            />
            <span style={{ fontSize: '12px', color: '#94a3b8' }}>Editor de Email Marketing</span>
          </div>
        </div>

        <div className={styles['header-actions']}>
          <button className={styles['back-btn']} onClick={() => setShowPreview(true)} title="Vista Previa">
            <Eye size={20} />
          </button>
          <button className={styles['back-btn']} onClick={() => onSave?.({ title, blocks })} title="Guardar Plantilla">
            <Save size={20} />
          </button>
          <button className={styles['back-btn']} onClick={() => setShowScheduleModal(true)} title="Programar o Enviar">
            <Clock size={20} />
          </button>
          <button 
            className={`${styles['back-btn']} ${styles['header-send-btn']}`}
            onClick={() => setShowScheduleModal(true)}
          >
            <Send size={18} />
            Enviar Campaña
          </button>
        </div>
      </header>

      <div className={styles['builder-main']}>
        <Toolbar onAddBlock={addBlock} />
        
        <Canvas 
          blocks={blocks}
          selectedBlockId={selectedBlockId}
          onSelectBlock={setSelectedBlockId}
          onDeleteBlock={deleteBlock}
        />

        <PropertiesPanel 
          selectedBlock={selectedBlock}
          onUpdateBlock={updateBlock}
        />
      </div>

      {showPreview && (
        <PreviewModal 
          blocks={blocks} 
          onClose={() => setShowPreview(false)} 
        />
      )}

      {showScheduleModal && (
        <ScheduleModal 
          onClose={() => setShowScheduleModal(false)}
          onConfirm={handleScheduleConfirm}
          availableRecipients={availableRecipients}
          initialScheduledAt={initialScheduledAt}
          initialRecipientIds={initialRecipientIds}
        />
      )}
    </div>
  );
};

export default EmailBuilder;
