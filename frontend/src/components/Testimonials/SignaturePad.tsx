import { useCallback, useEffect, useRef, useState } from 'react'
import { Eraser, PenLine, Type, Upload } from 'lucide-react'

import styles from './SignaturePad.module.css'

/** Cómo se produjo la firma. Viaja al servidor: es parte de la evidencia. */
export type SignatureMode = 'drawn' | 'uploaded' | 'typed'

interface SignaturePadProps {
  /**
   * Se llama con la firma como data URL PNG y con la modalidad usada, o con
   * cadena vacía cuando no hay firma. Las tres modalidades se normalizan a PNG
   * aquí, así que el servidor recibe siempre lo mismo.
   */
  onChange: (dataURL: string, mode: SignatureMode) => void
  /** Nombre del firmante, para proponerlo en la modalidad escrita. */
  suggestedName?: string
  hint?: string
  disabled?: boolean
}

/** Formatos que acepta la carga de una foto de la firma. */
const ACCEPTED = ['image/png', 'image/jpeg', 'image/webp']
/** Tope del archivo ORIGEN. Una foto de móvil ronda 2-5 MB. */
const MAX_UPLOAD_BYTES = 8 * 1024 * 1024
/**
 * Tope de la imagen YA procesada. El servidor rechaza por encima de 512 KB, así
 * que aquí se deja margen y se reduce hasta caber en lugar de fallar al enviar.
 */
const MAX_OUTPUT_BYTES = 380 * 1024

/** Caja donde se encaja cualquier firma, en píxeles CSS. */
const BOX_W = 560
const BOX_H = 170

/** Tipografías de firma. Las carga index.html. */
const FONTS = [
  { label: 'Dancing Script', css: "'Dancing Script', cursive", size: 58 },
  { label: 'Great Vibes', css: "'Great Vibes', cursive", size: 60 },
  { label: 'Allura', css: "'Allura', cursive", size: 64 },
  { label: 'Sacramento', css: "'Sacramento', cursive", size: 62 },
  { label: 'Caveat', css: "'Caveat', cursive", size: 56 },
]

/** Peso aproximado en bytes de un data URL base64. */
const dataURLBytes = (url: string) => Math.ceil((url.length - url.indexOf(',') - 1) * 0.75)

/**
 * Recuadro de firma con tres modalidades, al estilo de los firmadores al uso:
 * trazarla, cargar una foto de la firma real, o escribir el nombre con una
 * tipografía caligráfica.
 *
 * Las tres acaban produciendo el MISMO artefacto —un PNG como data URL— para que
 * el servidor no tenga tres caminos que validar. Lo que sí viaja aparte es la
 * modalidad, porque no prueban lo mismo y la constancia debe decirlo.
 */
