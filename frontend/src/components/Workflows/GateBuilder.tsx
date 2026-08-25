import { useMemo, useState } from 'react'
import { Plus, Trash2, ChevronUp, ChevronDown, AlertCircle, Eye, Lock } from 'lucide-react'
import {
  workflowService,
  GATE_LIMITS,
  type BoardGate,
  type GateField,
  type GateFieldType,
  type GateInput,
} from '../../services/workflow.service'
import { Select } from '../ui/Select'
import styles from './GateBuilder.module.css'

// Constructor de puertas: define QUÉ se pregunta antes de dejar entrar una tarjeta en
// una columna.
//
// Es la única pantalla del producto que permite bloquear el trabajo de otra persona,
// así que está escrita alrededor de esa idea: se ve todo el tiempo lo que va a ver
// quien mueva la tarjeta, y los errores se explican donde se cometen —el servidor
// vuelve a validarlo todo, pero enterarse al guardar es enterarse tarde.

const TIPOS: { value: GateFieldType; label: string; hint: string }[] = [
  { value: 'text', label: 'Texto corto', hint: 'Una línea: un número de expediente, un nombre.' },
  { value: 'textarea', label: 'Texto largo', hint: 'Varias líneas: un resumen, un motivo.' },
  { value: 'select', label: 'Lista de opciones', hint: 'Elegir una de varias respuestas fijas.' },
  { value: 'url', label: 'Enlace', hint: 'Se exige que empiece por http:// o https://' },
  { value: 'file', label: 'Archivo', hint: 'Se sube antes de mover la tarjeta.' },
  { value: 'date', label: 'Fecha', hint: '' },
  { value: 'number', label: 'Número', hint: 'Admite mínimo y máximo.' },
]

interface GateBuilderProps {
  boardId: number
  phases: { id: number; name: string }[]
  /** La puerta que se edita; ausente = una nueva. */
  gate?: BoardGate
  /** Columnas que ya tienen punto de control: una columna sólo admite uno. */
  takenPhases: { id: number; by: string }[]
  onSaved: () => void
  onCancel: () => void
}

/** Deriva una clave a partir de la etiqueta: es lo que se guarda en el historial. */
function claveDe(label: string, usadas: string[]): string {
  const base =
    label
      .toLowerCase()
      .normalize('NFD')
      .replace(/[̀-ͯ]/g, '')
      .replace(/[^a-z0-9]+/g, '_')
      .replace(/^_+|_+$/g, '')
      .slice(0, GATE_LIMITS.key) || 'campo'
  // El servidor exige que empiece por letra.
  const limpia = /^[a-z]/.test(base) ? base : `campo_${base}`.slice(0, GATE_LIMITS.key)
  if (!usadas.includes(limpia)) return limpia
  for (let i = 2; i < 50; i++) {
    const alt = `${limpia}_${i}`.slice(0, GATE_LIMITS.key)
    if (!usadas.includes(alt)) return alt
  }
  return `${limpia}_x`
}

const campoNuevo = (usadas: string[]): GateField => ({
  key: claveDe('campo', usadas),
  label: '',
  type: 'text',
  required: true,
})

