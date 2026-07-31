import { useCallback, useState } from 'react'

/**
 * Extrae las imágenes de un evento de pegado o de arrastrar-y-soltar.
 *
 * Se mira `kind === 'file'` y no solo el tipo: al copiar una imagen desde una
 * página web el portapapeles trae además su HTML y su URL como texto, y sin
 * este filtro se intentaría subir una cadena.
 */
function imagesFrom(data: DataTransfer | null | undefined): File[] {
  if (!data) return []
  const out: File[] = []
  // items al pegar (el portapapeles no rellena `files` de forma fiable en todos
  // los navegadores), files al soltar.
  if (data.items?.length) {
    for (let i = 0; i < data.items.length; i++) {
      const it = data.items[i]
      if (it.kind === 'file' && it.type.startsWith('image/')) {
        const f = it.getAsFile()
        if (f) out.push(f)
      }
    }
  }
  if (out.length === 0 && data.files?.length) {
    for (let i = 0; i < data.files.length; i++) {
      const f = data.files[i]
      if (f.type.startsWith('image/')) out.push(f)
    }
  }
  return out
}

interface UseImagePasteReturn {
  /** Engánchalo al onPaste del textarea o del contenedor. */
  onPaste: (e: React.ClipboardEvent) => void
  /** Engánchalo al onDrop para soltar imágenes encima. */
  onDrop: (e: React.DragEvent) => void
  /** Hay imágenes subiéndose ahora mismo. */
  isUploading: boolean
}

/**
 * Pegar (o soltar) imágenes y entregárselas a quien sepa qué hacer con ellas.
 *
 * Vive aquí y no dentro de un componente porque lo necesitan dos sitios que no
 * se parecen en nada: el editor de descripciones de tarea, que las inserta en
 * línea, y el expediente de empresa, que las cuelga como adjuntos. Lo común es
 * sacarlas del portapapeles; qué se hace con ellas no lo es.
 *
 * `onFiles` recibe TODAS las imágenes del pegado de una vez (se puede pegar más
 * de una), para que quien las reciba decida si las sube en serie o en paralelo.
 */
export function useImagePaste(onFiles: (files: File[]) => Promise<void> | void): UseImagePasteReturn {
  const [isUploading, setIsUploading] = useState(false)

  const handle = useCallback(
    async (files: File[], preventDefault: () => void) => {
      if (files.length === 0) return // pegado normal: texto, que siga su curso
      preventDefault()
      setIsUploading(true)
      try {
        await onFiles(files)
      } finally {
        setIsUploading(false)
      }
    },
    [onFiles],
  )

  const onPaste = useCallback(
    (e: React.ClipboardEvent) => {
      void handle(imagesFrom(e.clipboardData), () => e.preventDefault())
    },
    [handle],
  )

  const onDrop = useCallback(
    (e: React.DragEvent) => {
      void handle(imagesFrom(e.dataTransfer), () => e.preventDefault())
    },
    [handle],
  )

  return { onPaste, onDrop, isUploading }
}
