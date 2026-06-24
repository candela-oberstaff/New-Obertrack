import { useMemo, useState } from 'react'
import { UploadCloud, X, FileSpreadsheet, Download, AlertCircle, CheckCircle2, Loader2 } from 'lucide-react'
import { adminService, employerService } from '../../../services/api'

type ImportStatus = 'ok' | 'error' | 'conflict'
interface ImportRow {
  row: number
  data: Record<string, string>
  status: ImportStatus
  message?: string
  existing?: { id: number; name: string; email: string }
}
interface PreviewData {
  companies?: ImportRow[]
  professionals: ImportRow[]
  summary: {
    companies?: { ok: number; error: number; conflict: number; total: number }
    professionals: { ok: number; error: number; conflict: number; total: number }
  }
}
type Action = 'create' | 'overwrite' | 'skip'
interface Cred { name: string; email: string; company: string; temp_password: string }
interface ExecResult {
  companies?: { created: number; updated: number; skipped: number; errors: { email: string; error: string }[] }
  professionals: { created: number; updated: number; skipped: number; errors: { email: string; error: string }[] }
  credentials: Cred[]
}

const overlay: React.CSSProperties = { position: 'fixed', inset: 0, zIndex: 1000, display: 'flex', alignItems: 'center', justifyContent: 'center', background: 'rgba(15,23,42,0.5)' }
const panel: React.CSSProperties = { background: '#fff', borderRadius: 20, padding: 28, width: '100%', maxWidth: 940, maxHeight: '90vh', overflowY: 'auto', boxShadow: '0 25px 50px -12px rgba(0,0,0,0.25)' }
const labelStyle: React.CSSProperties = { fontSize: 13, fontWeight: 700, color: '#334155' }

function StatusPill({ row }: { row: ImportRow }) {
  if (row.status === 'ok') return <span style={{ ...pill, background: '#d1fae5', color: '#059669' }}>Nuevo</span>
  if (row.status === 'conflict') return <span style={{ ...pill, background: '#fef3c7', color: '#b45309' }}>Ya existe</span>
  return <span style={{ ...pill, background: '#fee2e2', color: '#dc2626' }}>Error</span>
}
const pill: React.CSSProperties = { display: 'inline-block', padding: '2px 9px', borderRadius: 999, fontSize: 11, fontWeight: 700 }

