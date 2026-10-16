import { useEffect, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { AlertTriangle, BellOff, Database, Eraser, Save, Search, ShieldAlert } from 'lucide-react'
import { Button, Select, Skeleton, useConfirm } from '../ui'
import { usePrompt } from '../ui/PromptProvider'
import { useNotification } from '../../context/NotificationContext'
import { MCP_STATUS_KEY, mcpService, type McpConnection, type McpQuota, type McpSettings } from '../../services/mcp.service'
import { errorMessage, formatDate } from './format'
import styles from '../../pages/Integraciones.module.css'

// Panel "Uso de IA" (solo superadmin): conexiones de todos, uso por persona y
// día, cuotas y permiso por rol, interruptores de pausa y datos mínimos,
// dominios extra, estado del rol de base dedicado y alertas.

const OVERVIEW_KEY = ['mcp', 'admin', 'overview'] as const

function Switch({ checked, onChange, label, disabled }: { checked: boolean; onChange: (v: boolean) => void; label: string; disabled?: boolean }) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      aria-label={label}
      className={styles.switch}
      disabled={disabled}
      onClick={() => onChange(!checked)}
    />
  )
}

const DB_MODE: Record<string, { label: string; cls: string }> = {
  dedicated: { label: 'Rol de solo lectura dedicado', cls: styles.pillGreen },
  general: { label: 'Pool general (READ ONLY por consulta)', cls: styles.pillAmber },
  fallback: { label: 'CRÍTICO: el rol dedicado falló', cls: styles.pillRed },
}

const ALERT_LABEL: Record<string, string> = {
  quota_80: '80 % de la cuota',
  quota_100: 'Cuota agotada',
  token_reuse: 'Reuso de token',
}

function QuotaRow({ q, onSaved }: { q: McpQuota; onSaved: () => void }) {
  const [draft, setDraft] = useState(q)
  const { success, error: showError } = useNotification()
  useEffect(() => setDraft(q), [q])
  const dirty = draft.enabled !== q.enabled || draft.daily_calls !== q.daily_calls || draft.max_rows !== q.max_rows
  const save = useMutation({
    mutationFn: () => mcpService.updateQuota(draft),
    onSuccess: () => {
      success(`Cuota de ${q.label} guardada`)
      onSaved()
    },
    onError: (err) => showError(errorMessage(err, 'No se pudo guardar la cuota')),
  })
  return (
    <tr>
      <td><strong>{q.label}</strong></td>
      <td>
        <Switch checked={draft.enabled} label={`Habilitar para ${q.label}`} onChange={(v) => setDraft({ ...draft, enabled: v })} />
      </td>
      <td className={styles.num}>
        <input
          type="number"
          className={styles.numInput}
          min={0}
          max={100000}
          value={draft.daily_calls}
          aria-label={`Consultas por día de ${q.label}`}
          onChange={(e) => setDraft({ ...draft, daily_calls: Number(e.target.value) })}
        />
      </td>
      <td className={styles.num}>
        <input
          type="number"
          className={styles.numInput}
          min={1}
          max={500}
          value={draft.max_rows}
          aria-label={`Filas máximas de ${q.label}`}
          onChange={(e) => setDraft({ ...draft, max_rows: Number(e.target.value) })}
        />
      </td>
      <td className={styles.num}>
        <Button size="sm" variant={dirty ? 'primary' : 'secondary'} disabled={!dirty} loading={save.isPending} onClick={() => save.mutate()}>
          Guardar
        </Button>
      </td>
    </tr>
  )
}

