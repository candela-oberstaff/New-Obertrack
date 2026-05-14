import React from 'react';
import { Trash2, Plus } from 'lucide-react';
import { Block } from '../types';
import styles from '../Builder.module.css';

const SocialIcons = {
  facebook: (props: any) => <svg viewBox="0 0 24 24" width="24" height="24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" {...props}><path d="M18 2h-3a5 5 0 0 0-5 5v3H7v4h3v8h4v-8h3l1-4h-4V7a1 1 0 0 1 1-1h3z" /></svg>,
  instagram: (props: any) => <svg viewBox="0 0 24 24" width="24" height="24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" {...props}><rect x="2" y="2" width="20" height="20" rx="5" ry="5" /><path d="M16 11.37A4 4 0 1 1 12.63 8 4 4 0 0 1 16 11.37z" /><line x1="17.5" y1="6.5" x2="17.51" y2="6.5" /></svg>,
  twitter: (props: any) => <svg viewBox="0 0 24 24" width="24" height="24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" {...props}><path d="M4 4l11.733 16h4.267l-11.733-16zM4 20l6.768-6.768m2.464-2.464l6.768-6.768" /></svg>,
  linkedin: (props: any) => <svg viewBox="0 0 24 24" width="24" height="24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" {...props}><path d="M16 8a6 6 0 0 1 6 6v7h-4v-7a2 2 0 0 0-2-2 2 2 0 0 0-2 2v7h-4v-7a6 6 0 0 1 6-6z" /><rect x="2" y="9" width="4" height="12" /><circle cx="4" cy="4" r="2"/></svg>,
  youtube: (props: any) => <svg viewBox="0 0 24 24" width="24" height="24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" {...props}><path d="M22.54 6.42a2.78 2.78 0 0 0-1.94-2C18.88 4 12 4 12 4s-6.88 0-8.6.42a2.78 2.78 0 0 0-1.94 2C1 8.14 1 12 1 12s0 3.86.42 5.58a2.78 2.78 0 0 0 1.94 2C5.12 20 12 20 12 20s6.88 0 8.6-.42a2.78 2.78 0 0 0 1.94-2C23 15.86 23 12 23 12s0-3.86-.42-5.58z" /><polygon points="9.75 15.02 15.5 12 9.75 8.98 9.75 15.02" /></svg>
};

interface CanvasProps {
  blocks: Block[];
  selectedBlockId: string | null;
  onSelectBlock: (id: string) => void;
  onDeleteBlock: (id: string, e: React.MouseEvent) => void;
}

const Canvas: React.FC<CanvasProps> = ({ blocks, selectedBlockId, onSelectBlock, onDeleteBlock }) => {
  return (
    <main className={styles['builder-canvas']}>
      <div className={styles['email-paper']}>
        {blocks.length === 0 ? (
          <div className={styles['canvas-empty']}>
            <Plus size={40} color="#e2e8f0" style={{ marginBottom: '12px' }} />
            <p style={{ fontSize: '14px' }}>Arrastra bloques aquí para diseñar tu email</p>
          </div>
        ) : (
          blocks.map((block) => (
            <div
              key={block.id}
              className={`${styles['rendered-block']} ${selectedBlockId === block.id ? styles.active : ''}`}
              onClick={() => onSelectBlock(block.id)}
            >
              {selectedBlockId === block.id && (
                <div className={styles['block-actions']}>
                  <button
                    className={styles['action-btn']}
                    onClick={(e) => onDeleteBlock(block.id, e)}
                  >
                    <Trash2 size={14} />
                  </button>
                </div>
              )}
              
              {block.type === 'text' && (
                <div className={styles['text-block']} style={block.style}>
                  {block.content || 'Escribe tu texto aquí...'}
                </div>
              )}
              
              {block.type === 'button' && (
                <div className={styles['button-block']}>
                  {block.style.linkUrl ? (
                    <a
                      href={block.style.linkUrl}
                      target="_blank"
                      rel="noopener noreferrer"
                      className={styles['button-element']}
                      style={{ ...block.style, textDecoration: 'none' }}
                      onClick={(e) => e.stopPropagation()}
                    >
                      {block.content}
                    </a>
                  ) : (
                    <button className={styles['button-element']} style={block.style}>
                      {block.content}
                    </button>
                  )}
                </div>
              )}
              
              {block.type === 'image' && (
                <div className={styles['image-block']}>
                  <img
                    src={block.content}
                    alt=""
                    style={{ width: block.style.width || '100%' }}
                  />
                </div>
              )}
              
              {block.type === 'divider' && <div className={styles['divider-block']} />}
              
              {block.type === 'spacer' && <div className={styles['spacer-block']} />}
              
              {block.type === 'social' && (
                <div style={{ display: 'flex', justifyContent: 'center', gap: '16px', padding: '12px 0' }}>
                  {Object.entries(block.content).map(([key, val]: [string, any]) => {
                    if (!val.active) return null;
                    const Icon = (SocialIcons as any)[key];
                    return (
                      <div key={key}>
                        <Icon size={24} color="#64748b" />
                      </div>
                    );
                  })}
                </div>
              )}
            </div>
          ))
        )}
      </div>
    </main>
  );
};

export default Canvas;
