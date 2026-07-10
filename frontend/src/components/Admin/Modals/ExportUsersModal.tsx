import { useMemo, useState } from 'react'
import { Download, X, FileSpreadsheet, Loader2 } from 'lucide-react'

interface ExportUser {
  id: number
  name?: string
  email?: string
  user_type?: string
  is_active?: boolean
  is_manager?: boolean
  empleador_id?: number
  company_name?: string
  job_title?: string
  phone_number?: string
  identity_document?: string
  country?: string
  state?: string
  city?: string
  address?: string
  location?: string
  created_at?: string
  updated_at?: string
}

type CellType = StringConstructor | BooleanConstructor
interface FieldDef {
  key: string
  label: string
  type: CellType
  width: number
  value: (u: ExportUser, companyName: (u: ExportUser) => string) => string | boolean
}

const TYPE_LABEL: Record<string, string> = {
  profesional: 'Profesional',
  empleador: 'Empleador',
  customer_success: 'Customer Success',
  superadmin: 'Superadmin',
}

const FIELDS: FieldDef[] = [
  { key: 'name', label: 'Nombre', type: String, width: 28, value: u => u.name || '' },
  { key: 'email', label: 'Email', type: String, width: 32, value: u => u.email || '' },
  { key: 'user_type', label: 'Tipo', type: String, width: 18, value: u => TYPE_LABEL[u.user_type || ''] || u.user_type || '' },
  { key: 'is_active', label: 'Estado', type: String, width: 12, value: u => (u.is_active === false ? 'Inactivo' : 'Activo') },
  { key: 'company', label: 'Empresa', type: String, width: 30, value: (u, companyName) => companyName(u) },
  { key: 'is_manager', label: 'Es manager', type: Boolean, width: 12, value: u => !!u.is_manager },
  { key: 'job_title', label: 'Cargo', type: String, width: 22, value: u => u.job_title || '' },
  { key: 'phone_number', label: 'Teléfono', type: String, width: 18, value: u => u.phone_number || '' },
  { key: 'identity_document', label: 'Documento', type: String, width: 18, value: u => u.identity_document || '' },
  { key: 'country', label: 'País', type: String, width: 16, value: u => u.country || '' },
  { key: 'state', label: 'Estado / Provincia', type: String, width: 20, value: u => u.state || '' },
  { key: 'city', label: 'Ciudad', type: String, width: 18, value: u => u.city || '' },
  { key: 'address', label: 'Dirección', type: String, width: 28, value: u => u.address || '' },
  { key: 'location', label: 'Ubicación', type: String, width: 20, value: u => u.location || '' },
  { key: 'created_at', label: 'Registrado', type: String, width: 20, value: u => u.created_at || '' },
  { key: 'updated_at', label: 'Actualizado', type: String, width: 20, value: u => u.updated_at || '' },
]

const DEFAULT_SELECTED = ['name', 'email', 'user_type', 'is_active', 'company']
const PREVIEW_ROWS = 6

const cellText = (v: string | boolean): string => (typeof v === 'boolean' ? (v ? 'Sí' : 'No') : v)

const overlay: React.CSSProperties = { position: 'fixed', inset: 0, zIndex: 1000, display: 'flex', alignItems: 'center', justifyContent: 'center', background: 'rgba(15,23,42,0.5)' }
const panel: React.CSSProperties = { background: '#fff', borderRadius: 20, padding: 28, width: '100%', maxWidth: 860, maxHeight: '90vh', overflowY: 'auto', boxShadow: '0 25px 50px -12px rgba(0,0,0,0.25)' }
const btnPrimary: React.CSSProperties = { display: 'inline-flex', alignItems: 'center', gap: 8, padding: '10px 18px', borderRadius: 12, border: 'none', background: 'linear-gradient(135deg, var(--primary), var(--primary-dark, #1d4ed8))', color: '#fff', fontWeight: 700, fontSize: 14, cursor: 'pointer' }
const btnSecondary: React.CSSProperties = { display: 'inline-flex', alignItems: 'center', gap: 6, padding: '10px 16px', borderRadius: 12, border: '1px solid #e2e8f0', background: '#f8fafc', color: '#475569', fontWeight: 600, fontSize: 14, cursor: 'pointer' }
const linkBtn: React.CSSProperties = { background: 'none', border: 'none', color: '#6d28d9', fontWeight: 700, fontSize: 12.5, cursor: 'pointer', padding: 0 }

