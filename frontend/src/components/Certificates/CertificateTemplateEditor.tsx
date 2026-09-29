import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { ArrowLeft, Save, Upload, Eye, Plus, Trash2 } from 'lucide-react'

import { Select } from '../ui'
import { useNotification } from '../../context/NotificationContext'
import { uploadService } from '../../services/upload.service'
import {
  certificateService,
  templateImageUrl,
  type CertificateField,
  type CertificateFieldKey,
  type CertificateTemplate,
} from '../../services/certificate.service'
import styles from '../Admin/InductionSettings.module.css'

interface Props {
  /** null = plantilla nueva. */
  template: CertificateTemplate | null
  onSaved: (t: CertificateTemplate) => void
  onBack: () => void
}

const FIELD_LABEL: Record<CertificateFieldKey, string> = {
  name: 'Nombre del profesional',
  program: 'Programa',
  date: 'Fecha',
  code: 'Código de verificación',
  text: 'Texto libre',
}

/** Lo que se pinta en el editor en lugar de cada campo. */
const SAMPLE: Record<CertificateFieldKey, string> = {
  name: 'María Fernanda Pérez',
  program: 'Programa de ejemplo',
  date: '25 de septiembre de 2026',
  code: 'OBT-EJEM-PLO1',
  text: 'Texto libre',
}

const DEFAULT_FIELDS: CertificateField[] = [
  { key: 'name', x: 50, y: 48, size: 32, color: '#0f172a', align: 'C', bold: true, font: 'Helvetica' },
  { key: 'program', x: 50, y: 62, size: 18, color: '#334155', align: 'C', bold: false, font: 'Helvetica' },
  { key: 'date', x: 50, y: 74, size: 12, color: '#64748b', align: 'C', bold: false, font: 'Helvetica' },
  { key: 'code', x: 50, y: 93, size: 9, color: '#94a3b8', align: 'C', bold: false, font: 'Courier' },
]

/** Ancho de página A4 en mm según orientación, para escalar la tipografía. */
const PAGE_WIDTH_MM = { L: 297, P: 210 }

const FONT_FAMILY: Record<CertificateField['font'], string> = {
  Helvetica: 'Helvetica, Arial, sans-serif',
  Times: '"Times New Roman", Times, serif',
  Courier: '"Courier New", Courier, monospace',
}

/**
 * Editor visual de una plantilla: se sube el diseño (PNG/JPG en A4), se
 * arrastran los campos sobre la imagen y se ajusta su tipografía. Las
 * posiciones van en porcentaje, así el PDF coincide con lo que se ve aquí.
 */
