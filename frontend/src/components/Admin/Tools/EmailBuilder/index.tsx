import React, { useState, useCallback, useMemo, useEffect } from 'react';
import {
  ChevronLeft, Save, Send, Eye, Code, Type, MousePointerClick,
  Image, Minus, AlignJustify, Share2, Trash2, ChevronUp, ChevronDown,
  Laptop, Smartphone
} from 'lucide-react';
import { EmailBuilderProps } from './types';
import RecipientSelector from '../../../../pages/Email/RecipientSelector';
import { emailService, EmailTemplate } from '../../../../services/emailService';

// ─── Local block types ────────────────────────────────────────────────────────
type BlockType = 'text' | 'button' | 'image' | 'divider' | 'spacer' | 'social';

interface EmailBlock {
  id: string;
  type: BlockType;
  content: string;
  style: Record<string, string>;
}

const PALETTE: { type: BlockType; label: string; icon: React.ReactNode }[] = [
  { type: 'text',    label: 'Texto',     icon: <Type size={14} /> },
  { type: 'button',  label: 'Botón',     icon: <MousePointerClick size={14} /> },
  { type: 'image',   label: 'Imagen',    icon: <Image size={14} /> },
  { type: 'divider', label: 'Divisor',   icon: <Minus size={14} /> },
  { type: 'spacer',  label: 'Espaciado', icon: <AlignJustify size={14} /> },
  { type: 'social',  label: 'Social',    icon: <Share2 size={14} /> },
];

const uid = () => Math.random().toString(36).slice(2, 9);

