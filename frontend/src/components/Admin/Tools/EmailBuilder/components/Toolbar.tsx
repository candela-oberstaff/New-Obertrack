import React from 'react';
import { Type, Image as ImageIcon, Square, Minus, Share2, Plus } from 'lucide-react';
import { BlockType } from '../types';
import styles from '../Builder.module.css';

interface ToolbarProps {
  onAddBlock: (type: BlockType) => void;
}

const Toolbar: React.FC<ToolbarProps> = ({ onAddBlock }) => {
  const tools: { type: BlockType; icon: any; label: string }[] = [
    { type: 'text', icon: Type, label: 'Texto' },
    { type: 'image', icon: ImageIcon, label: 'Imagen' },
    { type: 'button', icon: Square, label: 'Botón' },
    { type: 'divider', icon: Minus, label: 'Divisor' },
    { type: 'social', icon: Share2, label: 'Social' },
    { type: 'spacer', icon: Plus, label: 'Espacio' },
  ];

  return (
    <aside className={styles['builder-sidebar']}>
      <div className={styles['sidebar-content']}>
        {tools.map((tool) => (
          <button
            key={tool.type}
            className={styles['block-item']}
            onClick={() => onAddBlock(tool.type)}
          >
            <tool.icon size={20} />
            <span>{tool.label}</span>
          </button>
        ))}
      </div>
    </aside>
  );
};

export default Toolbar;
