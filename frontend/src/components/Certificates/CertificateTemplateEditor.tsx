import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  AlignHorizontalJustifyCenter,
  ArrowLeft,
  ArrowRight,
  Bold,
  Calendar,
  CaseUpper,
  Check,
  Eye,
  GraduationCap,
  Hash,
  Minus,
  PenLine,
  Plus,
  Save,
  TextAlignCenter,
  TextAlignEnd,
  TextAlignStart,
  Trash2,
  Type,
  Upload,
  User,
  type LucideIcon,
} from 'lucide-react'

import { Button, Modal } from '../ui'
import SignaturePad from '../Testimonials/SignaturePad'
import { snapPosition, type SnapGuide } from './snapGuides'

/** Distancia (en píxeles de pantalla) a la que una guía atrae al campo. */
const SNAP_PX = 6
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

const STEPS = ['Diseño', 'Campos', 'Revisar']

const FIELD_LABEL: Record<CertificateFieldKey, string> = {
  name: 'Nombre del profesional',
  program: 'Programa',
  date: 'Fecha',
  code: 'Código de verificación',
  text: 'Texto libre',
  signature: 'Firma',
}

/** Nombre corto para los mosaicos de «Agregar». */
const FIELD_SHORT: Record<CertificateFieldKey, string> = {
  name: 'Nombre',
  program: 'Programa',
  date: 'Fecha',
  code: 'Código',
  text: 'Texto',
  signature: 'Firma',
}

const FIELD_ICON: Record<CertificateFieldKey, LucideIcon> = {
  name: User,
  program: GraduationCap,
  date: Calendar,
  code: Hash,
  text: Type,
  signature: PenLine,
}

const FONTS: { value: CertificateField['font']; label: string; short: string }[] = [
  { value: 'Poppins', label: 'Poppins', short: 'Poppins' },
  { value: 'Spartan', label: 'League Spartan (solo Bold)', short: 'Spartan' },
  { value: 'Helvetica', label: 'Helvetica', short: 'Helvetica' },
  { value: 'Times', label: 'Times', short: 'Times' },
  { value: 'Courier', label: 'Courier', short: 'Courier' },
]

const ALIGNS: { value: CertificateField['align']; label: string; icon: LucideIcon }[] = [
  { value: 'L', label: 'Alinear a la izquierda', icon: TextAlignStart },
  { value: 'C', label: 'Centrar', icon: TextAlignCenter },
  { value: 'R', label: 'Alinear a la derecha', icon: TextAlignEnd },
]

/** Colores de la marca a un clic; el último botón abre el selector libre. */
const SWATCHES = ['#000000', '#0f172a', '#fa3ab4', '#cc33cc', '#64748b', '#94a3b8', '#ffffff']

function Swatches({ value, onChange }: { value: string; onChange: (color: string) => void }) {
  const current = value.toLowerCase()
  const custom = !SWATCHES.includes(current)
  return (
    <div className={styles.swatches}>
      {SWATCHES.map((c) => (
        <button
          key={c}
          type="button"
          className={current === c ? styles.swatchOn : styles.swatch}
          style={{ background: c }}
          title={c}
          aria-label={`Color ${c}`}
          aria-pressed={current === c}
          onClick={() => onChange(c)}
        />
      ))}
      <label
        className={custom ? styles.swatchOn : styles.swatch}
        style={{ background: custom ? value : 'conic-gradient(red, yellow, lime, aqua, blue, magenta, red)' }}
        title="Otro color"
      >
        <input type="color" value={value} onChange={(e) => onChange(e.target.value)} aria-label="Otro color" />
      </label>
    </div>
  )
}

/** Lo que se pinta en el editor en lugar de cada campo. */
const SAMPLE: Record<CertificateFieldKey, string> = {
  name: 'María Fernanda Pérez',
  program: 'Prueba de Inducción',
  date: '25 de septiembre de 2026',
  code: 'OBT-EJEM-PLO1',
  text: 'Texto libre',
  signature: '',
}

/**
 * Variables del texto libre: el certificado las rellena con los datos de quien
 * aprueba, así el párrafo lo escribe la app y el diseño puede ir limpio.
 */
const TEXT_VARIABLES: { token: string; label: string; sample: string }[] = [
  { token: '{programa}', label: 'Programa', sample: 'Prueba de Inducción' },
  { token: '{nombre}', label: 'Nombre', sample: 'María Fernanda Pérez' },
  { token: '{fecha}', label: 'Fecha', sample: '25 de septiembre de 2026' },
  { token: '{codigo}', label: 'Código', sample: 'OBT-EJEM-PLO1' },
]
const VARIABLE_RE = /(\{(?:programa|nombre|fecha|codigo)\})/g

