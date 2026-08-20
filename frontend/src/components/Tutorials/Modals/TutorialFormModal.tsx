import { useMemo, useRef, useState } from 'react'
import { Check, AlertCircle, PlayCircle, HardDrive, Video, Image as ImageIcon, FileText, Upload, Trash2, Users, BellRing, Layers, Eye, MousePointerClick, Repeat } from 'lucide-react'
import { TUTORIAL_ICON_NAMES, TutorialIcon } from '../icons'
import { parseVideoUrl, getProviderLabel } from '../utils'
import { useDirtySnapshot } from '../../ui/useCloseGuard'
import { Modal, Button, Select, toISODate } from '../../ui'
import { RichTextEditor } from '../../Tasks/RichTextEditor'
import { TargetPicker } from './TargetPicker'
import { DateTimeField } from './DateTimeField'
import { NovedadOverlay } from '../NovedadOverlay'
import { uploadService } from '../../../services/api'
import { useNotification } from '../../../context/NotificationContext'
import type { CreateTutorialInput, Tutorial, TutorialAudience, TutorialContentType } from '../../../types'
import styles from '../../../pages/Tutoriales.module.css'

/**
 * Cuánto insiste el aviso emergente. Se ofrecen plazos cerrados en vez de un
 * número libre: la decisión real es "qué tan urgente es", no un día más o uno
 * menos, y el 0 tiene que estar a la vista para poder publicar sin interrumpir.
 */
const ANNOUNCE_DAYS_OPTIONS = [
  { value: 0, label: 'Sin aviso emergente (solo notificación)' },
  { value: 1, label: '1 día' },
  { value: 2, label: '2 días' },
  { value: 3, label: '3 días' },
  { value: 7, label: '1 semana' },
  { value: 15, label: '15 días' },
  { value: 30, label: '30 días' },
]

/**
 * Destinos del boton de accion. Se ofrecen las pantallas reales de Obertrack en
 * vez de pedir una ruta escrita a mano: quien publica una novedad no tiene por
 * que saberse los caminos de la aplicacion, y una ruta mal escrita es un boton
 * que lleva a un error delante de toda la empresa.
 */
const CTA_EXTERNAL = '__external__'

const CTA_DESTINATIONS = [
  { value: '', label: 'Sin botón de acción' },
  { value: '/dashboard', label: 'Panel principal' },
  { value: '/tasks', label: 'Tareas' },
  { value: '/work-hours', label: 'Horas' },
  { value: '/chat', label: 'Chat' },
  { value: '/organigrama', label: 'Organigrama' },
  { value: '/empresa', label: 'Profesionales (empresas)' },
  { value: '/reports', label: 'Reportes (empresas)' },
  { value: '/novedades', label: 'Novedades' },
  { value: '/soporte', label: 'Soporte' },
  { value: '/profile', label: 'Mi perfil' },
  { value: CTA_EXTERNAL, label: 'Otro enlace…' },
]

const AUDIENCE_OPTIONS = [
  { value: 'all', label: 'Todos (empresas y profesionales)' },
  { value: 'empleador', label: 'Empresas' },
  { value: 'profesional', label: 'Profesionales' },
  { value: 'manager', label: 'Managers (con equipo a cargo)' },
]

/**
 * Veces que puede aparecer el aviso a una misma persona. Es un tope aparte del
 * plazo en días: uno corta por tiempo y el otro por insistencia. Manda el que
 * se cumpla primero.
 */
const ANNOUNCE_SHOWS_OPTIONS = [
  { value: 0, label: 'Sin límite de veces' },
  { value: 1, label: '1 vez' },
  { value: 2, label: '2 veces' },
  { value: 3, label: '3 veces' },
  { value: 5, label: '5 veces' },
  { value: 10, label: '10 veces' },
]

