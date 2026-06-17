// ─── Shared Email Types ─────────────────────────────────────────────────────
// Used by EmailBuilder, GestorPlantillas, and EmailMarketing

export type BlockType = 'text' | 'button' | 'image' | 'divider' | 'spacer' | 'social';

export interface EmailBlock {
  id: string;
  type: BlockType;
  content: string;
  style: Record<string, string>;
}

export interface EmailTemplate {
  id?: number;
  title: string;
  subject: string;
  content: string; // JSON string of EmailBlock[]
  type: string;
  is_active?: boolean;
  created_at?: string;
}

// ─── Block defaults ─────────────────────────────────────────────────────────
export const defaultBlock = (type: BlockType): Omit<EmailBlock, 'id'> => {
  switch (type) {
    case 'text':    return { type, content: 'Escribe tu texto aquí...', style: { fontSize: '16px', color: '#1e293b' } };
    case 'button':  return { type, content: 'Haz clic aquí', style: { backgroundColor: '#7c3aed', color: '#ffffff', borderRadius: '8px', align: 'center', linkUrl: '#' } };
    case 'image':   return { type, content: '', style: { width: '100%' } };
    case 'divider': return { type, content: '', style: { borderColor: '#e2e8f0', borderHeight: '1px', borderStyle: 'solid' } };
    case 'spacer':  return { type, content: '', style: { height: '24px' } };
    case 'social':  return { type, content: '{}', style: {} };
    default:        return { type, content: '', style: {} };
  }
};

export const uid = (): string => Math.random().toString(36).slice(2, 9);

// ─── HTML Compiler ──────────────────────────────────────────────────────────
export function compileBlocksToHTML(blocks: EmailBlock[]): string {
  const rows = blocks.map(block => {
    switch (block.type) {
      case 'text':
        return `<div style="padding:12px 24px;font-size:${block.style.fontSize||'16px'};color:${block.style.color||'#1e293b'};line-height:1.6;font-family:sans-serif;white-space:pre-wrap;">${block.content}</div>`;
      case 'button': {
        const align = block.style.align || 'center';
        return `<div style="padding:12px 24px;text-align:${align};"><a href="${block.style.linkUrl||'#'}" target="_blank" style="display:inline-block;padding:12px 28px;background-color:${block.style.backgroundColor||'#7c3aed'};color:${block.style.color||'#ffffff'};border-radius:${block.style.borderRadius||'8px'};font-weight:bold;text-decoration:none;font-size:15px;font-family:sans-serif;">${block.content}</a></div>`;
      }
      case 'image':
        if (!block.content) return '';
        return `<div style="padding:12px 24px;text-align:center;"><img src="${block.content}" alt="" style="width:${block.style.width||'100%'};max-width:100%;border-radius:4px;" /></div>`;
      case 'divider':
        return `<div style="padding:8px 24px;"><hr style="border:none;border-top:${block.style.borderHeight||'1px'} ${block.style.borderStyle||'solid'} ${block.style.borderColor||'#e2e8f0'};margin:0;" /></div>`;
      case 'spacer':
        return `<div style="height:${block.style.height||'24px'};"></div>`;
      case 'social':
        return `<div style="padding:12px 24px;text-align:center;font-family:sans-serif;"><a href="#" style="margin:0 8px;color:#7c3aed;font-weight:600;text-decoration:none;font-size:13px;">Facebook</a><a href="#" style="margin:0 8px;color:#7c3aed;font-weight:600;text-decoration:none;font-size:13px;">Instagram</a><a href="#" style="margin:0 8px;color:#7c3aed;font-weight:600;text-decoration:none;font-size:13px;">LinkedIn</a></div>`;
      default: return '';
    }
  }).join('\n');

  return `<div style="background-color:#ffffff;width:100%;max-width:600px;margin:0 auto;overflow:hidden;border-radius:8px;box-shadow:0 4px 12px rgba(0,0,0,0.05);border:1px solid #e2e8f0;">
  <div style="background-color:#f5f2fb;padding:20px;text-align:center;border-bottom:1px solid #e2e8f0;">
    <span style="font-size:12px;color:#7c3aed;font-weight:bold;letter-spacing:0.1em;text-transform:uppercase;font-family:sans-serif;">Obertrack</span>
  </div>
  ${rows}
  <div style="background-color:#f8fafc;padding:16px;text-align:center;font-size:11px;color:#94a3b8;border-top:1px solid #e2e8f0;font-family:sans-serif;">
    © 2026 Obertrack · Este mensaje fue enviado automáticamente.
  </div>
</div>`;
}

// ─── JSON serialization ─────────────────────────────────────────────────────
export function blocksToJSON(blocks: EmailBlock[]): string {
  return JSON.stringify(blocks.map(({ id: _id, ...rest }) => rest));
}

export function blocksFromJSON(json: string): EmailBlock[] {
  try {
    const raw = JSON.parse(json || '[]');
    return raw.map((b: any, i: number) => ({
      id: String(i) + '_' + Math.random().toString(36).slice(2, 5),
      type: b.type || 'text',
      content: b.content || '',
      style: b.style || {}
    }));
  } catch {
    return [];
  }
}

// ─── Palette (shared between components) ────────────────────────────────────
export const PALETTE: { type: BlockType; label: string }[] = [
  { type: 'text',    label: 'Texto' },
  { type: 'button',  label: 'Botón' },
  { type: 'image',   label: 'Imagen' },
  { type: 'divider', label: 'Divisor' },
  { type: 'spacer',  label: 'Espaciado' },
  { type: 'social',  label: 'Social' },
];
