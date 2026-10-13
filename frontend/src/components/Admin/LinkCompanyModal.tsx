import { useEffect, useState } from 'react'
import { adminService, authService } from '../../services/api'
import { Modal, Button } from '../ui'
import { Select } from '../ui/Select'

interface LinkCompanyModalProps {
  isOpen: boolean
  userId: number
  userName: string
  /** Empresas que no se ofrecen (donde ya trabaja o de la que acaba de salir). */
  excludeCompanyIds?: number[]
  /** Cargo con el que se abre el formulario (el que tenía en la empresa anterior). */
  defaultJobTitle?: string
  /** Empresa de la que viene, para dejarlo escrito como motivo de ingreso. */
  fromCompany?: string
  onClose: () => void
  onLinked: () => void
}

const inputStyle = { width: '100%', padding: '10px 12px', border: '1px solid #cbd5e1', borderRadius: '10px', fontSize: '14px' }
const labelStyle = { display: 'block', fontSize: '0.8rem', fontWeight: 600, color: '#334155', marginBottom: '6px' } as const

/**
 * Vincula a un profesional dado de baja con otra empresa. Abre un empleo nuevo:
 * el de la empresa anterior queda finalizado en su expediente, con su historial
 * congelado, y el nuevo empieza limpio. Si no tenía empresa activa, la nueva
 * pasa a serlo y la persona vuelve a operar con normalidad.
 */
export function LinkCompanyModal({
  isOpen,
  userId,
  userName,
  excludeCompanyIds = [],
  defaultJobTitle = '',
  fromCompany,
  onClose,
  onLinked,
}: LinkCompanyModalProps) {
  const [companies, setCompanies] = useState<{ id: number; name: string }[]>([])
  const [companyId, setCompanyId] = useState<number | ''>('')
  // Managers de todas las empresas; se filtra por la elegida. Sin manager, sus
  // horas no tienen quién las apruebe en la empresa nueva.
  const [managers, setManagers] = useState<{ id: number; name: string; empleador_id?: number | null }[]>([])
  const [managerId, setManagerId] = useState<number | ''>('')
  const [jobTitle, setJobTitle] = useState(defaultJobTitle)
  const [reason, setReason] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!isOpen) return
    setCompanyId('')
    setManagerId('')
    setJobTitle(defaultJobTitle)
    setReason(fromCompany ? `Reubicación tras su salida de ${fromCompany}` : '')
    setError(null)
    if (companies.length === 0) {
      authService.getPublicCompanies().then(setCompanies).catch(() => { /* selector vacío si falla */ })
    }
    if (managers.length === 0) {
      adminService.getUsers({ is_manager: 'true', limit: 1000 })
        .then((res: any) => setManagers(res?.data ?? (Array.isArray(res) ? res : [])))
        .catch(() => { /* sin managers: el campo queda vacío */ })
    }
  }, [isOpen]) // eslint-disable-line react-hooks/exhaustive-deps

  const companyManagers = companyId
    ? managers.filter((m) => m.empleador_id === companyId && m.id !== userId)
    : []

  const submit = async () => {
    if (!companyId) return
    setBusy(true)
    setError(null)
    try {
      await adminService.addUserEmployment(userId, {
        company_id: Number(companyId),
        job_title: jobTitle.trim() || undefined,
        start_reason: reason.trim() || undefined,
        manager_id: managerId ? Number(managerId) : undefined,
      })
      onLinked()
      onClose()
    } catch (err: any) {
      setError(err?.response?.data?.error ?? 'No se pudo vincular a la empresa')
    } finally {
      setBusy(false)
    }
  }

  return (
    <Modal
      isOpen={isOpen}
      isDirty={companyId !== ''}
      onClose={onClose}
      title="Vincular a otra empresa"
      size="sm"
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={busy}>Cancelar</Button>
          <Button onClick={submit} loading={busy} disabled={!companyId}>Vincular</Button>
        </>
      }
    >
      <p style={{ fontSize: '0.85rem', color: '#64748b', marginTop: 0 }}>
        {userName} empezará en la empresa que elijas con un expediente nuevo. Lo de
        {fromCompany ? ` ${fromCompany}` : ' su empresa anterior'} se conserva en su historial.
      </p>
      <div style={{ marginBottom: '14px' }}>
        <label style={labelStyle}>Empresa</label>
        <Select
          fullWidth
          searchable
          value={companyId}
          onChange={(v) => { setCompanyId(v ? Number(v) : ''); setManagerId('') }}
          placeholder="Selecciona una empresa..."
          options={companies
            .filter((c) => !excludeCompanyIds.includes(c.id))
            .map((c) => ({ value: c.id, label: c.name }))}
        />
      </div>
      <div style={{ marginBottom: '14px' }}>
        <label style={labelStyle}>Cargo en esa empresa</label>
        <input type="text" value={jobTitle} onChange={(e) => setJobTitle(e.target.value)} placeholder="Ej: Asistente comercial" style={inputStyle} />
      </div>
      <div style={{ marginBottom: '14px' }}>
        <label style={labelStyle}>Manager en esa empresa</label>
        <Select
          fullWidth
          searchable
          value={managerId}
          onChange={(v) => setManagerId(v ? Number(v) : '')}
          disabled={!companyId}
          placeholder={!companyId ? 'Elige primero la empresa' : companyManagers.length === 0 ? 'Esa empresa no tiene managers' : 'Sin manager por ahora'}
          options={companyManagers.map((m) => ({ value: m.id, label: m.name }))}
        />
        {companyId && !managerId && (
          <small style={{ display: 'block', marginTop: '6px', fontSize: '0.78rem', color: '#b45309' }}>
            Sin manager nadie aprobará sus horas en esa empresa. Puedes asignarlo después desde su ficha.
          </small>
        )}
      </div>
      <div>
        <label style={labelStyle}>Motivo de ingreso (opcional)</label>
        <input type="text" value={reason} onChange={(e) => setReason(e.target.value)} style={inputStyle} />
      </div>
      {error && <p style={{ color: '#dc2626', fontWeight: 600, fontSize: '0.85rem', marginTop: '10px' }}>{error}</p>}
    </Modal>
  )
}