/** Los tres formatos que puede tener una novedad, en el orden del selector. */
const CONTENT_TYPES: { value: TutorialContentType; label: string; hint: string; icon: typeof Video }[] = [
  { value: 'video', label: 'Video', hint: 'Drive o YouTube', icon: Video },
  { value: 'imagen', label: 'Imagen', hint: 'Flyer o captura', icon: ImageIcon },
  { value: 'texto', label: 'Texto', hint: 'Con formato e imágenes', icon: FileText },
]

/** Hoy en AAAA-MM-DD: el suelo de las fechas programables. */
function today(): string {
  return toISODate(new Date())
}

interface TutorialFormModalProps {
  isOpen: boolean
  isEditing: boolean
  isSaving: boolean
  formData: CreateTutorialInput
  setFormData: React.Dispatch<React.SetStateAction<CreateTutorialInput>>
  availableCategories: string[]
  onClose: () => void
  onSubmit: (e: React.FormEvent) => void
}

export function TutorialFormModal({
  isOpen,
  isEditing,
  isSaving,
  formData,
  setFormData,
  availableCategories,
  onClose,
  onSubmit,
}: TutorialFormModalProps) {
  const { error } = useNotification()
  const fileInputRef = useRef<HTMLInputElement>(null)
  const [isUploading, setIsUploading] = useState(false)
  const [previewing, setPreviewing] = useState(false)
  // Enlace externo: se recuerda aparte porque la lista no puede representarlo.
  const [externalCTA, setExternalCTA] = useState(
    () => !!formData.cta_url && !CTA_DESTINATIONS.some(d => d.value === formData.cta_url),
  )
  // Casi todas las novedades se publican al guardarlas: las fechas solo
  // aparecen cuando alguien las pide.
  const [scheduling, setScheduling] = useState(() => !!formData.publish_at || !!formData.expires_at)

  const isVideo = formData.content_type === 'video'
  const urlInfo = useMemo(() => parseVideoUrl(formData.google_drive_url), [formData.google_drive_url])
  const urlState: 'idle' | 'valid' | 'invalid' =
    !formData.google_drive_url.trim() ? 'idle' : urlInfo ? 'valid' : 'invalid'

  const field = styles['tutorial-form-field']
  const half = styles['tutorial-form-half']

  const handlePickImage = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    // El input se limpia siempre: si no, volver a elegir el mismo archivo tras
    // un error no dispara el evento.
    e.target.value = ''
    if (!file) return
    if (!file.type.startsWith('image/')) {
      error('El archivo debe ser una imagen')
      return
    }
    setIsUploading(true)
    try {
      const { url } = await uploadService.upload(file)
      setFormData(prev => ({ ...prev, image_url: url }))
    } catch {
      error('No se pudo subir la imagen')
    } finally {
      setIsUploading(false)
    }
  }

  return (
    <Modal
      isDirty={useDirtySnapshot(formData)}
      isOpen={isOpen}
      onClose={onClose}
      title={isEditing ? 'Editar novedad' : 'Nueva novedad'}
      size="lg"
      footer={
        <>
          {/* Previsualizar antes de publicar: hasta ahora el aviso se estrenaba
              delante de toda la empresa sin que nadie lo hubiera visto. */}
          <Button variant="secondary" onClick={() => setPreviewing(true)} disabled={isSaving}>
            <Eye size={15} /> Previsualizar
          </Button>
          <Button variant="secondary" onClick={onClose} disabled={isSaving}>Cancelar</Button>
          <Button type="submit" form="tutorial-form" loading={isSaving}>
            {isEditing ? 'Guardar cambios' : 'Crear novedad'}
          </Button>
        </>
      }
    >
      {/* El formulario va en dos columnas: los campos cortos se emparejan y solo
          el contenido ocupa el ancho completo. Antes era una sola columna larga
          en la que había que desplazarse para ver de qué trataba la novedad. */}
      <form onSubmit={onSubmit} id="tutorial-form" className={styles['tutorial-form-body']}>
        {/* El formato va primero porque decide qué se pide después. */}
        <div className={styles['tutorial-form-section']}>Contenido</div>

        <div className={field}>
          <div className={styles['tutorial-type-grid']}>
            {CONTENT_TYPES.map(({ value, label, hint, icon: Icon }) => (
              <button
                key={value}
                type="button"
                className={`${styles['tutorial-type-option']} ${formData.content_type === value ? styles['selected'] : ''}`}
                onClick={() => setFormData({ ...formData, content_type: value })}
              >
                <Icon size={18} />
                <strong>{label}</strong>
                <small>{hint}</small>
              </button>
            ))}
          </div>
        </div>

        {isVideo && (
          <div className={field}>
            <label>Link del video</label>
            <div className={`${styles['tutorial-url-input-wrapper']} ${styles[`url-${urlState}`]}`}>
              <input
                type="url"
                value={formData.google_drive_url}
                onChange={(e) => setFormData({ ...formData, google_drive_url: e.target.value })}
                placeholder="https://drive.google.com/file/d/... o https://youtu.be/..."
              />
              {urlState === 'valid' && urlInfo && (
                <span className={`${styles['tutorial-url-badge']} ${styles[`provider-${urlInfo.provider}`]}`}>
                  {urlInfo.provider === 'youtube' ? <PlayCircle size={14} /> : <HardDrive size={14} />}
                  {getProviderLabel(urlInfo.provider)}
                  <Check size={14} />
                </span>
              )}
              {urlState === 'invalid' && (
                <span className={`${styles['tutorial-url-badge']} ${styles['provider-invalid']}`}>
                  <AlertCircle size={14} />
                  No válido
                </span>
              )}
            </div>
            <small className={styles['tutorial-form-hint']}>
              Solo <strong>Google Drive</strong> (<code>/file/d/{'{ID}'}/</code>) o <strong>YouTube</strong>, públicos o no listados.
            </small>
          </div>
        )}

        {formData.content_type === 'imagen' && (
          <div className={field}>
            <label>Imagen</label>
            {formData.image_url ? (
              <div className={styles['tutorial-image-preview']}>
                <img src={formData.image_url} alt="Vista previa de la novedad" />
                <div className={styles['tutorial-image-preview-actions']}>
                  <Button variant="secondary" onClick={() => fileInputRef.current?.click()} disabled={isUploading}>
                    Cambiar
                  </Button>
                  <Button variant="secondary" onClick={() => setFormData({ ...formData, image_url: '' })}>
                    <Trash2 size={15} /> Quitar
                  </Button>
                </div>
              </div>
            ) : (
              <button
                type="button"
                className={styles['tutorial-image-drop']}
                onClick={() => fileInputRef.current?.click()}
                disabled={isUploading}
              >
                <Upload size={20} />
                <strong>{isUploading ? 'Subiendo…' : 'Subir imagen'}</strong>
                <small>PNG, JPG o WebP</small>
              </button>
            )}
            <input
              ref={fileInputRef}
              type="file"
              accept="image/*"
              onChange={handlePickImage}
              style={{ display: 'none' }}
            />
          </div>
        )}

        {formData.content_type === 'texto' && (
          <div className={field}>
            <label>Texto de la novedad</label>
            <RichTextEditor
              value={formData.body}
              onChange={(body) => setFormData(prev => ({ ...prev, body }))}
              placeholder="Escribe el anuncio. Puedes pegar imágenes y capturas directamente."
            />
          </div>
        )}

        <div className={field}>
          <label>Título</label>
          <input
            type="text"
            value={formData.title}
            onChange={(e) => setFormData({ ...formData, title: e.target.value })}
            placeholder="Ej: Nuevo flujo para registrar horas"
            required
            autoFocus
          />
        </div>

        <div className={field}>
          <label>Descripción</label>
          <textarea
            value={formData.description}
            onChange={(e) => setFormData({ ...formData, description: e.target.value })}
            placeholder="Resumen breve. Es lo que se lee en la tarjeta y en la notificación."
            rows={2}
          />
        </div>

        <div className={styles['tutorial-form-section']}>Publicación</div>

        <div className={`${field} ${isVideo ? half : ''}`}>
          <label>Categoría</label>
          <input
            type="text"
            list="tutorial-categories"
            value={formData.category}
            onChange={(e) => setFormData({ ...formData, category: e.target.value })}
            placeholder="Ej: Onboarding, Tareas..."
          />
          <datalist id="tutorial-categories">
            {availableCategories.map((cat) => (
              <option key={cat} value={cat} />
            ))}
          </datalist>
        </div>

        {isVideo && (
          <div className={`${field} ${half}`}>
            <label>Duración (min)</label>
            <input
              type="number"
              min={0}
              value={formData.duration_min}
              onChange={(e) => setFormData({ ...formData, duration_min: Number(e.target.value) })}
            />
          </div>
        )}

        {/* Tipo de cuenta y publico objetivo son UNA sola decision —a quien le
            llega—, y el contador del final es su resultado. Separados en dos
            campos sueltos no se leian como lo que son. */}
        <div className={`${field} ${styles['tutorial-audience-card']}`}>
          <div className={styles['tutorial-audience-card-head']}>
            <Users size={16} />
            <strong>¿A quién le llega?</strong>
          </div>

          <div className={styles['tutorial-audience-card-row']}>
            <Select
              options={AUDIENCE_OPTIONS}
              value={formData.audience}
              onChange={(value) => setFormData({ ...formData, audience: value as TutorialAudience })}
              leftIcon={<Layers size={15} />}
              fullWidth
              ariaLabel="Tipo de cuenta al que va dirigida la novedad"
            />
          </div>

          <TargetPicker
            audience={formData.audience}
            value={formData.target}
            onChange={(target) => setFormData(prev => ({ ...prev, target }))}
          />
        </div>

        <div className={`${field} ${half}`}>
          <label>Aviso al iniciar sesión</label>
          <Select
            options={ANNOUNCE_DAYS_OPTIONS}
            value={formData.announce_days}
            onChange={(value) => setFormData({ ...formData, announce_days: Number(value) })}
            leftIcon={<BellRing size={15} />}
            fullWidth
            ariaLabel="Días que el aviso emergente insiste con la novedad"
          />
          <small className={styles['tutorial-form-hint']}>
            Sale a pantalla completa hasta que la persona lo cierra, o hasta cumplirse el plazo.
          </small>
        </div>

        {formData.announce_days > 0 && (
          <div className={`${field} ${half}`}>
            <label>Veces que aparece</label>
            <Select
              options={ANNOUNCE_SHOWS_OPTIONS}
              value={formData.announce_max_shows}
              onChange={(value) => setFormData({ ...formData, announce_max_shows: Number(value) })}
              leftIcon={<Repeat size={15} />}
              fullWidth
              ariaLabel="Veces que el aviso puede aparecerle a una misma persona"
            />
            <small className={styles['tutorial-form-hint']}>
              Tope por persona. Deja de salir con lo que ocurra primero: el plazo o estas veces.
            </small>
          </div>
        )}

        <div className={`${field} ${half}`}>
          <label>Botón de acción</label>
          <Select
            options={CTA_DESTINATIONS}
            value={externalCTA ? CTA_EXTERNAL : formData.cta_url}
            onChange={(value) => {
              const destination = String(value)
              if (destination === CTA_EXTERNAL) {
                setExternalCTA(true)
                setFormData({ ...formData, cta_url: '' })
                return
              }
              setExternalCTA(false)
              // Elegir a dónde lleva ya sugiere cómo llamarlo: quien no quiera
              // el texto propuesto lo cambia, pero nadie parte de un campo
              // vacío que no sabe cómo rellenar.
              const suggested = CTA_DESTINATIONS.find(d => d.value === destination)?.label ?? ''
              setFormData({
                ...formData,
                cta_url: destination,
                cta_label: destination === ''
                  ? ''
                  : formData.cta_label || `Ir a ${suggested.replace(/ \(.*\)$/, '')}`,
              })
            }}
            leftIcon={<MousePointerClick size={15} />}
            fullWidth
            ariaLabel="Destino del botón de acción de la novedad"
          />
          <small className={styles['tutorial-form-hint']}>
            Opcional. Es lo que convierte la novedad en algo que se hace, y su clic se mide.
          </small>
        </div>

        {(!!formData.cta_url || externalCTA) && (
          <div className={`${field} ${half}`}>
            <label>Texto del botón</label>
            <input
              type="text"
              value={formData.cta_label}
              onChange={(e) => setFormData({ ...formData, cta_label: e.target.value })}
              placeholder="Ej: Registrar mis horas"
            />
          </div>
        )}

        {externalCTA && (
          <div className={field}>
            <label>Enlace externo</label>
            <input
              type="url"
              value={formData.cta_url}
              onChange={(e) => setFormData({ ...formData, cta_url: e.target.value })}
              placeholder="https://..."
            />
          </div>
        )}

        <label className={styles['tutorial-form-toggle']}>
          <input
            type="checkbox"
            checked={scheduling}
            onChange={(e) => {
              setScheduling(e.target.checked)
              // Al apagarlo se limpian las fechas: dejarlas guardadas y
              // ocultas publicaría sola una novedad que se creía inmediata.
              if (!e.target.checked) setFormData({ ...formData, publish_at: null, expires_at: null })
            }}
          />
          <span>
            Programar la publicación
            <small>Sin esto se publica al guardarla.</small>
          </span>
        </label>

        {scheduling && (
          <>
            <div className={`${field} ${half}`}>
              <label>Publicar el</label>
              <DateTimeField
                value={formData.publish_at}
                onChange={(publish_at) => setFormData({ ...formData, publish_at })}
                minDate={today()}
                ariaLabel="Fecha de publicación"
              />
            </div>

            <div className={`${field} ${half}`}>
              <label>Retirar el (opcional)</label>
              <DateTimeField
                value={formData.expires_at}
                onChange={(expires_at) => setFormData({ ...formData, expires_at })}
                // No se puede retirar antes de publicar: el calendario ni
                // siquiera ofrece esos días.
                minDate={formData.publish_at ? toISODate(new Date(formData.publish_at)) : today()}
                ariaLabel="Fecha de retiro"
              />
            </div>
          </>
        )}

        <label className={styles['tutorial-form-toggle']}>
          <input
            type="checkbox"
            checked={formData.require_ack}
            onChange={(e) => setFormData({ ...formData, require_ack: e.target.checked })}
          />
          <span>
            Exigir confirmación de lectura
            <small>El aviso no se puede cerrar sin pulsar "Confirmo que lo leí", y queda registrado quién lo hizo.</small>
          </span>
        </label>

        <div className={field}>
          <label>Icono de la tarjeta</label>
          <div className={styles['tutorial-icon-grid']}>
            {TUTORIAL_ICON_NAMES.map((name) => (
              <button
                key={name}
                type="button"
                className={`${styles['tutorial-icon-option']} ${formData.icon_name === name ? styles['selected'] : ''}`}
                onClick={() => setFormData({ ...formData, icon_name: name })}
                title={name}
              >
                <TutorialIcon name={name} size={20} />
              </button>
            ))}
          </div>
        </div>

        <label className={styles['tutorial-form-toggle']}>
          <input
            type="checkbox"
            checked={formData.is_active}
            onChange={(e) => setFormData({ ...formData, is_active: e.target.checked })}
          />
          <span>
            Visible para todos los usuarios
            <small>Al activarla se publica: se reparten las notificaciones y empieza el aviso.</small>
          </span>
        </label>
      </form>

      {previewing && (
        <NovedadOverlay
          // El formulario no es una novedad guardada: se arma una de mentira
          // con lo que hay escrito para verla exactamente como saldrá.
          tutorial={{ ...formData, id: 0, created_by: 0, created_at: '', updated_at: '' } as unknown as Tutorial}
          onDismiss={() => setPreviewing(false)}
          preview
        />
      )}
    </Modal>
  )
}
