import React from 'react';
import { Mail, Search } from 'lucide-react';
import CampaignCard from './CampaignCard';
import styles from '../EmailMarketing.module.css';

interface CampaignGridProps {
  campaigns: any[];
  loading: boolean;
  onEdit: (campaign: any) => void;
  onDelete: (campaignId: number) => void;
  onViewDetail: (campaign: any) => void;
  emptyMessage?: string;
}

const CampaignGrid: React.FC<CampaignGridProps> = ({ campaigns, loading, onEdit, onDelete, onViewDetail, emptyMessage }) => {
  if (loading) {
    return <div className={styles['loading-info']}>Cargando campañas...</div>;
  }

  if (campaigns.length === 0) {
    return (
      <div className={styles['empty-state']}>
        <div className={styles['empty-icon']}>
          {emptyMessage ? <Search size={32} /> : <Mail size={32} />}
        </div>
        <h3>{emptyMessage ? 'Sin resultados' : 'No hay campañas aún'}</h3>
        <p>{emptyMessage || 'Crea tu primera campaña de email marketing para conectar con tus usuarios.'}</p>
      </div>
    );
  }

  return (
    <div className={styles['list-grid']}>
      {campaigns.map((camp: any) => (
        <CampaignCard 
          key={camp.id} 
          campaign={camp} 
          onClick={() => onViewDetail(camp)}
          onEdit={() => onEdit(camp)}
          onDelete={() => onDelete(camp.id)}
        />
      ))}
    </div>
  );
};

export default CampaignGrid;
