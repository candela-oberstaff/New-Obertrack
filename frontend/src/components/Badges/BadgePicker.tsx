import { Select } from '../ui'
import { BADGE_COLORS, BADGE_ICON_NAMES, badgeIcon } from './badgeCatalog'
import { BadgeMedallion } from './BadgeMedallion'

export interface BadgeDraft {
  /** Nombre propio de la insignia; vacío = se usa el del bloque o programa. */
  title: string
  icon: string
  color: string
}

/** Una insignia ya definida en otro bloque o programa, para copiarla. */
export interface BadgePreset extends BadgeDraft {
  key: string
  /** De dónde sale: "Bloque · Bienvenida", "Programa · Inducción". */
  source: string
}

interface Props {
  value: BadgeDraft
  /** Nombre del bloque o programa: es el título si el propio está vacío. */
  fallbackTitle: string
  presets?: BadgePreset[]
  onChange: (next: BadgeDraft) => void
}

const swatchBase: React.CSSProperties = {
  width: 30,
  height: 30,
  borderRadius: '50%',
  border: '2px solid transparent',
  cursor: 'pointer',
  padding: 0,
  transition: 'transform 0.15s ease, box-shadow 0.15s ease',
}

const iconBtnBase: React.CSSProperties = {
  width: 40,
  height: 40,
  borderRadius: 10,
  border: '1px solid #e2e8f0',
  background: 'white',
  color: '#475569',
  display: 'inline-flex',
  alignItems: 'center',
  justifyContent: 'center',
  cursor: 'pointer',
  transition: 'all 0.15s ease',
}

const label: React.CSSProperties = {
  fontSize: 12,
  fontWeight: 700,
  color: '#64748b',
  textTransform: 'uppercase',
  letterSpacing: '0.04em',
  marginBottom: 8,
}

const inputStyle: React.CSSProperties = {
  width: '100%',
  padding: '10px 14px',
  border: '1px solid #e2e8f0',
  borderRadius: 12,
  fontFamily: 'inherit',
  fontSize: 14,
  color: '#0f172a',
  background: 'white',
  boxSizing: 'border-box',
}

/**
 * Define una insignia: nombre propio (o el del bloque/programa), icono del
 * set cerrado y color de la paleta, con vista previa del medallón tal como lo
 * verá el profesional. Si ya hay insignias definidas en otros bloques o
 * programas, se puede copiar una en vez de armarla desde cero.
 */
export function BadgePicker({ value, fallbackTitle, presets = [], onChange }: Props) {
  const effectiveTitle = value.title.trim() || fallbackTitle.trim() || 'Sin nombre'

  return (
    <div style={{ display: 'flex', gap: 24, alignItems: 'flex-start', flexWrap: 'wrap' }}>
      <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 8, minWidth: 130 }}>
        <BadgeMedallion icon={value.icon} color={value.color} size="lg" title={effectiveTitle} />
        <span style={{ fontSize: 12.5, fontWeight: 700, color: '#0f172a', textAlign: 'center', maxWidth: 150 }}>
          {effectiveTitle}
        </span>
        <span style={{ fontSize: 11, color: '#94a3b8', textAlign: 'center' }}>Vista previa</span>
      </div>

      <div style={{ flex: 1, minWidth: 280, display: 'flex', flexDirection: 'column', gap: 16 }}>
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(240px, 1fr))', gap: 14 }}>
          <div>
            <div style={label}>Nombre de la insignia</div>
            <input
              type="text"
              style={inputStyle}
              placeholder={fallbackTitle.trim() ? `Igual que el nombre: ${fallbackTitle.trim()}` : 'Ej. Bienvenida completada'}
              value={value.title}
              onChange={(e) => onChange({ ...value, title: e.target.value })}
            />
          </div>
          {presets.length > 0 && (
            <div>
              <div style={label}>O usar una existente</div>
              <Select
                fullWidth
                value=""
                placeholder="Copiar de otro bloque o programa..."
                onChange={(v) => {
                  const preset = presets.find((p) => p.key === String(v))
                  if (preset) onChange({ title: preset.title, icon: preset.icon, color: preset.color })
                }}
                options={presets.map((p) => ({
                  value: p.key,
                  label: `${p.source} · ${p.title || '(sin nombre propio)'}`,
                }))}
              />
            </div>
          )}
        </div>

        <div>
          <div style={label}>Icono</div>
          <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6 }}>
            {BADGE_ICON_NAMES.map((name) => {
              const Icon = badgeIcon(name)
              const active = name === value.icon
              return (
                <button
                  key={name}
                  type="button"
                  title={name}
                  aria-label={`Icono ${name}`}
                  aria-pressed={active}
                  onClick={() => onChange({ ...value, icon: name })}
                  style={{
                    ...iconBtnBase,
                    borderColor: active ? 'var(--primary)' : '#e2e8f0',
                    background: active ? '#fdf4ff' : 'white',
                    color: active ? 'var(--primary)' : '#475569',
                    boxShadow: active ? '0 0 0 3px rgba(204, 51, 204, 0.15)' : 'none',
                  }}
                >
                  <Icon size={18} />
                </button>
              )
            })}
          </div>
        </div>

        <div>
          <div style={label}>Color</div>
          <div style={{ display: 'flex', flexWrap: 'wrap', gap: 8 }}>
            {BADGE_COLORS.map((c) => {
              const active = c.key === value.color
              return (
                <button
                  key={c.key}
                  type="button"
                  title={c.label}
                  aria-label={`Color ${c.label}`}
                  aria-pressed={active}
                  onClick={() => onChange({ ...value, color: c.key })}
                  style={{
                    ...swatchBase,
                    background: `linear-gradient(145deg, ${c.from}, ${c.to})`,
                    borderColor: active ? '#0f172a' : 'transparent',
                    boxShadow: active ? '0 0 0 3px rgba(15, 23, 42, 0.15)' : 'none',
                    transform: active ? 'scale(1.1)' : 'none',
                  }}
                />
              )
            })}
          </div>
        </div>
      </div>
    </div>
  )
}

/** Arma la lista de insignias existentes a partir de bloques y programas. */
export function buildBadgePresets(
  blocks: { id: number; name: string; badge_title?: string; badge_icon: string; badge_color: string }[],
  programs: { id: number; name: string; badge_title?: string; badge_icon: string; badge_color: string }[],
  exclude?: { kind: 'block' | 'program'; id: number }
): BadgePreset[] {
  const out: BadgePreset[] = []
  for (const b of blocks) {
    if (exclude?.kind === 'block' && exclude.id === b.id) continue
    out.push({ key: `block:${b.id}`, source: `Bloque · ${b.name}`, title: b.badge_title || '', icon: b.badge_icon, color: b.badge_color })
  }
  for (const p of programs) {
    if (exclude?.kind === 'program' && exclude.id === p.id) continue
    out.push({ key: `program:${p.id}`, source: `Programa · ${p.name}`, title: p.badge_title || '', icon: p.badge_icon, color: p.badge_color })
  }
  return out
}