export default function CertificateTemplateEditor({ template, onSaved, onBack }: Props) {
  const { success, error: showError } = useNotification()

  const [name, setName] = useState(template?.name ?? '')
  const [imageFilename, setImageFilename] = useState(template?.image_filename ?? '')
  const [orientation, setOrientation] = useState<'L' | 'P'>(template?.orientation ?? 'L')
  const [fields, setFields] = useState<CertificateField[]>(
    template?.fields?.length ? template.fields : DEFAULT_FIELDS.map((f) => ({ ...f }))
  )
  const [selected, setSelected] = useState<number>(0)
  const [uploading, setUploading] = useState(false)
  const [saving, setSaving] = useState(false)
  const [previewing, setPreviewing] = useState(false)
  const [canvasWidth, setCanvasWidth] = useState(0)

  const canvasRef = useRef<HTMLDivElement>(null)
  const dragRef = useRef<{ index: number } | null>(null)

  // Ancho real del lienzo, para escalar los tamaños de letra como en el PDF.
  useEffect(() => {
    const el = canvasRef.current
    if (!el) return
    const update = () => setCanvasWidth(el.getBoundingClientRect().width)
    update()
    const obs = new ResizeObserver(update)
    obs.observe(el)
    return () => obs.disconnect()
  }, [imageFilename])

  const pxPerMm = canvasWidth > 0 ? canvasWidth / PAGE_WIDTH_MM[orientation] : 0

  const handleUpload = async (file: File | undefined) => {
    if (!file) return
    if (!['image/png', 'image/jpeg'].includes(file.type)) {
      showError('El diseño debe ser una imagen PNG o JPG.')
      return
    }
    setUploading(true)
    try {
      const up = await uploadService.upload(file)
      setImageFilename(up.filename)
      // Orientación según la imagen, igual que hará el servidor.
      const img = new Image()
      img.onload = () => setOrientation(img.naturalHeight > img.naturalWidth ? 'P' : 'L')
      img.src = templateImageUrl(up.filename)
      success('Diseño subido. Ahora coloca los campos.')
    } catch (err: any) {
      showError(err?.response?.data?.error ?? 'No se pudo subir el diseño.')
    } finally {
      setUploading(false)
    }
  }

  const updateField = (index: number, patch: Partial<CertificateField>) =>
    setFields((prev) => prev.map((f, i) => (i === index ? { ...f, ...patch } : f)))

  // Cualquier campo puede ir más de una vez (un diseño puede repetir el
  // nombre o el código). El campo nuevo se coloca un poco desplazado del que
  // ya existe para que no quede escondido encima.
  const addField = (key: CertificateFieldKey) => {
    const existing = fields.filter((f) => f.key === key)
    const base = existing.length > 0
      ? { ...existing[existing.length - 1], y: Math.min(96, existing[existing.length - 1].y + 8) }
      : DEFAULT_FIELDS.find((f) => f.key === key) ?? {
      key,
      x: 50,
      y: 84,
      size: 12,
      color: '#334155',
      align: 'C' as const,
      bold: false,
      font: 'Helvetica' as const,
    }
    const next: CertificateField = { ...base, key, text: key === 'text' ? 'por haber completado' : undefined }
    setFields((prev) => [...prev, next])
    setSelected(fields.length)
  }

  const removeField = (index: number) => {
    setFields((prev) => prev.filter((_, i) => i !== index))
    setSelected((cur) => (cur === index ? 0 : cur > index ? cur - 1 : cur))
  }

  // Arrastre: se convierte la posición del puntero a porcentaje del lienzo.
  const onPointerMove = useCallback((e: PointerEvent) => {
    const drag = dragRef.current
    const el = canvasRef.current
    if (!drag || !el) return
    const rect = el.getBoundingClientRect()
    const x = Math.min(100, Math.max(0, ((e.clientX - rect.left) / rect.width) * 100))
    const y = Math.min(100, Math.max(0, ((e.clientY - rect.top) / rect.height) * 100))
    setFields((prev) =>
      prev.map((f, i) => (i === drag.index ? { ...f, x: Math.round(x * 10) / 10, y: Math.round(y * 10) / 10 } : f))
    )
  }, [])

  const onPointerUp = useCallback(() => {
    dragRef.current = null
    window.removeEventListener('pointermove', onPointerMove)
    window.removeEventListener('pointerup', onPointerUp)
  }, [onPointerMove])

  // Teclado sobre un campo del lienzo: Suprimir/Retroceso lo quita, las
  // flechas lo mueven medio punto (con Shift, dos).
  const onFieldKeyDown = (index: number) => (e: React.KeyboardEvent) => {
    if (e.key === 'Delete' || e.key === 'Backspace') {
      e.preventDefault()
      removeField(index)
      return
    }
    const step = e.shiftKey ? 2 : 0.5
    const nudge: Record<string, { dx: number; dy: number }> = {
      ArrowLeft: { dx: -step, dy: 0 },
      ArrowRight: { dx: step, dy: 0 },
      ArrowUp: { dx: 0, dy: -step },
      ArrowDown: { dx: 0, dy: step },
    }
    const move = nudge[e.key]
    if (!move) return
    e.preventDefault()
    setFields((prev) =>
      prev.map((f, i) =>
        i === index
          ? {
              ...f,
              x: Math.round(Math.min(100, Math.max(0, f.x + move.dx)) * 10) / 10,
              y: Math.round(Math.min(100, Math.max(0, f.y + move.dy)) * 10) / 10,
            }
          : f
      )
    )
  }

  const startDrag = (index: number) => (e: React.PointerEvent) => {
    e.preventDefault()
    ;(e.currentTarget as HTMLElement).focus()
    setSelected(index)
    dragRef.current = { index }
    window.addEventListener('pointermove', onPointerMove)
    window.addEventListener('pointerup', onPointerUp)
  }

  const input = useMemo(
    () => ({ name: name.trim(), image_filename: imageFilename, fields }),
    [name, imageFilename, fields]
  )

  const handlePreview = async () => {
    if (!imageFilename) {
      showError('Sube el diseño antes de previsualizar.')
      return
    }
    setPreviewing(true)
    try {
      const blob = await certificateService.previewTemplate({ ...input, name: input.name || 'Vista previa' })
      const url = URL.createObjectURL(blob)
      window.open(url, '_blank', 'noopener,noreferrer')
      setTimeout(() => URL.revokeObjectURL(url), 60_000)
    } catch (err: any) {
      showError(err?.response?.data?.error ?? 'No se pudo generar la vista previa.')
    } finally {
      setPreviewing(false)
    }
  }

  const handleSave = async () => {
    if (!input.name) {
      showError('La plantilla necesita un nombre.')
      return
    }
    if (!imageFilename) {
      showError('Sube el diseño del certificado.')
      return
    }
    setSaving(true)
    try {
      const saved = template
        ? await certificateService.updateTemplate(template.id, input)
        : await certificateService.createTemplate(input)
      success(template ? 'Plantilla guardada.' : 'Plantilla creada.')
      onSaved(saved)
    } catch (err: any) {
      showError(err?.response?.data?.error ?? 'No se pudo guardar la plantilla.')
    } finally {
      setSaving(false)
    }
  }

  const current = fields[selected]

  return (
    <div>
      <div className={styles.editorHead}>
        <button type="button" className={styles.backBtn} onClick={onBack}>
          <ArrowLeft size={14} /> Certificados
        </button>
        <h3 className={styles.keyTitle}>{template ? template.name : 'Nueva plantilla de certificado'}</h3>
        {template && template.program_names.length > 0 && (
          <span className={styles.tag}>En uso: {template.program_names.join(', ')}</span>
        )}
      </div>

      {/* --- 1. Diseño --- */}
      <div className={styles.section}>
        <div className={styles.sectionHead}>
          <span className={styles.sectionNum}>1</span>
          <h3 className={styles.keyTitle}>Diseño</h3>
        </div>
        <p className={styles.sectionIntro}>
          Sube el diseño del equipo como imagen PNG o JPG en tamaño A4 (horizontal o vertical). Los textos se
          imprimen encima al emitir.
        </p>
        <div className={styles.grid}>
          <div className={`${styles.field} ${styles.fieldWide}`}>
            <label htmlFor="cert-name">Nombre de la plantilla</label>
            <input
              id="cert-name"
              type="text"
              placeholder="Ej. Certificado de inducción 2026"
              value={name}
              onChange={(e) => setName(e.target.value)}
              autoFocus={!template}
            />
          </div>
        </div>
        <div style={{ display: 'flex', gap: 12, alignItems: 'center', flexWrap: 'wrap', marginTop: 14 }}>
          <label className={styles.ghostBtn} style={{ cursor: uploading ? 'wait' : 'pointer' }}>
            <Upload size={16} /> {uploading ? 'Subiendo...' : imageFilename ? 'Cambiar diseño' : 'Subir diseño'}
            <input
              type="file"
              accept="image/png,image/jpeg"
              style={{ display: 'none' }}
              disabled={uploading}
              onChange={(e) => {
                void handleUpload(e.target.files?.[0])
                e.currentTarget.value = ''
              }}
            />
          </label>
          {imageFilename && (
            <span className={styles.tag}>{orientation === 'L' ? 'Horizontal (A4)' : 'Vertical (A4)'}</span>
          )}
        </div>
      </div>

      {/* --- 2. Campos --- */}
      <div className={styles.section}>
        <div className={styles.sectionHead}>
          <span className={styles.sectionNum}>2</span>
          <h3 className={styles.keyTitle}>Campos sobre el diseño</h3>
        </div>
        <p className={styles.sectionIntro}>
          Arrastra cada campo a su lugar. Lo que ves aquí es lo que se imprime; los datos de ejemplo se reemplazan
          por los reales al emitir. Una plantilla nueva ya trae nombre, programa, fecha y código colocados; haz clic
          en uno sobre el diseño para editarlo, Supr para quitarlo y las flechas para ajustarlo.
        </p>

        {!imageFilename ? (
          <div className={styles.empty}>Sube el diseño para colocar los campos.</div>
        ) : (
          <div style={{ display: 'grid', gridTemplateColumns: 'minmax(0, 1fr) 280px', gap: 20, alignItems: 'start' }}>
            <div
              ref={canvasRef}
              style={{
                position: 'relative',
                width: '100%',
                aspectRatio: orientation === 'L' ? '297 / 210' : '210 / 297',
                borderRadius: 12,
                overflow: 'hidden',
                border: '1px solid #e2e8f0',
                background: '#f8fafc',
                userSelect: 'none',
                touchAction: 'none',
              }}
            >
              <img
                src={templateImageUrl(imageFilename)}
                alt="Diseño del certificado"
                draggable={false}
                style={{ position: 'absolute', inset: 0, width: '100%', height: '100%', objectFit: 'fill' }}
              />
              {fields.map((f, i) => {
                const fontPx = Math.max(6, f.size * 0.3528 * pxPerMm)
                const translateX = f.align === 'C' ? '-50%' : f.align === 'R' ? '-100%' : '0'
                const active = i === selected
                return (
                  <div
                    key={`${f.key}-${i}`}
                    role="button"
                    tabIndex={0}
                    onPointerDown={startDrag(i)}
                    onClick={() => setSelected(i)}
                    onKeyDown={onFieldKeyDown(i)}
                    title={`${FIELD_LABEL[f.key]} · Supr para quitar, flechas para mover`}
                    style={{
                      position: 'absolute',
                      left: `${f.x}%`,
                      top: `${f.y}%`,
                      transform: `translate(${translateX}, -50%)`,
                      fontSize: fontPx,
                      fontFamily: FONT_FAMILY[f.font],
                      fontWeight: f.bold ? 700 : 400,
                      color: f.color,
                      whiteSpace: 'nowrap',
                      lineHeight: 1.2,
                      padding: '2px 4px',
                      borderRadius: 4,
                      cursor: 'grab',
                      // El foco del teclado se marca igual que la selección.
                      outline: active ? '2px solid #cc33cc' : '1px dashed rgba(204, 51, 204, 0.45)',
                      outlineOffset: 0,
                      background: active ? 'rgba(204, 51, 204, 0.08)' : 'transparent',
                    }}
                  >
                    {f.key === 'text' ? f.text || 'Texto libre' : SAMPLE[f.key]}
                  </div>
                )
              })}
            </div>

            <div style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
              <div>
                <div className={styles.smallLabel} style={{ marginBottom: 6 }}>
                  Agregar campo
                </div>
                <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6 }}>
                  {(Object.keys(FIELD_LABEL) as CertificateFieldKey[]).map((key) => {
                    const count = fields.filter((f) => f.key === key).length
                    return (
                      <button
                        key={key}
                        type="button"
                        className={styles.ghostBtnSm}
                        onClick={() => addField(key)}
                        title={count > 0 ? `Ya hay ${count} en el diseño; se agrega otro` : 'Agregar al diseño'}
                      >
                        <Plus size={12} /> {FIELD_LABEL[key]}
                        {count > 0 && (
                          <span className={styles.tagOk} style={{ marginLeft: 4, padding: '1px 7px' }}>
                            {count}
                          </span>
                        )}
                      </button>
                    )
                  })}
                </div>
              </div>

              {current && (
                <div style={{ border: '1px solid #e2e8f0', borderRadius: 12, padding: 14, display: 'flex', flexDirection: 'column', gap: 12 }}>
                  <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 8 }}>
                    <strong style={{ fontSize: 14, color: '#0f172a' }}>{FIELD_LABEL[current.key]}</strong>
                    <button
                      type="button"
                      className={styles.iconBtn}
                      title="Quitar campo"
                      aria-label="Quitar campo"
                      onClick={() => removeField(selected)}
                    >
                      <Trash2 size={14} />
                    </button>
                  </div>

                  {current.key === 'text' && (
                    <div className={styles.field}>
                      <label className={styles.smallLabel}>Texto</label>
                      <input
                        type="text"
                        value={current.text ?? ''}
                        onChange={(e) => updateField(selected, { text: e.target.value })}
                      />
                    </div>
                  )}

                  <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 10 }}>
                    <div className={styles.field}>
                      <label className={styles.smallLabel}>Tamaño (pt)</label>
                      <input
                        type="number"
                        min={6}
                        max={120}
                        value={current.size}
                        onChange={(e) => updateField(selected, { size: Number(e.target.value) || 12 })}
                      />
                    </div>
                    <div className={styles.field}>
                      <label className={styles.smallLabel}>Color</label>
                      <input
                        type="color"
                        value={current.color}
                        onChange={(e) => updateField(selected, { color: e.target.value })}
                        style={{ padding: 4, height: 42 }}
                      />
                    </div>
                  </div>

                  <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 10 }}>
                    <div className={styles.field}>
                      <label className={styles.smallLabel}>Fuente</label>
                      <Select
                        fullWidth
                        value={current.font}
                        onChange={(v) => updateField(selected, { font: v as CertificateField['font'] })}
                        options={[
                          { value: 'Helvetica', label: 'Helvetica' },
                          { value: 'Times', label: 'Times' },
                          { value: 'Courier', label: 'Courier' },
                        ]}
                      />
                    </div>
                    <div className={styles.field}>
                      <label className={styles.smallLabel}>Alineación</label>
                      <Select
                        fullWidth
                        value={current.align}
                        onChange={(v) => updateField(selected, { align: v as CertificateField['align'] })}
                        options={[
                          { value: 'L', label: 'Izquierda' },
                          { value: 'C', label: 'Centro' },
                          { value: 'R', label: 'Derecha' },
                        ]}
                      />
                    </div>
                  </div>

                  <label className={styles.checkRow}>
                    <input
                      type="checkbox"
                      checked={current.bold}
                      onChange={(e) => updateField(selected, { bold: e.target.checked })}
                    />
                    Negrita
                  </label>

                  <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 10 }}>
                    <div className={styles.field}>
                      <label className={styles.smallLabel}>X (%)</label>
                      <input
                        type="number"
                        min={0}
                        max={100}
                        step={0.5}
                        value={current.x}
                        onChange={(e) => updateField(selected, { x: Number(e.target.value) })}
                      />
                    </div>
                    <div className={styles.field}>
                      <label className={styles.smallLabel}>Y (%)</label>
                      <input
                        type="number"
                        min={0}
                        max={100}
                        step={0.5}
                        value={current.y}
                        onChange={(e) => updateField(selected, { y: Number(e.target.value) })}
                      />
                    </div>
                  </div>
                </div>
              )}
            </div>
          </div>
        )}
      </div>

      <div className={styles.stickyBar}>
        <span className={styles.muted}>
          {fields.length} {fields.length === 1 ? 'campo' : 'campos'} · {orientation === 'L' ? 'horizontal' : 'vertical'}
        </span>
        <button type="button" className={styles.ghostBtn} onClick={handlePreview} disabled={previewing || !imageFilename}>
          <Eye size={16} /> {previewing ? 'Generando...' : 'Vista previa PDF'}
        </button>
        <button type="button" className={styles.ghostBtn} onClick={onBack} disabled={saving}>
          Cancelar
        </button>
        <button type="button" className={styles.saveBtn} disabled={saving} onClick={handleSave}>
          <Save size={16} /> {saving ? 'Guardando...' : template ? 'Guardar plantilla' : 'Crear plantilla'}
        </button>
      </div>
    </div>
  )
}
