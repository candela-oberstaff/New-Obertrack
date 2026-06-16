import { BlockType } from './types';

export const getDefaultContent = (type: BlockType) => {
  switch (type) {
    case 'text': return '';
    case 'button': return 'Haz clic aquí';
    case 'image': return 'https://via.placeholder.com/600x300';
    case 'social': return {
      facebook: { active: true, url: '' },
      instagram: { active: true, url: '' },
      twitter: { active: true, url: '' },
      linkedin: { active: false, url: '' },
      youtube: { active: false, url: '' }
    };
    case 'divider': return null;
    case 'spacer': return null;
    case 'columns': return {
      leftBlock: { id: 'l1', type: 'text', content: 'Columna Izquierda', style: { fontSize: '14px', color: '#1e293b' } },
      rightBlock: { id: 'r1', type: 'text', content: 'Columna Derecha', style: { fontSize: '14px', color: '#1e293b' } }
    };
    default: return '';
  }
};

export const getDefaultStyle = (type: BlockType) => {
  switch (type) {
    case 'text': return { fontSize: '16px', color: '#1e293b' };
    case 'button': return { backgroundColor: '#3b82f6', color: '#ffffff', borderRadius: '8px' };
    case 'image': return { width: '100%', borderRadius: '4px' };
    case 'social': return {};
    case 'columns': return { padding: '16px 0', gap: '20px' };
    default: return {};
  }
};
