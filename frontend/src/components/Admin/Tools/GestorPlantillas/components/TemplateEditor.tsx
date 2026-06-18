import React, { useState, useCallback } from 'react';
import { X, Save, ChevronLeft } from 'lucide-react';
import { EmailTemplate } from '../../../../../services/emailService';
import { blocksFromJSON, blocksToJSON, EmailBlock } from '../../Common/emailTypes';
import { uploadService } from '../../../../../services/upload.service';
import styles from './TemplateEditor.module.css';

type TemplateStep = 'meta' | 'builder';

interface TemplateEditorProps {
  initial?: EmailTemplate;
  onSave: (data: Partial<EmailTemplate>) => Promise<void>;
  onClose: () => void;
}

const TemplateEditor: React.FC<TemplateEditorProps> = ({ initial, onSave, onClose }) => {
  const [step, setStep] = useState<TemplateStep>('meta');
  const [title, setTitle] = useState(initial?.title ?? '');
  const [subject, setSubject] = useState(initial?.subject ?? '');
  const [blocks, setBlocks] = useState<EmailBlock[]>(() => {
    return blocksFromJSON(initial?.content ?? '[]');
  });
  const [saving, setSaving] = useState(false);

  const handleSave = async () => {
    if (!title || !subject) return;
    setSaving(true);
    try {
      await onSave({ title, subject, type: 'gestor', content: blocksToJSON(blocks), is_active: true });
    } finally {
      setSaving(false);
    }
  };

  const StepIcon = ({ active, done }: { active: boolean; done: boolean }) => (
    <span className={`${styles.stepIcon} ${active ? styles.stepIconActive : ''} ${done ? styles.stepIconDone : ''}`}>
      {done ? '✓' : active ? '●' : '○'}
    </span>
  );

  return (
    <div className={styles.modalBackdrop}>
      <div className={styles.modal}>
        {/* Header */}
        <div className={styles.header}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
            {step === 'builder' && (
              <button className={styles.iconBtn} onClick={() => setStep('meta')}>
                <ChevronLeft size={18} />
              </button>
            )}
            <h2 className={styles.title}>
              {initial ? 'Editar Plantilla' : 'Nueva Plantilla'}
            </h2>
          </div>
          <button className={styles.iconBtn} onClick={onClose}>
            <X size={18} />
          </button>
        </div>

        {/* Steps */}
        <div className={styles.steps}>
          <div className={`${styles.step} ${step === 'meta' ? styles.stepActive : ''}`}>
            <StepIcon active={step === 'meta'} done={step === 'builder'} />
            1. Datos
          </div>
          <div className={styles.stepLine} />
          <div className={`${styles.step} ${step === 'builder' ? styles.stepActive : ''}`}>
            <StepIcon active={step === 'builder'} done={false} />
            2. Diseño
          </div>
        </div>

        {/* Body */}
        <div className={styles.body}>
          {step === 'meta' ? (
            <div className={styles.form}>
              <div className={styles.formGroup}>
                <label className={styles.formLabel}>Nombre de la plantilla *</label>
                <input
                  className={styles.formInput}
                  value={title}
                  onChange={e => setTitle(e.target.value)}
                  placeholder="Ej: Newsletter Mensual"
                  autoFocus
                />
              </div>
              <div className={styles.formGroup}>
                <label className={styles.formLabel}>Asunto del email *</label>
                <input
                  className={styles.formInput}
                  value={subject}
                  onChange={e => setSubject(e.target.value)}
                  placeholder="Ej: ¡Novedades de este mes!"
                />
              </div>
            </div>
          ) : (
            <div className={styles.builderContainer}>
              <TemplateBlockEditor blocks={blocks} onChange={setBlocks} />
            </div>
          )}
        </div>

        {/* Footer */}
        <div className={styles.footer}>
          {step === 'meta' ? (
            <>
              <button className={styles.btnSecondary} onClick={onClose}>Cancelar</button>
              <button
                className={styles.btnPrimary}
                disabled={!title || !subject}
                onClick={() => setStep('builder')}
              >
                Siguiente → Diseñar
              </button>
            </>
          ) : (
            <>
              <button className={styles.btnSecondary} onClick={() => setStep('meta')}>
                ← Atrás
              </button>
              <button
                className={styles.btnPrimary}
                disabled={saving}
                onClick={handleSave}
              >
                <Save size={14} />
                {saving ? 'Guardando...' : initial ? 'Guardar cambios' : 'Guardar plantilla'}
              </button>
            </>
          )}
        </div>
      </div>
    </div>
  );
};