export function ExportUsersModal({
  users,
  companyName,
  filtered,
  onClose,
}: {
  users: ExportUser[]
  companyName: (u: ExportUser) => string
  filtered: boolean
  onClose: () => void
}) {
  const [selected, setSelected] = useState<Set<string>>(new Set(DEFAULT_SELECTED))
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const orderedSelected = useMemo(() => FIELDS.filter(f => selected.has(f.key)), [selected])

  const toggle = (key: string) => setSelected(prev => {
    const next = new Set(prev)
    if (next.has(key)) next.delete(key); else next.add(key)
    return next
  })

  const handleExport = async () => {
    if (orderedSelected.length === 0 || users.length === 0) return
    setBusy(true); setError(null)
    try {
      // Carga diferida: la librería de xlsx solo pesa si de verdad se exporta.
      const { default: writeXlsxFile } = await import('write-excel-file/browser')
      const header = orderedSelected.map(f => ({
        value: f.label, type: String, fontWeight: 'bold' as const, backgroundColor: '#EDE9FE', color: '#5B21B6',
      }))
      const body = users.map(u =>
        orderedSelected.map(f => ({ value: f.value(u, companyName), type: f.type })),
      )
      const { toFile } = await writeXlsxFile([header, ...body], {
        sheet: 'Usuarios',
        stickyRowsCount: 1,
        columns: orderedSelected.map(f => ({ width: f.width })),
      })
      await toFile('usuarios_obertrack.xlsx')
      onClose()
    } catch {
      setError('No se pudo generar el archivo. Intentá de nuevo.')
      setBusy(false)
    }
  }

  return (
    <div style={overlay} onClick={onClose}>
      <div style={panel} onClick={e => e.stopPropagation()}>
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 18 }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
            <div style={{ width: 40, height: 40, borderRadius: 12, background: 'linear-gradient(135deg, var(--primary), var(--primary-dark, #1d4ed8))', display: 'flex', alignItems: 'center', justifyContent: 'center', color: '#fff' }}>
              <FileSpreadsheet size={20} />
            </div>
            <div>
              <h2 style={{ margin: 0, fontSize: 20, fontWeight: 800, color: '#0f172a' }}>Exportar a Excel</h2>
              <p style={{ margin: 0, fontSize: 13, color: '#64748b' }}>
                {users.length} usuario(s){filtered ? ' (según los filtros actuales)' : ''}
              </p>
            </div>
          </div>
          <button onClick={onClose} style={{ background: 'none', border: 'none', cursor: 'pointer', color: '#94a3b8' }}><X size={22} /></button>
        </div>

        {error && (
          <div style={{ background: 'rgba(239,68,68,0.12)', border: '1px solid rgba(239,68,68,0.3)', color: '#dc2626', padding: '11px 14px', borderRadius: 10, marginBottom: 16, fontSize: 13, fontWeight: 600 }}>{error}</div>
        )}

        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 10 }}>
          <span style={{ fontSize: 13, fontWeight: 700, color: '#334155' }}>Elegí las columnas ({orderedSelected.length})</span>
          <div style={{ display: 'flex', gap: 14 }}>
            <button type="button" style={linkBtn} onClick={() => setSelected(new Set(FIELDS.map(f => f.key)))}>Todas</button>
            <button type="button" style={linkBtn} onClick={() => setSelected(new Set(DEFAULT_SELECTED))}>Básicas</button>
          </div>
        </div>

        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(200px, 1fr))', gap: 8, marginBottom: 20 }}>
          {FIELDS.map(f => {
            const on = selected.has(f.key)
            return (
              <label
                key={f.key}
                style={{
                  display: 'flex', alignItems: 'center', gap: 9, padding: '9px 12px', borderRadius: 10, cursor: 'pointer',
                  border: `1px solid ${on ? '#c4b5fd' : '#e2e8f0'}`, background: on ? '#f5f3ff' : '#fff',
                  fontSize: 13.5, fontWeight: 600, color: on ? '#5b21b6' : '#475569',
                }}
              >
                <input type="checkbox" checked={on} onChange={() => toggle(f.key)} style={{ accentColor: '#6d28d9', width: 15, height: 15 }} />
                {f.label}
              </label>
            )
          })}
        </div>

        <div style={{ fontSize: 13, fontWeight: 700, color: '#334155', marginBottom: 8 }}>
          Vista previa {users.length > PREVIEW_ROWS && <span style={{ fontWeight: 500, color: '#94a3b8' }}>· primeras {PREVIEW_ROWS} de {users.length} filas</span>}
        </div>
        <div style={{ border: '1px solid #e2e8f0', borderRadius: 10, overflow: 'hidden', marginBottom: 20 }}>
          {orderedSelected.length === 0 ? (
            <div style={{ padding: '24px 12px', textAlign: 'center', color: '#94a3b8', fontSize: 13 }}>Elegí al menos una columna.</div>
          ) : (
            <div style={{ overflowX: 'auto' }}>
              <table style={{ borderCollapse: 'collapse', fontSize: 12.5, minWidth: '100%', whiteSpace: 'nowrap' }}>
                <thead>
                  <tr style={{ background: '#f5f3ff', textAlign: 'left' }}>
                    {orderedSelected.map(f => (
                      <th key={f.key} style={{ padding: '8px 12px', fontWeight: 700, color: '#5b21b6', borderBottom: '1px solid #e2e8f0' }}>{f.label}</th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {users.slice(0, PREVIEW_ROWS).map((u, i) => (
                    <tr key={u.id} style={{ background: i % 2 ? '#fbfaff' : '#fff' }}>
                      {orderedSelected.map(f => {
                        const text = cellText(f.value(u, companyName))
                        return (
                          <td key={f.key} style={{ padding: '7px 12px', color: text ? '#334155' : '#cbd5e1', borderTop: '1px solid #f1f5f9', maxWidth: 220, overflow: 'hidden', textOverflow: 'ellipsis' }}>
                            {text || '—'}
                          </td>
                        )
                      })}
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>

        <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 10 }}>
          <button type="button" style={btnSecondary} onClick={onClose} disabled={busy}>Cancelar</button>
          <button type="button" style={{ ...btnPrimary, opacity: orderedSelected.length === 0 || users.length === 0 ? 0.5 : 1 }} onClick={handleExport} disabled={busy || orderedSelected.length === 0 || users.length === 0}>
            {busy ? <Loader2 size={16} className="animate-spin" /> : <Download size={16} />}
            {busy ? 'Generando...' : `Exportar ${users.length}`}
          </button>
        </div>
      </div>
    </div>
  )
}
