import { useCallback, useEffect, useMemo, useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import {
  RotateCcw, Trash2, RefreshCw, AlertTriangle, Undo2, CheckCircle2,
  Users, Inbox, Contact, ClipboardList, GraduationCap, LayoutGrid, CheckSquare, Siren, FileText,
} from 'lucide-react'
import { adminService, type TrashItem, type TrashTypeInfo } from '../../services/admin.service'
import styles from './Admin.module.css'

const refKey = (t: string, id: number) => `${t}:${id}`

const TYPE_META: Record<string, { icon: React.ElementType; bg: string; color: string }> = {
  users: { icon: Users, bg: '#e0e7ff', color: '#4338ca' },
  tickets: { icon: Inbox, bg: '#dbeafe', color: '#1d4ed8' },
  contacts: { icon: Contact, bg: '#cffafe', color: '#0e7490' },
  incidents: { icon: AlertTriangle, bg: '#fee2e2', color: '#b91c1c' },
  surveys: { icon: ClipboardList, bg: '#fef3c7', color: '#b45309' },
  tutorials: { icon: GraduationCap, bg: '#f3e8ff', color: '#7e22ce' },
  boards: { icon: LayoutGrid, bg: '#dcfce7', color: '#15803d' },
  tasks: { icon: CheckSquare, bg: '#e0f2fe', color: '#0369a1' },
  emergency_templates: { icon: Siren, bg: '#ffe4e6', color: '#be123c' },
}
const FALLBACK_META = { icon: FileText, bg: '#f1f5f9', color: '#475569' }
const metaFor = (type: string) => TYPE_META[type] || FALLBACK_META

export function TrashPanel() {
  const qc = useQueryClient()
  const [items, setItems] = useState<TrashItem[]>([])
  const [types, setTypes] = useState<TrashTypeInfo[]>([])
  const [loading, setLoading] = useState(true)
  const [typeFilter, setTypeFilter] = useState<string>('')
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [busy, setBusy] = useState(false)
  const [msg, setMsg] = useState<{ tone: 'success' | 'error' | 'info'; text: string } | null>(null)
  const [purgeTargets, setPurgeTargets] = useState<{ type: string; id: number }[] | null>(null)

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const data = await adminService.getTrash()
      setItems(data.items || [])
      setTypes(data.types || [])
      setSelected(new Set())
    } catch (err: any) {
      setMsg({ tone: 'error', text: err?.response?.data?.error ?? 'No se pudo cargar la papelera.' })
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { load() }, [load])

  const countByType = useMemo(() => {
    const m = new Map<string, number>()
    items.forEach((it) => m.set(it.type, (m.get(it.type) || 0) + 1))
    return m
  }, [items])

  const visible = useMemo(() => {
    const rows = typeFilter ? items.filter((it) => it.type === typeFilter) : items
    return [...rows].sort((a, b) => (b.deleted_at || '').localeCompare(a.deleted_at || ''))
  }, [items, typeFilter])

  const allVisibleSelected = visible.length > 0 && visible.every((it) => selected.has(refKey(it.type, it.id)))
  const toggle = (t: string, id: number) => setSelected((prev) => {
    const next = new Set(prev); const k = refKey(t, id)
    next.has(k) ? next.delete(k) : next.add(k)
    return next
  })
  const toggleAll = () => setSelected((prev) =>
    visible.every((it) => prev.has(refKey(it.type, it.id)))
      ? new Set()
      : new Set(visible.map((it) => refKey(it.type, it.id)))
  )

  const selectedRefs = useMemo(() =>
    Array.from(selected).map((k) => {
      const i = k.lastIndexOf(':')
      return { type: k.slice(0, i), id: Number(k.slice(i + 1)) }
    }), [selected])

  const doRestore = async (refs: { type: string; id: number }[]) => {
    if (refs.length === 0) return
    setBusy(true); setMsg(null)
    try {
      const res = await adminService.restoreTrash(refs)
      const n = res?.restored ?? 0
      const f = res?.failed?.length ?? 0
      setMsg({
        tone: f ? 'info' : 'success',
        text: `${n === 1 ? 'Restauraste 1 elemento. Ya está' : `Restauraste ${n} elementos. Ya están`} de vuelta en su sección${f ? ` (${f} no se pudieron)` : ''}.`,
      })
      qc.invalidateQueries()
      await load()
    } catch (err: any) {
      setMsg({ tone: 'error', text: err?.response?.data?.error ?? 'No se pudo restaurar.' })
    } finally { setBusy(false) }
  }

  const doPurge = async () => {
    if (!purgeTargets || purgeTargets.length === 0) return
    setBusy(true); setMsg(null)
    try {
      const res = await adminService.purgeTrash(purgeTargets)
      const n = res?.purged ?? 0
      const f = res?.failed?.length ?? 0
      setMsg({
        tone: f ? 'info' : 'success',
        text: `Eliminaste ${n} ${n === 1 ? 'elemento' : 'elementos'} para siempre${f ? ` (${f} no se pudieron, quizás tienen datos vinculados)` : ''}.`,
      })
      setPurgeTargets(null)
      qc.invalidateQueries()
      await load()
    } catch (err: any) {
      setMsg({ tone: 'error', text: err?.response?.data?.error ?? 'No se pudo eliminar definitivamente.' })
    } finally { setBusy(false) }
  }

  const restoreAllVisible = () => doRestore(visible.map((it) => ({ type: it.type, id: it.id })))

  const relTime = (s: string | null) => {
    if (!s) return '—'
    const diff = Date.now() - new Date(s).getTime()
    const mins = Math.floor(diff / 60000)
    if (mins < 1) return 'recién'
    if (mins < 60) return `hace ${mins} min`
    const h = Math.floor(mins / 60)
    if (h < 24) return `hace ${h} h`
    const d = Math.floor(h / 24)
    if (d < 30) return `hace ${d} día${d === 1 ? '' : 's'}`
    const mo = Math.floor(d / 30)
    return `hace ${mo} mes${mo === 1 ? '' : 'es'}`
  }
  const fullDate = (s: string | null) => s ? new Date(s).toLocaleString('es-ES', { day: '2-digit', month: 'long', year: 'numeric', hour: '2-digit', minute: '2-digit' }) : ''

  const restoreAllLabel = typeFilter
    ? `Restaurar ${types.find((t) => t.key === typeFilter)?.label ?? 'todo'}`
    : 'Restaurar todo'

  return (
    <div className={styles['admin-content'] || 'admin-content'}>
      <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', gap: 12, flexWrap: 'wrap', marginBottom: 16 }}>
        <div>
          <h3 style={{ margin: 0, fontSize: 18, fontWeight: 800, color: '#0f172a', display: 'flex', alignItems: 'center', gap: 8 }}>
            <Trash2 size={20} /> Papelera
            <span style={{ fontSize: 13, fontWeight: 700, color: '#64748b', background: '#f1f5f9', borderRadius: 999, padding: '2px 10px' }}>{items.length}</span>
          </h3>
          <p style={{ margin: '4px 0 0', fontSize: 13, color: '#64748b' }}>
            Lo que borres queda acá. Restauralo para devolverlo a su sección, o eliminalo para siempre.
          </p>
        </div>
        <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
          {visible.length > 0 && (
            <button
              onClick={restoreAllVisible}
              disabled={busy || loading}
              title={`Restaura los ${visible.length} elementos visibles`}
              style={{ display: 'inline-flex', alignItems: 'center', gap: 6, padding: '8px 16px', borderRadius: 10, border: 'none', background: 'var(--primary, #7c3aed)', color: '#fff', fontSize: 13, fontWeight: 700, cursor: (busy || loading) ? 'progress' : 'pointer' }}
            >
              <Undo2 size={15} /> {restoreAllLabel} ({visible.length})
            </button>
          )}
          <button
            onClick={load}
            disabled={busy || loading}
            style={{ display: 'inline-flex', alignItems: 'center', gap: 6, padding: '8px 14px', borderRadius: 10, border: '1px solid #e2e8f0', background: '#fff', color: '#475569', fontSize: 13, fontWeight: 600, cursor: 'pointer' }}
          >
            <RefreshCw size={15} className={loading ? styles['spin'] : ''} /> Actualizar
          </button>
        </div>
      </div>

      {items.length > 0 && (
        <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap', marginBottom: 14 }}>
          <button onClick={() => setTypeFilter('')} style={chipStyle(typeFilter === '')}>
            Todos <span style={chipCount}>{items.length}</span>
          </button>
          {types.filter((t) => (countByType.get(t.key) || 0) > 0).map((t) => {
            const M = metaFor(t.key)
            return (
              <button key={t.key} onClick={() => setTypeFilter(t.key)} style={chipStyle(typeFilter === t.key)}>
                <M.icon size={13} style={{ color: M.color }} /> {t.label} <span style={chipCount}>{countByType.get(t.key) || 0}</span>
              </button>
            )
          })}
        </div>
      )}

      {selected.size > 0 && (
        <div style={{ display: 'flex', alignItems: 'center', gap: 12, flexWrap: 'wrap', padding: '10px 14px', marginBottom: 12, background: 'rgba(124,58,237,0.06)', border: '1px solid #e9d5ff', borderRadius: 12 }}>
          <span style={{ fontWeight: 700, color: '#6d28d9' }}>{selected.size} seleccionado(s)</span>
          <button onClick={() => doRestore(selectedRefs)} disabled={busy}
            style={{ display: 'inline-flex', alignItems: 'center', gap: 6, padding: '9px 16px', borderRadius: 10, border: 'none', background: 'var(--primary, #7c3aed)', color: '#fff', fontWeight: 700, fontSize: 13, cursor: busy ? 'progress' : 'pointer' }}>
            <RotateCcw size={15} /> Restaurar ({selected.size})
          </button>
          <button onClick={() => setPurgeTargets(selectedRefs)} disabled={busy}
            style={{ display: 'inline-flex', alignItems: 'center', gap: 6, padding: '9px 16px', borderRadius: 10, border: 'none', background: '#dc2626', color: '#fff', fontWeight: 700, fontSize: 13, cursor: busy ? 'progress' : 'pointer' }}>
            <Trash2 size={15} /> Eliminar definitivamente ({selected.size})
          </button>
          <button onClick={() => setSelected(new Set())} disabled={busy}
            style={{ padding: '9px 14px', border: '1px solid #e2e8f0', borderRadius: 10, background: 'transparent', color: '#64748b', fontSize: 13, fontWeight: 600, cursor: 'pointer' }}>
            Limpiar
          </button>
        </div>
      )}

      {msg && (
        <div style={{
          display: 'flex', alignItems: 'flex-start', gap: 10, padding: '11px 14px', margin: '0 0 14px', borderRadius: 10, fontSize: 13, fontWeight: 600,
          ...(msg.tone === 'success'
            ? { background: '#dcfce7', border: '1px solid #86efac', color: '#15803d' }
            : msg.tone === 'error'
            ? { background: '#fee2e2', border: '1px solid #fecaca', color: '#b91c1c' }
            : { background: '#eff6ff', border: '1px solid #bfdbfe', color: '#1d4ed8' }),
        }}>
          {msg.tone === 'success' ? <CheckCircle2 size={17} style={{ flexShrink: 0, marginTop: 1 }} /> : <AlertTriangle size={17} style={{ flexShrink: 0, marginTop: 1 }} />}
          <span>{msg.text}</span>
        </div>
      )}

      {!loading && items.length === 0 ? (
        <div style={{ textAlign: 'center', padding: '56px 16px', color: '#94a3b8' }}>
          <div style={{ width: 64, height: 64, borderRadius: 20, background: '#f1f5f9', display: 'inline-flex', alignItems: 'center', justifyContent: 'center', marginBottom: 14 }}>
            <Trash2 size={30} />
          </div>
          <h4 style={{ margin: '0 0 4px', color: '#475569', fontSize: 16, fontWeight: 700 }}>La papelera está vacía</h4>
          <p style={{ margin: 0, fontSize: 13 }}>Lo que elimines desde cualquier módulo aparecerá aquí para recuperarlo.</p>
        </div>
      ) : (
        <div className={styles['users-table'] || 'users-table'}>
          <table>
            <thead>
              <tr>
                <th style={{ width: 36 }}>
                  <input type="checkbox" checked={allVisibleSelected} onChange={toggleAll} disabled={visible.length === 0} style={{ cursor: 'pointer' }} />
                </th>
                <th>Elemento</th>
                <th>Tipo</th>
                <th>Eliminado</th>
                <th style={{ textAlign: 'right' }}>Acciones</th>
              </tr>
            </thead>
            <tbody>
              {loading && (
                <tr><td colSpan={5} style={{ textAlign: 'center', padding: '32px 16px', color: '#94a3b8' }}>Cargando…</td></tr>
              )}
              {!loading && visible.map((it) => {
                const M = metaFor(it.type)
                return (
                  <tr key={refKey(it.type, it.id)}>
                    <td style={{ width: 36 }}>
                      <input type="checkbox" checked={selected.has(refKey(it.type, it.id))} onChange={() => toggle(it.type, it.id)} style={{ cursor: 'pointer' }} />
                    </td>
                    <td>
                      <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
                        <span style={{ width: 36, height: 36, borderRadius: 10, background: M.bg, color: M.color, display: 'inline-flex', alignItems: 'center', justifyContent: 'center', flexShrink: 0 }}>
                          <M.icon size={18} />
                        </span>
                        <div style={{ minWidth: 0 }}>
                          <div style={{ fontWeight: 600, color: '#0f172a', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{it.title || `#${it.id}`}</div>
                          {it.subtitle && <div style={{ fontSize: 12, color: '#94a3b8', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{it.subtitle}</div>}
                        </div>
                      </div>
                    </td>
                    <td>
                      <span style={{ display: 'inline-flex', alignItems: 'center', gap: 7, padding: '4px 11px', borderRadius: 999, background: M.bg, color: M.color, fontSize: 12, fontWeight: 700, whiteSpace: 'nowrap', lineHeight: 1.6 }}>
                        <span style={{ width: 6, height: 6, borderRadius: 999, background: M.color, flexShrink: 0 }} />
                        {it.type_label}
                      </span>
                    </td>
                    <td style={{ fontSize: 13, color: '#64748b', whiteSpace: 'nowrap' }} title={fullDate(it.deleted_at)}>{relTime(it.deleted_at)}</td>
                    <td>
                      <div style={{ display: 'flex', gap: 8, justifyContent: 'flex-end' }}>
                        <button title="Restaurar" onClick={() => doRestore([{ type: it.type, id: it.id }])} disabled={busy}
                          className={styles['btn-icon'] || 'btn-icon'}>
                          <RotateCcw size={16} />
                        </button>
                        <button title="Eliminar definitivamente" onClick={() => setPurgeTargets([{ type: it.type, id: it.id }])} disabled={busy}
                          className={`${styles['btn-icon'] || 'btn-icon'} ${styles['danger'] || ''}`}>
                          <Trash2 size={16} />
                        </button>
                      </div>
                    </td>
                  </tr>
                )
              })}
              {!loading && visible.length === 0 && items.length > 0 && (
                <tr><td colSpan={5} style={{ textAlign: 'center', padding: '28px 16px', color: '#94a3b8' }}>No hay elementos de este tipo.</td></tr>
              )}
            </tbody>
          </table>
        </div>
      )}

      {purgeTargets && (
        <div className={styles['modal-overlay']} onClick={() => !busy && setPurgeTargets(null)}>
          <div className={styles['modal']} onClick={(e) => e.stopPropagation()}>
            <h2 style={{ display: 'flex', alignItems: 'center', gap: 8 }}><AlertTriangle size={20} color="#dc2626" /> Eliminar definitivamente</h2>
            <p>Vas a eliminar <strong>{purgeTargets.length}</strong> elemento(s) <strong>para siempre</strong>.</p>
            <p className={styles['warning-text']}>
              Esta acción no se puede deshacer. Si algún elemento tiene datos dependientes, puede
              omitirse (te lo reportamos). Para solo recuperarlo, usá "Restaurar".
            </p>
            <div className={styles['modal-actions']}>
              <button className={styles['btn-secondary']} onClick={() => setPurgeTargets(null)} disabled={busy}>Cancelar</button>
              <button className={styles['btn-danger']} onClick={doPurge} disabled={busy}>
                {busy ? 'Eliminando…' : `Eliminar ${purgeTargets.length} para siempre`}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

const chipStyle = (active: boolean): React.CSSProperties => ({
  display: 'inline-flex', alignItems: 'center', gap: 6, padding: '7px 12px', borderRadius: 999,
  border: active ? '1px solid var(--primary, #7c3aed)' : '1px solid #e2e8f0',
  background: active ? 'rgba(124,58,237,0.10)' : '#fff',
  color: active ? '#6d28d9' : '#475569', fontSize: 13, fontWeight: 600, cursor: 'pointer',
})
const chipCount: React.CSSProperties = { fontSize: 11, fontWeight: 800, background: 'rgba(0,0,0,0.06)', borderRadius: 999, padding: '1px 7px' }
