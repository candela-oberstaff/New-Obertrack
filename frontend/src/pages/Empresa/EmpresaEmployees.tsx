import { useMemo, useState, useRef, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Search, Eye, UserPlus, Copy, Check, UploadCloud, Download, Trash2, X, ChevronDown } from 'lucide-react'
import { userService, employerService } from '../../services/api'
import { useAuth } from '../../context/AuthContext'
import { useConfirm } from '../../components/ui/ConfirmProvider'
import { useNotification } from '../../context/NotificationContext'
import Avatar from '../../components/Common/Avatar'
import { Modal, Button, Skeleton } from '../../components/ui'
import { Select } from '../../components/ui/Select'
import { CreateUserModal, type CreateUserForm } from '../../components/Admin/Modals/CreateUserModal'
import { ImportUsersModal } from '../../components/Admin/Modals/ImportUsersModal'
import { ExportUsersModal } from '../../components/Admin/Modals/ExportUsersModal'
import styles from '../../components/Admin/Admin.module.css'

const EMPTY_CREATE_FORM: CreateUserForm = {
  name: '', email: '', password: '', userType: 'profesional',
  jobTitle: '', phoneNumber: '', selectedCompanyId: '', managerId: '',
  companyName: '', industry: '', country: '', province: '', city: '', location: '', address: '',
}