export default function UsoIA() {
  const qc = useQueryClient()
  const confirm = useConfirm()
  const prompt = usePrompt()
  const { success, error: showError } = useNotification()
  const [days, setDays] = useState(7)
  const [connStatus, setConnStatus] = useState<'active' | 'all'>('active')
  const [search, setSearch] = useState('')
  const [settings, setSettings] = useState<McpSettings | null>(null)

  const overview = useQuery({ queryKey: OVERVIEW_KEY, queryFn: mcpService.getOverview })
  const usage = useQuery({ queryKey: ['mcp', 'admin', 'usage', days], queryFn: () => mcpService.getUsage(days) })
  const conns = useQuery({
    queryKey: ['mcp', 'admin', 'connections', connStatus, search],
    queryFn: () => mcpService.listAllConnections(connStatus, search),
  })

  useEffect(() => {
    if (overview.data) setSettings(overview.data.settings)
  }, [overview.data])

  const refreshAll = () => {
    qc.invalidateQueries({ queryKey: ['mcp'] })
  }

  const saveSettings = useMutation({
    mutationFn: (s: McpSettings) => mcpService.updateSettings(s),
    onSuccess: () => {
      success('Ajustes guardados')
      qc.invalidateQueries({ queryKey: OVERVIEW_KEY })
      qc.invalidateQueries({ queryKey: MCP_STATUS_KEY })
    },
    onError: (err) => showError(errorMessage(err, 'No se pudieron guardar los ajustes')),
  })

  const togglePause = async (paused: boolean) => {
    if (!settings) return
    if (paused) {
      const ok = await confirm({
        title: 'Pausar asistentes de IA',
        message: 'Todos los asistentes conectados dejarán de recibir datos hasta que lo reanudes. Las conexiones no se borran.',
        confirmLabel: 'Pausar',
        variant: 'danger',
      })
      if (!ok) return
    }
    const next = { ...settings, paused }
    setSettings(next)
    saveSettings.mutate(next)
  }

  const toggleMinimal = (minimal_data: boolean) => {
    if (!settings) return
    const next = { ...settings, minimal_data }
    setSettings(next)
    saveSettings.mutate(next)
  }

  const revoke = async (c: McpConnection) => {
    const reason = await prompt({
      title: 'Revocar conexión',
      message: `Se cortará el acceso de «${c.client_name}» de ${c.user_name || c.user_email}. Indica el motivo (queda en la auditoría).`,
      placeholder: 'Motivo',
      confirmLabel: 'Revocar',
    })
    if (!reason?.trim()) return
    try {
      await mcpService.adminRevoke(c.id, reason.trim())
      success('Conexión revocada')
      refreshAll()
    } catch (err) {
      showError(errorMessage(err, 'No se pudo revocar'))
    }
  }

  const markSeen = async () => {
    try {
      await mcpService.markAlertsSeen()
      qc.invalidateQueries({ queryKey: OVERVIEW_KEY })
    } catch (err) {
      showError(errorMessage(err, 'No se pudieron marcar las alertas'))
    }
  }

  const purge = async () => {
    const ok = await confirm({
      title: 'Purgar conexiones vencidas',
      message: 'Se borran las conexiones vencidas y las revocadas hace más de 30 días, con sus tokens. La auditoría se conserva.',
      confirmLabel: 'Purgar',
      variant: 'danger',
    })
    if (!ok) return
    try {
      const r = await mcpService.purge()
      success(`Purgadas ${r.connections} conexiones y ${r.tokens} tokens`)
      refreshAll()
    } catch (err) {
      showError(errorMessage(err, 'No se pudo purgar'))
    }
  }

  if (overview.isLoading || !settings) {
    return (
      <div className={styles.stack}>
        <Skeleton height={90} />
        <Skeleton height={240} />
      </div>
    )
  }
  if (overview.isError || !overview.data) {
    return <div className={styles.bannerRed}>No se pudo cargar el panel. Recarga la página.</div>
  }
  const o = overview.data
  const db = DB_MODE[o.db_status.mode] ?? DB_MODE.general

  return (
    <div className={styles.stack}>
      {o.db_status.mode === 'fallback' && (
        <div className={styles.bannerRed}>
          <ShieldAlert size={18} />
          <span>
            <strong>El rol de base de datos dedicado falló</strong> y las consultas pasaron al pool general (siguen siendo de solo lectura).
            Detalle: {o.db_status.detail}
          </span>
        </div>
      )}

      <div className={styles.statGrid}>
        <div className={styles.stat}>
          <div className={styles.statLabel}>Conexiones activas</div>
          <div className={styles.statValue}>{o.totals.active_connections}</div>
        </div>
        <div className={styles.stat}>
          <div className={styles.statLabel}>Consultas hoy</div>
          <div className={styles.statValue}>{o.totals.calls_today}</div>
        </div>
        <div className={styles.stat}>
          <div className={styles.statLabel}>Personas hoy</div>
          <div className={styles.statValue}>{o.totals.users_today}</div>
        </div>
        <div className={styles.stat}>
          <div className={styles.statLabel}>Base de datos</div>
          <div style={{ marginTop: 8 }}>
            <span className={db.cls}><Database size={13} /> {db.label}</span>
          </div>
          <p className={styles.muted} style={{ marginTop: 6, fontSize: 12.5 }}>{o.db_status.detail}</p>
        </div>
      </div>

      <section className={styles.card}>
        <div className={styles.cardHead}>
          <h2 className={styles.cardTitle}><AlertTriangle size={18} /> Alertas pendientes</h2>
          {o.alerts.length > 0 && (
            <Button size="sm" variant="secondary" leftIcon={<BellOff size={14} />} onClick={markSeen}>Marcar todas como vistas</Button>
          )}
        </div>
        {o.alerts.length === 0 ? (
          <div className={styles.empty}>Sin alertas pendientes.</div>
        ) : (
          o.alerts.map((a) => (
            <div key={a.id} className={styles.alertItem}>
              <span className={a.kind === 'token_reuse' || a.kind === 'quota_100' ? styles.pillRed : styles.pillAmber}>
                {ALERT_LABEL[a.kind] ?? a.kind}
              </span>
              <div>
                <div>{a.detail}</div>
                <span className={styles.sub}>{formatDate(a.created_at)}</span>
              </div>
            </div>
          ))
        )}
      </section>

      <section className={styles.card}>
        <h2 className={styles.cardTitle}>Interruptores</h2>
        <div className={styles.switchRow}>
          <div className={styles.switchText}>
            <strong>Pausar asistentes de IA</strong>
            <p className={styles.muted}>Corta las respuestas a todos los asistentes conectados, con un mensaje que lo explica.</p>
          </div>
          <Switch checked={settings.paused} label="Pausar asistentes de IA" disabled={saveSettings.isPending} onChange={togglePause} />
        </div>
        <div className={styles.switchRow}>
          <div className={styles.switchText}>
            <strong>Modo datos mínimos</strong>
            <p className={styles.muted}>Quita de las respuestas las descripciones, actividades, comentarios, notas y motivos, y recorta los textos largos.</p>
          </div>
          <Switch checked={settings.minimal_data} label="Modo datos mínimos" disabled={saveSettings.isPending} onChange={toggleMinimal} />
        </div>
        <div className={styles.switchRow} style={{ display: 'block' }}>
          <div className={styles.switchText}>
            <strong>Dominios extra para redirect_uri</strong>
            <p className={styles.muted}>
              Además de ChatGPT, Claude, Grok, Gemini, Perplexity, GitHub y Cursor. Uno por línea (se admiten sus subdominios).
            </p>
          </div>
          <textarea
            className={styles.textarea}
            style={{ marginTop: 8 }}
            value={settings.extra_redirect_domains}
            placeholder="asistente.miempresa.com"
            onChange={(e) => setSettings({ ...settings, extra_redirect_domains: e.target.value })}
            aria-label="Dominios extra"
          />
          <div style={{ marginTop: 8 }}>
            <Button
              size="sm"
              leftIcon={<Save size={14} />}
              loading={saveSettings.isPending}
              disabled={settings.extra_redirect_domains === o.settings.extra_redirect_domains}
              onClick={() => saveSettings.mutate(settings)}
            >
              Guardar dominios
            </Button>
          </div>
        </div>
      </section>

      <section className={styles.card}>
        <h2 className={styles.cardTitle}>Acceso y cuotas por rol</h2>
        <p className={styles.muted} style={{ margin: '6px 0 12px' }}>
          Un rol habilitado ve «Integraciones» en el menú y puede conectar su asistente; al deshabilitarlo, sus conexiones dejan de responder.
          La cuota cuenta las consultas del día (whoami no cuenta) y el máximo de filas por respuesta.
        </p>
        <div className={styles.tableWrap}>
          <table className={styles.table}>
            <thead>
              <tr>
                <th>Rol</th>
                <th>Habilitado</th>
                <th className={styles.num}>Consultas / día</th>
                <th className={styles.num}>Filas máx.</th>
                <th aria-label="Acciones" />
              </tr>
            </thead>
            <tbody>
              {o.quotas.map((q) => (
                <QuotaRow key={q.role_name} q={q} onSaved={() => qc.invalidateQueries({ queryKey: OVERVIEW_KEY })} />
              ))}
            </tbody>
          </table>
        </div>
      </section>

      <section className={styles.card}>
        <div className={styles.cardHead}>
          <h2 className={styles.cardTitle}>Uso por persona y día</h2>
          <Select
            value={days}
            onChange={(v) => setDays(Number(v))}
            options={[
              { value: 1, label: 'Hoy' },
              { value: 7, label: 'Últimos 7 días' },
              { value: 30, label: 'Últimos 30 días' },
              { value: 90, label: 'Últimos 90 días' },
            ]}
          />
        </div>
        {usage.isLoading ? (
          <Skeleton height={120} />
        ) : (usage.data?.by_user_day.length ?? 0) === 0 ? (
          <div className={styles.empty}>Nadie usó los asistentes en este período.</div>
        ) : (
          <>
            <div className={styles.tableWrap}>
              <table className={styles.table}>
                <thead>
                  <tr>
                    <th>Día</th>
                    <th>Persona</th>
                    <th className={styles.num}>Consultas</th>
                    <th className={styles.num}>Errores</th>
                    <th className={styles.num}>Filas</th>
                    <th className={styles.num}>Tiempo medio</th>
                  </tr>
                </thead>
                <tbody>
                  {usage.data!.by_user_day.map((r) => (
                    <tr key={`${r.day}-${r.user_id}`}>
                      <td>{r.day}</td>
                      <td>{r.user_name}<span className={styles.sub}>{r.user_email}</span></td>
                      <td className={styles.num}>{r.calls}</td>
                      <td className={styles.num}>{r.errors}</td>
                      <td className={styles.num}>{r.rows}</td>
                      <td className={styles.num}>{Math.round(r.avg_ms)} ms</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            <h3 className={styles.cardTitle} style={{ fontSize: 15, margin: '18px 0 8px' }}>Por herramienta</h3>
            <div className={styles.tableWrap}>
              <table className={styles.table}>
                <thead>
                  <tr>
                    <th>Herramienta</th>
                    <th className={styles.num}>Consultas</th>
                    <th className={styles.num}>Errores</th>
                    <th className={styles.num}>Tiempo medio</th>
                  </tr>
                </thead>
                <tbody>
                  {usage.data!.by_tool.map((r) => (
                    <tr key={r.tool}>
                      <td><code className={styles.inlineCode}>{r.tool}</code></td>
                      <td className={styles.num}>{r.calls}</td>
                      <td className={styles.num}>{r.errors}</td>
                      <td className={styles.num}>{Math.round(r.avg_ms)} ms</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </>
        )}
      </section>

      <section className={styles.card}>
        <div className={styles.cardHead}>
          <h2 className={styles.cardTitle}>Conexiones</h2>
          <div className={styles.toolbar}>
            <Select
              value={connStatus}
              onChange={(v) => setConnStatus(v as 'active' | 'all')}
              options={[
                { value: 'active', label: 'Activas' },
                { value: 'all', label: 'Todas (incluye revocadas)' },
              ]}
            />
            <Button variant="secondary" size="sm" leftIcon={<Eraser size={14} />} onClick={purge}>Purgar vencidas</Button>
          </div>
        </div>
        <div className={styles.formRow} style={{ marginTop: 0 }}>
          <Search size={16} color="#64748b" />
          <input
            className={styles.input}
            placeholder="Buscar por persona, correo o asistente"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            aria-label="Buscar conexiones"
          />
        </div>
        {conns.isLoading ? (
          <Skeleton height={100} />
        ) : (conns.data?.length ?? 0) === 0 ? (
          <div className={styles.empty}>No hay conexiones.</div>
        ) : (
          <div className={styles.tableWrap}>
            <table className={styles.table}>
              <thead>
                <tr>
                  <th>Persona</th>
                  <th>Asistente</th>
                  <th>Creada</th>
                  <th>Último uso</th>
                  <th>Estado</th>
                  <th aria-label="Acciones" />
                </tr>
              </thead>
              <tbody>
                {conns.data!.map((c) => {
                  const expired = new Date(c.expires_at).getTime() < Date.now()
                  return (
                    <tr key={c.id}>
                      <td>{c.user_name}<span className={styles.sub}>{c.user_email}</span></td>
                      <td>
                        {c.client_name}
                        <span className={styles.sub}>{c.personal ? 'Token personal' : 'OAuth'}{c.ip ? ` · ${c.ip}` : ''}</span>
                      </td>
                      <td>{formatDate(c.created_at)}</td>
                      <td>{c.last_used_at ? formatDate(c.last_used_at) : '—'}</td>
                      <td>
                        {c.revoked_at ? (
                          <span className={styles.pillRed} title={c.revoke_reason}>Revocada</span>
                        ) : expired ? (
                          <span className={styles.pillAmber}>Vencida</span>
                        ) : (
                          <span className={styles.pillGreen}>Activa</span>
                        )}
                        {c.revoked_at && c.revoke_reason && <span className={styles.sub}>{c.revoke_reason}</span>}
                      </td>
                      <td className={styles.num}>
                        {!c.revoked_at && !expired && (
                          <Button variant="ghost" size="sm" onClick={() => revoke(c)}>Revocar</Button>
                        )}
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
        )}
      </section>
    </div>
  )
}
