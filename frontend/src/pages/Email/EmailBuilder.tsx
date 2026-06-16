import { useState, useCallback, useMemo } from 'react'
import {
  Type, Image, Minus, AlignJustify, Share2,
  Trash2, ChevronUp, ChevronDown, MousePointerClick, Code, Eye, Laptop, Smartphone
} from 'lucide-react'
import styles from './EmailBuilder.module.css'

// ─── Block types & defaults ─────────────────────────────────────────────────
export type BlockType = 'text' | 'button' | 'image' | 'divider' | 'spacer' | 'social'

export interface EmailBlock {
  id: string
  type: BlockType
  content: string
  style: Record<string, string>
}

const PALETTE: { type: BlockType; label: string; icon: React.ReactNode }[] = [
  { type: 'text',    label: 'Texto',     icon: <Type size={14} /> },
  { type: 'button',  label: 'Botón',     icon: <MousePointerClick size={14} /> },
  { type: 'image',   label: 'Imagen',    icon: <Image size={14} /> },
  { type: 'divider', label: 'Divisor',   icon: <Minus size={14} /> },
  { type: 'spacer',  label: 'Espaciado', icon: <AlignJustify size={14} /> },
  { type: 'social',  label: 'Social',    icon: <Share2 size={14} /> },
]

const defaultBlock = (type: BlockType): Omit<EmailBlock, 'id'> => {
  switch (type) {
    case 'text':    return { type, content: 'Escribe tu texto aquí...', style: { fontSize: '16px', color: '#1e293b' } }
    case 'button':  return { type, content: 'Haz clic aquí', style: { backgroundColor: '#7c3aed', color: '#ffffff', borderRadius: '8px', align: 'center', linkUrl: '#' } }
    case 'image':   return { type, content: '', style: { width: '100%' } }
    case 'divider': return { type, content: '', style: { borderColor: '#e2e8f0', borderHeight: '1px', borderStyle: 'solid' } }
    case 'spacer':  return { type, content: '', style: { height: '24px' } }
    case 'social':  return { type, content: '{}', style: {} }
    default:        return { type, content: '', style: {} }
  }
}

const uid = () => Math.random().toString(36).slice(2, 9)

// Helper to compile blocks into final HTML structure
export function compileBlocksToHTML(blocks: EmailBlock[]): string {
  const rows = blocks.map(block => {
    switch (block.type) {
      case 'text':
        return `<div style="padding: 12px 24px; font-size: ${block.style.fontSize || '16px'}; color: ${block.style.color || '#1e293b'}; line-height: 1.6; font-family: sans-serif; white-space: pre-wrap;">${block.content}</div>`
      case 'button': {
        const align = block.style.align || 'center'
        return `<div style="padding: 12px 24px; text-align: ${align};">
          <a href="${block.style.linkUrl || '#'}" target="_blank" style="display: inline-block; padding: 12px 28px; background-color: ${block.style.backgroundColor || '#7c3aed'}; color: ${block.style.color || '#ffffff'}; border-radius: ${block.style.borderRadius || '8px'}; font-weight: bold; text-decoration: none; font-size: 15px; font-family: sans-serif;">
            ${block.content}
          </a>
        </div>`
      }
      case 'image':
        if (!block.content) return ''
        return `<div style="padding: 12px 24px; text-align: center;">
          <img src="${block.content}" alt="" style="width: ${block.style.width || '100%'}; max-width: 100%; border-radius: 4px; display: inline-block;" />
        </div>`
      case 'divider':
        return `<div style="padding: 8px 24px;">
          <hr style="border: none; border-top: ${block.style.borderHeight || '1px'} ${block.style.borderStyle || 'solid'} ${block.style.borderColor || '#e2e8f0'}; margin: 0;" />
        </div>`
      case 'spacer':
        return `<div style="height: ${block.style.height || '24px'};"></div>`
      case 'social':
        return `<div style="padding: 12px 24px; text-align: center; font-family: sans-serif;">
          <a href="#" style="margin: 0 8px; color: #7c3aed; font-weight: 600; text-decoration: none; font-size: 13px;">Facebook</a>
          <a href="#" style="margin: 0 8px; color: #7c3aed; font-weight: 600; text-decoration: none; font-size: 13px;">Instagram</a>
          <a href="#" style="margin: 0 8px; color: #7c3aed; font-weight: 600; text-decoration: none; font-size: 13px;">LinkedIn</a>
        </div>`
      default:
        return ''
    }
  }).join('\n')

  return `
<div style="background-color: #ffffff; width: 100%; max-width: 600px; margin: 0 auto; overflow: hidden; border-radius: 8px; box-shadow: 0 4px 12px rgba(0,0,0,0.05); border: 1px solid #e2e8f0;">
  <div style="background-color: #f5f2fb; padding: 20px; text-align: center; border-bottom: 1px solid #e2e8f0;">
    <span style="font-size: 12px; color: #7c3aed; font-weight: bold; letter-spacing: 0.1em; text-transform: uppercase; font-family: sans-serif;">Obertrack</span>
  </div>
  ${rows}
  <div style="background-color: #f8fafc; padding: 16px; text-align: center; font-size: 11px; color: #94a3b8; border-top: 1px solid #e2e8f0; font-family: sans-serif;">
    © 2026 Obertrack · Este mensaje fue enviado automáticamente.
  </div>
</div>`
}