/** El párrafo que trae un texto libre nuevo: el de los certificados de Oberstaff. */
const DEFAULT_TEXT_FIELD: CertificateField = {
  key: 'text',
  text: 'Por haber completado y aprobado satisfactoriamente la {programa}, demostrando las competencias, conocimientos y alineación requeridos.',
  x: 50,
  y: 62,
  size: 13,
  color: '#0f172a',
  align: 'C',
  bold: false,
  font: 'Poppins',
  wrap: 60,
  highlight: '#fa3ab4',
}

/** Rango del ancho del párrafo: el mismo que acepta el servidor. */
const PARAGRAPH_WRAP = { min: 10, max: 95 }

/** Pinta un texto libre con las variables resaltadas, como sale en el PDF. */
function renderParagraph(f: CertificateField) {
  const parts = (f.text || 'Texto libre').split(VARIABLE_RE)
  return parts.map((part, i) => {
    const variable = TEXT_VARIABLES.find((v) => v.token === part)
    if (!variable) return <span key={i}>{part}</span>
    return (
      <span
        key={i}
        style={{ color: f.highlight || f.color, fontWeight: f.font === 'Poppins' ? 600 : 700 }}
      >
        {variable.sample}
      </span>
    )
  })
}

/** Ancho de la firma en % de la página: el mismo rango que acepta el servidor. */
const SIGNATURE_WIDTH = { min: 5, max: 60, initial: 20 }

/** Convierte el data URL PNG del SignaturePad en un archivo para subirlo. */
async function dataURLToFile(dataURL: string, filename: string): Promise<File> {
  const blob = await (await fetch(dataURL)).blob()
  return new File([blob], filename, { type: 'image/png' })
}

const DEFAULT_FIELDS: CertificateField[] = [
  // El nombre, en la tipografía y el rosado de la marca (#fa3ab4, el de
  // "CERTIFICADO" en el diseño de Oberstaff). Tiene que coincidir con
  // DefaultCertificateFields del backend.
  { key: 'name', x: 50, y: 48, size: 32, color: '#fa3ab4', align: 'C', bold: true, font: 'Poppins' },
  // El programa hace de título: League Spartan, en mayúsculas y en negro.
  { key: 'program', x: 50, y: 62, size: 18, color: '#000000', align: 'C', bold: true, font: 'Spartan', upper: true },
  { key: 'date', x: 50, y: 74, size: 12, color: '#64748b', align: 'C', bold: false, font: 'Helvetica' },
  { key: 'code', x: 50, y: 93, size: 9, color: '#94a3b8', align: 'C', bold: false, font: 'Courier' },
]

/** Ancho de página A4 en mm según orientación, para escalar la tipografía. */
const PAGE_WIDTH_MM = { L: 297, P: 210 }

const FONT_FAMILY: Record<CertificateField['font'], string> = {
  Helvetica: 'Helvetica, Arial, sans-serif',
  Times: '"Times New Roman", Times, serif',
  Courier: '"Courier New", Courier, monospace',
  Poppins: 'Poppins, sans-serif',
  Spartan: '"League Spartan", sans-serif',
}

/** Peso con el que se ve cada fuente, igual que en el PDF. */
function fontWeight(f: CertificateField): number {
  if (f.font === 'Spartan') return 700 // solo existe en Bold
  if (!f.bold) return 400
  return f.font === 'Poppins' ? 600 : 700 // la "negrita" de Poppins es la semibold
}