export function ImportUsersModal({ onClose, onDone, employerMode = false }: { onClose: () => void; onDone?: () => void; employerMode?: boolean }) {
  const svc = employerMode ? employerService : adminService
  const [step, setStep] = useState<'upload' | 'preview' | 'result'>('upload')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [fileName, setFileName] = useState('')
  const [preview, setPreview] = useState<PreviewData | null>(null)
  const [decisions, setDecisions] = useState<Record<string, Action>>({})
  const [result, setResult] = useState<ExecResult | null>(null)

  const handleFile = async (file: File) => {
    setBusy(true); setError(null)
    try {
      const data: PreviewData = await svc.importPreview(file)
      setPreview(data)
      setFileName(file.name)
      const d: Record<string, Action> = {}
      ;(data.companies ?? []).forEach(r => { if (r.status === 'conflict') d[`emp-${r.row}`] = 'skip' })
      data.professionals.forEach(r => { if (r.status === 'conflict') d[`pro-${r.row}`] = 'skip' })
      setDecisions(d)
      setStep('preview')
    } catch (err: any) {
      setError(err?.response?.data?.error ?? 'No se pudo leer el archivo.')
    } finally {
      setBusy(false)
    }
  }

  const actionFor = (entity: 'emp' | 'pro', r: ImportRow): Action => {
    if (r.status === 'error') return 'skip'
    if (r.status === 'conflict') return decisions[`${entity}-${r.row}`] ?? 'skip'
    return 'create'
  }

  const toExecRows = (entity: 'emp' | 'pro', rows: ImportRow[]) =>
    rows
      .filter(r => r.status !== 'error')
      .map(r => ({ action: actionFor(entity, r), data: r.data }))
      .filter(r => r.action !== 'skip')

  const counts = useMemo(() => {
    if (!preview) return { create: 0, overwrite: 0 }
    let create = 0, overwrite = 0
    ;([['emp', preview.companies ?? []], ['pro', preview.professionals]] as const).forEach(([e, rows]) =>
      rows.forEach(r => {
        const a = actionFor(e, r)
        if (a === 'create') create++
        if (a === 'overwrite') overwrite++
      }),
    )
    return { create, overwrite }
  }, [preview, decisions])

  const handleExecute = async () => {
    if (!preview) return
    setBusy(true); setError(null)
    try {
      const res: ExecResult = await svc.importExecute({
        companies: preview.companies ? toExecRows('emp', preview.companies) : [],
        professionals: toExecRows('pro', preview.professionals),
      })
      setResult(res)
      setStep('result')
      onDone?.()
    } catch (err: any) {
      setError(err?.response?.data?.error ?? 'No se pudo ejecutar la importación.')
    } finally {
      setBusy(false)
    }
  }

  const downloadCreds = () => {
    if (!result?.credentials?.length) return
    const rows = employerMode
      ? [['Nombre', 'Email', 'Contraseña temporal'], ...result.credentials.map(c => [c.name, c.email, c.temp_password])]
      : [['Nombre', 'Email', 'Empresa', 'Contraseña temporal'], ...result.credentials.map(c => [c.name, c.email, c.company, c.temp_password])]
    const csv = '﻿' + rows.map(r => r.map(v => `"${String(v).replace(/"/g, '""')}"`).join(',')).join('\n')
    const blob = new Blob([csv], { type: 'text/csv;charset=utf-8;' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url; a.download = 'credenciales_importacion.csv'
    document.body.appendChild(a); a.click(); a.remove(); URL.revokeObjectURL(url)
  }

  return (
    <div style={overlay} onClick={onClose}>
      <div style={panel} onClick={e => e.stopPropagation()}>
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 20 }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
            <div style={{ width: 40, height: 40, borderRadius: 12, background: 'linear-gradient(135deg, var(--primary), var(--primary-dark, #1d4ed8))', display: 'flex', alignItems: 'center', justifyContent: 'center', color: '#fff' }}>
              <UploadCloud size={20} />
            </div>
            <div>
              <h2 style={{ margin: 0, fontSize: 20, fontWeight: 800, color: '#0f172a' }}>{employerMode ? 'Importar profesionales' : 'Importar desde Excel'}</h2>
              <p style={{ margin: 0, fontSize: 13, color: '#64748b' }}>{employerMode ? 'A tu empresa, desde un Excel' : 'Empresas y profesionales'}</p>
            </div>
          </div>
          <button onClick={onClose} style={{ background: 'none', border: 'none', cursor: 'pointer', color: '#94a3b8' }}><X size={22} /></button>
        </div>

        {error && (
          <div style={{ background: 'rgba(239,68,68,0.12)', border: '1px solid rgba(239,68,68,0.3)', color: '#dc2626', padding: '11px 14px', borderRadius: 10, marginBottom: 16, fontSize: 13, fontWeight: 600 }}>{error}</div>
        )}

        {step === 'upload' && (
          <div>
            <label
              style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 10, padding: '40px 20px', border: '2px dashed #cbd5e1', borderRadius: 16, background: '#f8fafc', cursor: busy ? 'progress' : 'pointer', textAlign: 'center' }}
            >
              {busy ? <Loader2 size={32} className="animate-spin" color="#6d28d9" /> : <FileSpreadsheet size={32} color="#6d28d9" />}
              <span style={{ fontWeight: 700, color: '#334155' }}>{busy ? 'Leyendo archivo...' : 'Hacé clic para elegir tu archivo .xlsx'}</span>
              <span style={{ fontSize: 13, color: '#94a3b8' }}>Usá la plantilla para asegurar el formato correcto.</span>
              <input
                type="file"
                accept=".xlsx"
                disabled={busy}
                style={{ display: 'none' }}
                onChange={e => { const f = e.target.files?.[0]; if (f) handleFile(f) }}
              />
            </label>
            <button
              type="button"
              onClick={() => svc.downloadImportTemplate()}
              style={{ marginTop: 14, display: 'inline-flex', alignItems: 'center', gap: 6, padding: '9px 14px', border: '1px solid #ddd6fe', borderRadius: 10, background: '#fff', color: '#6d28d9', fontWeight: 700, fontSize: 13, cursor: 'pointer' }}
            >
              <Download size={15} /> Descargar plantilla
            </button>
          </div>
        )}

        {step === 'preview' && preview && (
          <div>
            <p style={{ margin: '0 0 14px', fontSize: 13, color: '#64748b' }}>
              Archivo: <strong>{fileName}</strong>. Revisá las filas; en los conflictos elegí sobreescribir u omitir. Las filas con error se omiten.
            </p>
            {preview.companies && (
              <PreviewTable
                title="Empresas"
                entity="emp"
                rows={preview.companies}
                nameKey="nombre_responsable"
                companyKey="nombre_empresa"
                showCompany
                decisions={decisions}
                setDecision={(row, a) => setDecisions(d => ({ ...d, [`emp-${row}`]: a }))}
              />
            )}
            <PreviewTable
              title="Profesionales"
              entity="pro"
              rows={preview.professionals}
              nameKey="nombre"
              companyKey="empresa"
              showCompany={!employerMode}
              decisions={decisions}
              setDecision={(row, a) => setDecisions(d => ({ ...d, [`pro-${row}`]: a }))}
            />

            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 12, marginTop: 20, flexWrap: 'wrap' }}>
              <span style={{ fontSize: 13, color: '#475569', fontWeight: 600 }}>
                Se crearán <strong>{counts.create}</strong> y se sobreescribirán <strong>{counts.overwrite}</strong>.
              </span>
              <div style={{ display: 'flex', gap: 10 }}>
                <button type="button" onClick={() => { setStep('upload'); setPreview(null) }} disabled={busy} style={btnSecondary}>Atrás</button>
                <button type="button" onClick={handleExecute} disabled={busy || (counts.create + counts.overwrite === 0)} style={btnPrimary}>
                  {busy ? 'Importando...' : `Importar (${counts.create + counts.overwrite})`}
                </button>
              </div>
            </div>
          </div>
        )}

        {step === 'result' && result && (
          <div>
            <div style={{ display: 'flex', alignItems: 'center', gap: 10, padding: '12px 14px', borderRadius: 12, background: 'rgba(16,185,129,0.1)', color: '#059669', fontWeight: 700, marginBottom: 16 }}>
              <CheckCircle2 size={20} /> Importación finalizada
            </div>
            <div style={{ display: 'grid', gridTemplateColumns: result.companies ? '1fr 1fr' : '1fr', gap: 12, marginBottom: 16 }}>
              {result.companies && <ResultCard title="Empresas" r={result.companies} />}
              <ResultCard title="Profesionales" r={result.professionals} />
            </div>

            {((result.companies?.errors.length ?? 0) > 0 || result.professionals.errors.length > 0) && (
              <div style={{ marginBottom: 16 }}>
                <div style={{ ...labelStyle, marginBottom: 6, color: '#b91c1c', display: 'flex', alignItems: 'center', gap: 6 }}><AlertCircle size={15} /> Errores</div>
                <div style={{ maxHeight: 140, overflowY: 'auto', border: '1px solid #fee2e2', borderRadius: 10, padding: 10 }}>
                  {[...(result.companies?.errors ?? []), ...result.professionals.errors].map((e, i) => (
                    <div key={i} style={{ fontSize: 12.5, color: '#475569' }}><strong>{e.email}</strong>: {e.error}</div>
                  ))}
                </div>
              </div>
            )}

            {result.credentials.length > 0 && (
              <div style={{ marginBottom: 16 }}>
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 6 }}>
                  <span style={labelStyle}>Contraseñas temporales ({result.credentials.length})</span>
                  <button type="button" onClick={downloadCreds} style={{ ...btnSecondary, padding: '6px 12px', fontSize: 12 }}><Download size={14} /> Descargar CSV</button>
                </div>
                <p style={{ margin: '0 0 8px', fontSize: 12, color: '#b45309', fontWeight: 600 }}>Compartilas por un canal seguro; no se vuelven a mostrar.</p>
                <div style={{ maxHeight: 220, overflowY: 'auto', border: '1px solid #e2e8f0', borderRadius: 10 }}>
                  <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 12.5 }}>
                    <thead>
                      <tr style={{ background: '#f8fafc', textAlign: 'left' }}>
                        <th style={th}>Nombre</th><th style={th}>Email</th>{!employerMode && <th style={th}>Empresa</th>}<th style={th}>Contraseña</th>
                      </tr>
                    </thead>
                    <tbody>
                      {result.credentials.map((c, i) => (
                        <tr key={i} style={{ borderTop: '1px solid #f1f5f9' }}>
                          <td style={td}>{c.name}</td><td style={td}>{c.email}</td>{!employerMode && <td style={td}>{c.company}</td>}
                          <td style={{ ...td, fontFamily: 'monospace', userSelect: 'all' }}>{c.temp_password}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </div>
            )}

            <div style={{ display: 'flex', justifyContent: 'flex-end' }}>
              <button type="button" onClick={onClose} style={btnPrimary}>Listo</button>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}