export default function SignaturePad({
  onChange,
  suggestedName = '',
  hint,
  disabled = false,
}: SignaturePadProps) {
  const [mode, setMode] = useState<SignatureMode>('drawn')

  // --- Trazo ---
  const canvasRef = useRef<HTMLCanvasElement>(null)
  const drawingRef = useRef(false)
  const [hasInk, setHasInk] = useState(false)

  // --- Carga ---
  const fileRef = useRef<HTMLInputElement>(null)
  const [uploaded, setUploaded] = useState('')
  const [uploadError, setUploadError] = useState('')
  const [processing, setProcessing] = useState(false)

  // --- Escrito ---
  const [typedName, setTypedName] = useState(suggestedName)
  const [fontIndex, setFontIndex] = useState(0)

  // Cambiar de pestaña invalida lo que hubiera: cada modalidad produce su propia
  // firma y mezclarlas dejaría enviada una que ya no se está viendo.
  const switchMode = (next: SignatureMode) => {
    if (next === mode) return
    setMode(next)
    setUploadError('')
    onChange('', next)
    if (next === 'drawn') setHasInk(false)
  }

  /* ===================== Trazo ===================== */

  // Ajusta el buffer del canvas a su tamaño real en pantalla. Cambiar width o
  // height LIMPIA el canvas, así que solo se hace cuando cambia de verdad.
  const resize = useCallback(() => {
    const canvas = canvasRef.current
    if (!canvas) return
    const rect = canvas.getBoundingClientRect()
    const dpr = window.devicePixelRatio || 1
    const w = Math.round(rect.width * dpr)
    const h = Math.round(rect.height * dpr)
    if (canvas.width === w && canvas.height === h) return

    canvas.width = w
    canvas.height = h
    const ctx = canvas.getContext('2d')
    if (!ctx) return
    ctx.scale(dpr, dpr)
    ctx.lineWidth = 2.2
    ctx.lineCap = 'round'
    ctx.lineJoin = 'round'
    ctx.strokeStyle = '#1a1330'

    setHasInk(false)
    onChange('', 'drawn')
  }, [onChange])

  useEffect(() => {
    if (mode !== 'drawn') return
    resize()
    window.addEventListener('resize', resize)
    return () => window.removeEventListener('resize', resize)
  }, [resize, mode])

  const pointAt = (e: React.PointerEvent<HTMLCanvasElement>) => {
    const rect = e.currentTarget.getBoundingClientRect()
    return { x: e.clientX - rect.left, y: e.clientY - rect.top }
  }

  const handleDown = (e: React.PointerEvent<HTMLCanvasElement>) => {
    if (disabled) return
    const ctx = canvasRef.current?.getContext('2d')
    if (!ctx) return
    // Capturar el puntero mantiene el trazo aunque el dedo salga del recuadro.
    e.currentTarget.setPointerCapture(e.pointerId)
    drawingRef.current = true

    const { x, y } = pointAt(e)
    ctx.beginPath()
    ctx.moveTo(x, y)
    // Un toque seco también deja marca.
    ctx.lineTo(x, y)
    ctx.stroke()
  }

  const handleMove = (e: React.PointerEvent<HTMLCanvasElement>) => {
    if (!drawingRef.current || disabled) return
    const ctx = canvasRef.current?.getContext('2d')
    if (!ctx) return
    const { x, y } = pointAt(e)
    ctx.lineTo(x, y)
    ctx.stroke()
  }

  const handleUp = (e: React.PointerEvent<HTMLCanvasElement>) => {
    if (!drawingRef.current) return
    drawingRef.current = false
    if (e.currentTarget.hasPointerCapture(e.pointerId)) {
      e.currentTarget.releasePointerCapture(e.pointerId)
    }
    const canvas = canvasRef.current
    if (!canvas) return
    setHasInk(true)
    onChange(canvas.toDataURL('image/png'), 'drawn')
  }

  const clearDrawing = () => {
    const canvas = canvasRef.current
    const ctx = canvas?.getContext('2d')
    if (!canvas || !ctx) return
    // El contexto está escalado por dpr: hay que limpiar el buffer completo.
    ctx.save()
    ctx.setTransform(1, 0, 0, 1, 0, 0)
    ctx.clearRect(0, 0, canvas.width, canvas.height)
    ctx.restore()
    setHasInk(false)
    onChange('', 'drawn')
  }

  /* ===================== Carga de una foto ===================== */

  const handleFile = async (file: File) => {
    setUploadError('')
    if (!ACCEPTED.includes(file.type)) {
      setUploadError('Solo aceptamos imágenes PNG, JPG o WEBP.')
      return
    }
    if (file.size > MAX_UPLOAD_BYTES) {
      setUploadError('La imagen pesa demasiado (máximo 8 MB).')
      return
    }

    setProcessing(true)
    try {
      const png = await imageToSignaturePNG(file)
      setUploaded(png)
      onChange(png, 'uploaded')
    } catch {
      setUploadError('No pudimos leer esa imagen. Prueba con otra.')
    } finally {
      setProcessing(false)
    }
  }

  const clearUpload = () => {
    setUploaded('')
    setUploadError('')
    if (fileRef.current) fileRef.current.value = ''
    onChange('', 'uploaded')
  }

  /* ===================== Nombre escrito ===================== */

  // Se redibuja al cambiar el nombre o la fuente. Espera a que la tipografía
  // esté cargada: si no, el canvas pinta con la de respaldo y la firma sale con
  // otra letra distinta de la que se está viendo en pantalla.
  useEffect(() => {
    if (mode !== 'typed') return
    const name = typedName.trim()
    if (!name) {
      onChange('', 'typed')
      return
    }
    let alive = true
    const font = FONTS[fontIndex]
    const spec = `${font.size}px ${font.css}`

    const paint = () => {
      if (!alive) return
      onChange(textToSignaturePNG(name, spec), 'typed')
    }

    if (document.fonts?.load) {
      document.fonts.load(spec, name).then(paint).catch(paint)
    } else {
      paint()
    }
    return () => {
      alive = false
    }
  }, [mode, typedName, fontIndex, onChange])

  /* ===================== Render ===================== */

  const tabs: { key: SignatureMode; label: string; icon: React.ReactNode }[] = [
    { key: 'drawn', label: 'Dibujar', icon: <PenLine size={15} /> },
    { key: 'uploaded', label: 'Subir imagen', icon: <Upload size={15} /> },
    { key: 'typed', label: 'Escribir', icon: <Type size={15} /> },
  ]

  return (
    <div className={styles.wrap}>
      <div className={styles.tabs} role="tablist" aria-label="Forma de firmar">
        {tabs.map((t) => (
          <button
            key={t.key}
            type="button"
            role="tab"
            aria-selected={mode === t.key}
            className={`${styles.tab} ${mode === t.key ? styles.tabOn : ''}`}
            onClick={() => switchMode(t.key)}
            disabled={disabled}
          >
            {t.icon} {t.label}
          </button>
        ))}
      </div>

      {mode === 'drawn' && (
        <>
          <div className={`${styles.pad} ${disabled ? styles.padDisabled : ''}`}>
            <canvas
              ref={canvasRef}
              className={styles.canvas}
              onPointerDown={handleDown}
              onPointerMove={handleMove}
              onPointerUp={handleUp}
              onPointerCancel={handleUp}
              aria-label="Recuadro para firmar"
            />
            {!hasInk && (
              <div className={styles.placeholder} aria-hidden>
                <PenLine size={18} />
                <span>Firma aquí</span>
              </div>
            )}
            <div className={styles.baseline} aria-hidden />
          </div>

          <div className={styles.footer}>
            <span className={styles.hint}>
              {hint ?? 'Puedes firmar con el ratón, el dedo o un lápiz.'}
            </span>
            <button
              type="button"
              className={styles.clearBtn}
              onClick={clearDrawing}
              disabled={disabled || !hasInk}
            >
              <Eraser size={14} /> Limpiar
            </button>
          </div>
        </>
      )}

      {mode === 'uploaded' && (
        <>
          <div className={`${styles.pad} ${disabled ? styles.padDisabled : ''}`}>
            {uploaded ? (
              <img src={uploaded} alt="Firma cargada" className={styles.preview} />
            ) : (
              <button
                type="button"
                className={styles.dropZone}
                onClick={() => fileRef.current?.click()}
                disabled={disabled || processing}
              >
                <Upload size={20} />
                <span>{processing ? 'Procesando...' : 'Elegir una imagen de tu firma'}</span>
                <span className={styles.dropHint}>PNG, JPG o WEBP · hasta 8 MB</span>
              </button>
            )}
            <input
              ref={fileRef}
              type="file"
              accept={ACCEPTED.join(',')}
              className={styles.fileInput}
              onChange={(e) => {
                const f = e.target.files?.[0]
                if (f) void handleFile(f)
              }}
            />
          </div>

          {uploadError && <p className={styles.error}>{uploadError}</p>}

          <div className={styles.footer}>
            <span className={styles.hint}>
              {hint ?? 'Sube una foto o un escaneo de tu firma sobre papel blanco.'}
            </span>
            {uploaded && (
              <button type="button" className={styles.clearBtn} onClick={clearUpload} disabled={disabled}>
                <Eraser size={14} /> Quitar
              </button>
            )}
          </div>
        </>
      )}

      {mode === 'typed' && (
        <>
          <input
            className={styles.typedInput}
            value={typedName}
            onChange={(e) => setTypedName(e.target.value)}
            placeholder="Escribe tu nombre"
            disabled={disabled}
            aria-label="Nombre para la firma"
          />

          <div className={styles.fontList}>
            {FONTS.map((f, i) => (
              <button
                key={f.label}
                type="button"
                className={`${styles.fontOption} ${fontIndex === i ? styles.fontOn : ''}`}
                style={{ fontFamily: f.css }}
                onClick={() => setFontIndex(i)}
                disabled={disabled}
                aria-pressed={fontIndex === i}
                title={f.label}
              >
                {typedName.trim() || 'Tu nombre'}
              </button>
            ))}
          </div>

          <p className={styles.hint}>
            {hint ??
              'Se guarda como imagen, igual que un trazo. La constancia deja claro que la escribiste.'}
          </p>
        </>
      )}
    </div>
  )
}