export function GateBuilder({ boardId, phases, gate, takenPhases, onSaved, onCancel }: GateBuilderProps) {
  const [name, setName] = useState(gate?.name ?? '')
  const [phaseId, setPhaseId] = useState<number>(gate?.phase_id ?? 0)
  const [title, setTitle] = useState(gate?.form?.title ?? '')
  const [description, setDescription] = useState(gate?.form?.description ?? '')
  const [fields, setFields] = useState<GateField[]>(
    gate?.form?.fields?.length ? gate.form.fields : [campoNuevo([])],
  )
  const [enabled, setEnabled] = useState(gate?.enabled ?? true)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')

  // Las columnas ocupadas por OTRA puerta no se ofrecen: el servidor las rechaza, y
  // enterarse al guardar es enterarse tarde.
  const opcionesColumna = useMemo(
    () =>
      phases.map((p) => {
        const ocupada = takenPhases.find((t) => t.id === p.id && p.id !== gate?.phase_id)
        return {
          value: p.id,
          label: ocupada ? `${p.name} — ya tiene "${ocupada.by}"` : p.name,
          disabled: !!ocupada,
        }
      }),
    [phases, takenPhases, gate],
  )

  const setField = (i: number, patch: Partial<GateField>) =>
    setFields((prev) => prev.map((f, idx) => (idx === i ? { ...f, ...patch } : f)))

  const mover = (i: number, delta: number) =>
    setFields((prev) => {
      const j = i + delta
      if (j < 0 || j >= prev.length) return prev
      const copia = [...prev]
      ;[copia[i], copia[j]] = [copia[j], copia[i]]
      return copia
    })

  // Los mismos avisos que dará el servidor, dichos aquí. No sustituyen a su
  // validación —nunca se confía en el cliente— pero evitan el viaje.
  const problemas = useMemo(() => {
    const out: string[] = []
    if (!name.trim()) out.push('La puerta necesita un nombre.')
    if (!phaseId) out.push('Elige la columna que será el punto de control.')
    if (!title.trim()) out.push('El formulario necesita un título: es lo primero que se lee al moverla.')
    if (!fields.length) out.push('Añade al menos un campo.')
    fields.forEach((f, i) => {
      if (!f.label.trim()) out.push(`El campo ${i + 1} no tiene etiqueta.`)
      if (f.type === 'select' && !(f.options ?? []).some((o) => o.label.trim())) {
        out.push(`"${f.label || `Campo ${i + 1}`}" es una lista y no tiene opciones: nadie podría responderlo.`)
      }
      if (f.type === 'number' && f.min != null && f.max != null && f.min > f.max) {
        out.push(`"${f.label || `Campo ${i + 1}`}" tiene un mínimo mayor que su máximo.`)
      }
    })
    return out
  }, [name, phaseId, title, fields])

  const guardar = async () => {
    if (problemas.length) return
    setSaving(true)
    setError('')
    const usadas: string[] = []
    const input: GateInput = {
      board_id: boardId,
      phase_id: phaseId,
      name: name.trim(),
      enabled,
      form: {
        title: title.trim(),
        description: description.trim() || undefined,
        fields: fields.map((f) => {
          // La clave se deriva de la etiqueta al guardar, no mientras se escribe:
          // recalcularla en cada tecla cambiaría la clave de un campo ya respondido.
          const key = f.key && /^[a-z][a-z0-9_]*$/.test(f.key) ? f.key : claveDe(f.label, usadas)
          usadas.push(key)
          return {
            ...f,
            key,
            label: f.label.trim(),
            help: f.help?.trim() || undefined,
            placeholder: f.placeholder?.trim() || undefined,
            options:
              f.type === 'select'
                ? (f.options ?? [])
                    .filter((o) => o.label.trim())
                    .map((o) => ({ value: o.value || o.label.trim().toLowerCase(), label: o.label.trim() }))
                : undefined,
          }
        }),
      },
    }
    try {
      if (gate) await workflowService.updateGate(gate.id, input)
      else await workflowService.createGate(input)
      onSaved()
    } catch (e: any) {
      setError(e?.response?.data?.error || 'No se pudo guardar la puerta')
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className={styles['builder']}>
      <div className={styles['grid']}>
        <label className={styles['campo']}>
          <span>Nombre de la puerta</span>
          <input
            value={name}
            maxLength={GATE_LIMITS.name}
            onChange={(e) => setName(e.target.value)}
            placeholder="Control de entrega"
          />
          <small>Es como aparece en esta lista y en el registro de actividad.</small>
        </label>

        <label className={styles['campo']}>
          <span>Columna que será el punto de control</span>
          <Select
            value={phaseId || ''}
            onChange={(v) => setPhaseId(Number(v))}
            options={opcionesColumna}
            placeholder="Elige una columna…"
            fullWidth
          />
          <small>Nadie podrá mover una tarjeta a esa columna sin rellenar el formulario.</small>
        </label>
      </div>

      <div className={styles['grid']}>
        <label className={styles['campo']}>
          <span>Título del formulario</span>
          <input
            value={title}
            maxLength={GATE_LIMITS.title}
            onChange={(e) => setTitle(e.target.value)}
            placeholder="Antes de pasar a revisión"
          />
        </label>
        <label className={styles['campo']}>
          <span>Descripción (opcional)</span>
          <input
            value={description}
            maxLength={GATE_LIMITS.description}
            onChange={(e) => setDescription(e.target.value)}
            placeholder="Deja constancia de qué entregas."
          />
        </label>
      </div>

      <div className={styles['campos-head']}>
        <h4>Qué se pregunta</h4>
        <span>
          {fields.length} de {GATE_LIMITS.fields}
        </span>
      </div>

      <ul className={styles['campos']}>
        {fields.map((f, i) => (
          <li key={i} className={styles['campo-fila']}>
            <div className={styles['campo-cabecera']}>
              <input
                className={styles['etiqueta']}
                value={f.label}
                maxLength={GATE_LIMITS.label}
                onChange={(e) => setField(i, { label: e.target.value })}
                placeholder={`Pregunta ${i + 1}`}
              />
              <Select
                value={f.type}
                onChange={(v) => setField(i, { type: v as GateFieldType })}
                options={TIPOS.map((t) => ({ value: t.value, label: t.label }))}
                ariaLabel={`Tipo del campo ${i + 1}`}
              />
              <label className={styles['obligatorio']}>
                <input
                  type="checkbox"
                  checked={f.required}
                  onChange={(e) => setField(i, { required: e.target.checked })}
                />
                Obligatorio
              </label>
              <div className={styles['acciones']}>
                <button type="button" onClick={() => mover(i, -1)} disabled={i === 0} title="Subir">
                  <ChevronUp size={15} />
                </button>
                <button
                  type="button"
                  onClick={() => mover(i, 1)}
                  disabled={i === fields.length - 1}
                  title="Bajar"
                >
                  <ChevronDown size={15} />
                </button>
                <button
                  type="button"
                  className={styles['borrar']}
                  onClick={() => setFields((prev) => prev.filter((_, idx) => idx !== i))}
                  disabled={fields.length === 1}
                  title={fields.length === 1 ? 'Un formulario necesita al menos un campo' : 'Quitar'}
                >
                  <Trash2 size={15} />
                </button>
              </div>
            </div>

            <input
              className={styles['ayuda']}
              value={f.help ?? ''}
              maxLength={GATE_LIMITS.help}
              onChange={(e) => setField(i, { help: e.target.value })}
              placeholder="Texto de ayuda (opcional): qué esperas exactamente en esta respuesta"
            />

            {f.type === 'select' && (
              <div className={styles['opciones']}>
                {(f.options ?? []).map((o, j) => (
                  <div key={j} className={styles['opcion']}>
                    <input
                      value={o.label}
                      maxLength={GATE_LIMITS.optionText}
                      onChange={(e) => {
                        const opts = [...(f.options ?? [])]
                        opts[j] = { value: o.value, label: e.target.value }
                        setField(i, { options: opts })
                      }}
                      placeholder={`Opción ${j + 1}`}
                    />
                    <button
                      type="button"
                      onClick={() =>
                        setField(i, { options: (f.options ?? []).filter((_, idx) => idx !== j) })
                      }
                      title="Quitar opción"
                    >
                      <Trash2 size={13} />
                    </button>
                  </div>
                ))}
                <button
                  type="button"
                  className={styles['anadir-opcion']}
                  disabled={(f.options ?? []).length >= GATE_LIMITS.options}
                  onClick={() =>
                    setField(i, { options: [...(f.options ?? []), { value: '', label: '' }] })
                  }
                >
                  <Plus size={13} /> Añadir opción
                </button>
              </div>
            )}

            {f.type === 'number' && (
              <div className={styles['rango']}>
                <label>
                  Mínimo
                  <input
                    type="number"
                    value={f.min ?? ''}
                    onChange={(e) =>
                      setField(i, { min: e.target.value === '' ? undefined : Number(e.target.value) })
                    }
                  />
                </label>
                <label>
                  Máximo
                  <input
                    type="number"
                    value={f.max ?? ''}
                    onChange={(e) =>
                      setField(i, { max: e.target.value === '' ? undefined : Number(e.target.value) })
                    }
                  />
                </label>
              </div>
            )}
          </li>
        ))}
      </ul>

      <button
        type="button"
        className={styles['anadir']}
        disabled={fields.length >= GATE_LIMITS.fields}
        onClick={() => setFields((prev) => [...prev, campoNuevo(prev.map((f) => f.key))])}
      >
        <Plus size={15} />
        {fields.length >= GATE_LIMITS.fields
          ? `Máximo ${GATE_LIMITS.fields} campos`
          : 'Añadir campo'}
      </button>

      {/* Vista previa. Una puerta se escribe una vez y la rellena mucha gente muchas
          veces: ver el resultado mientras se define es lo que evita publicar un
          formulario que sólo entiende quien lo escribió. */}
      <div className={styles['previa']}>
        <div className={styles['previa-head']}>
          <Eye size={14} /> Así lo verá quien mueva la tarjeta
        </div>
        <div className={styles['previa-modal']}>
          <span className={styles['previa-badge']}>
            <Lock size={11} /> PASO OBLIGATORIO
          </span>
          <h5>{title.trim() || 'Título del formulario'}</h5>
          {description.trim() && <p>{description}</p>}
          {fields.map((f, i) => (
            <div key={i} className={styles['previa-campo']}>
              <label>
                {f.label.trim() || `Pregunta ${i + 1}`}
                {f.required && <b> *</b>}
              </label>
              {f.type === 'textarea' ? (
                <div className={styles['previa-textarea']} />
              ) : f.type === 'select' ? (
                <div className={styles['previa-input']}>
                  {(f.options ?? []).find((o) => o.label.trim())?.label ?? 'Selecciona…'}
                </div>
              ) : f.type === 'file' ? (
                <div className={styles['previa-boton']}>Elegir archivo</div>
              ) : (
                <div className={styles['previa-input']}>{f.placeholder ?? ''}</div>
              )}
              {f.help?.trim() && <small>{f.help}</small>}
            </div>
          ))}
        </div>
      </div>

      {problemas.length > 0 && (
        <ul className={styles['problemas']}>
          {problemas.map((p, i) => (
            <li key={i}>
              <AlertCircle size={13} /> {p}
            </li>
          ))}
        </ul>
      )}

      {error && (
        <div className={styles['error']} role="alert">
          <AlertCircle size={14} /> {error}
        </div>
      )}

      {/* Editar una puerta encendida cambia lo que se le pide a quien esté moviendo
          una tarjeta ahora mismo. Decirlo antes de guardar es más honesto que
          descubrirlo por un compañero. */}
      {gate?.enabled && (
        <p className={styles['aviso-vivo']}>
          Esta puerta está activa: el formulario nuevo se aplicará al instante, también
          a quien esté moviendo una tarjeta ahora.
        </p>
      )}

      <div className={styles['pie']}>
        <label className={styles['activa']}>
          <input type="checkbox" checked={enabled} onChange={(e) => setEnabled(e.target.checked)} />
          Activa
        </label>
        <div className={styles['pie-botones']}>
          <button type="button" onClick={onCancel} disabled={saving}>
            Cancelar
          </button>
          <button
            type="button"
            className={styles['guardar']}
            onClick={guardar}
            disabled={saving || problemas.length > 0}
          >
            {saving ? 'Guardando…' : gate ? 'Guardar cambios' : 'Crear puerta'}
          </button>
        </div>
      </div>
    </div>
  )
}
