import React, { useState, useEffect } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Plus } from 'lucide-react';
import EmailBuilder from '../EmailBuilder';
import { emailService } from '../../../../services/emailService';
import CampaignGrid from './components/CampaignGrid';
import CampaignDetailPanel from './components/CampaignDetailPanel';
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
      <CampaignGrid 
        campaigns={campaigns} 
        loading={loading} 
        onEdit={handleEdit} 
        onDelete={handleDelete}
        onViewDetail={handleViewDetail}
      />

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