/* ===================== Normalización a PNG ===================== */

/**
 * Convierte la imagen elegida en un PNG que quepa en la caja de firma y por
 * debajo del tope del servidor.
 *
 * Reduce hasta caber en lugar de rechazar: una foto de móvil de 4000 px no cabe
 * ni de lejos, y decirle a alguien "tu firma pesa mucho" cuando el navegador
 * puede arreglarlo solo es trasladarle un problema nuestro.
 */
async function imageToSignaturePNG(file: File): Promise<string> {
  const img = await loadImage(URL.createObjectURL(file))
  let scale = Math.min(BOX_W / img.width, BOX_H / img.height, 1)

  for (let intento = 0; intento < 6; intento++) {
    const w = Math.max(1, Math.round(img.width * scale))
    const h = Math.max(1, Math.round(img.height * scale))
    const canvas = document.createElement('canvas')
    canvas.width = w
    canvas.height = h
    const ctx = canvas.getContext('2d')
    if (!ctx) throw new Error('sin canvas')
    ctx.drawImage(img, 0, 0, w, h)
    whiteToTransparent(ctx, w, h)

    const url = canvas.toDataURL('image/png')
    if (dataURLBytes(url) <= MAX_OUTPUT_BYTES) {
      URL.revokeObjectURL(img.src)
      return url
    }
    scale *= 0.8
  }
  URL.revokeObjectURL(img.src)
  throw new Error('no se pudo reducir lo suficiente')
}

