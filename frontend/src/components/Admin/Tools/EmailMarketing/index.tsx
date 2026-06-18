import React, { useState, useEffect, useMemo } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Plus, Search } from 'lucide-react';
import EmailBuilder from '../EmailBuilder';
import { emailService } from '../../../../services/emailService';
import CampaignGrid from './components/CampaignGrid';
import CampaignDetailPanel from './components/CampaignDetailPanel';
import Pagination from '../Common/Pagination';
import styles from './EmailMarketing.module.css';
import commonStyles from '../Tools.module.css';
import { useConfirm } from '../../../ui/ConfirmProvider';

interface EmailMarketingProps {
  onToggleFullScreen: (isFull: boolean) => void;
  setHeaderAction: (node: React.ReactNode) => void;
}

const EmailMarketing: React.FC<EmailMarketingProps> = ({ onToggleFullScreen, setHeaderAction }) => {
  const [showBuilder, setShowBuilder] = useState(false);
  const [editingCampaignId, setEditingCampaignId] = useState<number | null>(null);
  const [selectedCampaign, setSelectedCampaign] = useState<any | null>(null);
  const [search, setSearch] = useState('');
  const [dateFrom, setDateFrom] = useState('');
  const [dateTo, setDateTo] = useState('');
  const [page, setPage] = useState(1);
  const ITEMS_PER_PAGE = 12;
  const confirm = useConfirm();
  const qc = useQueryClient();

  const { data: campaigns = [], isLoading: loading } = useQuery<any[]>({
    queryKey: ['email-campaigns'],
    queryFn: () => emailService.getCampaigns(),
  });

  const { data: availableRecipients = [] } = useQuery<any[]>({
    queryKey: ['email-recipients'],
    queryFn: async () => (await emailService.getAvailableRecipients()).data || [],
  });

  const fetchCampaigns = () => qc.invalidateQueries({ queryKey: ['email-campaigns'] });

  useEffect(() => { setPage(1); }, [search, dateFrom, dateTo]);

  const filtered = useMemo(() => {
    return (campaigns || []).filter((c: any) => {
      const q = search.toLowerCase();
      if (q && !(c.title || '').toLowerCase().includes(q) && !(c.subject || '').toLowerCase().includes(q)) return false;
      if (c.created_at) {
        const d = new Date(c.created_at);
        if (dateFrom && d < new Date(dateFrom)) return false;
        if (dateTo) {
          const end = new Date(dateTo);
          end.setHours(23, 59, 59, 999);
          if (d > end) return false;
        }
      }
      return true;
    });
  }, [campaigns, search, dateFrom, dateTo]);

  const totalPages = Math.max(1, Math.ceil(filtered.length / ITEMS_PER_PAGE));
  const safePage = Math.min(page, totalPages);
  const paginatedCampaigns = filtered.slice((safePage - 1) * ITEMS_PER_PAGE, safePage * ITEMS_PER_PAGE);

  useEffect(() => {
    if (!showBuilder) {
      setHeaderAction(
        <button className={commonStyles['btn-primary']} onClick={() => {
          setShowBuilder(true);
          onToggleFullScreen(true);
        }}>
          <Plus size={16} /> Nueva Campaña
        </button>
      );
    } else {
      setHeaderAction(null);
    }
    return () => setHeaderAction(null);
  }, [showBuilder]);

  const handleSave = async (data: { title: string, subject: string, blocks: any }) => {
    try {
      const { title, subject, blocks } = data;
      if (editingCampaignId) {
        const camp = campaigns.find(c => c.id === editingCampaignId);
        if (camp) {
          if (camp.template_id) {
            await emailService.updateTemplate(camp.template_id, {
              title: title,
              subject: subject,
              content: JSON.stringify(blocks)
            });
          }
          await emailService.updateCampaign(camp.id, { title: title, subject: subject });
          await fetchCampaigns();
        }
      } else {
        const template = await emailService.createTemplate({
          title: title,
          subject: subject || 'Asunto de la campaña',
          content: JSON.stringify(blocks),
          type: 'campaign'
        });
        await emailService.createCampaign({
          template_id: template.id,
          title: title,
          subject: subject || 'Asunto de la campaña',
          status: 'draft'
        });
        await fetchCampaigns();
      }
      closeBuilder();
    } catch (error) {
      console.error("Error saving campaign:", error);
      alert("Error al guardar en la base de datos");
    }
  };

  const getRecipientsCount = (recipients: any): number => {
    if (Array.isArray(recipients)) return recipients.length;
    if (recipients && typeof recipients === 'object') {
      const uCount = (recipients.userIds || []).length;
      const eCount = (recipients.expressContacts || []).length;
      return uCount + eCount;
    }
    return 0;
  };

  const handleSend = async (data: { title: string, subject: string, blocks: any, recipientIds: any }) => {
    try {
      const { title, subject, blocks, recipientIds } = data;
      let campaignId = editingCampaignId;

      if (!campaignId) {
        const template = await emailService.createTemplate({
          title: title,
          subject: subject || 'Asunto de la campaña',
          content: JSON.stringify(blocks),
          type: 'campaign'
        });
        const newCampaign = await emailService.createCampaign({
          template_id: template.id,
          title: title,
          subject: subject || 'Asunto de la campaña',
          status: 'draft',
          recipients: getRecipientsCount(recipientIds),
          recipient_list: JSON.stringify(recipientIds)
        });
        campaignId = newCampaign.id;
      } else {
        const camp = campaigns.find(c => c.id === editingCampaignId);
        if (camp && camp.template_id) {
          await emailService.updateTemplate(camp.template_id, {
            title: title,
            subject: subject,
            content: JSON.stringify(blocks)
          });
          await emailService.updateCampaign(camp.id, { 
            title,
            subject,
            recipients: getRecipientsCount(recipientIds),
            recipient_list: JSON.stringify(recipientIds)
          });
        }
      }

      if (campaignId) {
        await emailService.sendCampaign(campaignId);
        alert('¡Campaña enviada con éxito!');
        closeBuilder();
        fetchCampaigns();
      }
    } catch (error) {
      console.error("Error sending campaign:", error);
      alert('Error al enviar la campaña.');
    }
  };

  const handleSchedule = async (data: { title: string, subject: string, blocks: any, date: string, recipientIds: any }) => {
    try {
      const { title, subject, blocks, date, recipientIds } = data;
      let campaignId = editingCampaignId;

      if (!campaignId) {
        const template = await emailService.createTemplate({
          title: title,
          subject: subject || 'Asunto de la campaña',
          content: JSON.stringify(blocks),
          type: 'campaign'
        });
        const newCampaign = await emailService.createCampaign({
          template_id: template.id,
          title: title,
          subject: subject || 'Asunto de la campaña',
          status: 'draft',
          recipients: getRecipientsCount(recipientIds),
          recipient_list: JSON.stringify(recipientIds)
        });
        campaignId = newCampaign.id;
      } else {
        const camp = campaigns.find(c => c.id === editingCampaignId);
        if (camp && camp.template_id) {
          await emailService.updateTemplate(camp.template_id, {
            title: title,
            subject: subject,
            content: JSON.stringify(blocks)
          });
          await emailService.updateCampaign(camp.id, { 
            title,
            subject,
            recipients: getRecipientsCount(recipientIds),
            recipient_list: JSON.stringify(recipientIds)
          });
        }
      }

      if (campaignId) {
        await emailService.updateCampaign(campaignId, {
          status: 'scheduled',
          scheduled_at: date,
          recipients: getRecipientsCount(recipientIds),
          recipient_list: JSON.stringify(recipientIds)
        });
        alert(`¡Campaña programada para el ${new Date(date).toLocaleString()}!`);
        closeBuilder();
        fetchCampaigns();
      }
    } catch (error) {
      console.error("Error scheduling campaign:", error);
      alert('Error al programar la campaña.');
    }
  };

  const handleViewDetail = (camp: any) => {
    setSelectedCampaign(camp);
  };

  const handleEdit = (camp: any) => {
    setSelectedCampaign(null);
    setEditingCampaignId(camp.id);
    setShowBuilder(true);
    onToggleFullScreen(true);
  };

  const handleDelete = async (campaignId: number) => {
    const ok = await confirm({
      title: 'Eliminar campaña',
      message: '¿Estás seguro de que deseas eliminar esta campaña?',
      confirmLabel: 'Eliminar',
      variant: 'danger',
    });
    if (!ok) return;
    try {
      await emailService.deleteCampaign(campaignId);
      setSelectedCampaign(null);
      fetchCampaigns();
    } catch (error) {
      console.error("Error deleting campaign:", error);
      alert("Error al eliminar la campaña.");
    }
  };

  const closeBuilder = () => {
    setShowBuilder(false);
    setEditingCampaignId(null);
    onToggleFullScreen(false);
  };

  if (showBuilder) {
    const currentCampaign = editingCampaignId 
      ? campaigns.find(c => c.id === editingCampaignId)
      : null;
    
    const currentBlocksJson = currentCampaign?.template?.content || '[]';
    let initialBlocks = [];
    try {
      initialBlocks = JSON.parse(currentBlocksJson || '[]');
    } catch (e) {
      console.error("Error parsing blocks:", e);
    }

    let initialRecipientIds: any = { groupIds: [], userIds: [], expressContacts: [] };
    try {
      if (currentCampaign?.recipient_list) {
        const parsed = JSON.parse(currentCampaign.recipient_list);
        if (Array.isArray(parsed)) {
          initialRecipientIds = { groupIds: [], userIds: parsed, expressContacts: [] };
        } else {
          initialRecipientIds = parsed;
        }
      }
    } catch (e) {
      console.error("Error parsing recipient_list:", e);
    }

    return (
      <div style={{ flex: 1, display: 'flex', flexDirection: 'column', minHeight: 0, overflow: 'hidden' }}>
        <EmailBuilder 
          onBack={closeBuilder} 
          onSave={handleSave}
          onSend={handleSend}
          onSchedule={handleSchedule}
          availableRecipients={availableRecipients}
          initialBlocks={initialBlocks}
          initialTitle={currentCampaign?.title || 'Nueva Campaña'}
          initialSubject={currentCampaign?.subject || ''}
          initialScheduledAt={currentCampaign?.scheduled_at || undefined}
          initialRecipientIds={initialRecipientIds}
        />
      </div>
    );
  }

  return (
    <div className={styles['email-section']}>
      {/* Search & Filter Bar */}
      {!showBuilder && campaigns.length > 0 && (
        <div style={{ display: 'flex', gap: 10, alignItems: 'center', marginBottom: 16, flexWrap: 'wrap' }}>
          <div style={{ position: 'relative', flex: 1, minWidth: 200 }}>
            <Search size={15} style={{ position: 'absolute', left: 10, top: '50%', transform: 'translateY(-50%)', color: '#94a3b8', pointerEvents: 'none' }} />
            <input
              type="text"
              placeholder="Buscar por nombre o asunto..."
              value={search}
              onChange={e => setSearch(e.target.value)}
              style={{ width: '100%', border: '1px solid #e2e8f0', borderRadius: 8, padding: '8px 12px 8px 34px', fontSize: 13, outline: 'none', color: '#1e293b', background: '#fff', boxSizing: 'border-box' }}
            />
          </div>
          <input type="date" value={dateFrom} onChange={e => setDateFrom(e.target.value)}
            style={{ border: '1px solid #e2e8f0', borderRadius: 8, padding: '8px 12px', fontSize: 13, outline: 'none', color: '#1e293b', background: '#fff' }}
            title="Desde" />
          <input type="date" value={dateTo} onChange={e => setDateTo(e.target.value)}
            style={{ border: '1px solid #e2e8f0', borderRadius: 8, padding: '8px 12px', fontSize: 13, outline: 'none', color: '#1e293b', background: '#fff' }}
            title="Hasta" />
          {(dateFrom || dateTo) && (
            <button onClick={() => { setDateFrom(''); setDateTo(''); }}
              style={{ padding: '8px 12px', border: '1px solid #e2e8f0', borderRadius: 8, background: '#fff', color: '#ef4444', fontSize: 12, fontWeight: 600, cursor: 'pointer', whiteSpace: 'nowrap' }}
              title="Limpiar filtros de fecha">✕ Limpiar</button>
          )}
          <span style={{ fontSize: 12, color: '#94a3b8', whiteSpace: 'nowrap' }}>{filtered.length} de {campaigns.length}</span>
        </div>
      )}

      <CampaignGrid 
        campaigns={paginatedCampaigns} 
        loading={loading} 
        onEdit={handleEdit} 
        onDelete={handleDelete}
        onViewDetail={handleViewDetail}
        emptyMessage={campaigns.length > 0 ? 'No se encontraron campañas con los filtros aplicados.' : undefined}
      />

      {!showBuilder && campaigns.length > 0 && filtered.length > 0 && (
        <Pagination currentPage={safePage} totalPages={totalPages} onPageChange={setPage} />
      )}

      {selectedCampaign && (
        <CampaignDetailPanel
          campaign={selectedCampaign}
          onClose={() => setSelectedCampaign(null)}
          onEdit={(camp) => handleEdit(camp)}
          onDelete={async (id) => { await handleDelete(id); }}
        />
      )}
    </div>
  );
};

export default EmailMarketing;