/**
 * Editor visual de una plantilla, como asistente de tres pasos: se sube el
 * diseño (PNG/JPG en A4), se arrastran los campos sobre la imagen y se ajusta
 * su tipografía, y se revisa con el PDF de ejemplo. Las posiciones van en
 * porcentaje, así el PDF coincide con lo que se ve aquí.
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
  // Una plantilla nueva empieza por el diseño; una existente, por los campos.
  const [step, setStep] = useState(template ? 1 : 0)
  const [dragOver, setDragOver] = useState(false)

  const canvasRef = useRef<HTMLDivElement>(null)
  // offsetX/Y: desde dónde se agarró el campo (en % de la página), para que
  // no salte al punto de anclaje al empezar a arrastrar.
  const dragRef = useRef<{ index: number; offsetX: number; offsetY: number } | null>(null)
  // Guías visibles mientras se arrastra (como las de Canva).
  const [guides, setGuides] = useState<{ vertical: SnapGuide | null; horizontal: SnapGuide | null } | null>(null)
  // Los campos al momento, para calcular las guías desde los eventos del puntero.
  const fieldsRef = useRef(fields)
  fieldsRef.current = fields

  // Ancho real del lienzo, para escalar los tamaños de letra como en el PDF.
  useEffect(() => {
    const el = canvasRef.current
    if (!el) return
    const update = () => setCanvasWidth(el.getBoundingClientRect().width)
    update()
    const obs = new ResizeObserver(update)
    obs.observe(el)
    return () => obs.disconnect()
    // El lienzo vive en el paso 2: al volver a él hay que medirlo de nuevo.
  }, [imageFilename, step])

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
  // Firma: se crea dibujándola, escribiéndola o subiendo una foto (el mismo
  // SignaturePad que los testimonios), se sube como PNG y queda como un campo
  // de imagen. signatureFor = índice del campo a reemplazar, null = nueva,
  // undefined = modal cerrado.
  const [signatureFor, setSignatureFor] = useState<number | null | undefined>(undefined)
  const [signatureDraft, setSignatureDraft] = useState('')
  const [signatureSaving, setSignatureSaving] = useState(false)

  const closeSignature = () => {
    setSignatureFor(undefined)
    setSignatureDraft('')
  }

  const saveSignature = async () => {
    if (!signatureDraft) return
    setSignatureSaving(true)
    try {
      const up = await uploadService.upload(await dataURLToFile(signatureDraft, 'firma.png'))
      if (signatureFor !== null && signatureFor !== undefined) {
        updateField(signatureFor, { image: up.filename })
      } else {
        const next: CertificateField = {
          key: 'signature',
          image: up.filename,
          width: SIGNATURE_WIDTH.initial,
          x: 69,
          y: 80,
          align: 'C',
          // No aplican a una imagen; el servidor los limpia.
          size: 12,
          color: '#0f172a',
          bold: false,
          font: 'Helvetica',
        }
        setFields((prev) => [...prev, next])
        setSelected(fields.length)
      }
      closeSignature()
    } catch (err: any) {
      showError(err?.response?.data?.error ?? 'No se pudo guardar la firma.')
    } finally {
      setSignatureSaving(false)
    }
  }

  const addField = (key: CertificateFieldKey) => {
    if (key === 'signature') {
      setSignatureDraft('')
      setSignatureFor(null)
      return
    }
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
    const next: CertificateField = key === 'text'
      ? existing.length > 0 ? { ...base } : { ...DEFAULT_TEXT_FIELD }
      : { ...base, key, text: undefined }
    setFields((prev) => [...prev, next])
    setSelected(fields.length)
  }

  // Inserta una variable donde está el cursor del texto libre (o al final).
  const textAreaRef = useRef<HTMLTextAreaElement>(null)
  const insertVariable = (token: string) => {
    const f = fields[selected]
    if (!f || f.key !== 'text') return
    const text = f.text ?? ''
    const el = textAreaRef.current
    const start = el?.selectionStart ?? text.length
    const end = el?.selectionEnd ?? text.length
    updateField(selected, { text: text.slice(0, start) + token + text.slice(end) })
    requestAnimationFrame(() => {
      el?.focus()
      el?.setSelectionRange(start + token.length, start + token.length)
    })
  }

  const removeField = (index: number) => {
    setFields((prev) => prev.filter((_, i) => i !== index))
    setSelected((cur) => (cur === index ? 0 : cur > index ? cur - 1 : cur))
  }

  // Arrastre: se convierte la posición del puntero a porcentaje del lienzo y
  // se ajusta a las guías (centro de la página y los demás campos). Con Alt
  // se arrastra libre, sin imán.
  const onPointerMove = useCallback((e: PointerEvent) => {
    const drag = dragRef.current
    const el = canvasRef.current
    if (!drag || !el) return
    const rect = el.getBoundingClientRect()
    const rawX = Math.min(100, Math.max(0, ((e.clientX - rect.left) / rect.width) * 100 - drag.offsetX))
    const rawY = Math.min(100, Math.max(0, ((e.clientY - rect.top) / rect.height) * 100 - drag.offsetY))
    let x = Math.round(rawX * 10) / 10
    let y = Math.round(rawY * 10) / 10
    if (e.altKey) {
      setGuides(null)
    } else {
      const others = fieldsRef.current.filter((_, i) => i !== drag.index)
      const snap = snapPosition({ x: rawX, y: rawY }, others, (SNAP_PX / rect.width) * 100, (SNAP_PX / rect.height) * 100)
      if (snap.vertical) x = snap.x
      if (snap.horizontal) y = snap.y
      setGuides(snap.vertical || snap.horizontal ? { vertical: snap.vertical, horizontal: snap.horizontal } : null)
    }
    setFields((prev) => prev.map((f, i) => (i === drag.index ? { ...f, x, y } : f)))
  }, [])

  const onPointerUp = useCallback(() => {
    dragRef.current = null
    setGuides(null)
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
    const rect = canvasRef.current?.getBoundingClientRect()
    const f = fields[index]
    dragRef.current = {
      index,
      offsetX: rect ? ((e.clientX - rect.left) / rect.width) * 100 - f.x : 0,
      offsetY: rect ? ((e.clientY - rect.top) / rect.height) * 100 - f.y : 0,
    }
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

  // --- Asistente por pasos ---------------------------------------------------
  const goTo = (target: number) => {
    if (target > 0 && (!name.trim() || !imageFilename)) {
      showError(!name.trim() ? 'Primero ponle un nombre a la plantilla.' : 'Primero sube el diseño.')
      setStep(0)
      return
    }
    setStep(Math.max(0, Math.min(STEPS.length - 1, target)))
  }
  const isLast = step === STEPS.length - 1
  const stepDone = [!!name.trim() && !!imageFilename, !!imageFilename && fields.length > 0, false]
  const fieldSummary = Array.from(new Set(fields.map((f) => FIELD_LABEL[f.key]))).join(', ')
  const stepSub = [
    imageFilename ? (orientation === 'L' ? 'Horizontal' : 'Vertical') : 'Imagen A4',
    `${fields.length} ${fields.length === 1 ? 'campo' : 'campos'}`,
    'PDF de ejemplo',
  ]

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

      <ol
        className={styles.wizard}
        aria-label="Pasos de la plantilla"
        style={{ gridTemplateColumns: `repeat(${STEPS.length}, minmax(0, 1fr))` }}
      >
        {STEPS.map((label, i) => (
          <li key={label}>
            <button
              type="button"
              className={i === step ? styles.wizardStepActive : styles.wizardStep}
              aria-current={i === step ? 'step' : undefined}
              onClick={() => goTo(i)}
            >
              <span className={styles.wizardNum} data-done={stepDone[i] && i !== step}>
                {stepDone[i] && i !== step ? <Check size={14} strokeWidth={3} /> : i + 1}
              </span>
              <span className={styles.stepText}>
                <span className={styles.stepLabel}>{label}</span>
                <span className={styles.stepSub}>{stepSub[i]}</span>
              </span>
            </button>
          </li>
        ))}
      </ol>

      {/* --- 1. Diseño --- */}
      <div className={styles.section} hidden={step !== 0}>
        <h3 className={styles.wizardTitle}>Sube el diseño</h3>
        <p className={styles.wizardIntro}>
          La imagen del certificado tal como la hizo el equipo de diseño, sin los datos de la
          persona: esos se imprimen encima al emitir.
        </p>
        <div className={styles.wizardSplit}>
          <div>
            <div className={styles.field}>
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
            <ul className={styles.certTips}>
              <li>
                <Check size={14} /> PNG o JPG, en tamaño A4.
              </li>
              <li>
                <Check size={14} /> Horizontal o vertical: se detecta solo.
              </li>
              <li>
                <Check size={14} /> Deja libres las zonas del nombre, la fecha y el código.
              </li>
            </ul>
          </div>

          {/* Zona para soltar o elegir la imagen; con diseño, su vista previa. */}
          <label
            className={dragOver ? styles.dropZoneOver : imageFilename ? styles.dropZoneFilled : styles.dropZone}
            onDragOver={(e) => {
              e.preventDefault()
              setDragOver(true)
            }}
            onDragLeave={() => setDragOver(false)}
            onDrop={(e) => {
              e.preventDefault()
              setDragOver(false)
              void handleUpload(e.dataTransfer.files?.[0])
            }}
            style={{ cursor: uploading ? 'wait' : 'pointer' }}
          >
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
            {imageFilename ? (
              <>
                <img
                  src={templateImageUrl(imageFilename)}
                  alt="Diseño del certificado"
                  className={orientation === 'L' ? styles.dropPreviewL : styles.dropPreviewP}
                />
                <span className={styles.dropMeta}>
                  <span className={styles.tag}>{orientation === 'L' ? 'Horizontal (A4)' : 'Vertical (A4)'}</span>
                  <span className={styles.dropHint}>
                    <Upload size={14} /> {uploading ? 'Subiendo...' : 'Suelta otra imagen o haz clic para cambiarla'}
                  </span>
                </span>
              </>
            ) : (
              <>
                <span className={styles.dropIcon}>
                  <Upload size={24} />
                </span>
                <strong>{uploading ? 'Subiendo...' : 'Arrastra aquí el diseño'}</strong>
                <span className={styles.dropHint}>o haz clic para elegirlo · PNG o JPG</span>
              </>
            )}
          </label>
        </div>
      </div>

      {/* --- 2. Campos --- */}
      <div className={styles.section} hidden={step !== 1}>
        <h3 className={styles.wizardTitle}>Coloca los datos sobre el diseño</h3>
        <p className={styles.wizardIntro}>
          Lo que ves es lo que se imprime; los datos de ejemplo se cambian por los reales al emitir.
          Ya vienen colocados el nombre, el programa, la fecha y el código.
        </p>
        <div className={styles.certKeys}>
          <span><kbd>Arrastrar</kbd> mueve</span>
          <span><kbd>Clic</kbd> edita</span>
          <span><kbd>Flechas</kbd> ajustan</span>
          <span><kbd>Supr</kbd> quita</span>
          <span><kbd>Alt</kbd> sin imán</span>
          <span className={styles.certKeyGuide}>Las guías rosas centran y alinean</span>
        </div>

        {!imageFilename ? (
          <div className={styles.empty}>Sube el diseño para colocar los campos.</div>
        ) : (
          <div style={{ display: 'grid', gridTemplateColumns: 'minmax(0, 1fr) 320px', gap: 20, alignItems: 'start' }}>
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
                      // La firma ocupa un ancho fijo de la página, como en el PDF.
                      ...(f.key === 'signature' ? { width: `${f.width ?? SIGNATURE_WIDTH.initial}%`, lineHeight: 0 } : {}),
                      fontSize: fontPx,
                      fontFamily: FONT_FAMILY[f.font],
                      fontWeight: fontWeight(f),
                      textTransform: f.upper ? 'uppercase' : 'none',
                      color: f.color,
                      whiteSpace: f.key === 'text' ? (f.wrap ? 'pre-line' : 'pre') : 'nowrap',
                      ...(f.key === 'text' && f.wrap ? { width: `${f.wrap}%`, textAlign: f.align === 'C' ? 'center' : f.align === 'R' ? 'right' : 'left' } : {}),
                      // El mismo interlineado que el PDF (1,2 una línea; 1,25 el párrafo).
                      lineHeight: f.key === 'text' ? 1.25 : 1.2,
                      padding: '2px 4px',
                      borderRadius: 4,
                      cursor: 'grab',
                      // El foco del teclado se marca igual que la selección.
                      outline: active ? '2px solid #cc33cc' : '1px dashed rgba(204, 51, 204, 0.45)',
                      outlineOffset: 0,
                      background: active ? 'rgba(204, 51, 204, 0.08)' : 'transparent',
                    }}
                  >
                    {f.key === 'signature' ? (
                      <img
                        src={templateImageUrl(f.image ?? '')}
                        alt="Firma"
                        draggable={false}
                        style={{ display: 'block', width: '100%', height: 'auto', pointerEvents: 'none' }}
                      />
                    ) : f.key === 'text' ? renderParagraph(f) : SAMPLE[f.key]}
                  </div>
                )
              })}
              {guides?.vertical && (
                <div
                  aria-hidden
                  style={{
                    position: 'absolute', top: 0, bottom: 0, left: `${guides.vertical.at}%`, width: 0,
                    borderLeft: `1px ${guides.vertical.center ? 'solid' : 'dashed'} #fa3ab4`,
                    pointerEvents: 'none', zIndex: 5,
                  }}
                />
              )}
              {guides?.horizontal && (
                <div
                  aria-hidden
                  style={{
                    position: 'absolute', left: 0, right: 0, top: `${guides.horizontal.at}%`, height: 0,
                    borderTop: `1px ${guides.horizontal.center ? 'solid' : 'dashed'} #fa3ab4`,
                    pointerEvents: 'none', zIndex: 5,
                  }}
                />
              )}
            </div>

            <div className={styles.propsPanel}>
              {/* --- Agregar --- */}
              <div className={styles.propsBlock}>
                <span className={styles.propsTitle}>Agregar</span>
                <div className={styles.addGrid}>
                  {(Object.keys(FIELD_LABEL) as CertificateFieldKey[]).map((key) => {
                    const Icon = FIELD_ICON[key]
                    const count = fields.filter((f) => f.key === key).length
                    return (
                      <button
                        key={key}
                        type="button"
                        className={styles.addTile}
                        onClick={() => addField(key)}
                        title={count > 0 ? `Ya hay ${count} en el diseño; se agrega otro` : 'Agregar al diseño'}
                      >
                        <Icon size={18} />
                        <span>{FIELD_SHORT[key]}</span>
                        {count > 0 && <span className={styles.addTileCount}>{count}</span>}
                      </button>
                    )
                  })}
                </div>
              </div>

              {/* --- Elementos: elegir uno aunque esté tapado por otro --- */}
              <div className={styles.propsBlock}>
                <span className={styles.propsTitle}>En el diseño · {fields.length}</span>
                <div className={styles.layerList}>
                  {fields.map((f, i) => {
                    const Icon = FIELD_ICON[f.key]
                    return (
                      <button
                        key={`${f.key}-${i}`}
                        type="button"
                        className={i === selected ? styles.layerItemOn : styles.layerItem}
                        onClick={() => setSelected(i)}
                      >
                        <Icon size={14} />
                        <span className={styles.layerName}>{FIELD_LABEL[f.key]}</span>
                        <span className={styles.layerSample}>
                          {f.key === 'signature' ? 'Imagen' : f.key === 'text' ? (f.text || '').slice(0, 24) : SAMPLE[f.key]}
                        </span>
                      </button>
                    )
                  })}
                </div>
              </div>

              {current && (
                <div className={styles.propsBlock}>
                  <div className={styles.propsHeadRow}>
                    <span className={styles.propsFieldName}>
                      {(() => {
                        const Icon = FIELD_ICON[current.key]
                        return <Icon size={16} />
                      })()}
                      {FIELD_LABEL[current.key]}
                    </span>
                    <button
                      type="button"
                      className={styles.iconBtn}
                      title="Quitar del diseño (Supr)"
                      aria-label="Quitar campo"
                      onClick={() => removeField(selected)}
                    >
                      <Trash2 size={14} />
                    </button>
                  </div>

                  {current.key === 'signature' && (
                    <>
                      <div className={styles.signaturePreview}>
                        <img src={templateImageUrl(current.image ?? '')} alt="Firma" />
                        <button
                          type="button"
                          className={styles.ghostBtnSm}
                          onClick={() => {
                            setSignatureDraft('')
                            setSignatureFor(selected)
                          }}
                        >
                          <PenLine size={12} /> Cambiar firma
                        </button>
                      </div>
                      <div className={styles.propRow}>
                        <span className={styles.propLabel}>Ancho</span>
                        <input
                          type="range"
                          className={styles.propRange}
                          min={SIGNATURE_WIDTH.min}
                          max={SIGNATURE_WIDTH.max}
                          value={current.width ?? SIGNATURE_WIDTH.initial}
                          aria-label="Ancho de la firma (% de la página)"
                          onChange={(e) => updateField(selected, { width: Number(e.target.value) })}
                        />
                        <span className={styles.propValue}>{current.width ?? SIGNATURE_WIDTH.initial}%</span>
                      </div>
                    </>
                  )}

                  {current.key === 'text' && (
                    <>
                      <div className={styles.field}>
                        <label className={styles.propLabel}>Texto</label>
                        <textarea
                          ref={textAreaRef}
                          rows={4}
                          value={current.text ?? ''}
                          onChange={(e) => updateField(selected, { text: e.target.value })}
                        />
                        <div className={styles.varChips}>
                          {TEXT_VARIABLES.map((v) => (
                            <button
                              key={v.token}
                              type="button"
                              onClick={() => insertVariable(v.token)}
                              title={`Se reemplaza por ${v.label.toLowerCase()} de cada certificado`}
                            >
                              <Plus size={11} /> {v.label}
                            </button>
                          ))}
                        </div>
                      </div>
                      <div className={styles.propRow}>
                        <span className={styles.propLabel}>Ancho</span>
                        <input
                          type="range"
                          className={styles.propRange}
                          min={0}
                          max={PARAGRAPH_WRAP.max}
                          value={current.wrap ?? 0}
                          aria-label="Ancho del párrafo (% de la página)"
                          onChange={(e) => {
                            const w = Number(e.target.value) || 0
                            updateField(selected, { wrap: w < PARAGRAPH_WRAP.min ? 0 : w })
                          }}
                        />
                        <span className={styles.propValue}>{current.wrap ? `${current.wrap}%` : 'Una línea'}</span>
                      </div>
                      <div>
                        <span className={styles.propLabel}>Color de las variables</span>
                        <Swatches
                          value={current.highlight || current.color}
                          onChange={(c) => updateField(selected, { highlight: c })}
                        />
                      </div>
                    </>
                  )}

                  {current.key !== 'signature' && (
                    <>
                      <div>
                        <span className={styles.propLabel}>Fuente</span>
                        <div className={styles.fontGrid}>
                          {FONTS.map((font) => (
                            <button
                              key={font.value}
                              type="button"
                              className={current.font === font.value ? styles.fontTileOn : styles.fontTile}
                              onClick={() => updateField(selected, { font: font.value })}
                              title={font.label}
                            >
                              <span
                                style={{
                                  fontFamily: FONT_FAMILY[font.value],
                                  fontWeight: font.value === 'Spartan' ? 700 : 500,
                                }}
                              >
                                Aa
                              </span>
                              <small>{font.short}</small>
                            </button>
                          ))}
                        </div>
                      </div>

                      <div className={styles.propRow}>
                        <span className={styles.propLabel}>Tamaño</span>
                        <div className={styles.stepper}>
                          <button
                            type="button"
                            aria-label="Más pequeño"
                            onClick={() => updateField(selected, { size: Math.max(6, current.size - 1) })}
                          >
                            <Minus size={14} />
                          </button>
                          <input
                            type="number"
                            min={6}
                            max={120}
                            value={current.size}
                            aria-label="Tamaño (pt)"
                            onChange={(e) => updateField(selected, { size: Number(e.target.value) || 12 })}
                          />
                          <button
                            type="button"
                            aria-label="Más grande"
                            onClick={() => updateField(selected, { size: Math.min(120, current.size + 1) })}
                          >
                            <Plus size={14} />
                          </button>
                        </div>
                        <span className={styles.muted} style={{ fontSize: 12.5 }}>
                          pt
                        </span>
                      </div>

                      <div className={styles.propRow}>
                        <span className={styles.propLabel}>Estilo</span>
                        <div className={styles.toolGroup}>
                          <button
                            type="button"
                            className={current.bold ? styles.toolOn : styles.tool}
                            aria-pressed={current.bold}
                            title="Negrita"
                            disabled={current.font === 'Spartan'}
                            onClick={() => updateField(selected, { bold: !current.bold })}
                          >
                            <Bold size={15} />
                          </button>
                          <button
                            type="button"
                            className={current.upper ? styles.toolOn : styles.tool}
                            aria-pressed={!!current.upper}
                            title="Mayúsculas"
                            onClick={() => updateField(selected, { upper: !current.upper })}
                          >
                            <CaseUpper size={16} />
                          </button>
                        </div>
                      </div>
                    </>
                  )}

                  <div className={styles.propRow}>
                    <span className={styles.propLabel}>Alinear</span>
                    <div className={styles.toolGroup}>
                      {ALIGNS.map(({ value, label, icon: Icon }) => (
                        <button
                          key={value}
                          type="button"
                          className={current.align === value ? styles.toolOn : styles.tool}
                          aria-pressed={current.align === value}
                          title={label}
                          onClick={() => updateField(selected, { align: value })}
                        >
                          <Icon size={15} />
                        </button>
                      ))}
                    </div>
                  </div>

                  {current.key !== 'signature' && (
                    <div>
                      <span className={styles.propLabel}>Color</span>
                      <Swatches value={current.color} onChange={(c) => updateField(selected, { color: c })} />
                    </div>
                  )}

                  <div className={styles.propRow}>
                    <span className={styles.propLabel}>Posición</span>
                    <div className={styles.posInputs}>
                      <label>
                        X
                        <input
                          type="number"
                          min={0}
                          max={100}
                          step={0.5}
                          value={current.x}
                          aria-label="X (%)"
                          onChange={(e) => updateField(selected, { x: Number(e.target.value) })}
                        />
                      </label>
                      <label>
                        Y
                        <input
                          type="number"
                          min={0}
                          max={100}
                          step={0.5}
                          value={current.y}
                          aria-label="Y (%)"
                          onChange={(e) => updateField(selected, { y: Number(e.target.value) })}
                        />
                      </label>
                    </div>
                    <button
                      type="button"
                      className={styles.tool}
                      title="Centrar en la página"
                      onClick={() => updateField(selected, { x: 50, align: 'C' })}
                    >
                      <AlignHorizontalJustifyCenter size={15} />
                    </button>
                  </div>
                </div>
              )}
            </div>
          </div>
        )}
      </div>

      {/* --- 3. Revisar --- */}
      <div className={styles.section} hidden={step !== 2}>
        <h3 className={styles.wizardTitle}>Revisa y {template ? 'guarda' : 'crea'} la plantilla</h3>
        <p className={styles.wizardIntro}>
          Antes de guardar, mira el PDF de ejemplo: es exactamente lo que recibirá el profesional.
        </p>
        <div className={styles.reviewSplit}>
          <div className={styles.reviewList}>
            <div className={styles.reviewRow}>
              <span className={styles.reviewLabel}>Nombre</span>
              <span className={styles.reviewValue}>{name || '—'}</span>
              <button type="button" className={styles.linkBtn} onClick={() => goTo(0)}>
                Cambiar
              </button>
            </div>
            <div className={styles.reviewRow}>
              <span className={styles.reviewLabel}>Diseño</span>
              <span className={styles.reviewValue}>
                {imageFilename ? (orientation === 'L' ? 'Horizontal (A4)' : 'Vertical (A4)') : 'Sin diseño'}
              </span>
              <button type="button" className={styles.linkBtn} onClick={() => goTo(0)}>
                Cambiar
              </button>
            </div>
            <div className={styles.reviewRow}>
              <span className={styles.reviewLabel}>Datos impresos</span>
              <span className={styles.reviewValue}>
                {fields.length} {fields.length === 1 ? 'campo' : 'campos'}
                <small>{fieldSummary}</small>
              </span>
              <button type="button" className={styles.linkBtn} onClick={() => goTo(1)}>
                Editar
              </button>
            </div>
            <div className={styles.reviewRow}>
              <span className={styles.reviewLabel}>En uso</span>
              <span className={styles.reviewValue}>
                {template && template.program_names.length > 0 ? (
                  template.program_names.join(', ')
                ) : (
                  <small style={{ fontSize: 13.5 }}>
                    Ningún programa todavía. Se elige en el paso «Revisar» de cada programa.
                  </small>
                )}
              </span>
            </div>
          </div>

          <aside className={styles.certCard} aria-label="Vista previa del certificado">
            <span className={styles.blockPreviewLabel}>Así se ve</span>
            {imageFilename && (
              <img
                src={templateImageUrl(imageFilename)}
                alt={`Diseño ${name}`}
                className={orientation === 'L' ? styles.certThumbL : styles.certThumbP}
              />
            )}
            <button type="button" className={styles.saveBtn} onClick={handlePreview} disabled={previewing || !imageFilename}>
              <Eye size={16} /> {previewing ? 'Generando...' : 'Ver PDF de ejemplo'}
            </button>
            <span className={styles.badgeCardHint}>Se abre en otra pestaña con datos de ejemplo.</span>
          </aside>
        </div>
      </div>

      {signatureFor !== undefined && (
        <Modal
          isOpen
          isDirty={!!signatureDraft}
          onClose={closeSignature}
          title={signatureFor === null ? 'Agregar firma' : 'Cambiar firma'}
          size="lg"
          footer={
            <>
              <Button variant="secondary" onClick={closeSignature} disabled={signatureSaving}>Cancelar</Button>
              <Button onClick={saveSignature} loading={signatureSaving} disabled={!signatureDraft}>Usar firma</Button>
            </>
          }
        >
          <SignaturePad
            onChange={(dataURL) => setSignatureDraft(dataURL)}
            hint="Dibuja la firma, escríbela o sube una foto. Se guarda como imagen y se coloca encima del diseño."
            disabled={signatureSaving}
          />
        </Modal>
      )}

      <div className={styles.stickyBar}>
        <span className={styles.muted}>
          Paso {step + 1} de {STEPS.length}
        </span>
        {step === 1 && (
          <button type="button" className={styles.ghostBtn} onClick={handlePreview} disabled={previewing || !imageFilename}>
            <Eye size={16} /> {previewing ? 'Generando...' : 'Vista previa PDF'}
          </button>
        )}
        {step === 0 ? (
          <button type="button" className={styles.ghostBtn} onClick={onBack} disabled={saving}>
            {template ? 'Volver' : 'Cancelar'}
          </button>
        ) : (
          <button type="button" className={styles.ghostBtn} onClick={() => goTo(step - 1)} disabled={saving}>
            <ArrowLeft size={16} /> Atrás
          </button>
        )}
        {/* Una plantilla ya creada se puede guardar desde cualquier paso. */}
        {template && !isLast && (
          <button type="button" className={styles.ghostBtn} disabled={saving} onClick={handleSave}>
            <Save size={16} /> {saving ? 'Guardando...' : 'Guardar'}
          </button>
        )}
        {isLast ? (
          <button type="button" className={styles.saveBtn} disabled={saving} onClick={handleSave}>
            <Save size={16} /> {saving ? 'Guardando...' : template ? 'Guardar plantilla' : 'Crear plantilla'}
          </button>
        ) : (
          <button type="button" className={styles.saveBtn} onClick={() => goTo(step + 1)}>
            Siguiente <ArrowRight size={16} />
          </button>
        )}
      </div>
    </div>
  )
}
