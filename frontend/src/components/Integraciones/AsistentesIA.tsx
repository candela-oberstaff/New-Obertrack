import { useState, type ReactNode } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Bot, Check, ChevronDown, ChevronRight, ChevronUp, Copy, KeyRound, Link2, Lock, PauseCircle, Trash2 } from 'lucide-react'
import { Button, Skeleton, useConfirm } from '../ui'
import { useNotification } from '../../context/NotificationContext'
import { MCP_STATUS_KEY, mcpService, type McpConnection } from '../../services/mcp.service'
import styles from '../../pages/Integraciones.module.css'
import { errorMessage, formatDate } from './format'
import { ClaudeLogo, GeminiLogo, GrokLogo, OpenAILogo } from './AiLogos'
import ConnectionGuide, { type ClientKey } from './ConnectionGuide'
import { useCopy } from './useCopy'

// Tarjeta "Asistentes de IA": cada persona conecta su propio asistente (ChatGPT,
// Claude, Grok, Gemini CLI) al servidor MCP de solo lectura y ve lo mismo que
// en la web con su rol.

const CONNECTIONS_KEY = ['mcp', 'connections'] as const
const TOKENS_KEY = ['mcp', 'tokens'] as const

const CLIENTS: { key: ClientKey; label: string; hint: string; logo: ReactNode; dev?: boolean }[] = [
  { key: 'chatgpt', label: 'ChatGPT', hint: 'Conector con inicio de sesión', logo: <OpenAILogo /> },
  { key: 'claude', label: 'Claude', hint: 'Conector personalizado', logo: <ClaudeLogo /> },
  { key: 'grok', label: 'Grok', hint: 'Conector personalizado', logo: <GrokLogo /> },
  { key: 'gemini', label: 'Gemini CLI', hint: 'Con token personal', logo: <GeminiLogo />, dev: true },
]