// ─── Block preview renderers (visual mode inside editor) ────────────────────
function BlockPreview({ block }: { block: EmailBlock }) {
  switch (block.type) {
    case 'text':
      return (
        <div style={{ padding: '12px 24px', fontSize: block.style.fontSize || '16px', color: block.style.color || '#1e293b', lineHeight: 1.6, whiteSpace: 'pre-wrap' }}>
          {block.content}
        </div>
      )
    case 'button': {
      const align = block.style.align || 'center'
      return (
        <div style={{ padding: '12px 24px', textAlign: align as React.CSSProperties['textAlign'] }}>
          <span style={{ display: 'inline-block', padding: '12px 28px', background: block.style.backgroundColor || '#7c3aed', color: block.style.color || '#fff', borderRadius: block.style.borderRadius || '8px', fontWeight: 600, fontSize: '15px', cursor: 'pointer' }}>
            {block.content}
          </span>
        </div>
      )
    }
    case 'image':
      return (
        <div style={{ padding: '12px 24px' }}>
          {block.content
            ? <img src={block.content} alt="" style={{ width: block.style.width || '100%', maxWidth: '100%', display: 'block', borderRadius: '4px' }} />
            : <div style={{ background: '#f1f5f9', border: '2px dashed #cbd5e1', borderRadius: '8px', height: '100px', display: 'flex', alignItems: 'center', justifyContent: 'center', color: '#94a3b8', fontSize: '13px' }}>
                <Image size={20} style={{ marginRight: 8 }} /> Pega URL de imagen en el inspector
              </div>
          }
        </div>
      )
    case 'divider':
      return (
        <div style={{ padding: '8px 24px' }}>
          <hr style={{ border: 'none', borderTop: `${block.style.borderHeight || '1px'} ${block.style.borderStyle || 'solid'} ${block.style.borderColor || '#e2e8f0'}`, margin: 0 }} />
        </div>
      )
    case 'spacer':
      return <div style={{ height: block.style.height || '24px', background: '#f8fafc', borderTop: '1px dashed #e2e8f0', borderBottom: '1px dashed #e2e8f0', margin: '0 24px' }} />
    case 'social':
      return (
        <div style={{ padding: '12px 24px', textAlign: 'center', display: 'flex', justifyContent: 'center', gap: 12 }}>
          {['Facebook', 'Instagram', 'LinkedIn'].map(n => (
            <span key={n} style={{ fontSize: '13px', color: '#7c3aed', fontWeight: 600 }}>{n}</span>
          ))}
        </div>
      )
    default:
      return null
  }
}

