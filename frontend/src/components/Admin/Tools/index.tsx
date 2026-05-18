import React, { useState } from 'react';
import { Mail, ClipboardList } from 'lucide-react';
import EmailMarketing from './EmailMarketing';
import Surveys from './Surveys';
import styles from './Tools.module.css';

type ToolTab = 'email' | 'surveys';

const Tools: React.FC = () => {
  const [activeTab, setActiveTab] = useState<ToolTab>('email');
  const [isFullScreen, setIsFullScreen] = useState(false);
  const [extraAction, setExtraAction] = useState<React.ReactNode>(null);

  const renderContent = () => {
    switch (activeTab) {
      case 'email':
        return <EmailMarketing onToggleFullScreen={setIsFullScreen} setHeaderAction={setExtraAction} />;
      case 'surveys':
        return <Surveys setHeaderAction={setExtraAction} />;
      default:
        return <EmailMarketing onToggleFullScreen={setIsFullScreen} setHeaderAction={setExtraAction} />;
    }
  };

  return (
    <div className={`${styles['tools-page']} ${isFullScreen ? styles['fullscreen-mode'] : ''}`}>
      {!isFullScreen && (
        <div className={styles['tools-top-bar']}>
          <div className={styles['tools-header']}>
            <h1>Admin Tools</h1>
          </div>

          <div className={styles['tools-nav-group']}>
            <div className={styles['tools-tabs']}>
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
            </div>
            
            <div className={styles['mobile-tabs']}>
              <select 
                value={activeTab} 
                onChange={(e) => setActiveTab(e.target.value as ToolTab)}
              >
                <option value="email">Email</option>
                <option value="surveys">Encuestas</option>
              </select>
            </div>
            {extraAction}
          </div>
        </div>
      )}

      <div className={styles['tools-content']}>
        {renderContent()}
      </div>
    </div>
  );
};

export default Tools;
