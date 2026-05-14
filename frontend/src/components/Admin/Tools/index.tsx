import React, { useState } from 'react';
import { Mail, ClipboardList, BarChart3 } from 'lucide-react';
import EmailMarketing from './EmailMarketing';
import Surveys from './Surveys';
import Metrics from './Metrics';
import styles from './Tools.module.css';

type ToolTab = 'email' | 'surveys' | 'metrics';

const Tools: React.FC = () => {
  const [activeTab, setActiveTab] = useState<ToolTab>('email');
  const [isFullScreen, setIsFullScreen] = useState(false);

  const renderContent = () => {
    switch (activeTab) {
      case 'email':
        return <EmailMarketing onToggleFullScreen={setIsFullScreen} />;
      case 'surveys':
        return <Surveys />;
      case 'metrics':
        return <Metrics />;
      default:
        return <EmailMarketing onToggleFullScreen={setIsFullScreen} />;
    }
  };

  return (
    <div className={`${styles['tools-page']} ${isFullScreen ? styles['fullscreen-mode'] : ''}`}>
      {!isFullScreen && (
        <>
          <div className={styles['tools-header']}>
            <h1>Herramientas Admin</h1>
            <p>Gestiona campañas de marketing, encuestas de satisfacción y métricas de rendimiento.</p>
          </div>

          <div className={styles['tools-tabs']}>
            <button 
              className={`${styles['tab-btn']} ${activeTab === 'email' ? styles.active : ''}`}
              onClick={() => setActiveTab('email')}
            >
              <Mail size={18} />
              Email Marketing
            </button>
            <button 
              className={`${styles['tab-btn']} ${activeTab === 'surveys' ? styles.active : ''}`}
              onClick={() => setActiveTab('surveys')}
            >
              <ClipboardList size={18} />
              Encuestas
            </button>
            <button 
              className={`${styles['tab-btn']} ${activeTab === 'metrics' ? styles.active : ''}`}
              onClick={() => setActiveTab('metrics')}
            >
              <BarChart3 size={18} />
              Métricas
            </button>
          </div>
        </>
      )}

      <div className={styles['tools-content']}>
        {renderContent()}
      </div>
    </div>
  );
};

export default Tools;
