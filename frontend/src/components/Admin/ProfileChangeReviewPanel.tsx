import { useEffect, useState } from 'react'
import { profileChangeService } from '../../services/api'
import { Button } from '../ui'
import type { ProfileChangeRequest, User } from '../../types'

const FIELD_LABELS: Record<string, string> = {
  name: 'Nombre',
  phone_number: 'Teléfono',
  country: 'País',
  state: 'Provincia / Estado',
  city: 'Ciudad',
  location: 'Dirección',
  job_title: 'Puesto / Cargo',
  identity_document: 'Documento de identidad',
}

function currentOf(user: User, field: string): string {
  const map: Record<string, string | undefined> = {
    name: user.name,
    phone_number: user.phone_number,
    country: user.country,
    state: user.state,
    city: user.city,
    location: user.location,
    job_title: user.job_title,
    identity_document: user.identity_document,
  }
  return map[field] || ''
}

interface Props {
  userId: number
  user: User
  canReview: boolean
  onApplied?: () => void
}

export function ProfileChangeReviewPanel({ userId, user, canReview, onApplied }: Props) {
  const [req, setReq] = useState<ProfileChangeRequest | null>(null)
  const [values, setValues] = useState<Record<string, string>>({})
  const [busy, setBusy] = useState(false)
  const [msg, setMsg] = useState('')
  const [rejecting, setRejecting] = useState(false)
  const [reason, setReason] = useState('')

  useEffect(() => {
    if (!canReview) return
    let cancelled = false
    profileChangeService.getForUser(userId).then(r => {
      if (cancelled) return
      setReq(r)
      if (r) {
        try { setValues(JSON.parse(r.changes) || {}) } catch { setValues({}) }
      }
    }).catch(() => {})
    return () => { cancelled = true }
  }, [userId, canReview])

  if (!canReview || !req) return null

  const fields = Object.keys(values)

  const apply = async () => {
    setBusy(true); setMsg('')
    try {
      await profileChangeService.apply(req.id, values)
      setReq(null)
      onApplied?.()
    } catch (e: any) {
      setMsg(e?.response?.data?.error || e?.message || 'No se pudo aplicar')
    } finally { setBusy(false) }
  }

  const reject = async () => {
    setBusy(true); setMsg('')
    try {
      await profileChangeService.reject(req.id, reason)
      setReq(null)
      onApplied?.()
    } catch (e: any) {
      setMsg(e?.response?.data?.error || e?.message || 'No se pudo rechazar')
    } finally { setBusy(false) }
  }

  const labelStyle: React.CSSProperties = { fontSize: '0.78rem', fontWeight: 600, color: '#92400e' }
  const currentStyle: React.CSSProperties = { fontSize: '0.8rem', color: '#94a3b8' }
  const inputStyle: React.CSSProperties = { width: '100%', padding: '8px 10px', borderRadius: '8px', border: '1px solid #cbd5e1', fontSize: '0.85rem' }

  return (
    <div style={{ marginTop: '1rem', padding: '1rem 1.25rem', borderRadius: '12px', border: '1px solid #fcd34d', background: '#fffbeb' }}>
      <h3 style={{ margin: 0, fontSize: '1rem', color: '#92400e' }}>Solicitud de actualización de datos</h3>
      <p style={{ margin: '6px 0 14px', fontSize: '0.83rem', color: '#a16207' }}>
        El profesional solicitó estos cambios. Revísalos, ajusta si es necesario y aplícalos.
      </p>
      {req.note && (
        <p style={{ margin: '0 0 14px', fontSize: '0.83rem', color: '#78350f' }}><strong>Motivo:</strong> {req.note}</p>
      )}

      <div style={{ display: 'flex', flexDirection: 'column', gap: '12px' }}>
        {fields.map(f => (
          <div key={f} style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
            <span style={labelStyle}>{FIELD_LABELS[f] || f}</span>
            <span style={currentStyle}>Actual: {currentOf(user, f) || '(vacío)'}</span>
            {f === 'identity_document' ? (
              <a href={values[f]} target="_blank" rel="noopener noreferrer" style={{ fontSize: '0.85rem', color: '#8b5cf6' }}>
                Ver documento propuesto
              </a>
            ) : (
              <input style={inputStyle} value={values[f] ?? ''} onChange={e => setValues({ ...values, [f]: e.target.value })} />
            )}
          </div>
        ))}
      </div>

      {msg && <div style={{ marginTop: '10px', color: '#dc2626', fontSize: '0.82rem' }}>{msg}</div>}

      {!rejecting ? (
        <div style={{ display: 'flex', gap: '8px', marginTop: '16px' }}>
          <Button variant="primary" onClick={apply} loading={busy}>Aplicar cambios</Button>
          <Button variant="secondary" onClick={() => setRejecting(true)} disabled={busy}>Rechazar</Button>
        </div>
      ) : (
        <div style={{ marginTop: '16px' }}>
          <textarea
            value={reason}
            onChange={e => setReason(e.target.value)}
            rows={2}
            placeholder="Motivo del rechazo (opcional)"
            style={{ ...inputStyle, resize: 'vertical' }}
          />
          <div style={{ display: 'flex', gap: '8px', marginTop: '8px' }}>
            <Button variant="danger" onClick={reject} loading={busy}>Confirmar rechazo</Button>
            <Button variant="secondary" onClick={() => setRejecting(false)} disabled={busy}>Cancelar</Button>
          </div>
        </div>
      )}
    </div>
  )
}