// Lista de profesionales para el EMPLEADOR (user_type === 'empleador'), acotada a
// SU empresa por el backend (/users/employees). Tabla clásica, espejo del panel
// admin (Admin.tsx) pero dentro del tenant del empleador.
export default function EmpresaEmployees() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const { user: employer } = useAuth()
  const confirm = useConfirm()
  const { error: showError, success: showSuccess } = useNotification()
  const [search, setSearch] = useState('')
  const [roleFilter, setRoleFilter] = useState<'all' | 'manager' | 'profesional'>('all')

  const [showImport, setShowImport] = useState(false)
  const [showExport, setShowExport] = useState(false)
  const [showActions, setShowActions] = useState(false)
  const actionsRef = useRef<HTMLDivElement>(null)
  const [showCreate, setShowCreate] = useState(false)

  // Selección para acciones masivas (borrar / asignar manager).
  const [selectedIds, setSelectedIds] = useState<Set<number>>(new Set())
  const [bulkManagerId, setBulkManagerId] = useState<string>('')
  const [bulkBusy, setBulkBusy] = useState(false)
  const [form, setForm] = useState<CreateUserForm>({ ...EMPTY_CREATE_FORM })
  const [creating, setCreating] = useState(false)
  const [createErr, setCreateErr] = useState<string | null>(null)
  // Contraseña temporal devuelta por el backend (se muestra una sola vez).
  const [tempPassword, setTempPassword] = useState<string | null>(null)
  const [copied, setCopied] = useState(false)

  const { data, isLoading, isError } = useQuery({
    queryKey: ['employer', 'employees'],
    queryFn: () => userService.getEmployees(),
  })

  // Cierra el menú "Acciones" al hacer clic fuera.
  useEffect(() => {
    if (!showActions) return
    const onClick = (e: MouseEvent) => {
      if (actionsRef.current && !actionsRef.current.contains(e.target as Node)) {
        setShowActions(false)
      }
    }
    document.addEventListener('mousedown', onClick)
    return () => document.removeEventListener('mousedown', onClick)
  }, [showActions])

  const openCreate = () => {
    setForm({ ...EMPTY_CREATE_FORM })
    setCreateErr(null)
    setTempPassword(null)
    setCopied(false)
    setShowCreate(true)
  }

  const closeCreate = () => {
    if (creating) return
    setShowCreate(false)
  }

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault()
    setCreating(true)
    setCreateErr(null)
    try {
      const payload: Parameters<typeof employerService.createEmployee>[0] = {
        name: form.name.trim(),
        email: form.email.trim(),
      }
      const jt = form.jobTitle.trim()
      if (jt) payload.job_title = jt
      const phone = form.phoneNumber.trim()
      if (phone) payload.phone_number = phone
      if (form.country) payload.country = form.country
      if (form.province) payload.state = form.province
      const city = form.city.trim()
      if (city) payload.city = city
      const location = form.location.trim()
      if (location) payload.location = location
      if (form.managerId) payload.manager_id = Number(form.managerId)
      const res = await employerService.createEmployee(payload)
      setTempPassword(res.temp_password)
      queryClient.invalidateQueries({ queryKey: ['employer', 'employees'] })
    } catch (err: any) {
      setCreateErr(err?.response?.data?.error ?? 'No se pudo crear el profesional.')
    } finally {
      setCreating(false)
    }
  }

  const copyTempPassword = async () => {
    if (!tempPassword) return
    try {
      await navigator.clipboard.writeText(tempPassword)
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    } catch { /* clipboard no disponible: el usuario la copia a mano */ }
  }

  const employees = useMemo(() => data ?? [], [data])
  const totalManagers = employees.filter((u) => u.is_manager).length
  const managers = useMemo(
    () => employees.filter((u) => u.is_manager).map((u) => ({ id: u.id, name: u.name })),
    [employees],
  )
  const companyName = employer?.company_name || employer?.name || 'Mi empresa'

  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase()
    return employees.filter((u) => {
      if (roleFilter === 'manager' && !u.is_manager) return false
      if (roleFilter === 'profesional' && u.is_manager) return false
      if (!q) return true
      return u.name?.toLowerCase().includes(q) || u.email?.toLowerCase().includes(q)
    })
  }, [employees, search, roleFilter])

  // ── Selección masiva ────────────────────────────────────────────────────────
  const isBulkSelectable = (u: { id: number; is_superadmin?: boolean }) =>
    !u.is_superadmin && u.id !== employer?.id
  const selectableIds = useMemo(
    () => filtered.filter(isBulkSelectable).map((u) => u.id),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [filtered, employer?.id],
  )
  const allSelected = selectableIds.length > 0 && selectableIds.every((id) => selectedIds.has(id))
  const toggleSelect = (id: number) =>
    setSelectedIds((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  const toggleSelectAll = () =>
    setSelectedIds((prev) =>
      selectableIds.every((id) => prev.has(id)) ? new Set<number>() : new Set<number>(selectableIds),
    )
  const clearSelection = () => setSelectedIds(new Set())

  const handleBulkAssign = async () => {
    if (selectedIds.size === 0 || bulkManagerId === '') return
    const managerId = bulkManagerId === '__unassign__' ? null : Number(bulkManagerId)
    setBulkBusy(true)
    try {
      const res = await employerService.bulkAssignManager(Array.from(selectedIds), managerId)
      const a = res?.assigned ?? 0
      const s = res?.skipped ?? 0
      showSuccess(
        `${managerId === null ? 'Manager quitado' : 'Manager asignado'} a ${a} profesional(es)` +
          (s ? ` · Omitidos ${s}.` : '.'),
      )
      clearSelection()
      setBulkManagerId('')
      queryClient.invalidateQueries({ queryKey: ['employer', 'employees'] })
    } catch (err: any) {
      showError(err?.response?.data?.error ?? 'No se pudo asignar el manager.')
    } finally {
      setBulkBusy(false)
    }
  }

  const handleBulkDelete = async () => {
    if (selectedIds.size === 0) return
    const ok = await confirm({
      title: 'Eliminar profesionales',
      message: `¿Eliminar ${selectedIds.size} profesional(es) seleccionado(s)? Esta acción no se puede deshacer. Los managers con equipo a cargo se omitirán (reasigna su equipo primero).`,
      confirmLabel: 'Eliminar',
      variant: 'danger',
    })
    if (!ok) return
    setBulkBusy(true)
    try {
      const res = await employerService.bulkDeleteUsers(Array.from(selectedIds))
      const deleted = res?.deleted ?? 0
      const skipped = res?.skipped ?? []
      if (skipped.length > 0) {
        const detail = skipped.map((s) => `${s.name || `#${s.id}`} (${s.reason})`).join(', ')
        showSuccess(`Eliminados ${deleted}. Omitidos ${skipped.length}: ${detail}`)
      } else {
        showSuccess(`Eliminados ${deleted} profesional(es).`)
      }
      clearSelection()
      queryClient.invalidateQueries({ queryKey: ['employer', 'employees'] })
    } catch (err: any) {
      showError(err?.response?.data?.error ?? 'No se pudieron eliminar los profesionales.')
    } finally {
      setBulkBusy(false)
    }
  }

  return (
    <div style={{ padding: '1.5rem' }}>
      <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', gap: '1rem', flexWrap: 'wrap' }}>
        <div>
          <h1 style={{ margin: 0, fontSize: '1.6rem', fontWeight: 700, color: '#0f172a' }}>Profesionales</h1>
          <p style={{ margin: '4px 0 1.25rem', color: '#64748b', fontSize: '0.9rem' }}>
            {companyName} · {employees.length} profesional(es) · {totalManagers} manager(s)
          </p>
        </div>
        <div style={{ display: 'flex', gap: 8, flexShrink: 0 }}>
          <div ref={actionsRef} style={{ position: 'relative' }}>
            <button
              type="button"
              onClick={() => setShowActions((v) => !v)}
              style={{ display: 'inline-flex', alignItems: 'center', gap: 6, padding: '0.6rem 1rem', borderRadius: 10, border: '1px solid #e2e8f0', background: '#fff', color: '#475569', fontWeight: 600, fontSize: '0.9rem', cursor: 'pointer' }}
            >
              Acciones
              <ChevronDown size={16} style={{ transform: showActions ? 'rotate(180deg)' : 'none', transition: 'transform 0.15s ease' }} />
            </button>
            {showActions && (
              <div
                style={{ position: 'absolute', top: 'calc(100% + 6px)', right: 0, zIndex: 20, minWidth: 180, background: '#fff', border: '1px solid #e2e8f0', borderRadius: 12, boxShadow: '0 12px 28px rgba(15,23,42,0.12)', padding: 6, display: 'flex', flexDirection: 'column', gap: 2 }}
              >
                <button
                  type="button"
                  onClick={() => { setShowActions(false); setShowImport(true) }}
                  style={{ display: 'flex', alignItems: 'center', gap: 10, width: '100%', padding: '0.6rem 0.75rem', borderRadius: 8, border: 'none', background: 'transparent', color: '#6d28d9', fontWeight: 600, fontSize: '0.9rem', cursor: 'pointer', textAlign: 'left' }}
                  onMouseEnter={(e) => (e.currentTarget.style.background = '#f5f3ff')}
                  onMouseLeave={(e) => (e.currentTarget.style.background = 'transparent')}
                >
                  <UploadCloud size={16} /> Importar
                </button>
                <button
                  type="button"
                  onClick={() => { setShowActions(false); setShowExport(true) }}
                  style={{ display: 'flex', alignItems: 'center', gap: 10, width: '100%', padding: '0.6rem 0.75rem', borderRadius: 8, border: 'none', background: 'transparent', color: '#475569', fontWeight: 600, fontSize: '0.9rem', cursor: 'pointer', textAlign: 'left' }}
                  onMouseEnter={(e) => (e.currentTarget.style.background = '#f8fafc')}
                  onMouseLeave={(e) => (e.currentTarget.style.background = 'transparent')}
                >
                  <Download size={16} /> Exportar
                </button>
              </div>
            )}
          </div>
          <button
            type="button"
            onClick={openCreate}
            style={{ display: 'inline-flex', alignItems: 'center', gap: 6, padding: '0.6rem 1rem', borderRadius: 10, border: 'none', background: '#2563eb', color: '#fff', fontWeight: 600, fontSize: '0.9rem', cursor: 'pointer' }}
          >
            <UserPlus size={16} /> Crear profesional
          </button>
        </div>
      </div>

      {/* Buscador + filtro por rol */}
      <div style={{ display: 'flex', gap: 10, alignItems: 'center', marginBottom: '1.25rem', flexWrap: 'wrap' }}>
        <div style={{ position: 'relative', flex: '1 1 320px', maxWidth: 420 }}>
          <Search size={18} style={{ position: 'absolute', left: 12, top: '50%', transform: 'translateY(-50%)', color: '#94a3b8' }} />
          <input
            type="text"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Buscar por nombre o email..."
            style={{ width: '100%', padding: '0.65rem 0.75rem 0.65rem 2.4rem', borderRadius: 10, border: '1px solid #e2e8f0', fontSize: '0.95rem', outline: 'none', boxSizing: 'border-box' }}
          />
        </div>
        <div style={{ minWidth: 190 }}>
          <Select
            value={roleFilter}
            onChange={(v) => setRoleFilter(v as 'all' | 'manager' | 'profesional')}
            ariaLabel="Filtrar por rol"
            options={[
              { value: 'all', label: 'Todos los roles' },
              { value: 'profesional', label: 'Profesionales' },
              { value: 'manager', label: 'Managers' },
            ]}
          />
        </div>
      </div>

      {/* Barra de acciones masivas */}
      {selectedIds.size > 0 && (
        <div style={{ display: 'flex', alignItems: 'center', gap: 12, flexWrap: 'wrap', padding: '0.7rem 1rem', marginBottom: '1rem', borderRadius: 12, background: '#eef2ff', border: '1px solid #c7d2fe' }}>
          <span style={{ fontWeight: 700, color: '#3730a3', fontSize: '0.9rem' }}>
            {selectedIds.size} seleccionado(s)
          </span>
          <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
            <select
              value={bulkManagerId}
              onChange={(e) => setBulkManagerId(e.target.value)}
              disabled={bulkBusy}
              style={{ padding: '0.5rem 0.6rem', borderRadius: 8, border: '1px solid #c7d2fe', background: '#fff', fontSize: '0.85rem', color: '#334155', cursor: 'pointer' }}
            >
              <option value="">Asignar manager…</option>
              {managers.map((m) => (
                <option key={m.id} value={String(m.id)}>{m.name}</option>
              ))}
              <option value="__unassign__">Quitar manager</option>
            </select>
            <button
              type="button"
              onClick={handleBulkAssign}
              disabled={bulkBusy || bulkManagerId === ''}
              style={{ padding: '0.5rem 0.9rem', borderRadius: 8, border: 'none', background: bulkManagerId === '' ? '#c7d2fe' : '#4f46e5', color: '#fff', fontWeight: 600, fontSize: '0.85rem', cursor: bulkManagerId === '' ? 'not-allowed' : 'pointer' }}
            >
              Asignar
            </button>
          </div>
          <button
            type="button"
            onClick={handleBulkDelete}
            disabled={bulkBusy}
            style={{ display: 'inline-flex', alignItems: 'center', gap: 6, padding: '0.5rem 0.9rem', borderRadius: 8, border: '1px solid #fecaca', background: '#fff', color: '#dc2626', fontWeight: 600, fontSize: '0.85rem', cursor: 'pointer' }}
          >
            <Trash2 size={15} /> Eliminar
          </button>
          <button
            type="button"
            onClick={clearSelection}
            disabled={bulkBusy}
            style={{ display: 'inline-flex', alignItems: 'center', gap: 4, padding: '0.5rem 0.7rem', borderRadius: 8, border: '1px solid #e2e8f0', background: '#fff', color: '#64748b', fontWeight: 600, fontSize: '0.85rem', cursor: 'pointer', marginLeft: 'auto' }}
          >
            <X size={15} /> Limpiar
          </button>
        </div>
      )}

      {isLoading ? (
        <div style={{ display: 'grid', gap: '0.5rem' }}>
          {Array.from({ length: 5 }).map((_, i) => <Skeleton key={i} height={48} />)}
        </div>
      ) : isError ? (
        <div style={{ padding: '2rem', textAlign: 'center', color: '#ef4444' }}>No se pudieron cargar los profesionales.</div>
      ) : (
        <div className={styles['users-table']}>
          <table>
            <thead>
              <tr>
                <th style={{ width: 40, textAlign: 'center' }}>
                  <input
                    type="checkbox"
                    checked={allSelected}
                    onChange={toggleSelectAll}
                    disabled={selectableIds.length === 0}
                    title="Seleccionar todos"
                    style={{ cursor: 'pointer' }}
                  />
                </th>
                <th>Profesional</th>
                <th>Email</th>
                <th>Tipo</th>
                <th>Estado</th>
                <th>Acciones</th>
              </tr>
            </thead>
            <tbody>
              {filtered.length === 0 && (
                <tr>
                  <td colSpan={6} style={{ textAlign: 'center', padding: '32px 16px', color: '#94a3b8' }}>
                    {search || roleFilter !== 'all'
                      ? 'No hay profesionales que coincidan con el filtro.'
                      : 'Aún no hay profesionales en tu empresa.'}
                  </td>
                </tr>
              )}
              {filtered.map((u) => (
                <tr key={u.id}>
                  <td style={{ textAlign: 'center' }}>
                    {isBulkSelectable(u) && (
                      <input
                        type="checkbox"
                        checked={selectedIds.has(u.id)}
                        onChange={() => toggleSelect(u.id)}
                        style={{ cursor: 'pointer' }}
                      />
                    )}
                  </td>
                  <td>
                    <div className={styles['user-cell']}>
                      <Avatar src={u.avatar} name={u.name} size="sm" />
                      <span>{u.name}</span>
                    </div>
                  </td>
                  <td>{u.email}</td>
                  <td>
                    <span className={`${styles['badge']} ${styles[u.user_type] || ''}`}>
                      {u.is_manager ? 'Manager' : u.user_type}
                    </span>
                  </td>
                  <td>
                    <span className={`${styles['status-badge']} ${u.is_active ? styles['active'] : styles['inactive']}`}>
                      {u.is_active ? 'Activo' : 'Inactivo'}
                    </span>
                  </td>
                  <td>
                    <button
                      type="button"
                      onClick={() => navigate(`/empresa/employees/${u.id}`)}
                      title="Ver detalle"
                      style={{ display: 'inline-flex', alignItems: 'center', gap: 6, padding: '0.45rem 0.9rem', borderRadius: 8, border: '1px solid #e2e8f0', background: '#f8fafc', color: '#334155', fontWeight: 600, fontSize: '0.85rem', cursor: 'pointer' }}
                    >
                      <Eye size={15} /> Ver
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {showImport && (
        <ImportUsersModal
          employerMode
          onClose={() => setShowImport(false)}
          onDone={() => queryClient.invalidateQueries({ queryKey: ['employer', 'employees'] })}
        />
      )}

      {showExport && (
        <ExportUsersModal
          users={filtered}
          companyName={() => companyName}
          filtered={search.trim().length > 0}
          onClose={() => setShowExport(false)}
        />
      )}

      {showCreate && !tempPassword && (
        <CreateUserModal
          employerMode
          form={form}
          setForm={setForm}
          onClose={closeCreate}
          onSubmit={handleCreate}
          loading={creating}
          error={createErr ?? undefined}
          publicCompanies={[]}
          managers={managers}
        />
      )}

      {tempPassword && (
        <Modal
          isOpen
          onClose={() => { setShowCreate(false); setTempPassword(null) }}
          title="Profesional creado"
          size="md"
          footer={<Button onClick={() => { setShowCreate(false); setTempPassword(null) }}>Listo</Button>}
        >
          <div style={{ display: 'flex', flexDirection: 'column', gap: '0.85rem' }}>
            <div style={{ padding: '0.7rem 0.9rem', borderRadius: 10, background: 'rgba(16,185,129,0.1)', color: '#059669', fontSize: '0.88rem', fontWeight: 600 }}>
              Profesional creado correctamente.
            </div>
            <div>
              <label style={{ display: 'block', fontSize: '0.8rem', fontWeight: 600, color: '#334155', marginBottom: '6px' }}>Contraseña temporal</label>
              <div style={{ display: 'flex', gap: '8px', alignItems: 'center' }}>
                <code style={{ flex: 1, padding: '0.6rem 0.75rem', borderRadius: 8, border: '1px solid #e2e8f0', background: '#f8fafc', fontSize: '0.95rem', fontFamily: 'monospace', userSelect: 'all', wordBreak: 'break-all' }}>{tempPassword}</code>
                <Button type="button" variant="secondary" onClick={copyTempPassword}>
                  {copied ? <Check size={15} /> : <Copy size={15} />} {copied ? 'Copiada' : 'Copiar'}
                </Button>
              </div>
            </div>
            <p style={{ margin: 0, fontSize: '0.8rem', color: '#b45309', fontWeight: 600 }}>
              Compártela por un canal seguro; no se vuelve a mostrar.
            </p>
          </div>
        </Modal>
      )}
    </div>
  )
}