const th: React.CSSProperties = { padding: '8px 10px', fontWeight: 700, color: '#64748b' }
const td: React.CSSProperties = { padding: '7px 10px', color: '#334155' }
const btnPrimary: React.CSSProperties = { padding: '10px 18px', borderRadius: 12, border: 'none', background: 'linear-gradient(135deg, var(--primary), var(--primary-dark, #1d4ed8))', color: '#fff', fontWeight: 700, fontSize: 14, cursor: 'pointer' }
const btnSecondary: React.CSSProperties = { display: 'inline-flex', alignItems: 'center', gap: 6, padding: '10px 16px', borderRadius: 12, border: '1px solid #e2e8f0', background: '#f8fafc', color: '#475569', fontWeight: 600, fontSize: 14, cursor: 'pointer' }

function ResultCard({ title, r }: { title: string; r: NonNullable<ExecResult['companies']> }) {
  return (
    <div style={{ border: '1px solid #e2e8f0', borderRadius: 12, padding: 14 }}>
      <div style={{ fontWeight: 800, color: '#0f172a', marginBottom: 8 }}>{title}</div>
      <div style={{ display: 'flex', gap: 14, fontSize: 13, color: '#475569', flexWrap: 'wrap' }}>
        <span>✅ Creados: <strong>{r.created}</strong></span>
        <span>♻️ Actualizados: <strong>{r.updated}</strong></span>
        <span>⏭️ Omitidos: <strong>{r.skipped}</strong></span>
        {r.errors.length > 0 && <span style={{ color: '#b91c1c' }}>⚠️ Errores: <strong>{r.errors.length}</strong></span>}
      </div>
    </div>
  )
}

