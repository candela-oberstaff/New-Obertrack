import { useState, useEffect, useCallback } from 'react'
import { Star, X, Plus } from 'lucide-react'
import { adminService } from '../../services/admin.service'
import { employerService } from '../../services/employer.service'
import { Select } from '../ui/Select'

type Manager = { manager_id: number; name: string; is_primary: boolean }

interface Props {
  userId: number
  employmentId: number
  companyId: number
  managerOptions: { id: number; name: string }[]
  onChanged?: () => void
  // "admin" (default) pega a /admin con adminService; "employer" pega a /employer
  // con employerService (mismos handlers, acotado a la empresa del empleador).
  mode?: 'admin' | 'employer'
}

// Editor del CONJUNTO de managers de un empleo (multi-manager, Fase 3).
// Chips tipo pill: el principal se resalta en ámbar con ★ dorada; en los demás
// la ★ es la acción "marcar principal"; la × quita. Gateado por el flag.
export default function EmploymentManagersEditor({ userId, employmentId, companyId, managerOptions, onChanged, mode = 'admin' }: Props) {
  const [managers, setManagers] = useState<Manager[]>([])
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [adding, setAdding] = useState(false)
  const [addId, setAddId] = useState<number | ''>('')

  // Operaciones según el modo: admin pega a /admin, employer a /employer.
  const ops = mode === 'employer'
    ? {
        list: (uid: number, eid: number) => employerService.getEmployerEmploymentManagers(uid, eid),
        add: (uid: number, eid: number, mid: number) => employerService.addEmployerEmploymentManager(uid, eid, mid),
        remove: (uid: number, eid: number, mid: number) => employerService.removeEmployerEmploymentManager(uid, eid, mid),
        setPrimary: (uid: number, eid: number, mid: number) => employerService.setPrimaryEmployerEmploymentManager(uid, eid, mid),
      }
    : {
        list: (uid: number, eid: number) => adminService.getEmploymentManagers(uid, eid),
        add: (uid: number, eid: number, mid: number) => adminService.addEmploymentManager(uid, eid, mid),
        remove: (uid: number, eid: number, mid: number) => adminService.removeEmploymentManager(uid, eid, mid),
        setPrimary: (uid: number, eid: number, mid: number) => adminService.setPrimaryEmploymentManager(uid, eid, mid),
      }

  const load = useCallback(async () => {
    try {
      setError(null)
      setManagers(await ops.list(userId, employmentId))
    } catch (err: any) {
      setError(err?.response?.data?.error ?? 'No se pudieron cargar los managers.')
    }
    // ops se deriva de mode; las funciones de servicio son estables.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [userId, employmentId, mode])

  useEffect(() => { load() }, [load])

  const afterAction = useCallback(async () => {
    await load()
    onChanged?.()
  }, [load, onChanged])

  const runAction = useCallback(async (fn: () => Promise<unknown>) => {
    setBusy(true); setError(null)
    try {
      await fn()
      await afterAction()
    } catch (err: any) {
      setError(err?.response?.data?.error ?? 'No se pudo completar la acción.')
    } finally {
      setBusy(false)
    }
  }, [afterAction])

  const handleSetPrimary = (managerId: number) =>
    runAction(() => ops.setPrimary(userId, employmentId, managerId))

  const handleRemove = (managerId: number) =>
    runAction(() => ops.remove(userId, employmentId, managerId))

  const handleAdd = (managerId: number) => {
    setAdding(false)
    setAddId('')
    return runAction(() => ops.add(userId, employmentId, managerId))
  }

  // Candidatos: managers del tenant que no son el propio profesional ni están ya asignados.
  const assignedIds = new Set(managers.map(m => m.manager_id))
  const candidates = managerOptions.filter(m => m.id !== userId && !assignedIds.has(m.id))

  const chipBase: React.CSSProperties = {
    display: 'inline-flex',
    alignItems: 'center',
    gap: '5px',
    padding: '3px 5px 3px 4px',
    borderRadius: '999px',
    border: '1px solid #e2e8f0',
    background: '#fff',
    fontSize: '0.78rem',
    fontWeight: 600,
    color: '#334155',
  }
  const primaryChip: React.CSSProperties = { ...chipBase, border: '1px solid #fcd34d', background: '#fffbeb' }

  const dot = (primary: boolean): React.CSSProperties => ({
    display: 'inline-flex',
    alignItems: 'center',
    justifyContent: 'center',
    width: 20,
    height: 20,
    borderRadius: '50%',
    background: primary ? '#f59e0b' : '#6366f1',
    color: '#fff',
    fontSize: '0.64rem',
    fontWeight: 700,
    flexShrink: 0,
  })

  const iconBtn: React.CSSProperties = {
    display: 'inline-flex',
    alignItems: 'center',
    justifyContent: 'center',
    border: 'none',
    background: 'transparent',
    padding: '2px',
    cursor: busy ? 'progress' : 'pointer',
    lineHeight: 0,
    transition: 'color 0.15s',
  }

  return (
    <div data-company-id={companyId} style={{ display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: '8px', width: '100%' }}>
      <div style={{ display: 'flex', flexWrap: 'wrap', gap: '6px', alignItems: 'center' }}>
        {managers.length === 0 && (
          <span style={{ fontSize: '0.78rem', color: '#94a3b8', fontStyle: 'italic' }}>Sin managers</span>
        )}
        {managers.map(m => (
          <span key={m.manager_id} style={m.is_primary ? primaryChip : chipBase} title={m.is_primary ? 'Manager principal' : 'Manager adicional'}>
            <span style={dot(m.is_primary)}>{(m.name || '?').charAt(0).toUpperCase()}</span>
            <span style={{ maxWidth: 120, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{m.name}</span>
            {m.is_primary ? (
              <span title="Principal" style={{ display: 'inline-flex', padding: '2px' }}>
                <Star size={13} fill="#f59e0b" color="#f59e0b" />
              </span>
            ) : (
              <button
                type="button"
                onClick={() => handleSetPrimary(m.manager_id)}
                disabled={busy}
                title="Marcar como principal"
                style={{ ...iconBtn, color: '#cbd5e1' }}
                onMouseOver={e => { e.currentTarget.style.color = '#f59e0b' }}
                onMouseOut={e => { e.currentTarget.style.color = '#cbd5e1' }}
              >
                <Star size={13} />
              </button>
            )}
            <button
              type="button"
              onClick={() => handleRemove(m.manager_id)}
              disabled={busy}
              title="Quitar manager"
              style={{ ...iconBtn, color: '#cbd5e1' }}
              onMouseOver={e => { e.currentTarget.style.color = '#ef4444' }}
              onMouseOut={e => { e.currentTarget.style.color = '#cbd5e1' }}
            >
              <X size={14} />
            </button>
          </span>
        ))}
      </div>

      {adding ? (
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
          <div style={{ minWidth: 190 }}>
            <Select
              fullWidth
              placeholder="Selecciona un manager…"
              value={addId}
              onChange={v => { if (v !== '' && v != null) handleAdd(Number(v)) }}
              options={candidates.map(c => ({ value: c.id, label: c.name }))}
            />
          </div>
          <button
            type="button"
            onClick={() => { setAdding(false); setAddId('') }}
            disabled={busy}
            style={{ border: 'none', background: 'transparent', color: '#94a3b8', fontSize: '0.78rem', fontWeight: 600, cursor: 'pointer', padding: '2px 4px' }}
          >
            Cancelar
          </button>
        </div>
      ) : (
        <button
          type="button"
          onClick={() => setAdding(true)}
          disabled={busy || candidates.length === 0}
          title={candidates.length === 0 ? 'No hay más managers disponibles' : 'Agregar otro manager'}
          style={{ display: 'inline-flex', alignItems: 'center', gap: '5px', padding: '5px 12px', borderRadius: '999px', border: '1px dashed #cbd5e1', background: '#fff', color: '#6366f1', fontWeight: 600, cursor: (busy || candidates.length === 0) ? 'not-allowed' : 'pointer', fontSize: '0.78rem', opacity: candidates.length === 0 ? 0.5 : 1 }}
        >
          <Plus size={14} /> Agregar manager
        </button>
      )}

      {error && <span style={{ fontSize: '0.75rem', color: '#dc2626', fontWeight: 600 }}>{error}</span>}
    </div>
  )
}