// ─── Inline Editor for Template Blocks ──────────────────────────────────────
import { Type, Code, Eye, Image, Minus, AlignJustify, Share2, MousePointerClick, Trash2, ChevronUp, ChevronDown, Laptop, Smartphone } from 'lucide-react';
import { compileBlocksToHTML, uid, defaultBlock, PALETTE } from '../../Common/emailTypes';

type EditorMode = 'blocks' | 'code' | 'preview';
type PreviewDevice = 'desktop' | 'mobile';

function TemplateBlockEditor({ blocks, onChange }: { blocks: EmailBlock[]; onChange: (b: EmailBlock[]) => void }) {
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [mode, setMode] = useState<EditorMode>('blocks');
  const [previewDevice, setPreviewDevice] = useState<PreviewDevice>('desktop');
  const [rawHTML, setRawHTML] = useState('');

  const selectedBlock = blocks.find(b => b.id === selectedId) ?? null;

  const addBlock = useCallback((type: any) => {
    const nb: EmailBlock = { id: uid(), ...defaultBlock(type) };
    onChange([...blocks, nb]);
    setSelectedId(nb.id);
  }, [blocks, onChange]);

  const updateBlock = useCallback((updated: EmailBlock) => {
    onChange(blocks.map(b => b.id === updated.id ? updated : b));
  }, [blocks, onChange]);

  const removeBlock = useCallback((id: string) => {
    onChange(blocks.filter(b => b.id !== id));
    if (selectedId === id) setSelectedId(null);
  }, [blocks, onChange, selectedId]);

  const moveBlock = useCallback((id: string, dir: -1 | 1) => {
    const idx = blocks.findIndex(b => b.id === id);
    if (idx < 0) return;
    const newIdx = idx + dir;
    if (newIdx < 0 || newIdx >= blocks.length) return;
    const copy = [...blocks];
    [copy[idx], copy[newIdx]] = [copy[newIdx], copy[idx]];
    onChange(copy);
  }, [blocks, onChange]);

  const handleSwitchToCode = () => {
    setRawHTML(compileBlocksToHTML(blocks));
    setMode('code');
  };

  const handleRawHTMLChange = (val: string) => {
    setRawHTML(val);
    onChange([{ id: 'raw_code', type: 'text', content: val, style: { raw: 'true' } }]);
  };

  const previewContent = React.useMemo(() => {
    if (blocks[0]?.style?.raw === 'true') return blocks[0].content;
    return compileBlocksToHTML(blocks);
  }, [blocks]);

  const srcDoc = React.useMemo(() => `<!DOCTYPE html>
<html><head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <script src="https://cdn.tailwindcss.com"><\/script>
  <style>
    *, *::before, *::after { box-sizing: border-box; }
    body { margin: 0; padding: 16px; background: #f1f5f9; min-height: 100vh;
           display: flex; justify-content: center; align-items: flex-start;
           font-family: sans-serif; }
    img { max-width: 100%; }
    a { color: #7c3aed; }
  </style>
</head><body>
  <div style="width:100%;display:flex;justify-content:center;">${previewContent.replace(/\/api\/uploads\//g, '/api/public/uploads/')}</div>
</body></html>`, [previewContent]);

  const s = {
    root: { display: 'flex', flexDirection: 'column' as const, flex: 1, minHeight: 0, background: '#f8fafc', borderRadius: 12, overflow: 'hidden', border: '1px solid #e2e8f0' },
    tabBar: { display: 'flex', alignItems: 'center', gap: 4, background: '#f1f5f9', padding: '4px', borderRadius: 10 },
    tab: (active: boolean) => ({ padding: '7px 14px', borderRadius: 7, border: 'none', cursor: 'pointer', fontSize: 12, fontWeight: 600, display: 'flex', alignItems: 'center', gap: 6, background: active ? '#fff' : 'transparent', color: active ? '#1e293b' : '#64748b', boxShadow: active ? '0 1px 3px rgba(0,0,0,0.08)' : 'none' }),
    body: { display: 'flex', flex: 1, overflow: 'hidden', minHeight: 0 },
    palette: { width: 180, background: '#fff', borderRight: '1px solid #e2e8f0', display: 'flex', flexDirection: 'column' as const, overflowY: 'auto' as const, flexShrink: 0 },
    canvas: { flex: 1, overflowY: 'auto' as const, background: '#f8fafc', minHeight: 0 },
    canvasInner: { display: 'flex', flexDirection: 'column' as const, alignItems: 'center', padding: '16px', minHeight: '100%' },
    emailWrap: { width: '100%', maxWidth: 600, background: '#fff', borderRadius: 12, overflow: 'hidden', boxShadow: '0 4px 20px rgba(0,0,0,0.05)', border: '1px solid #e2e8f0' },
    inspector: { width: 220, background: '#fff', borderLeft: '1px solid #e2e8f0', display: 'flex', flexDirection: 'column' as const, overflowY: 'auto' as const, flexShrink: 0 },
  };

  return (
    <div style={s.root}>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '10px 16px', background: '#fff', borderBottom: '1px solid #e2e8f0' }}>
        <span style={{ fontSize: 11, fontWeight: 700, color: '#7c3aed', textTransform: 'uppercase', letterSpacing: '0.06em' }}>Editor Visual</span>
        <div style={s.tabBar}>
          <button style={s.tab(mode === 'blocks')} onClick={() => setMode('blocks')}><Type size={12} /> Visual</button>
          <button style={s.tab(mode === 'code')} onClick={handleSwitchToCode}><Code size={12} /> HTML</button>
          <button style={s.tab(mode === 'preview')} onClick={() => setMode('preview')}><Eye size={12} /> Preview</button>
        </div>
        {mode === 'preview' && (
          <div style={{ display: 'flex', gap: 4, background: '#f1f5f9', padding: 4, borderRadius: 8 }}>
            <button onClick={() => setPreviewDevice('desktop')} style={{ width: 28, height: 28, borderRadius: 6, border: 'none', cursor: 'pointer', display: 'flex', alignItems: 'center', justifyContent: 'center', background: previewDevice === 'desktop' ? '#fff' : 'transparent', color: previewDevice === 'desktop' ? '#7c3aed' : '#94a3b8' }} title="Escritorio"><Laptop size={13} /></button>
            <button onClick={() => setPreviewDevice('mobile')} style={{ width: 28, height: 28, borderRadius: 6, border: 'none', cursor: 'pointer', display: 'flex', alignItems: 'center', justifyContent: 'center', background: previewDevice === 'mobile' ? '#fff' : 'transparent', color: previewDevice === 'mobile' ? '#7c3aed' : '#94a3b8' }} title="Móvil"><Smartphone size={13} /></button>
          </div>
        )}
      </div>

      <div style={s.body}>
        {mode === 'blocks' && (
          <>
            <div style={s.palette}>
              <div style={{ padding: '10px 14px', fontSize: 9, fontWeight: 700, color: '#94a3b8', textTransform: 'uppercase', letterSpacing: '0.08em', borderBottom: '1px solid #f1f5f9' }}>Bloques</div>
              {PALETTE.map(p => {
                const icons: Record<string, React.ReactNode> = { text: <Type size={12} />, button: <MousePointerClick size={12} />, image: <Image size={12} />, divider: <Minus size={12} />, spacer: <AlignJustify size={12} />, social: <Share2 size={12} /> };
                return (
                  <button key={p.type} onClick={() => addBlock(p.type)}
                    style={{ display: 'flex', alignItems: 'center', gap: 8, padding: '9px 14px', background: 'transparent', border: 'none', color: '#64748b', cursor: 'pointer', fontSize: 12, fontWeight: 500, textAlign: 'left', width: '100%' }}
                    onMouseEnter={e => { (e.currentTarget as HTMLElement).style.background = '#f8fafc'; (e.currentTarget as HTMLElement).style.color = '#1e293b'; }}
                    onMouseLeave={e => { (e.currentTarget as HTMLElement).style.background = 'transparent'; (e.currentTarget as HTMLElement).style.color = '#64748b'; }}
                  >
                    <span style={{ width: 24, height: 24, borderRadius: 6, background: '#f5f2fb', display: 'flex', alignItems: 'center', justifyContent: 'center', color: '#7c3aed' }}>{icons[p.type]}</span>
                    {p.label}
                  </button>
                );
              })}
            </div>

            <div style={s.canvas}>
              <div style={s.canvasInner}>
                <div style={s.emailWrap}>
                  <div style={{ background: '#f5f2fb', padding: '12px 20px', textAlign: 'center', borderBottom: '1px solid #e8e3f5' }}>
                    <span style={{ fontSize: 9, color: '#7c3aed', fontWeight: 700, textTransform: 'uppercase', letterSpacing: '0.1em' }}>Obertrack</span>
                  </div>
                  {blocks.length === 0 || (blocks.length === 1 && blocks[0].style?.raw === 'true') ? (
                    <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', padding: '40px 24px', color: '#cbd5e1', fontSize: 12, textAlign: 'center' }}>
                      <AlignJustify size={24} style={{ opacity: 0.3, marginBottom: 8 }} />
                      <p style={{ margin: 0 }}>Agregá bloques para diseñar tu plantilla</p>
                    </div>
                  ) : (
                    blocks.filter(b => b.style?.raw !== 'true').map(b => (
                      <div key={b.id} onClick={() => setSelectedId(b.id)}
                        style={{ position: 'relative', margin: '8px 12px', padding: '4px', borderRadius: 8, border: selectedId === b.id ? '2px solid #7c3aed' : '1px dashed #e2e8f0', cursor: 'pointer', transition: 'all 0.15s' }}>
                        <BlockPreviewContent block={b} />
                        {selectedId === b.id && (
                          <div style={{ position: 'absolute', top: -12, right: 12, display: 'flex', gap: 4, zIndex: 10 }} onClick={e => e.stopPropagation()}>
                            <button onClick={() => moveBlock(b.id, -1)} style={{ width: 22, height: 22, borderRadius: 6, border: 'none', background: '#7c3aed', color: '#fff', cursor: 'pointer', display: 'flex', alignItems: 'center', justifyContent: 'center' }}><ChevronUp size={11} /></button>
                            <button onClick={() => moveBlock(b.id, 1)} style={{ width: 22, height: 22, borderRadius: 6, border: 'none', background: '#7c3aed', color: '#fff', cursor: 'pointer', display: 'flex', alignItems: 'center', justifyContent: 'center' }}><ChevronDown size={11} /></button>
                            <button onClick={() => removeBlock(b.id)} style={{ width: 22, height: 22, borderRadius: 6, border: 'none', background: '#ef4444', color: '#fff', cursor: 'pointer', display: 'flex', alignItems: 'center', justifyContent: 'center' }}><Trash2 size={11} /></button>
                          </div>
                        )}
                      </div>
                    ))
                  )}
                  <div style={{ background: '#f8fafc', padding: '12px 20px', textAlign: 'center', fontSize: 10, color: '#94a3b8', borderTop: '1px solid #e2e8f0' }}>© 2026 Obertrack</div>
                </div>
              </div>
            </div>

            <div style={s.inspector}>
              <div style={{ padding: '10px 14px', fontSize: 9, fontWeight: 700, color: '#94a3b8', textTransform: 'uppercase', letterSpacing: '0.08em', borderBottom: '1px solid #f1f5f9', background: '#fafafa' }}>Propiedades</div>
              {selectedBlock && selectedBlock.style?.raw !== 'true' ? (
                <InspectorPanel block={selectedBlock} onChange={updateBlock} />
              ) : (
                <div style={{ flex: 1, display: 'flex', alignItems: 'center', justifyContent: 'center', color: '#cbd5e1', fontSize: 11, textAlign: 'center', padding: 16 }}>Seleccioná un bloque</div>
              )}
            </div>
          </>
        )}

        {mode === 'code' && (
          <div style={{ flex: 1, display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
            <textarea
              value={rawHTML || (blocks.length === 1 && blocks[0].style?.raw === 'true' ? blocks[0].content : compileBlocksToHTML(blocks))}
              onChange={e => handleRawHTMLChange(e.target.value)}
              style={{ flex: 1, width: '100%', background: '#0f172a', color: '#e2e8f0', fontFamily: 'monospace', fontSize: 12, padding: 16, outline: 'none', border: 'none', resize: 'none', lineHeight: 1.6 }}
              placeholder="<!-- Escribe tu HTML aquí... -->"
            />
          </div>
        )}

        {mode === 'preview' && (
          <div style={{ flex: 1, overflow: 'hidden', background: '#e2e8f0', display: 'flex', justifyContent: 'center', padding: 16 }}>
            <div style={{
              background: '#fff', borderRadius: 12, overflow: 'hidden', border: '1px solid #cbd5e1',
              width: previewDevice === 'desktop' ? '100%' : '375px',
              maxWidth: previewDevice === 'desktop' ? '650px' : '375px',
              display: 'flex', flexDirection: 'column'
            }}>
              <iframe title="Preview" srcDoc={srcDoc} style={{ width: '100%', height: 500, border: 'none' }} sandbox="allow-scripts allow-popups" />
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

// ─── Block preview ──────────────────────────────────────────────────────────
function BlockPreviewContent({ block }: { block: EmailBlock }) {
  switch (block.type) {
    case 'text':
      return <div style={{ padding: '10px 20px', fontSize: block.style.fontSize || '16px', color: block.style.color || '#1e293b', lineHeight: 1.6, whiteSpace: 'pre-wrap' }}>{block.content}</div>;
    case 'button': {
      const align = block.style.align || 'center';
      return (
        <div style={{ padding: '10px 20px', textAlign: align as React.CSSProperties['textAlign'] }}>
          <span style={{ display: 'inline-block', padding: '10px 24px', background: block.style.backgroundColor || '#7c3aed', color: block.style.color || '#fff', borderRadius: block.style.borderRadius || '8px', fontWeight: 600, fontSize: 14, cursor: 'pointer' }}>{block.content}</span>
        </div>
      );
    }
    case 'image':
      return (
        <div style={{ padding: '10px 20px' }}>
          {block.content
            ? <img src={block.content} alt="" style={{ width: block.style.width || '100%', maxWidth: '100%', display: 'block', borderRadius: 4 }} />
            : <div style={{ background: '#f1f5f9', border: '2px dashed #cbd5e1', borderRadius: 8, height: 80, display: 'flex', alignItems: 'center', justifyContent: 'center', color: '#94a3b8', fontSize: 12, gap: 6 }}>
                <Image size={16} /> Pega URL de imagen
              </div>
          }
        </div>
      );
    case 'divider':
      return <div style={{ padding: '6px 20px' }}><hr style={{ border: 'none', borderTop: `${block.style.borderHeight || '1px'} ${block.style.borderStyle || 'solid'} ${block.style.borderColor || '#e2e8f0'}`, margin: 0 }} /></div>;
    case 'spacer':
      return <div style={{ height: block.style.height || '24px' }} />;
    case 'social':
      return (
        <div style={{ padding: '10px 20px', textAlign: 'center', display: 'flex', justifyContent: 'center', gap: 10 }}>
          {['Facebook', 'Instagram', 'LinkedIn'].map(n => <span key={n} style={{ fontSize: 12, color: '#7c3aed', fontWeight: 600 }}>{n}</span>)}
        </div>
      );
    default:
      return null;
  }
}

// ─── Inspector ──────────────────────────────────────────────────────────────
function InspectorPanel({ block, onChange }: { block: EmailBlock; onChange: (b: EmailBlock) => void }) {
  const set = (key: string, val: string) =>
    onChange({ ...block, content: key === 'content' ? val : block.content, style: key === 'content' ? block.style : { ...block.style, [key]: val } });

  const fieldStyle: React.CSSProperties = { display: 'flex', flexDirection: 'column', gap: 4, padding: '8px 12px', borderBottom: '1px solid #f1f5f9' };
  const labelStyle: React.CSSProperties = { fontSize: 9, fontWeight: 700, color: '#94a3b8', textTransform: 'uppercase', letterSpacing: '0.06em' };
  const inputStyle: React.CSSProperties = { border: '1px solid #e2e8f0', borderRadius: 6, padding: '5px 8px', fontSize: 11, outline: 'none', color: '#1e293b', background: '#f8fafc', width: '100%' };

  return (
    <div>
      {block.type === 'text' && (
        <>
          <div style={fieldStyle}><label style={labelStyle}>Contenido</label><textarea style={{ ...inputStyle, resize: 'vertical', minHeight: 64, fontFamily: 'inherit' }} value={block.content} onChange={e => set('content', e.target.value)} rows={3} /></div>
          <div style={fieldStyle}>
            <label style={labelStyle}>Tamaño</label>
            <select style={inputStyle} value={block.style.fontSize || '16px'} onChange={e => set('fontSize', e.target.value)}>
              {['12px','14px','16px','18px','20px','24px','28px','32px'].map(s => <option key={s}>{s}</option>)}
            </select>
          </div>
          <div style={fieldStyle}>
            <label style={labelStyle}>Color</label>
            <div style={{ display: 'flex', gap: 6, alignItems: 'center' }}>
              <input type="color" style={{ width: 26, height: 26, borderRadius: 4, border: '1px solid #e2e8f0', padding: 1, cursor: 'pointer' }} value={block.style.color || '#1e293b'} onChange={e => set('color', e.target.value)} />
              <input style={{ ...inputStyle, flex: 1 }} value={block.style.color || '#1e293b'} onChange={e => set('color', e.target.value)} />
            </div>
          </div>
        </>
      )}
      {block.type === 'button' && (
        <>
          <div style={fieldStyle}><label style={labelStyle}>Texto</label><input style={inputStyle} value={block.content} onChange={e => set('content', e.target.value)} /></div>
          <div style={fieldStyle}><label style={labelStyle}>URL</label><input style={inputStyle} placeholder="https://..." value={block.style.linkUrl || ''} onChange={e => set('linkUrl', e.target.value)} /></div>
          <div style={fieldStyle}>
            <label style={labelStyle}>Fondo</label>
            <div style={{ display: 'flex', gap: 6, alignItems: 'center' }}>
              <input type="color" style={{ width: 26, height: 26, borderRadius: 4, border: '1px solid #e2e8f0', padding: 1, cursor: 'pointer' }} value={block.style.backgroundColor || '#7c3aed'} onChange={e => set('backgroundColor', e.target.value)} />
              <input style={{ ...inputStyle, flex: 1 }} value={block.style.backgroundColor || '#7c3aed'} onChange={e => set('backgroundColor', e.target.value)} />
            </div>
          </div>
          <div style={fieldStyle}>
            <label style={labelStyle}>Texto color</label>
            <div style={{ display: 'flex', gap: 6, alignItems: 'center' }}>
              <input type="color" style={{ width: 26, height: 26, borderRadius: 4, border: '1px solid #e2e8f0', padding: 1, cursor: 'pointer' }} value={block.style.color || '#ffffff'} onChange={e => set('color', e.target.value)} />
              <input style={{ ...inputStyle, flex: 1 }} value={block.style.color || '#ffffff'} onChange={e => set('color', e.target.value)} />
            </div>
          </div>
          <div style={fieldStyle}>
            <label style={labelStyle}>Alineación</label>
            <select style={inputStyle} value={block.style.align || 'center'} onChange={e => set('align', e.target.value)}>
              <option value="left">Izquierda</option><option value="center">Centro</option><option value="right">Derecha</option>
            </select>
          </div>
        </>
      )}
      {block.type === 'image' && (
        <>
          <div style={fieldStyle}>
            <label style={labelStyle}>URL de imagen</label>
            <div style={{ display: 'flex', gap: 6 }}>
              <input style={{ ...inputStyle, flex: 1 }} placeholder="https://..." value={block.content} onChange={e => set('content', e.target.value)} />
              <label style={{ display: 'inline-flex', alignItems: 'center', gap: 4, padding: '6px 12px', background: '#7c3aed', color: '#fff', borderRadius: 6, cursor: 'pointer', fontSize: 12, fontWeight: 600, whiteSpace: 'nowrap' }}>
                <input type="file" accept="image/*" style={{ display: 'none' }} onChange={async (e) => { const f = e.target.files?.[0]; if (!f) return; try { const r = await uploadService.upload(f); set('content', r.url) } catch { alert('Error al subir la imagen') } }} />
                Subir
              </label>
            </div>
          </div>
          <div style={fieldStyle}><label style={labelStyle}>Ancho</label><input style={inputStyle} placeholder="100%" value={block.style.width || '100%'} onChange={e => set('width', e.target.value)} /></div>
        </>
      )}
      {block.type === 'divider' && (
        <>
          <div style={fieldStyle}>
            <label style={labelStyle}>Color</label>
            <div style={{ display: 'flex', gap: 6, alignItems: 'center' }}>
              <input type="color" style={{ width: 26, height: 26, borderRadius: 4, border: '1px solid #e2e8f0', padding: 1, cursor: 'pointer' }} value={block.style.borderColor || '#e2e8f0'} onChange={e => set('borderColor', e.target.value)} />
              <input style={{ ...inputStyle, flex: 1 }} value={block.style.borderColor || '#e2e8f0'} onChange={e => set('borderColor', e.target.value)} />
            </div>
          </div>
          <div style={fieldStyle}>
            <label style={labelStyle}>Grosor</label>
            <select style={inputStyle} value={block.style.borderHeight || '1px'} onChange={e => set('borderHeight', e.target.value)}>
              {['1px','2px','3px','4px'].map(s => <option key={s}>{s}</option>)}
            </select>
          </div>
          <div style={fieldStyle}>
            <label style={labelStyle}>Estilo</label>
            <select style={inputStyle} value={block.style.borderStyle || 'solid'} onChange={e => set('borderStyle', e.target.value)}>
              <option value="solid">Sólido</option><option value="dashed">Guiones</option><option value="dotted">Puntos</option>
            </select>
          </div>
        </>
      )}
      {block.type === 'spacer' && (
        <div style={fieldStyle}>
          <label style={labelStyle}>Altura</label>
          <select style={inputStyle} value={block.style.height || '24px'} onChange={e => set('height', e.target.value)}>
            {['8px','16px','24px','32px','48px','64px'].map(s => <option key={s}>{s}</option>)}
          </select>
        </div>
      )}
      {block.type === 'social' && (
        <div style={fieldStyle}><p style={{ fontSize: 10, color: '#64748b', margin: 0 }}>Redes sociales se muestran automáticamente.</p></div>
      )}
    </div>
  );
}

export default TemplateEditor;