const defaultBlock = (type: BlockType): Omit<EmailBlock, 'id'> => {
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

// ─── HTML compiler ────────────────────────────────────────────────────────────
function compileBlocksToHTML(blocks: EmailBlock[]): string {
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

// ─── Block visual preview ─────────────────────────────────────────────────────
function BlockPreview({ block }: { block: EmailBlock }) {
  switch (block.type) {
    case 'text':
      return <div style={{ padding: '12px 24px', fontSize: block.style.fontSize || '16px', color: block.style.color || '#1e293b', lineHeight: 1.6, whiteSpace: 'pre-wrap' }}>{block.content}</div>;
    case 'button': {
      const align = block.style.align || 'center';
      return (
        <div style={{ padding: '12px 24px', textAlign: align as React.CSSProperties['textAlign'] }}>
          <span style={{ display: 'inline-block', padding: '12px 28px', background: block.style.backgroundColor || '#7c3aed', color: block.style.color || '#fff', borderRadius: block.style.borderRadius || '8px', fontWeight: 600, fontSize: '15px', cursor: 'pointer' }}>
            {block.content}
          </span>
        </div>
      );
    }
    case 'image':
      return (
        <div style={{ padding: '12px 24px' }}>
          {block.content
            ? <img src={block.content} alt="" style={{ width: block.style.width || '100%', maxWidth: '100%', display: 'block', borderRadius: '4px' }} />
            : <div style={{ background: '#f1f5f9', border: '2px dashed #cbd5e1', borderRadius: '8px', height: '80px', display: 'flex', alignItems: 'center', justifyContent: 'center', color: '#94a3b8', fontSize: '12px', gap: 6 }}>
                <Image size={18} /> Pega URL de imagen en el inspector
              </div>
          }
        </div>
      );
    case 'divider':
      return <div style={{ padding: '8px 24px' }}><hr style={{ border: 'none', borderTop: `${block.style.borderHeight || '1px'} ${block.style.borderStyle || 'solid'} ${block.style.borderColor || '#e2e8f0'}`, margin: 0 }} /></div>;
    case 'spacer':
      return <div style={{ height: block.style.height || '24px', background: '#f8fafc', borderTop: '1px dashed #e2e8f0', borderBottom: '1px dashed #e2e8f0', margin: '0 24px' }} />;
    case 'social':
      return (
        <div style={{ padding: '12px 24px', textAlign: 'center', display: 'flex', justifyContent: 'center', gap: 12 }}>
          {['Facebook', 'Instagram', 'LinkedIn'].map(n => <span key={n} style={{ fontSize: '13px', color: '#7c3aed', fontWeight: 600 }}>{n}</span>)}
        </div>
      );
    default: return null;
  }
}

// ─── Inspector panel ─────────────────────────────────────────────────────────
function Inspector({ block, onChange }: { block: EmailBlock; onChange: (b: EmailBlock) => void }) {
  const set = (key: string, val: string) =>
    onChange({ ...block, content: key === 'content' ? val : block.content, style: key === 'content' ? block.style : { ...block.style, [key]: val } });

  const fieldStyle: React.CSSProperties = { display: 'flex', flexDirection: 'column', gap: 4, padding: '8px 12px', borderBottom: '1px solid #f1f5f9' };
  const labelStyle: React.CSSProperties = { fontSize: 10, fontWeight: 700, color: '#94a3b8', textTransform: 'uppercase', letterSpacing: '0.06em' };
  const inputStyle: React.CSSProperties = { border: '1px solid #e2e8f0', borderRadius: 6, padding: '5px 8px', fontSize: 12, outline: 'none', color: '#1e293b', background: '#f8fafc', width: '100%' };
  const selectStyle: React.CSSProperties = { ...inputStyle, cursor: 'pointer' };

  return (
    <div>
      {block.type === 'text' && (
        <>
          <div style={fieldStyle}>
            <label style={labelStyle}>Contenido</label>
            <textarea style={{ ...inputStyle, resize: 'vertical', minHeight: 72 }} value={block.content} onChange={e => set('content', e.target.value)} rows={3} />
          </div>
          <div style={fieldStyle}>
            <label style={labelStyle}>Tamaño de fuente</label>
            <select style={selectStyle} value={block.style.fontSize || '16px'} onChange={e => set('fontSize', e.target.value)}>
              {['12px','14px','16px','18px','20px','24px','28px','32px'].map(s => <option key={s}>{s}</option>)}
            </select>
          </div>
          <div style={fieldStyle}>
            <label style={labelStyle}>Color</label>
            <div style={{ display: 'flex', gap: 6, alignItems: 'center' }}>
              <input type="color" style={{ width: 28, height: 28, borderRadius: 6, border: '1px solid #e2e8f0', padding: 2, cursor: 'pointer' }} value={block.style.color || '#1e293b'} onChange={e => set('color', e.target.value)} />
              <input style={{ ...inputStyle, flex: 1 }} value={block.style.color || '#1e293b'} onChange={e => set('color', e.target.value)} />
            </div>
          </div>
        </>
      )}
      {block.type === 'button' && (
        <>
          <div style={fieldStyle}><label style={labelStyle}>Texto</label><input style={inputStyle} value={block.content} onChange={e => set('content', e.target.value)} /></div>
          <div style={fieldStyle}><label style={labelStyle}>URL destino</label><input style={inputStyle} placeholder="https://..." value={block.style.linkUrl || ''} onChange={e => set('linkUrl', e.target.value)} /></div>
          <div style={fieldStyle}>
            <label style={labelStyle}>Color de fondo</label>
            <div style={{ display: 'flex', gap: 6, alignItems: 'center' }}>
              <input type="color" style={{ width: 28, height: 28, borderRadius: 6, border: '1px solid #e2e8f0', padding: 2, cursor: 'pointer' }} value={block.style.backgroundColor || '#7c3aed'} onChange={e => set('backgroundColor', e.target.value)} />
              <input style={{ ...inputStyle, flex: 1 }} value={block.style.backgroundColor || '#7c3aed'} onChange={e => set('backgroundColor', e.target.value)} />
            </div>
          </div>
          <div style={fieldStyle}>
            <label style={labelStyle}>Color texto</label>
            <div style={{ display: 'flex', gap: 6, alignItems: 'center' }}>
              <input type="color" style={{ width: 28, height: 28, borderRadius: 6, border: '1px solid #e2e8f0', padding: 2, cursor: 'pointer' }} value={block.style.color || '#ffffff'} onChange={e => set('color', e.target.value)} />
              <input style={{ ...inputStyle, flex: 1 }} value={block.style.color || '#ffffff'} onChange={e => set('color', e.target.value)} />
            </div>
          </div>
          <div style={fieldStyle}>
            <label style={labelStyle}>Alineación</label>
            <select style={selectStyle} value={block.style.align || 'center'} onChange={e => set('align', e.target.value)}>
              <option value="left">Izquierda</option><option value="center">Centro</option><option value="right">Derecha</option>
            </select>
          </div>
        </>
      )}
      {block.type === 'image' && (
        <>
          <div style={fieldStyle}><label style={labelStyle}>URL de imagen</label><input style={inputStyle} placeholder="https://..." value={block.content} onChange={e => set('content', e.target.value)} /></div>
          <div style={fieldStyle}><label style={labelStyle}>Ancho</label><input style={inputStyle} placeholder="100%" value={block.style.width || '100%'} onChange={e => set('width', e.target.value)} /></div>
        </>
      )}
      {block.type === 'divider' && (
        <>
          <div style={fieldStyle}>
            <label style={labelStyle}>Color</label>
            <div style={{ display: 'flex', gap: 6, alignItems: 'center' }}>
              <input type="color" style={{ width: 28, height: 28, borderRadius: 6, border: '1px solid #e2e8f0', padding: 2, cursor: 'pointer' }} value={block.style.borderColor || '#e2e8f0'} onChange={e => set('borderColor', e.target.value)} />
              <input style={{ ...inputStyle, flex: 1 }} value={block.style.borderColor || '#e2e8f0'} onChange={e => set('borderColor', e.target.value)} />
            </div>
          </div>
          <div style={fieldStyle}>
            <label style={labelStyle}>Grosor</label>
            <select style={selectStyle} value={block.style.borderHeight || '1px'} onChange={e => set('borderHeight', e.target.value)}>
              {['1px','2px','3px','4px'].map(s => <option key={s}>{s}</option>)}
            </select>
          </div>
          <div style={fieldStyle}>
            <label style={labelStyle}>Estilo</label>
            <select style={selectStyle} value={block.style.borderStyle || 'solid'} onChange={e => set('borderStyle', e.target.value)}>
              <option value="solid">Sólido</option><option value="dashed">Guiones</option><option value="dotted">Puntos</option>
            </select>
          </div>
        </>
      )}
      {block.type === 'spacer' && (
        <div style={fieldStyle}>
          <label style={labelStyle}>Altura</label>
          <select style={selectStyle} value={block.style.height || '24px'} onChange={e => set('height', e.target.value)}>
            {['8px','16px','24px','32px','48px','64px'].map(s => <option key={s}>{s}</option>)}
          </select>
        </div>
      )}
      {block.type === 'social' && (
        <div style={fieldStyle}><p style={{ fontSize: 11, color: '#64748b', margin: 0 }}>El bloque social muestra links a redes automáticamente.</p></div>
      )}
    </div>
  );
}

// ─── Send/Schedule modal ──────────────────────────────────────────────────────
interface SendModalProps {
  onClose: () => void;
  onConfirm: (recipientIds: any) => void;
  onSchedule: (recipientIds: any, date: string) => void;
  initialRecipientIds?: any;
  initialScheduledAt?: string;
}

function SendModal({ onClose, onConfirm, onSchedule, initialRecipientIds, initialScheduledAt }: SendModalProps) {
  const [recipients, setRecipients] = useState<{ userIds: number[]; groupIds: number[]; expressContacts: Array<{ name: string; email: string }> }>(
    initialRecipientIds || { userIds: [], groupIds: [], expressContacts: [] }
  );
  const [scheduleDate, setScheduleDate] = useState(initialScheduledAt || '');
  const total = recipients.userIds.length + (recipients.groupIds?.length ?? 0) + (recipients.expressContacts?.length ?? 0);

  const overlayStyle: React.CSSProperties = { position: 'fixed', inset: 0, background: 'rgba(0,0,0,0.5)', zIndex: 1000, display: 'flex', alignItems: 'center', justifyContent: 'center', padding: 24 };
  const modalStyle: React.CSSProperties = { background: '#fff', borderRadius: 16, width: '100%', maxWidth: 640, maxHeight: '90vh', overflow: 'auto', boxShadow: '0 25px 50px rgba(0,0,0,0.2)', display: 'flex', flexDirection: 'column' };

  return (
    <div style={overlayStyle} onClick={onClose}>
      <div style={modalStyle} onClick={e => e.stopPropagation()}>
        <div style={{ padding: '20px 24px', borderBottom: '1px solid #f1f5f9', display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <h2 style={{ margin: 0, fontSize: 16, fontWeight: 800, color: '#1e293b' }}>Destinatarios y envío</h2>
          <button onClick={onClose} style={{ background: 'none', border: 'none', cursor: 'pointer', color: '#94a3b8', fontSize: 20, lineHeight: 1 }}>×</button>
        </div>
        <div style={{ padding: 24, flex: 1 }}>
          <RecipientSelector value={recipients} onChange={setRecipients} />
          <div style={{ marginTop: 16, display: 'flex', flexDirection: 'column', gap: 6 }}>
            <label style={{ fontSize: 11, fontWeight: 700, color: '#94a3b8', textTransform: 'uppercase', letterSpacing: '0.06em' }}>Programar envío (opcional)</label>
            <input type="datetime-local" value={scheduleDate} onChange={e => setScheduleDate(e.target.value)}
              style={{ border: '1px solid #e2e8f0', borderRadius: 8, padding: '8px 12px', fontSize: 13, outline: 'none', color: '#1e293b' }} />
          </div>
        </div>
        <div style={{ padding: '16px 24px', borderTop: '1px solid #f1f5f9', display: 'flex', gap: 10, justifyContent: 'flex-end' }}>
          <button onClick={onClose} style={{ padding: '9px 20px', borderRadius: 8, border: '1px solid #e2e8f0', background: '#fff', color: '#64748b', fontSize: 13, fontWeight: 600, cursor: 'pointer' }}>Cancelar</button>
          {scheduleDate && (
            <button onClick={() => onSchedule(recipients, scheduleDate)} style={{ padding: '9px 20px', borderRadius: 8, border: 'none', background: '#f59e0b', color: '#fff', fontSize: 13, fontWeight: 700, cursor: 'pointer' }}>
              Programar
            </button>
          )}
          <button disabled={total === 0} onClick={() => onConfirm(recipients)}
            style={{ padding: '9px 20px', borderRadius: 8, border: 'none', background: total > 0 ? '#7c3aed' : '#e2e8f0', color: total > 0 ? '#fff' : '#94a3b8', fontSize: 13, fontWeight: 700, cursor: total > 0 ? 'pointer' : 'not-allowed', display: 'flex', alignItems: 'center', gap: 6 }}>
            <Send size={14} /> Enviar a {total} destinatario{total !== 1 ? 's' : ''}
          </button>
        </div>
      </div>
    </div>
  );
}

// ─── Main EmailBuilder ────────────────────────────────────────────────────────
type EditorMode = 'blocks' | 'code' | 'preview';
type PreviewDevice = 'desktop' | 'mobile';

const EmailBuilder: React.FC<EmailBuilderProps> = ({
  onBack, onSave, onSend, onSchedule,
  initialBlocks, initialTitle, initialSubject, initialScheduledAt, initialRecipientIds
}) => {
  const [title, setTitle] = useState(initialTitle || 'Nueva Campaña');
  const [subject, setSubject] = useState(initialSubject || '');
  const [blocks, setBlocks] = useState<EmailBlock[]>(() => {
    try {
      const raw = Array.isArray(initialBlocks) ? initialBlocks : [];
      return raw.map((b, i) => ({ id: String(i), type: (b.type as BlockType) || 'text', content: b.content || '', style: b.style || {} }));
    } catch { return []; }
  });
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [mode, setMode] = useState<EditorMode>('blocks');
  const [previewDevice, setPreviewDevice] = useState<PreviewDevice>('desktop');
  const [rawHTML, setRawHTML] = useState('');
  const [showSendModal, setShowSendModal] = useState(false);
  const [templates, setTemplates] = useState<EmailTemplate[]>([]);

  useEffect(() => {
    emailService.getTemplates()
      .then(res => setTemplates(res))
      .catch(err => console.error("Error fetching templates:", err));
  }, []);

  const handleSaveAsTemplate = async () => {
    const tName = prompt("Escribe el nombre de la nueva plantilla:");
    if (!tName) return;
    const tSubject = prompt("Escribe el asunto para esta plantilla:", subject || title);
    if (!tSubject) return;

    try {
      const created = await emailService.createTemplate({
        title: tName,
        subject: tSubject,
        content: JSON.stringify(blocks),
        type: 'campaign',
        is_active: true
      });
      setTemplates(prev => [...prev, created]);
      alert("¡Plantilla creada y guardada con éxito!");
    } catch (err) {
      console.error("Error creating template from builder:", err);
      alert("Error al guardar la plantilla.");
    }
  };

  const selectedBlock = blocks.find(b => b.id === selectedId) ?? null;

  const addBlock = useCallback((type: BlockType) => {
    const nb: EmailBlock = { id: uid(), ...defaultBlock(type) };
    setBlocks(prev => [...prev, nb]);
    setSelectedId(nb.id);
  }, []);

  const updateBlock = useCallback((updated: EmailBlock) => {
    setBlocks(prev => prev.map(b => b.id === updated.id ? updated : b));
  }, []);

  const removeBlock = useCallback((id: string) => {
    setBlocks(prev => prev.filter(b => b.id !== id));
    if (selectedId === id) setSelectedId(null);
  }, [selectedId]);

  const moveBlock = useCallback((id: string, dir: -1 | 1) => {
    setBlocks(prev => {
      const idx = prev.findIndex(b => b.id === id);
      if (idx < 0) return prev;
      const newIdx = idx + dir;
      if (newIdx < 0 || newIdx >= prev.length) return prev;
      const copy = [...prev];
      [copy[idx], copy[newIdx]] = [copy[newIdx], copy[idx]];
      return copy;
    });
  }, []);

  const handleSwitchToCode = () => {
    setRawHTML(compileBlocksToHTML(blocks));
    setMode('code');
  };

  const handleRawHTMLChange = (val: string) => {
    setRawHTML(val);
    setBlocks([{ id: 'raw_code', type: 'text', content: val, style: { raw: 'true' } }]);
  };

  const previewContent = useMemo(() => {
    if (blocks[0]?.style?.raw === 'true') return blocks[0].content;
    return compileBlocksToHTML(blocks);
  }, [blocks]);

  const srcDoc = useMemo(() => `<!DOCTYPE html>
<html><head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <script src="https://cdn.tailwindcss.com"></script>
  <style>body{margin:0;padding:16px;background:#f1f5f9;min-height:100vh;display:flex;justify-content:center;align-items:flex-start;}</style>
</head><body>
  <div style="width:100%;display:flex;justify-content:center;">
    ${previewContent}
  </div>
</body></html>`, [previewContent]);

  const handleSave = () => onSave?.({ title, subject, blocks });
  const handleSend = (recipientIds: any) => { onSend?.({ title, subject, blocks, recipientIds }); setShowSendModal(false); };
  const handleSchedule = (recipientIds: any, date: string) => { onSchedule?.({ title, subject, blocks, date, recipientIds }); setShowSendModal(false); };

  // ─── Styles (inline to avoid CSS module conflicts) ────────────────────────
  const s = {
    root: { display: 'flex', flexDirection: 'column' as const, flex: 1, minHeight: 0, background: '#f8fafc', fontFamily: 'inherit', overflow: 'hidden' },
    header: { display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '12px 20px', background: '#fff', borderBottom: '1px solid #e2e8f0', gap: 12, flexShrink: 0, minHeight: 70 },
    headerLeft: { display: 'flex', alignItems: 'center', gap: 12 },
    backBtn: { width: 36, height: 36, borderRadius: 8, border: '1px solid #e2e8f0', background: '#fff', cursor: 'pointer', display: 'flex', alignItems: 'center', justifyContent: 'center', color: '#64748b' },
    titleInput: { border: 'none', outline: 'none', fontSize: 16, fontWeight: 700, color: '#1e293b', background: 'transparent', minWidth: 200 },
    tabBar: { display: 'flex', alignItems: 'center', gap: 4, background: '#f1f5f9', padding: '4px', borderRadius: 10 },
    tab: (active: boolean) => ({ padding: '8px 16px', borderRadius: 7, border: 'none', cursor: 'pointer', fontSize: 13, fontWeight: 600, display: 'flex', alignItems: 'center', gap: 6, transition: 'all 0.15s', background: active ? '#fff' : 'transparent', color: active ? '#1e293b' : '#64748b', boxShadow: active ? '0 1px 3px rgba(0,0,0,0.08)' : 'none' }),
    headerActions: { display: 'flex', alignItems: 'center', gap: 10 },
    iconBtn: { width: 36, height: 36, borderRadius: 8, border: '1px solid #e2e8f0', background: '#fff', cursor: 'pointer', display: 'flex', alignItems: 'center', justifyContent: 'center', color: '#64748b' },
    sendBtn: { padding: '9px 20px', borderRadius: 8, border: 'none', background: '#7c3aed', color: '#fff', fontSize: 13, fontWeight: 700, cursor: 'pointer', display: 'flex', alignItems: 'center', gap: 6 },
    body: { display: 'flex', flex: 1, overflow: 'hidden', minHeight: 0 },
    palette: { width: 220, background: '#ffffff', borderRight: '1px solid #e2e8f0', display: 'flex', flexDirection: 'column' as const, overflowY: 'hidden' as const, flexShrink: 0 },
    paletteLabel: { padding: '12px 16px 8px', fontSize: 10, fontWeight: 700, color: '#94a3b8', textTransform: 'uppercase' as const, letterSpacing: '0.08em', borderBottom: '1px solid #f1f5f9' },
    paletteBtn: { display: 'flex', alignItems: 'center', gap: 10, padding: '12px 16px', background: 'transparent', border: 'none', color: '#64748b', cursor: 'pointer', fontSize: 13, fontWeight: 500, textAlign: 'left' as const, transition: 'all 0.15s' },
    canvas: { flex: 1, overflowY: 'auto' as const, background: '#f8fafc', minHeight: 0 },
    canvasInner: { display: 'flex', flexDirection: 'column' as const, alignItems: 'center', padding: '24px 16px', minHeight: '100%', boxSizing: 'border-box' as const },
    emailWrap: { width: '100%', maxWidth: 600, background: '#fff', borderRadius: 12, overflow: 'hidden', boxShadow: '0 4px 20px rgba(0,0,0,0.05)', border: '1px solid #e2e8f0', marginBottom: 24, flexShrink: 0 },
    emailHeader: { background: '#f5f2fb', padding: '16px 24px', borderBottom: '1px solid #e8e3f5', textAlign: 'center' as const },
    emailFooter: { background: '#f8fafc', padding: '16px 24px', borderTop: '1px solid #f1f5f9', textAlign: 'center' as const, fontSize: 11, color: '#94a3b8' },
    inspector: { width: 280, background: '#fff', borderLeft: '1px solid #e2e8f0', display: 'flex', flexDirection: 'column' as const, overflowY: 'auto' as const, flexShrink: 0, minHeight: 0 },
    inspectorLabel: { padding: '12px 16px 8px', fontSize: 10, fontWeight: 700, color: '#94a3b8', textTransform: 'uppercase' as const, letterSpacing: '0.08em', borderBottom: '1px solid #f1f5f9', background: '#fafafa' },
    empty: { display: 'flex', flexDirection: 'column' as const, alignItems: 'center', justifyContent: 'center', padding: '60px 24px', color: '#cbd5e1', gap: 10, fontSize: 13, textAlign: 'center' as const },
    deviceBar: { display: 'flex', alignItems: 'center', gap: 4, background: '#f1f5f9', padding: '4px', borderRadius: 8 },
    deviceBtn: (active: boolean) => ({ width: 32, height: 32, borderRadius: 6, border: 'none', cursor: 'pointer', display: 'flex', alignItems: 'center', justifyContent: 'center', background: active ? '#fff' : 'transparent', color: active ? '#7c3aed' : '#94a3b8', boxShadow: active ? '0 1px 3px rgba(0,0,0,0.08)' : 'none' }),
  };

  return (
    <div style={s.root}>
      {/* Header */}
      <header style={s.header}>
        <div style={s.headerLeft}>
          <button style={s.backBtn} onClick={onBack}><ChevronLeft size={18} /></button>
          <div>
            <input style={s.titleInput} value={title} onChange={e => setTitle(e.target.value)} placeholder="Nombre de la campaña..." />
            <div style={{ display: 'flex', alignItems: 'center', gap: 6, marginTop: 2 }}>
              <span style={{ fontSize: 11, color: '#94a3b8', fontWeight: 600 }}>Asunto:</span>
              <input
                style={{ border: 'none', borderBottom: '1px dashed #cbd5e1', outline: 'none', fontSize: 11, color: '#475569', background: 'transparent', width: 220 }}
                value={subject}
                onChange={e => setSubject(e.target.value)}
                placeholder="Escribe el asunto del email..."
              />
            </div>
          </div>
        </div>

        <div style={s.tabBar}>
          <button style={s.tab(mode === 'blocks')} onClick={() => setMode('blocks')}><Type size={12} /> Visual</button>
          <button style={s.tab(mode === 'code')} onClick={handleSwitchToCode}><Code size={12} /> Código</button>
          <button style={s.tab(mode === 'preview')} onClick={() => setMode('preview')}><Eye size={12} /> Preview</button>
        </div>

        <div style={s.headerActions}>
          {mode === 'preview' && (
            <div style={s.deviceBar}>
              <button style={s.deviceBtn(previewDevice === 'desktop')} onClick={() => setPreviewDevice('desktop')} title="Escritorio"><Laptop size={14} /></button>
              <button style={s.deviceBtn(previewDevice === 'mobile')} onClick={() => setPreviewDevice('mobile')} title="Móvil"><Smartphone size={14} /></button>
            </div>
          )}
          <button style={s.iconBtn} onClick={handleSave} title="Guardar"><Save size={16} /></button>
          <button style={s.sendBtn} onClick={() => setShowSendModal(true)}><Send size={14} /> Enviar campaña</button>
        </div>
      </header>

      {/* Body */}
      <div style={s.body}>
        {mode === 'blocks' && (
          <>
            {/* Palette */}
            <div style={s.palette}>
              {/* Blocks section - scrollable */}
              <div style={{ display: 'flex', flexDirection: 'column', flex: 1, overflowY: 'auto', minHeight: 0 }}>
                <div style={s.paletteLabel}>Bloques</div>
                {PALETTE.map(p => (
                  <button key={p.type} style={s.paletteBtn} onClick={() => addBlock(p.type)}
                    onMouseEnter={e => { (e.currentTarget as HTMLElement).style.background = '#f8fafc'; (e.currentTarget as HTMLElement).style.color = '#1e293b'; }}
                    onMouseLeave={e => { (e.currentTarget as HTMLElement).style.background = 'transparent'; (e.currentTarget as HTMLElement).style.color = '#64748b'; }}>
                    <span style={{ width: 28, height: 28, borderRadius: 6, background: '#f5f2fb', display: 'flex', alignItems: 'center', justifyContent: 'center', color: '#7c3aed' }}>{p.icon}</span>
                    {p.label}
                  </button>
                ))}
              </div>

              {/* Templates section - always visible at bottom */}
              <div style={{ borderTop: '1px solid #f1f5f9', flexShrink: 0 }}>
                <div style={s.paletteLabel}>Plantillas</div>
                <div style={{ padding: '8px 12px', display: 'flex', flexDirection: 'column', gap: 8 }}>
                  <select
                    style={{
                      border: '1px solid #cbd5e1',
                      borderRadius: 8,
                      padding: '7px 10px',
                      fontSize: 12,
                      width: '100%',
                      cursor: 'pointer',
                      background: '#fff',
                      outline: 'none',
                      color: '#334155',
                      maxHeight: 36,
                    }}
                    onChange={e => {
                      const selectedTplId = Number(e.target.value);
                      if (!selectedTplId) return;
                      const tpl = templates.find(t => t.id === selectedTplId);
                      if (tpl && confirm(`¿Cargar la plantilla "${tpl.title}"? Esto reemplazará el diseño actual.`)) {
                        try {
                          const parsed = JSON.parse(tpl.content || '[]');
                          const formatted = parsed.map((b: any, i: number) => ({
                            id: String(i) + '_' + Math.random().toString(36).slice(2, 5),
                            type: b.type || 'text',
                            content: b.content || '',
                            style: b.style || {}
                          }));
                          setBlocks(formatted);
                          if (tpl.subject) setSubject(tpl.subject);
                        } catch (err) {
                          console.error("Error loading template content:", err);
                          alert("Error al cargar la plantilla.");
                        }
                      }
                      e.target.value = ""; // Reset select
                    }}
                  >
                    <option value="">— Cargar plantilla —</option>
                    {templates.map(t => (
                      <option key={t.id} value={t.id}>{t.title}</option>
                    ))}
                  </select>

                  <button
                    onClick={handleSaveAsTemplate}
                    style={{
                      padding: '7px 10px',
                      borderRadius: 8,
                      border: '1px dashed #7c3aed',
                      background: '#f5f2fb',
                      color: '#7c3aed',
                      fontSize: 11,
                      fontWeight: 600,
                      cursor: 'pointer',
                      transition: 'all 0.15s',
                      textAlign: 'center',
                      width: '100%'
                    }}
                    onMouseEnter={e => { (e.currentTarget as HTMLElement).style.background = '#7c3aed'; (e.currentTarget as HTMLElement).style.color = '#fff'; }}
                    onMouseLeave={e => { (e.currentTarget as HTMLElement).style.background = '#f5f2fb'; (e.currentTarget as HTMLElement).style.color = '#7c3aed'; }}
                  >
                    Guardar como plantilla
                  </button>
                </div>
              </div>
            </div>

            {/* Canvas */}
            <div style={s.canvas}>
              <div style={s.canvasInner}>
              <div style={s.emailWrap}>
                <div style={s.emailHeader}>
                  <span style={{ fontSize: 10, color: '#7c3aed', fontWeight: 700, textTransform: 'uppercase', letterSpacing: '0.1em' }}>Obertrack</span>
                </div>
                {blocks.length === 0 || (blocks.length === 1 && blocks[0].style?.raw === 'true') ? (
                  <div style={s.empty}>
                    <AlignJustify size={30} style={{ opacity: 0.3 }} />
                    <p>Haz clic en los bloques de la izquierda<br />para comenzar a armar tu correo.</p>
                  </div>
                ) : (
                  blocks.filter(b => b.style?.raw !== 'true').map(b => (
                    <div key={b.id} onClick={() => setSelectedId(b.id)}
                      style={{
                        position: 'relative',
                        margin: '12px 16px',
                        padding: '4px',
                        borderRadius: '8px',
                        background: '#ffffff',
                        border: selectedId === b.id ? '2px solid #7c3aed' : '1px dashed #e2e8f0',
                        cursor: 'pointer',
                        transition: 'all 0.15s',
                        boxShadow: selectedId === b.id ? '0 4px 12px rgba(124,58,237,0.1)' : '0 1px 3px rgba(0,0,0,0.02)'
                      }}
                      onMouseEnter={e => { if (selectedId !== b.id) (e.currentTarget as HTMLElement).style.borderColor = '#7c3aed'; }}
                      onMouseLeave={e => { if (selectedId !== b.id) (e.currentTarget as HTMLElement).style.borderColor = '#e2e8f0'; }}>
                      <BlockPreview block={b} />
                      {selectedId === b.id && (
                        <div style={{ position: 'absolute', top: -12, right: 12, display: 'flex', gap: 4, zIndex: 10 }}
                          onClick={e => e.stopPropagation()}>
                          <button onClick={() => moveBlock(b.id, -1)} style={{ width: 24, height: 24, borderRadius: 6, border: 'none', background: '#7c3aed', color: '#fff', cursor: 'pointer', display: 'flex', alignItems: 'center', justifyContent: 'center', boxShadow: '0 2px 5px rgba(0,0,0,0.15)' }} title="Mover arriba"><ChevronUp size={13} /></button>
                          <button onClick={() => moveBlock(b.id, 1)} style={{ width: 24, height: 24, borderRadius: 6, border: 'none', background: '#7c3aed', color: '#fff', cursor: 'pointer', display: 'flex', alignItems: 'center', justifyContent: 'center', boxShadow: '0 2px 5px rgba(0,0,0,0.15)' }} title="Mover abajo"><ChevronDown size={13} /></button>
                          <button onClick={() => removeBlock(b.id)} style={{ width: 24, height: 24, borderRadius: 6, border: 'none', background: '#ef4444', color: '#fff', cursor: 'pointer', display: 'flex', alignItems: 'center', justifyContent: 'center', boxShadow: '0 2px 5px rgba(0,0,0,0.15)' }} title="Eliminar"><Trash2 size={13} /></button>
                        </div>
                      )}
                    </div>
                  ))
                )}
                <div style={s.emailFooter}>© 2026 Obertrack · Este mensaje fue enviado automáticamente.</div>
              </div>
              </div>
            </div>

            {/* Inspector */}
            <div style={s.inspector}>
              <div style={s.inspectorLabel}>Propiedades</div>
              {selectedBlock && selectedBlock.style?.raw !== 'true' ? (
                <Inspector block={selectedBlock} onChange={updateBlock} />
              ) : (
                <div style={{ flex: 1, display: 'flex', alignItems: 'center', justifyContent: 'center', color: '#cbd5e1', fontSize: 11, textAlign: 'center', padding: 16 }}>
                  Seleccioná un bloque para editarlo
                </div>
              )}
            </div>
          </>
        )}

        {mode === 'code' && (
          <div style={{ flex: 1, display: 'flex', flexDirection: 'column', overflow: 'hidden', background: '#0f172a' }}>
            <div style={{ padding: '8px 16px', borderBottom: '1px solid #1e293b', display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
              <span style={{ fontSize: 10, fontWeight: 700, color: '#475569', textTransform: 'uppercase', letterSpacing: '0.08em', display: 'flex', alignItems: 'center', gap: 6 }}>
                <Code size={12} style={{ color: '#a78bfa' }} /> Editor de código HTML / Tailwind
              </span>
              <span style={{ fontSize: 10, color: '#334155' }}>Soporte completo para HTML, CSS inline y Tailwind</span>
            </div>
            <textarea
              value={rawHTML || (blocks.length === 1 && blocks[0].style?.raw === 'true' ? blocks[0].content : compileBlocksToHTML(blocks))}
              onChange={e => handleRawHTMLChange(e.target.value)}
              style={{ flex: 1, width: '100%', background: '#020617', color: '#e2e8f0', fontFamily: 'monospace', fontSize: 12, padding: 16, outline: 'none', border: 'none', resize: 'none', lineHeight: 1.6 }}
              placeholder="<!-- Escribe tu HTML aquí... -->"
            />
          </div>
        )}

        {mode === 'preview' && (
          <div style={{ flex: 1, overflow: 'hidden', background: '#e2e8f0', display: 'flex', justifyContent: 'center', alignItems: 'flex-start', padding: 24 }}>
            <div style={{
              background: '#fff', borderRadius: 12, overflow: 'hidden', border: '1px solid #cbd5e1', boxShadow: '0 4px 24px rgba(0,0,0,0.1)',
              width: previewDevice === 'desktop' ? '100%' : '375px',
              maxWidth: previewDevice === 'desktop' ? '650px' : '375px',
              height: '100%', display: 'flex', flexDirection: 'column', transition: 'all 0.3s'
            }}>
              <div style={{ padding: '8px 16px', background: '#f8fafc', borderBottom: '1px solid #e2e8f0', display: 'flex', justifyContent: 'space-between', alignItems: 'center', fontSize: 10, color: '#94a3b8' }}>
                <span>Dispositivo: {previewDevice === 'desktop' ? 'Escritorio (650px)' : 'Móvil (375px)'}</span>
                <span style={{ background: '#f0fdf4', color: '#16a34a', padding: '2px 8px', borderRadius: 20, fontWeight: 700, textTransform: 'uppercase', letterSpacing: '0.05em' }}>iframe aislado</span>
              </div>
              <iframe title="Email Preview" srcDoc={srcDoc} style={{ flex: 1, width: '100%', border: 'none' }} sandbox="allow-popups allow-same-origin allow-scripts" />
            </div>
          </div>
        )}
      </div>

      {/* Send/Schedule Modal */}
      {showSendModal && (
        <SendModal
          onClose={() => setShowSendModal(false)}
          onConfirm={handleSend}
          onSchedule={handleSchedule}
          initialRecipientIds={initialRecipientIds}
          initialScheduledAt={initialScheduledAt}
        />
      )}
    </div>
  );
};

export default EmailBuilder;
