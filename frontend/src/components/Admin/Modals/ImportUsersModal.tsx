import { useMemo, useState } from 'react'
import { UploadCloud, X, FileSpreadsheet, Download, AlertCircle, CheckCircle2, Loader2, ChevronLeft, ChevronRight } from 'lucide-react'
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

const PAGE_SIZE = 25

const overlay: React.CSSProperties = { position: 'fixed', inset: 0, zIndex: 1000, display: 'flex', alignItems: 'center', justifyContent: 'center', background: 'rgba(15,23,42,0.5)' }
const panel: React.CSSProperties = { background: '#fff', borderRadius: 20, padding: 28, width: '100%', maxWidth: 940, maxHeight: '90vh', overflowY: 'auto', boxShadow: '0 25px 50px -12px rgba(0,0,0,0.25)' }
const labelStyle: React.CSSProperties = { fontSize: 13, fontWeight: 700, color: '#334155' }

function StatusPill({ row }: { row: ImportRow }) {
  if (row.status === 'ok') return <span style={{ ...pill, background: '#d1fae5', color: '#059669' }}>Nuevo</span>
  if (row.status === 'conflict') {
    const label = row.existing ? 'Ya existe' : 'Duplicada'
    return <span style={{ ...pill, background: '#fef3c7', color: '#b45309' }}>{label}</span>
  }
  return <span style={{ ...pill, background: '#fee2e2', color: '#dc2626' }}>Error</span>
}
const pill: React.CSSProperties = { display: 'inline-block', padding: '2px 9px', borderRadius: 999, fontSize: 11, fontWeight: 700 }

const isManagerValue = (v?: string) =>
  ['si', 'sí', 'yes', 'y', 'true', '1', 'x', 'verdadero'].includes((v ?? '').trim().toLowerCase())