// ─── Inspector ───────────────────────────────────────────────────────────────
function Inspector({ block, onChange }: { block: EmailBlock; onChange: (b: EmailBlock) => void }) {
  const set = (key: string, val: string) => onChange({ ...block, content: key === 'content' ? val : block.content, style: key === 'content' ? block.style : { ...block.style, [key]: val } })

  return (
    <div>
      {block.type === 'text' && (
        <>
          <div className={styles.propGroup}>
            <label className={styles.propLabel}>Contenido</label>
            <textarea className={styles.propTextarea} value={block.content} onChange={e => set('content', e.target.value)} rows={4} />
          </div>
          <div className={styles.propGroup}>
            <label className={styles.propLabel}>Tamaño de fuente</label>
            <select className={styles.propSelect} value={block.style.fontSize || '16px'} onChange={e => set('fontSize', e.target.value)}>
              {['12px','14px','16px','18px','20px','24px','28px','32px'].map(s => <option key={s}>{s}</option>)}
            </select>
          </div>
          <div className={styles.propGroup}>
            <label className={styles.propLabel}>Color de texto</label>
            <div className={styles.propColorRow}>
              <input type="color" className={styles.propColorSwatch} value={block.style.color || '#1e293b'} onChange={e => set('color', e.target.value)} />
              <input className={styles.propInput} value={block.style.color || '#1e293b'} onChange={e => set('color', e.target.value)} />
            </div>
          </div>
        </>
      )}

      {block.type === 'button' && (
        <>
          <div className={styles.propGroup}>
            <label className={styles.propLabel}>Texto del botón</label>
            <input className={styles.propInput} value={block.content} onChange={e => set('content', e.target.value)} />
          </div>
          <div className={styles.propGroup}>
            <label className={styles.propLabel}>URL de destino</label>
            <input className={styles.propInput} placeholder="https://..." value={block.style.linkUrl || ''} onChange={e => set('linkUrl', e.target.value)} />
          </div>
          <div className={styles.propGroup}>
            <label className={styles.propLabel}>Color de fondo</label>
            <div className={styles.propColorRow}>
              <input type="color" className={styles.propColorSwatch} value={block.style.backgroundColor || '#7c3aed'} onChange={e => set('backgroundColor', e.target.value)} />
              <input className={styles.propInput} value={block.style.backgroundColor || '#7c3aed'} onChange={e => set('backgroundColor', e.target.value)} />
            </div>
          </div>
          <div className={styles.propGroup}>
            <label className={styles.propLabel}>Color de texto</label>
            <div className={styles.propColorRow}>
              <input type="color" className={styles.propColorSwatch} value={block.style.color || '#ffffff'} onChange={e => set('color', e.target.value)} />
              <input className={styles.propInput} value={block.style.color || '#ffffff'} onChange={e => set('color', e.target.value)} />
            </div>
          </div>
          <div className={styles.propGroup}>
            <label className={styles.propLabel}>Alineación</label>
            <select className={styles.propSelect} value={block.style.align || 'center'} onChange={e => set('align', e.target.value)}>
              <option value="left">Izquierda</option>
              <option value="center">Centro</option>
              <option value="right">Derecha</option>
            </select>
          </div>
          <div className={styles.propGroup}>
            <label className={styles.propLabel}>Border radius</label>
            <input className={styles.propInput} value={block.style.borderRadius || '8px'} onChange={e => set('borderRadius', e.target.value)} />
          </div>
        </>
      )}

      {block.type === 'image' && (
        <>
          <div className={styles.propGroup}>
            <label className={styles.propLabel}>URL de imagen</label>
            <input className={styles.propInput} placeholder="https://..." value={block.content} onChange={e => set('content', e.target.value)} />
          </div>
          <div className={styles.propGroup}>
            <label className={styles.propLabel}>Ancho</label>
            <input className={styles.propInput} placeholder="100%" value={block.style.width || '100%'} onChange={e => set('width', e.target.value)} />
          </div>
        </>
      )}

      {block.type === 'divider' && (
        <>
          <div className={styles.propGroup}>
            <label className={styles.propLabel}>Color</label>
            <div className={styles.propColorRow}>
              <input type="color" className={styles.propColorSwatch} value={block.style.borderColor || '#e2e8f0'} onChange={e => set('borderColor', e.target.value)} />
              <input className={styles.propInput} value={block.style.borderColor || '#e2e8f0'} onChange={e => set('borderColor', e.target.value)} />
            </div>
          </div>
          <div className={styles.propGroup}>
            <label className={styles.propLabel}>Grosor</label>
            <select className={styles.propSelect} value={block.style.borderHeight || '1px'} onChange={e => set('borderHeight', e.target.value)}>
              {['1px','2px','3px','4px'].map(s => <option key={s}>{s}</option>)}
            </select>
          </div>
          <div className={styles.propGroup}>
            <label className={styles.propLabel}>Estilo</label>
            <select className={styles.propSelect} value={block.style.borderStyle || 'solid'} onChange={e => set('borderStyle', e.target.value)}>
              <option value="solid">Sólido</option>
              <option value="dashed">Guiones</option>
              <option value="dotted">Puntos</option>
            </select>
          </div>
        </>
      )}

      {block.type === 'spacer' && (
        <div className={styles.propGroup}>
          <label className={styles.propLabel}>Altura</label>
          <select className={styles.propSelect} value={block.style.height || '24px'} onChange={e => set('height', e.target.value)}>
            {['8px','16px','24px','32px','48px','64px'].map(s => <option key={s}>{s}</option>)}
          </select>
        </div>
      )}

      {block.type === 'social' && (
        <div className={styles.propGroup}>
          <p style={{ fontSize: 12, color: '#64748b', margin: 0 }}>El bloque social muestra links de redes automáticamente al renderizar el email.</p>
        </div>
      )}
    </div>
  )
}

