import React from 'react';
import { Settings } from 'lucide-react';
import { Block } from '../types';
import { Select } from '../../../../ui/Select';
import styles from '../Builder.module.css';

interface PropertiesPanelProps {
  selectedBlock: Block | undefined;
  onUpdateBlock: (id: string, updates: Partial<Block>) => void;
}

const PropertiesPanel: React.FC<PropertiesPanelProps> = ({ selectedBlock, onUpdateBlock }) => {
  if (!selectedBlock) {
    return (
      <aside className={styles['builder-properties']}>
        <div style={{ height: '100%', display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', color: '#94a3b8', textAlign: 'center', padding: '20px' }}>
          <Settings size={32} style={{ marginBottom: '12px', opacity: 0.5 }} />
          <p style={{ fontSize: '14px' }}>Selecciona un bloque para editar sus propiedades</p>
        </div>
      </aside>
    );
  }

  return (
    <aside className={styles['builder-properties']}>
      <div className={styles['prop-group']}>
        <h4>Configuración del Bloque</h4>

        {selectedBlock.type === 'text' && (
          <div className={styles['prop-control']}>
            <label>Contenido</label>
            <textarea
              value={selectedBlock.content}
              onChange={(e) => onUpdateBlock(selectedBlock.id, { content: e.target.value })}
            />
          </div>
        )}

        {selectedBlock.type === 'button' && (
          <>
            <div className={styles['prop-control']}>
              <label>Texto del Botón</label>
              <input
                type="text"
                value={selectedBlock.content}
                onChange={(e) => onUpdateBlock(selectedBlock.id, { content: e.target.value })}
              />
            </div>
            <div className={styles['prop-control']}>
              <label>URL del enlace</label>
              <input
                type="url"
                placeholder="https://..."
                value={selectedBlock.style.linkUrl || ''}
                onChange={(e) => onUpdateBlock(selectedBlock.id, {
                  style: { ...selectedBlock.style, linkUrl: e.target.value }
                })}
              />
            </div>
            <div className={styles['prop-control']}>
              <label>Radio de borde (px)</label>
              <input
                type="number"
                value={parseInt(selectedBlock.style.borderRadius) || 0}
                onChange={(e) => onUpdateBlock(selectedBlock.id, {
                  style: { ...selectedBlock.style, borderRadius: `${e.target.value}px` }
                })}
              />
            </div>
          </>
        )}

        {selectedBlock.type === 'image' && (
          <>
            <div className={styles['prop-control']}>
              <label>URL de la Imagen</label>
              <input
                type="text"
                value={selectedBlock.content}
                onChange={(e) => onUpdateBlock(selectedBlock.id, { content: e.target.value })}
              />
            </div>
            <div className={styles['prop-control']}>
              <label>Ancho (%)</label>
              <input
                type="range"
                min="10"
                max="100"
                value={parseInt(selectedBlock.style.width) || 100}
                onChange={(e) => onUpdateBlock(selectedBlock.id, {
                  style: { ...selectedBlock.style, width: `${e.target.value}%` }
                })}
              />
            </div>
          </>
        )}

        {selectedBlock.type === 'social' && (
          <div className={styles['prop-control']}>
            <label>Redes Sociales</label>
            {Object.entries(selectedBlock.content).map(([key, val]: [string, any]) => (
              <div key={key} style={{ marginBottom: '12px', padding: '12px', background: '#f8fafc', borderRadius: '8px', border: '1px solid #f1f5f9' }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: '10px', marginBottom: '8px' }}>
                  <input
                    type="checkbox"
                    checked={val.active}
                    onChange={(e) => onUpdateBlock(selectedBlock.id, {
                      content: { ...selectedBlock.content, [key]: { ...val, active: e.target.checked } }
                    })}
                  />
                  <span style={{ fontSize: '12px', fontWeight: 600, textTransform: 'capitalize' }}>{key}</span>
                </div>
                {val.active && (
                  <input
                    type="text"
                    placeholder="URL..."
                    value={val.url}
                    onChange={(e) => onUpdateBlock(selectedBlock.id, {
                      content: { ...selectedBlock.content, [key]: { ...val, url: e.target.value } }
                    })}
                  />
                )}
              </div>
            ))}
          </div>
        )}

        <hr style={{ margin: '24px 0', border: 'none', borderTop: '1px solid #f1f5f9' }} />
        
        <h4>Estilos Visuales</h4>

        <div className={styles['prop-control']}>
          <label>Color de fondo</label>
          <div style={{ display: 'flex', gap: '8px' }}>
            <input
              type="color"
              style={{ width: '40px', padding: '2px', height: '32px' }}
              value={selectedBlock.style.backgroundColor || '#ffffff'}
              onChange={(e) => onUpdateBlock(selectedBlock.id, {
                style: { ...selectedBlock.style, backgroundColor: e.target.value }
              })}
            />
            <input
              type="text"
              value={selectedBlock.style.backgroundColor || '#ffffff'}
              onChange={(e) => onUpdateBlock(selectedBlock.id, {
                style: { ...selectedBlock.style, backgroundColor: e.target.value }
              })}
            />
          </div>
        </div>

        {(selectedBlock.type === 'text' || selectedBlock.type === 'button') && (
          <div className={styles['prop-control']}>
            <label>Color de texto</label>
            <div style={{ display: 'flex', gap: '8px' }}>
              <input
                type="color"
                style={{ width: '40px', padding: '2px', height: '32px' }}
                value={selectedBlock.style.color || '#000000'}
                onChange={(e) => onUpdateBlock(selectedBlock.id, {
                  style: { ...selectedBlock.style, color: e.target.value }
                })}
              />
              <input
                type="text"
                value={selectedBlock.style.color || '#000000'}
                onChange={(e) => onUpdateBlock(selectedBlock.id, {
                  style: { ...selectedBlock.style, color: e.target.value }
                })}
              />
            </div>
          </div>
        )}

        {selectedBlock.type === 'text' && (
          <div className={styles['prop-control']}>
            <label>Tamaño de fuente</label>
            <Select
              fullWidth
              value={selectedBlock.style.fontSize}
              onChange={(v) => onUpdateBlock(selectedBlock.id, {
                style: { ...selectedBlock.style, fontSize: String(v) }
              })}
              options={['12px', '14px', '16px', '18px', '20px', '24px', '32px'].map(s => ({ value: s, label: s }))}
            />
          </div>
        )}
      </div>
    </aside>
  );
};

export default PropertiesPanel;
