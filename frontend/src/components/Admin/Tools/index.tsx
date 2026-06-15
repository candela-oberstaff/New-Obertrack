import React, { useState } from 'react';
import { Mail, ClipboardList, BarChart3, Users } from 'lucide-react';
import EmailMarketing from './EmailMarketing';
import Surveys from './Surveys';
import MetricsPage from '../../../pages/Metrics';
import GroupManager from './Common/GroupManager';
import { Select } from '../../ui/Select';
import styles from './Tools.module.css';

type ToolTab = 'email' | 'surveys' | 'metrics';

const Tools: React.FC = () => {
  const [activeTab, setActiveTab] = useState<ToolTab>('email');
  const [isFullScreen, setIsFullScreen] = useState(false);
  const [extraAction, setExtraAction] = useState<React.ReactNode>(null);
  const [showGroupManager, setShowGroupManager] = useState(false);

  const renderContent = () => {
    switch (activeTab) {
      case 'email':
        return <EmailMarketing onToggleFullScreen={setIsFullScreen} setHeaderAction={setExtraAction} />;
      case 'surveys':
        return <Surveys setHeaderAction={setExtraAction} />;
      case 'metrics':
        return <MetricsPage />;
      default:
        return <EmailMarketing onToggleFullScreen={setIsFullScreen} setHeaderAction={setExtraAction} />;
    }
  };

  return (
    <div className={`${styles['tools-page']} ${isFullScreen ? styles['fullscreen-mode'] : ''}`}>
      {!isFullScreen && (
        <div className={styles['tools-top-bar']} data-tour="tools-header">
          <div className={styles['tools-header']}>
            <h1>Admin Tools</h1>
          </div>

          <div className={styles['tools-nav-group']}>
            <div className={styles['tools-tabs']} data-tour="tools-tabs">
              <button 
                className={`${styles['tab-btn']} ${activeTab === 'email' ? styles.active : ''}`}
                onClick={() => setActiveTab('email')}
              >
                <Mail size={14} /> Email
              </button>
              <button 
                className={`${styles['tab-btn']} ${activeTab === 'surveys' ? styles.active : ''}`}
                onClick={() => setActiveTab('surveys')}
              >
                <ClipboardList size={14} /> Encuestas
              </button>
              <button 
                className={`${styles['tab-btn']} ${activeTab === 'metrics' ? styles.active : ''}`}
                onClick={() => setActiveTab('metrics')}
              >
                <BarChart3 size={14} /> Métricas
              </button>
            </div>

            <button
              className={styles['btn-outline']}
              onClick={() => setShowGroupManager(true)}
              style={{ display: 'flex', alignItems: 'center', gap: '6px' }}
            >
              <Users size={16} /> Gestionar Grupos
            </button>
            
            <div className={styles['mobile-tabs']}>
              <Select
                fullWidth
                value={activeTab}
                onChange={(v) => setActiveTab(v as ToolTab)}
                options={[
                  { value: 'email', label: 'Email' },
                  { value: 'surveys', label: 'Encuestas' },
                  { value: 'metrics', label: 'Métricas' },
                ]}
              />
            </div>
            <div data-tour="tools-extra-action">{extraAction}</div>
          </div>
        </div>
      )}

      <div className={styles['tools-content']} data-tour="tools-content">
        {renderContent()}
      </div>

      {showGroupManager && (
        <GroupManager onClose={() => setShowGroupManager(false)} />
      )}
    </div>
  );
};

export default Tools;