// ─── Main EmailBuilder component ─────────────────────────────────────────────
interface Props {
  blocks: EmailBlock[]
  onChange: (blocks: EmailBlock[]) => void
}

type EditorMode = 'blocks' | 'code' | 'preview'
type PreviewDevice = 'desktop' | 'mobile'

export default function EmailBuilder({ blocks, onChange }: Props) {
  const [selectedId, setSelectedId] = useState<string | null>(null)
  const [mode, setMode] = useState<EditorMode>('blocks')
  const [previewDevice, setPreviewDevice] = useState<PreviewDevice>('desktop')
  const [rawHTML, setRawHTML] = useState<string>('')

  const selectedBlock = blocks.find(b => b.id === selectedId) ?? null

  const addBlock = useCallback((type: BlockType) => {
    const newBlock: EmailBlock = { id: uid(), ...defaultBlock(type) }
    onChange([...blocks, newBlock])
    setSelectedId(newBlock.id)
  }, [blocks, onChange])

  const updateBlock = useCallback((updated: EmailBlock) => {
    onChange(blocks.map(b => b.id === updated.id ? updated : b))
  }, [blocks, onChange])

  const removeBlock = useCallback((id: string) => {
    onChange(blocks.filter(b => b.id !== id))
    if (selectedId === id) setSelectedId(null)
  }, [blocks, onChange, selectedId])

  const moveBlock = useCallback((id: string, dir: -1 | 1) => {
    const idx = blocks.findIndex(b => b.id === id)
    if (idx < 0) return
    const newIdx = idx + dir
    if (newIdx < 0 || newIdx >= blocks.length) return
    const copy = [...blocks]
    ;[copy[idx], copy[newIdx]] = [copy[newIdx], copy[idx]]
    onChange(copy)
  }, [blocks, onChange])

  // Handles switching into code mode
  const handleSwitchToCode = () => {
    const compiled = compileBlocksToHTML(blocks)
    setRawHTML(compiled)
    setMode('code')
  }

  // Handles raw HTML changes
  const handleRawHTMLChange = (val: string) => {
    setRawHTML(val)
    onChange([{ id: 'raw_code', type: 'text', content: val, style: { raw: 'true' } }])
  }

  // Determine current active content to show in preview iframe
  const previewContent = useMemo(() => {
    if (blocks[0]?.style?.raw === 'true') {
      return blocks[0].content
    }
    return compileBlocksToHTML(blocks)
  }, [blocks])

  // No CDN script — inline CSS only, avoids CSP script-src violation.
  const srcDoc = useMemo(() => {
    return `
<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <style>
    *, *::before, *::after { box-sizing: border-box; }
    body {
      margin: 0;
      padding: 16px;
      background-color: #f1f5f9;
      min-height: 100vh;
      display: flex;
      justify-content: center;
      align-items: flex-start;
      font-family: sans-serif;
    }
    img { max-width: 100%; }
    a { color: #7c3aed; }
  </style>
</head>
<body>
  <div class="w-full flex justify-center">
    ${previewContent}
  </div>
</body>
</html>`
  }, [previewContent])

  return (
    <div className="flex flex-col h-[550px] border border-slate-200 rounded-xl overflow-hidden bg-slate-50">
      {/* Editor Tabs / Toolbar */}
      <div className="flex items-center justify-between px-4 py-2 bg-white border-b border-slate-200">
        <div className="flex items-center gap-1.5 bg-slate-100 p-1 rounded-lg">
          <button
            onClick={() => setMode('blocks')}
            className={`px-3 py-1.5 rounded-md text-xs font-semibold flex items-center gap-1.5 transition-all ${
              mode === 'blocks' ? 'bg-white text-slate-800 shadow-sm' : 'text-slate-500 hover:text-slate-700'
            }`}
          >
            <Type size={13} /> Visual
          </button>
          <button
            onClick={handleSwitchToCode}
            className={`px-3 py-1.5 rounded-md text-xs font-semibold flex items-center gap-1.5 transition-all ${
              mode === 'code' ? 'bg-white text-slate-800 shadow-sm' : 'text-slate-500 hover:text-slate-700'
            }`}
          >
            <Code size={13} /> Código HTML
          </button>
          <button
            onClick={() => setMode('preview')}
            className={`px-3 py-1.5 rounded-md text-xs font-semibold flex items-center gap-1.5 transition-all ${
              mode === 'preview' ? 'bg-white text-slate-800 shadow-sm' : 'text-slate-500 hover:text-slate-700'
            }`}
          >
            <Eye size={13} /> Previsualización
          </button>
        </div>

        {mode === 'preview' && (
          <div className="flex items-center gap-1 bg-slate-100 p-1 rounded-lg">
            <button
              onClick={() => setPreviewDevice('desktop')}
              className={`p-1.5 rounded-md transition-all ${
                previewDevice === 'desktop' ? 'bg-white text-purple-600 shadow-sm' : 'text-slate-500 hover:text-slate-700'
              }`}
              title="Escritorio"
            >
              <Laptop size={14} />
            </button>
            <button
              onClick={() => setPreviewDevice('mobile')}
              className={`p-1.5 rounded-md transition-all ${
                previewDevice === 'mobile' ? 'bg-white text-purple-600 shadow-sm' : 'text-slate-500 hover:text-slate-700'
              }`}
              title="Móvil"
            >
              <Smartphone size={14} />
            </button>
          </div>
        )}
      </div>

      {/* Editor Body */}
      <div className="flex flex-1 overflow-hidden">
        {mode === 'blocks' && (
          <div className="flex flex-1 overflow-hidden">
            {/* Palette */}
            <div className="w-[180px] bg-slate-900 border-r border-slate-800 flex flex-col overflow-y-auto">
              <div className="px-4 py-3 text-[10px] font-bold tracking-wider text-slate-500 uppercase border-b border-slate-800">
                Bloques
              </div>
              {PALETTE.map(p => (
                <button
                  key={p.type}
                  className="flex items-center gap-2.5 px-4 py-3 bg-transparent border-none text-slate-300 hover:bg-slate-800 hover:text-white cursor-pointer text-xs font-medium text-left transition-all"
                  onClick={() => addBlock(p.type)}
                >
                  <span className="w-7 h-7 rounded bg-slate-800 flex items-center justify-center text-slate-400">
                    {p.icon}
                  </span>
                  {p.label}
                </button>
              ))}
            </div>

            {/* Canvas */}
            <div className="flex-1 overflow-y-auto bg-slate-100 flex flex-col items-center py-6 px-4">
              <div className="w-full max-w-[500px] bg-white rounded-lg shadow-sm border border-slate-200 overflow-hidden">
                <div className="bg-purple-50/50 py-3 text-center border-b border-slate-100">
                  <span className="text-[10px] text-purple-600 font-bold uppercase tracking-widest">Pre-Header Obertrack</span>
                </div>
                {blocks.length === 0 || (blocks.length === 1 && blocks[0].style?.raw === 'true') ? (
                  <div className="py-16 text-center text-slate-400">
                    <AlignJustify className="mx-auto mb-2.5 opacity-30" size={32} />
                    <p className="text-xs">Haz clic en los bloques de la izquierda<br />para comenzar a armar tu correo.</p>
                  </div>
                ) : (
                  blocks.filter(b => b.style?.raw !== 'true').map(b => (
                    <div
                      key={b.id}
                      className={`relative group border-2 border-transparent transition-all ${
                        selectedId === b.id ? 'border-purple-600' : 'hover:border-purple-600/30'
                      }`}
                      onClick={() => setSelectedId(b.id)}
                    >
                      <BlockPreview block={b} />
                      <div className="absolute top-1 right-1 hidden group-hover:flex items-center gap-1 z-20">
                        <button
                          className="w-5 h-5 rounded bg-purple-600 text-white flex items-center justify-center hover:bg-purple-700 text-[10px]"
                          title="Subir"
                          onClick={e => { e.stopPropagation(); moveBlock(b.id, -1) }}
                        >
                          <ChevronUp size={10} />
                        </button>
                        <button
                          className="w-5 h-5 rounded bg-purple-600 text-white flex items-center justify-center hover:bg-purple-700 text-[10px]"
                          title="Bajar"
                          onClick={e => { e.stopPropagation(); moveBlock(b.id, 1) }}
                        >
                          <ChevronDown size={10} />
                        </button>
                        <button
                          className="w-5 h-5 rounded bg-rose-600 text-white flex items-center justify-center hover:bg-rose-700 text-[10px]"
                          title="Eliminar"
                          onClick={e => { e.stopPropagation(); removeBlock(b.id) }}
                        >
                          <Trash2 size={10} />
                        </button>
                      </div>
                    </div>
                  ))
                )}
                <div className="bg-slate-50 py-3 text-center border-t border-slate-100 text-[9px] text-slate-400">
                  © 2026 Obertrack · Este mensaje fue enviado automáticamente.
                </div>
              </div>
            </div>

            {/* Inspector */}
            <div className="w-[220px] bg-white border-l border-slate-200 flex flex-col overflow-y-auto">
              <div className="px-4 py-3 text-[10px] font-bold tracking-wider text-slate-500 uppercase border-b border-slate-200 bg-slate-50">
                Propiedades
              </div>
              {selectedBlock && selectedBlock.style?.raw !== 'true' ? (
                <Inspector block={selectedBlock} onChange={updateBlock} />
              ) : (
                <div className="flex-1 flex items-center justify-center text-slate-300 text-xs text-center px-6 py-8">
                  Selecciona un bloque para editar sus propiedades
                </div>
              )}
            </div>
          </div>
        )}

        {mode === 'code' && (
          <div className="flex flex-1 flex-col overflow-hidden bg-slate-900">
            <div className="px-4 py-2 border-b border-slate-800 flex justify-between items-center">
              <span className="text-[10px] font-bold text-slate-400 uppercase tracking-widest flex items-center gap-1.5">
                <Code size={12} className="text-purple-400" /> Editor de código crudo / Tailwind
              </span>
              <span className="text-[9px] text-slate-500">
                Soporte completo para HTML, CSS inline, Tailwind y scripts JS.
              </span>
            </div>
            <textarea
              value={rawHTML || (blocks.length === 1 && blocks[0].style?.raw === 'true' ? blocks[0].content : compileBlocksToHTML(blocks))}
              onChange={e => handleRawHTMLChange(e.target.value)}
              className="flex-1 w-full bg-slate-950 text-slate-200 font-mono text-xs p-4 outline-none border-none resize-none leading-relaxed"
              placeholder="<!-- Escribe tu HTML/Tailwind aquí... -->"
            />
          </div>
        )}

        {mode === 'preview' && (
          <div className="flex-1 overflow-hidden bg-slate-200 flex justify-center items-start p-6">
            <div
              className="bg-white rounded-xl shadow-lg overflow-hidden border border-slate-300 transition-all duration-300 flex flex-col h-full"
              style={{
                width: previewDevice === 'desktop' ? '100%' : '375px',
                maxWidth: previewDevice === 'desktop' ? '650px' : '375px',
              }}
            >
              <div className="px-4 py-2 bg-slate-50 border-b border-slate-200 flex justify-between items-center text-[10px] text-slate-500">
                <span>Dispositivo: {previewDevice === 'desktop' ? 'Escritorio (650px)' : 'Móvil (375px)'}</span>
                <span className="bg-purple-100 text-purple-700 px-1.5 py-0.5 rounded font-bold uppercase">Iframe Aislado</span>
              </div>
              <iframe
                title="Preview"
                srcDoc={srcDoc}
                className="w-full flex-1 border-none bg-slate-100"
                sandbox="allow-popups"
              />
            </div>
          </div>
        )}
      </div>
    </div>
  )
}

// ─── Helper: serialize blocks to JSON string (for backend storage) ───────────
export function blocksToJSON(blocks: EmailBlock[]): string {
  return JSON.stringify(blocks.map(({ id: _id, ...rest }) => rest))
}
