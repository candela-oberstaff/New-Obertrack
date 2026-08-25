import { useEffect, useMemo, useState } from 'react'
import { AlertCircle, Lock, Paperclip } from 'lucide-react'
import type { TaskGateField, TaskGateRequirement } from '../../types'
import { uploadService } from '../../services/upload.service'
import { Select } from '../ui/Select'
import styles from './GateModal.module.css'

// El formulario de una puerta de fase.
//
// No conoce ninguna puerta: dibuja lo que el servidor le manda en el 422. Esa es la
// razón de que una puerta recién creada funcione sin desplegar frontend, y también
// de que este componente no valide reglas de negocio — sólo lo justo para no enviar
// un formulario obviamente vacío. La validación que cuenta es la del servidor, que
// vuelve a correr aunque alguien manipule esto.

interface GateModalProps {
  requirement: TaskGateRequirement
  onSubmit: (form: Record<string, unknown>) => Promise<void>
  onCancel: () => void
}

export function GateModal({ requirement, onSubmit, onCancel }: GateModalProps) {
  const { form, errors } = requirement
  const [values, setValues] = useState<Record<string, unknown>>({})
  const [sending, setSending] = useState(false)
  const [uploading, setUploading] = useState<string | null>(null)
  const [localError, setLocalError] = useState('')

  // Cerrar con Escape, igual que el resto de modales de la aplicación.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && !sending) onCancel()
    }
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [onCancel, sending])

  // Los campos obligatorios sin rellenar deshabilitan el envío. Es cortesía, no
  // seguridad: el servidor los exige igual.
  const faltan = useMemo(
    () => form.fields.some((f) => f.required && !String(values[f.key] ?? '').trim()),
    [form.fields, values],
  )

  const set = (key: string, value: unknown) => setValues((prev) => ({ ...prev, [key]: value }))

  const handleFile = async (field: TaskGateField, file: File | undefined) => {
    if (!file) return
    setUploading(field.key)
    setLocalError('')
    try {
      // El archivo se sube ANTES de enviar el formulario y lo que viaja es su URL.
      // Así el movimiento de la tarjeta no tiene que ser transaccional con un
      // fichero: cuando la puerta se evalúa, la subida ya terminó.
      const { url } = await uploadService.upload(file)
      set(field.key, url)
    } catch {
      setLocalError('No se pudo subir el archivo. Inténtalo de nuevo.')
    } finally {
      setUploading(null)
    }
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setSending(true)
    setLocalError('')
    try {
      await onSubmit(values)
    } catch {
      setLocalError('No se pudo completar el movimiento. Inténtalo de nuevo.')
    } finally {
      setSending(false)
    }
  }

  const renderField = (f: TaskGateField) => {
    const value = values[f.key] ?? ''
    const invalid = !!errors?.[f.key]
    const common = {
      id: `gate-${f.key}`,
      className: `${styles.input} ${invalid ? styles.inputError : ''}`,
      disabled: sending,
    }

    switch (f.type) {
      case 'textarea':
        return (
          <textarea
            {...common}
            rows={3}
            placeholder={f.placeholder}
            maxLength={f.max_length || undefined}
            value={String(value)}
            onChange={(e) => set(f.key, e.target.value)}
          />
        )
      case 'select':
        // El desplegable propio de la aplicación y no el nativo: el nativo lo pinta
        // el sistema operativo —azul de Windows, tipografía ajena— y en un modal que
        // bloquea el trabajo, parecer una pantalla de otro programa no ayuda a que la
        // gente lo rellene con confianza.
        return (
          <div className={invalid ? styles.selectError : undefined}>
            <Select
              value={String(value)}
              onChange={(v) => set(f.key, String(v))}
              options={(f.options ?? []).map((o) => ({ value: o.value, label: o.label }))}
              placeholder="Selecciona…"
              disabled={sending}
              ariaLabel={f.label}
              fullWidth
            />
          </div>
        )
      case 'file':
        return (
          <div className={styles.file}>
            <label className={styles.fileButton}>
              <Paperclip size={14} />
              {uploading === f.key ? 'Subiendo…' : 'Elegir archivo'}
              <input
                type="file"
                hidden
                disabled={sending || uploading === f.key}
                onChange={(e) => handleFile(f, e.target.files?.[0])}
              />
            </label>
            {!!value && (
              <a className={styles.fileLink} href={String(value)} target="_blank" rel="noreferrer">
                Ver archivo adjunto
              </a>
            )}
          </div>
        )
      case 'date':
        return (
          <input {...common} type="date" value={String(value)} onChange={(e) => set(f.key, e.target.value)} />
        )
      case 'number':
        return (
          <input
            {...common}
            type="number"
            min={f.min}
            max={f.max}
            value={String(value)}
            onChange={(e) => set(f.key, e.target.value)}
          />
        )
      case 'url':
        return (
          <input
            {...common}
            type="url"
            placeholder={f.placeholder || 'https://…'}
            value={String(value)}
            onChange={(e) => set(f.key, e.target.value)}
          />
        )
      default:
        return (
          <input
            {...common}
            type="text"
            placeholder={f.placeholder}
            maxLength={f.max_length || undefined}
            value={String(value)}
            onChange={(e) => set(f.key, e.target.value)}
          />
        )
    }
  }

  return (
    <div className={styles.overlay} role="dialog" aria-modal="true" aria-labelledby="gate-title">
      <form className={styles.modal} onSubmit={handleSubmit}>
        <header className={styles.header}>
          <span className={styles.badge}><Lock size={13} /> Paso obligatorio</span>
          <h2 id="gate-title">{form.title}</h2>
          {form.description && <p className={styles.description}>{form.description}</p>}
        </header>

        <div className={styles.body}>
          {form.fields.map((f) => (
            <div key={f.key} className={styles.field}>
              <label htmlFor={`gate-${f.key}`}>
                {f.label}
                {f.required && <span className={styles.required} aria-hidden="true"> *</span>}
              </label>
              {renderField(f)}
              {/* El mensaje del servidor gana al de ayuda: si hay corrección que
                  hacer, es lo único que interesa leer en ese momento. */}
              {errors?.[f.key]
                ? <span className={styles.error}>{errors[f.key]}</span>
                : f.help && <span className={styles.help}>{f.help}</span>}
            </div>
          ))}
        </div>

        {localError && (
          <div className={styles.banner} role="alert">
            <AlertCircle size={15} /> {localError}
          </div>
        )}

        <footer className={styles.footer}>
          <button type="button" className={styles.cancel} onClick={onCancel} disabled={sending}>
            Cancelar
          </button>
          <button type="submit" className={styles.submit} disabled={sending || faltan || !!uploading}>
            {sending ? 'Guardando…' : 'Continuar'}
          </button>
        </footer>
      </form>
    </div>
  )
}