const downloadCSV = (rows: string[][], filename: string) => {
  const csv = '﻿' + rows.map(r => r.map(v => `"${String(v).replace(/"/g, '""')}"`).join(',')).join('\n')
  const blob = new Blob([csv], { type: 'text/csv;charset=utf-8;' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url; a.download = filename
  document.body.appendChild(a); a.click(); a.remove(); URL.revokeObjectURL(url)
}

export function ImportUsersModal({ onClose, onDone, employerMode = false }: { onClose: () => void; onDone?: () => void; employerMode?: boolean }) {
  const svc = employerMode ? employerService : adminService
  const [step, setStep] = useState<'upload' | 'preview' | 'errors' | 'result'>('upload')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [fileName, setFileName] = useState('')
  const [preview, setPreview] = useState<PreviewData | null>(null)
  const [decisions, setDecisions] = useState<Record<string, Action>>({})
  const [result, setResult] = useState<ExecResult | null>(null)

  const handleFile = async (file: File) => {
    setBusy(true); setError(null)
    try {
      const raw: PreviewData = await svc.importPreview(file)
      // Go serializa un slice vacío como `null`, no `[]`. Si una hoja no trae
      // filas, el campo llega null; normalizamos para que todo lo de abajo pueda
      // tratar siempre con arreglos y no explote con "Cannot read ... of null".
      const data: PreviewData = {
        ...raw,
        companies: Array.isArray(raw.companies) ? raw.companies : undefined,
        professionals: Array.isArray(raw.professionals) ? raw.professionals : [],
      }
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

  // Las filas con error nunca se importan, así que viven en su propia pantalla y
  // no ensucian la revisión de las que sí van a entrar.
  const { okCompanies, okProfessionals, errCompanies, errProfessionals, errTotal } = useMemo(() => {
    const companies = preview?.companies ?? []
    const professionals = preview?.professionals ?? []
    const errC = companies.filter(r => r.status === 'error')
    const errP = professionals.filter(r => r.status === 'error')
    return {
      okCompanies: preview?.companies ? companies.filter(r => r.status !== 'error') : undefined,
      okProfessionals: professionals.filter(r => r.status !== 'error'),
      errCompanies: errC,
      errProfessionals: errP,
      errTotal: errC.length + errP.length,
    }
  }, [preview])

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
    ;([['emp', preview.companies ?? []], ['pro', preview.professionals ?? []]] as const).forEach(([e, rows]) =>
      rows.forEach(r => {
        const a = actionFor(e, r)
        if (a === 'create') create++
        if (a === 'overwrite') overwrite++
      }),
    )
    return { create, overwrite }
  }, [preview, decisions])

  const totalToImport = counts.create + counts.overwrite

  // Las filas con error y las que el usuario marcó "Omitir" se filtran antes de
  // llamar al backend, así que él las reporta como 0. El conteo real es de acá.
  const omitted = useMemo(() => {
    const tally = (entity: 'emp' | 'pro', rows: ImportRow[]) => {
      let skipped = 0, errored = 0
      rows.forEach(r => {
        if (r.status === 'error') errored++
        else if (actionFor(entity, r) === 'skip') skipped++
      })
      return { skipped, errored }
    }
    return {
      companies: tally('emp', preview?.companies ?? []),
      professionals: tally('pro', preview?.professionals ?? []),
    }
  }, [preview, decisions])

  const handleExecute = async () => {
    if (!preview) return
    setBusy(true); setError(null)
    try {
      const res: ExecResult = await svc.importExecute({
        companies: preview.companies ? toExecRows('emp', preview.companies) : [],
        professionals: toExecRows('pro', preview.professionals ?? []),
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
    downloadCSV(
      employerMode
        ? [['Nombre', 'Email', 'Contraseña temporal'], ...result.credentials.map(c => [c.name, c.email, c.temp_password])]
        : [['Nombre', 'Email', 'Empresa', 'Contraseña temporal'], ...result.credentials.map(c => [c.name, c.email, c.company, c.temp_password])],
      'credenciales_importacion.csv',
    )
  }

  const downloadErrors = async () => {
    setError(null)
    try {
      // Carga diferida: la librería de xlsx solo pesa en el bundle si se usa.
      const { default: writeXlsxFile } = await import('write-excel-file/browser')
      const header = ['Fila', 'Tipo', 'Nombre', 'Email', 'Empresa', 'Motivo'].map(value => ({
        value, type: String, fontWeight: 'bold' as const, backgroundColor: '#FEF2F2', color: '#B91C1C',
      }))
      const toRow = (tipo: string, nameKey: string, companyKey: string) => (r: ImportRow) => [
        { value: r.row, type: Number },
        { value: tipo, type: String },
        { value: r.data[nameKey] || '', type: String },
        { value: r.data.email || '', type: String },
        { value: r.data[companyKey] || '', type: String },
        { value: r.message || 'Fila inválida', type: String },
      ]
      const { toFile } = await writeXlsxFile(
        [
          header,
          ...errCompanies.map(toRow('Empresa', 'nombre_responsable', 'nombre_empresa')),
          ...errProfessionals.map(toRow('Profesional', 'nombre', 'empresa')),
        ],
        {
          sheet: 'Errores',
          stickyRowsCount: 1,
          columns: [{ width: 8 }, { width: 14 }, { width: 28 }, { width: 34 }, { width: 30 }, { width: 52 }],
        },
      )
      await toFile('filas_con_error.xlsx')
    } catch {
      setError('No se pudo generar el archivo de errores.')
    }
  }

  const importButton = (
    <button type="button" onClick={handleExecute} disabled={busy || totalToImport === 0} style={btnPrimary}>
      {busy ? 'Importando...' : `Importar (${totalToImport})`}
    </button>
  )

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
              Archivo: <strong>{fileName}</strong>. Revisá las filas; en los conflictos elegí sobreescribir u omitir.
            </p>

            {errTotal > 0 && (
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 12, flexWrap: 'wrap', background: 'rgba(239,68,68,0.08)', border: '1px solid rgba(239,68,68,0.25)', borderRadius: 12, padding: '11px 14px', marginBottom: 16 }}>
                <span style={{ display: 'flex', alignItems: 'center', gap: 8, fontSize: 13, color: '#b91c1c', fontWeight: 600 }}>
                  <AlertCircle size={16} />
                  {errTotal === 1 ? '1 fila no se puede importar' : `${errTotal} filas no se pueden importar`} y quedaron fuera de esta lista.
                </span>
                <button type="button" onClick={() => setStep('errors')} style={{ ...btnSecondary, padding: '6px 12px', fontSize: 12.5, borderColor: '#fecaca', color: '#b91c1c' }}>Ver detalle</button>
              </div>
            )}

            {okCompanies && (
              <PreviewTable
                title="Empresas"
                entity="emp"
                rows={okCompanies}
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
              rows={okProfessionals}
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
                {errTotal > 0
                  ? <button type="button" onClick={() => setStep('errors')} disabled={busy} style={btnPrimary}>Continuar</button>
                  : importButton}
              </div>
            </div>
          </div>
        )}

        {step === 'errors' && preview && (
          <div>
            <div style={{ display: 'flex', alignItems: 'center', gap: 10, padding: '12px 14px', borderRadius: 12, background: 'rgba(239,68,68,0.08)', color: '#b91c1c', marginBottom: 16 }}>
              <AlertCircle size={20} />
              <div>
                <div style={{ fontWeight: 700 }}>{errTotal === 1 ? '1 fila no se puede importar' : `${errTotal} filas no se pueden importar`}</div>
                <div style={{ fontSize: 12.5, color: '#7f1d1d' }}>Se van a omitir. Corregilas en el Excel y volvé a subirlo si las necesitás.</div>
              </div>
            </div>

            <div style={{ display: 'flex', justifyContent: 'flex-end', marginBottom: 10 }}>
              <button type="button" onClick={downloadErrors} style={{ ...btnSecondary, padding: '6px 12px', fontSize: 12 }}><Download size={14} /> Descargar errores (.xlsx)</button>
            </div>

            {errCompanies.length > 0 && (
              <ErrorTable title="Empresas" rows={errCompanies} nameKey="nombre_responsable" companyKey="nombre_empresa" showCompany />
            )}
            {errProfessionals.length > 0 && (
              <ErrorTable title="Profesionales" rows={errProfessionals} nameKey="nombre" companyKey="empresa" showCompany={!employerMode} />
            )}

            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 12, marginTop: 20, flexWrap: 'wrap' }}>
              <span style={{ fontSize: 13, color: '#475569', fontWeight: 600 }}>
                Se importarán <strong>{totalToImport}</strong> de {(preview.companies?.length ?? 0) + (preview.professionals?.length ?? 0)} filas.
              </span>
              <div style={{ display: 'flex', gap: 10 }}>
                <button type="button" onClick={() => setStep('preview')} disabled={busy} style={btnSecondary}>Atrás</button>
                {importButton}
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
              {result.companies && <ResultCard title="Empresas" r={result.companies} omitted={omitted.companies} />}
              <ResultCard title="Profesionales" r={result.professionals} omitted={omitted.professionals} />
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

function ResultCard({ title, r, omitted }: { title: string; r: NonNullable<ExecResult['companies']>; omitted: { skipped: number; errored: number } }) {
  // El backend solo ve las filas que le mandamos: lo que omitiste vos y las filas
  // con error del archivo hay que sumarlos acá o el resumen miente.
  const totalSkipped = r.skipped + omitted.skipped
  return (
    <div style={{ border: '1px solid #e2e8f0', borderRadius: 12, padding: 14 }}>
      <div style={{ fontWeight: 800, color: '#0f172a', marginBottom: 8 }}>{title}</div>
      <div style={{ display: 'flex', gap: 14, fontSize: 13, color: '#475569', flexWrap: 'wrap' }}>
        <span>✅ Creados: <strong>{r.created}</strong></span>
        <span>♻️ Actualizados: <strong>{r.updated}</strong></span>
        <span>⏭️ Omitidos: <strong>{totalSkipped}</strong></span>
        {omitted.errored > 0 && <span style={{ color: '#b45309' }}>🚫 Con error en el archivo: <strong>{omitted.errored}</strong></span>}
        {r.errors.length > 0 && <span style={{ color: '#b91c1c' }}>⚠️ Fallaron al crear: <strong>{r.errors.length}</strong></span>}
      </div>
    </div>
  )
}

/** Pagina un arreglo y expone los controles. La página se corrige sola si el
 *  arreglo se acorta (p. ej. al volver atrás y subir otro archivo). */
function usePagedRows(rows: ImportRow[]) {
  const [page, setPage] = useState(0)
  const pageCount = Math.max(1, Math.ceil(rows.length / PAGE_SIZE))
  const safePage = Math.min(page, pageCount - 1)
  const from = safePage * PAGE_SIZE
  const visible = rows.slice(from, from + PAGE_SIZE)
  return { visible, page: safePage, pageCount, setPage, from }
}

function Pager({ page, pageCount, setPage, from, shown, total }: { page: number; pageCount: number; setPage: (p: number) => void; from: number; shown: number; total: number }) {
  if (total <= PAGE_SIZE) return null
  const navBtn = (disabled: boolean): React.CSSProperties => ({
    display: 'inline-flex', alignItems: 'center', gap: 4, padding: '5px 10px', borderRadius: 8,
    border: '1px solid #e2e8f0', background: disabled ? '#f8fafc' : '#fff',
    color: disabled ? '#cbd5e1' : '#475569', fontSize: 12.5, fontWeight: 600,
    cursor: disabled ? 'default' : 'pointer',
  })
  return (
    <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 12, padding: '8px 10px', borderTop: '1px solid #f1f5f9', background: '#fcfdff', flexWrap: 'wrap' }}>
      <span style={{ fontSize: 12, color: '#94a3b8' }}>Mostrando {from + 1}–{from + shown} de {total}</span>
      <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
        <button type="button" onClick={() => setPage(page - 1)} disabled={page === 0} style={navBtn(page === 0)}>
          <ChevronLeft size={14} /> Anterior
        </button>
        <span style={{ fontSize: 12.5, color: '#475569', fontWeight: 600 }}>{page + 1} / {pageCount}</span>
        <button type="button" onClick={() => setPage(page + 1)} disabled={page >= pageCount - 1} style={navBtn(page >= pageCount - 1)}>
          Siguiente <ChevronRight size={14} />
        </button>
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
  const { visible, page, pageCount, setPage, from } = usePagedRows(rows)
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
            {visible.map(r => (
              <tr key={r.row} style={{ borderTop: '1px solid #f1f5f9' }}>
                <td style={td}>{r.row}</td>
                <td style={td}>
                  {r.data[nameKey] || '—'}
                  {isManagerValue(r.data.es_manager) && (
                    <span style={{ ...pill, marginLeft: 6, background: '#ede9fe', color: '#6d28d9', fontSize: 10 }}>Manager</span>
                  )}
                </td>
                <td style={td}>{r.data.email || '—'}</td>
                {showCompany && <td style={td}>{r.data[companyKey] || '—'}</td>}
                <td style={td}>
                  <StatusPill row={r} />
                  {r.status === 'conflict' && r.existing && <div style={{ fontSize: 11, color: '#94a3b8' }}>de {r.existing.name}</div>}
                  {r.status === 'conflict' && !r.existing && r.message && <div style={{ fontSize: 11, color: '#b45309' }}>{r.message}</div>}
                </td>
                <td style={td}>
                  {r.status === 'ok' && <span style={{ color: '#059669', fontWeight: 600 }}>Crear</span>}
                  {r.status === 'conflict' && (
                    <select
                      value={decisions[`${entity}-${r.row}`] ?? 'skip'}
                      onChange={e => setDecision(r.row, e.target.value as Action)}
                      style={{ padding: '5px 8px', borderRadius: 8, border: '1px solid #cbd5e1', fontSize: 12.5, background: '#fff' }}
                    >
                      <option value="skip">Omitir</option>
                      {r.existing
                        ? <option value="overwrite">Sobreescribir</option>
                        : <option value="create">Crear igual</option>}
                    </select>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        <Pager page={page} pageCount={pageCount} setPage={setPage} from={from} shown={visible.length} total={rows.length} />
      </div>
    </div>
  )
}

function ErrorTable({ title, rows, nameKey, companyKey, showCompany }: { title: string; rows: ImportRow[]; nameKey: string; companyKey: string; showCompany: boolean }) {
  const { visible, page, pageCount, setPage, from } = usePagedRows(rows)
  return (
    <div style={{ marginBottom: 18 }}>
      <div style={{ ...labelStyle, marginBottom: 8 }}>{title} ({rows.length})</div>
      <div style={{ border: '1px solid #fee2e2', borderRadius: 10, overflow: 'hidden' }}>
        <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 12.5 }}>
          <thead>
            <tr style={{ background: '#fef2f2', textAlign: 'left' }}>
              <th style={th}>Fila</th><th style={th}>Nombre</th><th style={th}>Email</th>{showCompany && <th style={th}>Empresa</th>}<th style={th}>Motivo</th>
            </tr>
          </thead>
          <tbody>
            {visible.map(r => (
              <tr key={r.row} style={{ borderTop: '1px solid #fef2f2' }}>
                <td style={td}>{r.row}</td>
                <td style={td}>{r.data[nameKey] || '—'}</td>
                <td style={td}>{r.data.email || '—'}</td>
                {showCompany && <td style={td}>{r.data[companyKey] || '—'}</td>}
                <td style={{ ...td, color: '#b91c1c' }}>{r.message || 'Fila inválida'}</td>
              </tr>
            ))}
          </tbody>
        </table>
        <Pager page={page} pageCount={pageCount} setPage={setPage} from={from} shown={visible.length} total={rows.length} />
      </div>
    </div>
  )
}