/**
 * Vuelve transparente el papel blanco de una foto.
 *
 * El umbral es alto (245) a propósito: solo se va el blanco casi puro. Bajarlo
 * se comería los trazos claros de un bolígrafo flojo y dejaría la firma con
 * agujeros, que es peor que un fondo con un punto de gris.
 */
function whiteToTransparent(ctx: CanvasRenderingContext2D, w: number, h: number) {
  try {
    const data = ctx.getImageData(0, 0, w, h)
    const px = data.data
    for (let i = 0; i < px.length; i += 4) {
      if (px[i] >= 245 && px[i + 1] >= 245 && px[i + 2] >= 245) px[i + 3] = 0
    }
    ctx.putImageData(data, 0, 0)
  } catch {
    // getImageData puede fallar por seguridad con imágenes de otro origen. No
    // es motivo para perder la firma: se queda con su fondo y ya está.
  }
}

/** Dibuja el nombre con la tipografía elegida y lo devuelve como PNG. */
function textToSignaturePNG(name: string, fontSpec: string): string {
  const canvas = document.createElement('canvas')
  canvas.width = BOX_W
  canvas.height = BOX_H
  const ctx = canvas.getContext('2d')
  if (!ctx) return ''

  ctx.font = fontSpec
  ctx.fillStyle = '#1a1330'
  ctx.textAlign = 'center'
  ctx.textBaseline = 'middle'

  // Un nombre largo no puede salirse de la caja: se estrecha hasta caber.
  const maxWidth = BOX_W - 40
  const measured = ctx.measureText(name).width
  if (measured > maxWidth) {
    ctx.setTransform(maxWidth / measured, 0, 0, 1, 0, 0)
    ctx.fillText(name, (BOX_W / 2) * (measured / maxWidth), BOX_H / 2)
  } else {
    ctx.fillText(name, BOX_W / 2, BOX_H / 2)
  }
  return canvas.toDataURL('image/png')
}

function loadImage(src: string): Promise<HTMLImageElement> {
  return new Promise((resolve, reject) => {
    const img = new Image()
    img.onload = () => resolve(img)
    img.onerror = reject
    img.src = src
  })
}
