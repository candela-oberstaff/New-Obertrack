import React, { useState, useEffect } from 'react';
import { Plus } from 'lucide-react';
import EmailBuilder from '../EmailBuilder';
import { emailService } from '../../../../services/emailService';
import CampaignGrid from './components/CampaignGrid';
import styles from './EmailMarketing.module.css';
import commonStyles from '../Tools.module.css';

interface EmailMarketingProps {
  onToggleFullScreen: (isFull: boolean) => void;
}

const EmailMarketing: React.FC<EmailMarketingProps> = ({ onToggleFullScreen }) => {
  const [showBuilder, setShowBuilder] = useState(false);
  const [editingCampaignId, setEditingCampaignId] = useState<number | null>(null);
  const [availableRecipients, setAvailableRecipients] = useState<any[]>([]);
  const [campaigns, setCampaigns] = useState<any[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    fetchCampaigns();
    fetchRecipients();
  }, []);

  const fetchCampaigns = async () => {
    try {
      setLoading(true);
      const data = await emailService.getCampaigns();
      setCampaigns(data);
    } catch (error) {
      console.error("Error fetching campaigns:", error);
    } finally {
      setLoading(false);
    }
  };

  const fetchRecipients = async () => {
    try {
      const response = await emailService.getAvailableRecipients();
      setAvailableRecipients(response.data || []);
    } catch (error) {
      console.error("Error fetching recipients:", error);
    }
  };

  const handleSave = async (data: { title: string, blocks: any }) => {
    try {
      const { title, blocks } = data;
      if (editingCampaignId) {
        const camp = campaigns.find(c => c.id === editingCampaignId);
        if (camp) {
          if (camp.template_id) {
            await emailService.updateTemplate(camp.template_id, {
              title: title,
              content: JSON.stringify(blocks)
            });
          }
          await emailService.updateCampaign(camp.id, { title: title });
          await fetchCampaigns();
        }
      } else {
        const template = await emailService.createTemplate({
          title: title,
          subject: 'Asunto de la campaña',
          content: JSON.stringify(blocks),
          type: 'campaign'
        });
        await emailService.createCampaign({
          template_id: template.id,
          title: title,
          subject: 'Contenido editado',
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

  const handleSend = async (data: { title: string, blocks: any, recipientIds: number[] }) => {
    try {
      const { title, blocks, recipientIds } = data;
      let campaignId = editingCampaignId;

      if (!campaignId) {
        const template = await emailService.createTemplate({
          title: title,
          subject: 'Asunto de la campaña',
          content: JSON.stringify(blocks),
          type: 'campaign'
        });
        const newCampaign = await emailService.createCampaign({
          template_id: template.id,
          title: title,
          subject: 'Contenido editado',
          status: 'draft',
          recipients: recipientIds.length,
          recipient_list: JSON.stringify(recipientIds)
        });
        campaignId = newCampaign.id;
      } else {
        const camp = campaigns.find(c => c.id === editingCampaignId);
        if (camp && camp.template_id) {
          await emailService.updateTemplate(camp.template_id, {
            title: title,
            content: JSON.stringify(blocks)
          });
          await emailService.updateCampaign(camp.id, { 
            title,
            recipients: recipientIds.length,
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

  const handleSchedule = async (data: { title: string, blocks: any, date: string, recipientIds: number[] }) => {
    try {
      const { title, blocks, date, recipientIds } = data;
      let campaignId = editingCampaignId;

      if (!campaignId) {
        const template = await emailService.createTemplate({
          title: title,
          subject: 'Asunto de la campaña',
          content: JSON.stringify(blocks),
          type: 'campaign'
        });
        const newCampaign = await emailService.createCampaign({
          template_id: template.id,
          title: title,
          subject: 'Contenido editado',
          status: 'draft',
          recipients: recipientIds.length,
          recipient_list: JSON.stringify(recipientIds)
        });
        campaignId = newCampaign.id;
      } else {
        const camp = campaigns.find(c => c.id === editingCampaignId);
        if (camp && camp.template_id) {
          await emailService.updateTemplate(camp.template_id, {
            title: title,
            content: JSON.stringify(blocks)
          });
          await emailService.updateCampaign(camp.id, { 
            title,
            recipients: recipientIds.length,
            recipient_list: JSON.stringify(recipientIds)
          });
        }
      }

      if (campaignId) {
        await emailService.updateCampaign(campaignId, {
          status: 'scheduled',
          scheduled_at: date,
          recipients: recipientIds.length,
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

  const handleEdit = (camp: any) => {
    setEditingCampaignId(camp.id);
    setShowBuilder(true);
    onToggleFullScreen(true);
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

    let initialRecipientIds: number[] = [];
    try {
      initialRecipientIds = currentCampaign?.recipient_list
        ? JSON.parse(currentCampaign.recipient_list)
        : [];
    } catch (e) {
      console.error("Error parsing recipient_list:", e);
    }

    return <EmailBuilder 
      onBack={closeBuilder} 
      onSave={handleSave}
      onSend={handleSend}
      onSchedule={handleSchedule}
      availableRecipients={availableRecipients}
      initialBlocks={initialBlocks}
      initialTitle={currentCampaign?.title || 'Nueva Campaña'}
      initialScheduledAt={currentCampaign?.scheduled_at || undefined}
      initialRecipientIds={initialRecipientIds}
    />;
  }

  return (
    <div className={styles['email-section']}>
      <div className={commonStyles['section-header']}>
        <h2>Campañas de Email</h2>
        <button className={commonStyles['btn-primary']} onClick={() => {
          setShowBuilder(true);
          onToggleFullScreen(true);
        }}>
          <Plus size={18} />
          Nueva Campaña
        </button>
      </div>

      <CampaignGrid 
        campaigns={campaigns} 
        loading={loading} 
        onEdit={handleEdit} 
      />
    </div>
  );
};

export default EmailMarketing;