function ConnectionsTable({ rows, onRevoke, busyId, emptyText }: {
  rows: McpConnection[]
  onRevoke: (c: McpConnection) => void
  busyId: number | null
  emptyText: string
}) {
  if (rows.length === 0) return <div className={styles.empty}>{emptyText}</div>
  return (
    <div className={styles.tableWrap}>
      <table className={styles.table}>
        <thead>
          <tr>
            <th>Nombre</th>
            <th>Creada</th>
            <th>Último uso</th>
            <th>Vence</th>
            <th aria-label="Acciones" />
          </tr>
        </thead>
        <tbody>
          {rows.map((c) => (
            <tr key={c.id}>
              <td><strong>{c.client_name}</strong></td>
              <td>{formatDate(c.created_at)}</td>
              <td>{c.last_used_at ? formatDate(c.last_used_at) : 'Sin uso todavía'}</td>
              <td>{formatDate(c.expires_at)}</td>
              <td className={styles.num}>
                <Button variant="ghost" size="sm" leftIcon={<Trash2 size={14} />} loading={busyId === c.id} onClick={() => onRevoke(c)}>
                  Revocar
                </Button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

export default function AsistentesIA() {
  const qc = useQueryClient()
  const confirm = useConfirm()
  const { success, error: showError } = useNotification()
  const { copied, copy } = useCopy()
  const [client, setClient] = useState<ClientKey | null>(null)
  // La última guía abierta sigue montada mientras el acordeón se cierra, para
  // que se vea plegarse en vez de desaparecer de golpe.
  const [shownClient, setShownClient] = useState<ClientKey | null>(null)
  const selected = CLIENTS.find((c) => c.key === shownClient)
  const toggleClient = (key: ClientKey | null) => {
    setClient(key)
    if (key) setShownClient(key)
  }
  const [tokenName, setTokenName] = useState('')
  const [newToken, setNewToken] = useState<string | null>(null)
  const [busyId, setBusyId] = useState<number | null>(null)

  const status = useQuery({ queryKey: MCP_STATUS_KEY, queryFn: mcpService.getStatus })
  const enabled = !!status.data?.enabled
  // Se refrescan al volver a esta pestaña: la autorización ocurre en otra, y
  // así la guía marca el asistente como conectado sin recargar.
  const connections = useQuery({ queryKey: CONNECTIONS_KEY, queryFn: mcpService.listConnections, enabled, refetchOnWindowFocus: true })
  const tokens = useQuery({ queryKey: TOKENS_KEY, queryFn: mcpService.listTokens, enabled })

  const createToken = useMutation({
    mutationFn: (name: string) => mcpService.createToken(name),
    onSuccess: (res) => {
      setNewToken(res.token)
      setTokenName('')
      qc.invalidateQueries({ queryKey: TOKENS_KEY })
    },
    onError: (err) => showError(errorMessage(err, 'No se pudo crear el token')),
  })

  const revoke = async (c: McpConnection) => {
    const ok = await confirm({
      title: c.personal ? 'Revocar token personal' : 'Revocar conexión',
      message: `«${c.client_name}» dejará de tener acceso a Obertrack de inmediato. Para volver a usarlo tendrás que conectarlo otra vez.`,
      confirmLabel: 'Revocar',
      variant: 'danger',
    })
    if (!ok) return
    setBusyId(c.id)
    try {
      if (c.personal) await mcpService.revokeToken(c.id)
      else await mcpService.revokeConnection(c.id)
      success('Acceso revocado')
      qc.invalidateQueries({ queryKey: c.personal ? TOKENS_KEY : CONNECTIONS_KEY })
    } catch (err) {
      showError(errorMessage(err, 'No se pudo revocar'))
    } finally {
      setBusyId(null)
    }
  }

  if (status.isLoading) {
    return (
      <div className={styles.stack}>
        <Skeleton height={160} />
        <Skeleton height={220} />
      </div>
    )
  }

  if (status.isError || !status.data) {
    return <div className={styles.bannerRed}>No se pudo cargar la integración. Recarga la página.</div>
  }

  const s = status.data
  if (!s.enabled) {
    return (
      <div className={styles.card}>
        <h2 className={styles.cardTitle}>
          <span className={styles.cardIcon}><Lock size={18} /></span> Asistentes de IA
        </h2>
        <p className={styles.muted} style={{ marginTop: 10 }}>
          Tu rol ({s.role_label}) todavía no tiene habilitada la conexión con asistentes de IA. Si la necesitas, pídesela a un administrador.
        </p>
      </div>
    )
  }

  return (
    <div className={styles.stack}>
      {s.paused && (
        <div className={styles.banner}>
          <PauseCircle size={18} />
          <span>Un administrador pausó el acceso de los asistentes. Tus conexiones siguen guardadas, pero no responderán hasta que se reanude.</span>
        </div>
      )}

      <section className={`${styles.card} ${styles.hero}`}>
        <div className={styles.cardHead}>
          <h2 className={styles.cardTitle}>
            <span className={styles.cardIcon}><Bot size={18} /></span> Asistentes de IA
          </h2>
        </div>
        <p className={styles.muted}>
          Conecta tu asistente de IA a Obertrack para preguntarle por tus jornadas, tareas o equipo en lenguaje natural.
          Verá exactamente lo mismo que tú ves en la web y no puede crear, editar ni borrar nada.
        </p>
        <div className={styles.pills}>
          <span className={styles.pillGreen}><Lock size={13} /> Solo lectura</span>
          <span className={styles.pill}>Rol: {s.role_label}</span>
          <span className={styles.pill}>{s.used_today} de {s.quota.daily_calls} consultas hoy</span>
        </div>
        {s.scope && <p className={styles.muted} style={{ marginTop: 12 }}><strong>Qué podrá ver:</strong> {s.scope}</p>}
      </section>

      <section className={styles.card}>
        <div className={styles.cardHead}>
          <h2 className={styles.cardTitle}>
            <span className={styles.cardIcon}><Link2 size={18} /></span> Conecta tu asistente
          </h2>
        </div>
        <p className={styles.pickerLabel} style={{ marginTop: 0 }}>URL del servidor</p>
        <div className={styles.urlRow}>
          <div className={styles.urlBox}>{s.server_url}</div>
          <Button variant="secondary" leftIcon={copied === 'url' ? <Check size={16} /> : <Copy size={16} />} onClick={() => copy(s.server_url, 'url')}>
            {copied === 'url' ? 'Copiada' : 'Copiar'}
          </Button>
        </div>

        <p className={styles.pickerLabel}>Elige tu asistente y toca su tarjeta para ver cómo conectarlo</p>
        {/* Acordeón: las instrucciones arrancan cerradas y se abren al tocar
            una tarjeta; tocarla otra vez las cierra, tocar otra las cambia. */}
        <div className={styles.clientPicker}>
          {CLIENTS.map((c) => {
            const open = client === c.key
            return (
              <button
                key={c.key}
                type="button"
                aria-expanded={open}
                aria-controls="mcp-guide-panel"
                className={open ? `${styles.clientCard} ${styles.clientCardActive}` : styles.clientCard}
                onClick={() => toggleClient(open ? null : c.key)}
              >
                <span className={styles.clientLogo}>{c.logo}</span>
                <span className={styles.clientText}>
                  <span className={styles.clientName}>{c.label}</span>
                  <span className={styles.clientHint}>{c.dev ? <span className={styles.devTag}>Desarrolladores</span> : c.hint}</span>
                </span>
                <ChevronDown size={18} className={`${styles.clientChevron} ${open ? styles.clientChevronOpen : ''}`} aria-hidden />
              </button>
            )
          })}
        </div>
        <div
          id="mcp-guide-panel"
          role="region"
          aria-label={selected ? `Cómo conectar ${selected.label}` : 'Instrucciones de conexión'}
          className={`${styles.guideCollapse} ${client ? styles.guideCollapseOpen : ''}`}
          hidden={!client && !shownClient}
          inert={!client}
        >
          <div className={styles.guideCollapseInner}>
            {shownClient && selected && (
              <div className={styles.guideBox}>
                <div className={styles.guideHead}>
                  <span className={styles.clientLogo}>{selected.logo}</span>
                  <strong>Cómo conectar {selected.label}</strong>
                  <button type="button" className={styles.guideClose} onClick={() => toggleClient(null)}>
                    Ocultar <ChevronUp size={15} />
                  </button>
                </div>
                <ConnectionGuide
                  client={shownClient}
                  url={s.server_url}
                  connections={connections.data ?? []}
                  personalTokens={tokens.data ?? []}
                  newToken={newToken}
                  creatingToken={createToken.isPending}
                  onCreateToken={() => {
                    setNewToken(null)
                    createToken.mutate('Gemini CLI')
                  }}
                />
              </div>
            )}
          </div>
        </div>
      </section>

      <section className={styles.card}>
        <div className={styles.cardHead}>
          <h2 className={styles.cardTitle}>Mis conexiones</h2>
        </div>
        {connections.isLoading ? (
          <Skeleton height={80} />
        ) : (
          <ConnectionsTable
            rows={connections.data ?? []}
            onRevoke={revoke}
            busyId={busyId}
            emptyText="Todavía no conectaste ningún asistente. Sigue la guía de arriba."
          />
        )}
      </section>

      <section className={styles.card}>
        <details className={styles.details} open={!!newToken || (tokens.data?.length ?? 0) > 0}>
          <summary>
            <ChevronRight size={16} className={styles.chevron} />
            <KeyRound size={16} /> Tokens personales
            <span className={styles.devTag}>Desarrolladores</span>
          </summary>
          <p className={styles.muted} style={{ marginTop: 10 }}>
            Para clientes que no admiten el inicio de sesión de Obertrack (como Gemini CLI). Cada token da el mismo acceso
            de solo lectura que tu usuario, vence a los 90 días y se muestra una sola vez.
          </p>
          <form
            className={styles.formRow}
            onSubmit={(e) => {
              e.preventDefault()
              setNewToken(null)
              createToken.mutate(tokenName.trim() || 'Token personal')
            }}
          >
            <input
              className={styles.input}
              placeholder="Nombre, por ejemplo «Gemini CLI del portátil»"
              value={tokenName}
              maxLength={100}
              onChange={(e) => setTokenName(e.target.value)}
              aria-label="Nombre del token"
            />
            <Button type="submit" loading={createToken.isPending} leftIcon={<KeyRound size={16} />}>
              Crear token
            </Button>
          </form>

          {newToken && (
            <div className={styles.secret}>
              <strong>Copia tu token ahora: no se volverá a mostrar.</strong>
              <div className={styles.urlRow} style={{ marginTop: 8 }}>
                <div className={styles.urlBox}>{newToken}</div>
                <Button variant="secondary" leftIcon={copied === 'token' ? <Check size={16} /> : <Copy size={16} />} onClick={() => copy(newToken, 'token')}>
                  {copied === 'token' ? 'Copiado' : 'Copiar'}
                </Button>
              </div>
            </div>
          )}

          <div style={{ marginTop: 12 }}>
            {tokens.isLoading ? (
              <Skeleton height={60} />
            ) : (
              <ConnectionsTable rows={tokens.data ?? []} onRevoke={revoke} busyId={busyId} emptyText="No tienes tokens personales activos." />
            )}
          </div>
        </details>
      </section>
    </div>
  )
}