function PreviewTable({
  title, entity, rows, nameKey, companyKey, showCompany, decisions, setDecision,
}: {
  title: string
  entity: 'emp' | 'pro'
  rows: ImportRow[]
  nameKey: string
  companyKey: string
  showCompany: boolean
  decisions: Record<string, Action>
  setDecision: (row: number, a: Action) => void
}) {
  if (rows.length === 0) return null
  return (
    <div style={{ marginBottom: 18 }}>
      <div style={{ ...labelStyle, marginBottom: 8 }}>{title} ({rows.length})</div>
      <div style={{ border: '1px solid #e2e8f0', borderRadius: 10, overflow: 'hidden' }}>
        <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 12.5 }}>
          <thead>
            <tr style={{ background: '#f8fafc', textAlign: 'left' }}>
              <th style={th}>Fila</th><th style={th}>Nombre</th><th style={th}>Email</th>{showCompany && <th style={th}>Empresa</th>}<th style={th}>Estado</th><th style={th}>Acción</th>
            </tr>
          </thead>
          <tbody>
            {rows.map(r => (
              <tr key={r.row} style={{ borderTop: '1px solid #f1f5f9', background: r.status === 'error' ? 'rgba(239,68,68,0.04)' : undefined }}>
                <td style={td}>{r.row}</td>
                <td style={td}>{r.data[nameKey] || '—'}</td>
                <td style={td}>{r.data.email || '—'}</td>
                {showCompany && <td style={td}>{r.data[companyKey] || '—'}</td>}
                <td style={td}>
                  <StatusPill row={r} />
                  {r.status === 'conflict' && r.existing && <div style={{ fontSize: 11, color: '#94a3b8' }}>de {r.existing.name}</div>}
                  {r.status === 'error' && <div style={{ fontSize: 11, color: '#dc2626' }}>{r.message}</div>}
                </td>
                <td style={td}>
                  {r.status === 'ok' && <span style={{ color: '#059669', fontWeight: 600 }}>Crear</span>}
                  {r.status === 'error' && <span style={{ color: '#94a3b8' }}>Se omite</span>}
                  {r.status === 'conflict' && (
                    <select
                      value={decisions[`${entity}-${r.row}`] ?? 'skip'}
                      onChange={e => setDecision(r.row, e.target.value as Action)}
                      style={{ padding: '5px 8px', borderRadius: 8, border: '1px solid #cbd5e1', fontSize: 12.5, background: '#fff' }}
                    >
                      <option value="skip">Omitir</option>
                      <option value="overwrite">Sobreescribir</option>
                    </select>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}
