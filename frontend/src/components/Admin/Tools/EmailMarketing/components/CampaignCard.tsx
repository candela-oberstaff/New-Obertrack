import React from 'react';
import { Users, Eye, MousePointer2, Trash2, Pencil } from 'lucide-react';
import styles from '../EmailMarketing.module.css';

interface CampaignCardProps {
  campaign: any;
  onClick: () => void;   // opens detail panel
  onEdit: () => void;    // opens editor
  onDelete: () => void;
}

const CampaignCard: React.FC<CampaignCardProps> = ({ campaign, onClick, onEdit, onDelete }) => {
  return (
    <div className={styles['tool-card']} onClick={onClick}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '8px' }}>
        <span className={`${styles['card-tag']} ${styles[campaign.status]}`}>
          {campaign.status === 'sent' ? 'Enviada' : campaign.status === 'draft' ? 'Borrador' : 'Programada'}
        </span>
        <div style={{ display: 'flex', gap: '4px' }}>
          {campaign.status !== 'sent' && (
            <button
              onClick={(e) => { e.stopPropagation(); onEdit(); }}
              style={{ background: 'none', border: 'none', color: '#8b5cf6', cursor: 'pointer', padding: '4px' }}
              title="Editar campaña"
            >
              <Pencil size={15} />
            </button>
          )}
          <button 
            onClick={(e) => { e.stopPropagation(); onDelete(); }}
            style={{ background: 'none', border: 'none', color: '#ef4444', cursor: 'pointer', padding: '4px' }}
            title="Eliminar campaña"
          >
            <Trash2 size={16} />
          </button>
        </div>
      </div>
      <h4>{campaign.title}</h4>
      <p>{campaign.subject || <span style={{ color: '#cbd5e1' }}>Sin asunto</span>}</p>
      
      <div className={styles['card-footer']}>
        <div className={styles['stats-mini']}>
          <div className={styles['stat-mini-item']} title="Enviados">
            <Users size={14} />
            {campaign.recipients || 0}
          </div>
          <div className={styles['stat-mini-item']} title="Aperturas">
            <Eye size={14} />
            {campaign.open_rate || 0}%
          </div>
          <div className={styles['stat-mini-item']} title="Clics">
            <MousePointer2 size={14} />
            {campaign.click_rate || 0}%
          </div>
        </div>
        <span>{campaign.sent_at
          ? `Enviada ${new Date(campaign.sent_at).toLocaleDateString()}`
          : campaign.created_at
          ? new Date(campaign.created_at).toLocaleDateString()
          : 'Borrador'}
        </span>
      </div>
    </div>
  );
};

export default CampaignCard;
