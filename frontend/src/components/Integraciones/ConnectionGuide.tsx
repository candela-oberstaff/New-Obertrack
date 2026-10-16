import type { ReactNode } from 'react'
import { AlertTriangle, Check, CheckCircle2, ChevronRight, Copy, ExternalLink, HelpCircle, KeyRound, MessageSquareText } from 'lucide-react'
import { Button } from '../ui'
import type { McpConnection } from '../../services/mcp.service'
import { useCopy } from './useCopy'
import styles from '../../pages/Integraciones.module.css'

// Guía de conexión por asistente. El objetivo es que funcione a la primera:
// requisitos antes de empezar, pasos con lo que hay que pegar a mano (con su
// botón de copiar), los nombres de los botones también en inglés (esas apps
// suelen estar en inglés), cómo comprobarlo y qué hacer si algo falla.
//
// Pasos verificados contra la ayuda oficial de cada asistente en oct-2026:
// Claude (support.claude.com, «Get started with custom connectors using
// remote MCP»), Grok (docs.x.ai/grok/connectors), Gemini CLI
// (docs/tools/mcp-server.md). Los menús de ChatGPT cambian a menudo, por eso
// su guía nombra las dos variantes y dice qué buscar si no coinciden.

export type ClientKey = 'chatgpt' | 'claude' | 'grok' | 'gemini'

const CONNECTOR_NAME = 'Obertrack'
const TEST_PROMPT = '¿Quién soy en Obertrack y qué puedo consultar?'

// Cómo reconocer en "Mis conexiones" que este asistente ya está conectado:
// cada uno se registra con su propio nombre de cliente.
const CLIENT_MATCH: Record<Exclude<ClientKey, 'gemini'>, string[]> = {
  chatgpt: ['chatgpt', 'openai'],
  claude: ['claude', 'anthropic'],
  grok: ['grok', 'xai', 'x.ai'],
}

function CopyChip({ value, id, label }: { value: string; id: string; label?: string }) {
  const { copied, copy } = useCopy()
  return (
    <span className={styles.copyChip}>
      <code>{value}</code>
      <button type="button" onClick={() => copy(value, id)} aria-label={`Copiar ${label ?? value}`} title="Copiar">
        {copied === id ? <Check size={14} /> : <Copy size={14} />}
      </button>
    </span>
  )
}

function CodeWithCopy({ code, id }: { code: string; id: string }) {
  const { copied, copy } = useCopy()
  return (
    <div className={styles.codeWrap}>
      <pre className={styles.codeBlock}>{code}</pre>
      <button type="button" className={styles.codeCopy} onClick={() => copy(code, id)}>
        {copied === id ? <><Check size={14} /> Copiado</> : <><Copy size={14} /> Copiar</>}
      </button>
    </div>
  )
}

/** Etiqueta de botón o menú, con su nombre en inglés entre paréntesis. */
function UI({ es, en }: { es: string; en?: string }) {
  return (
    <>
      <strong>{es}</strong>
      {en && en !== es && <span className={styles.enLabel}> ({en})</span>}
    </>
  )
}

function ExtLink({ href, children }: { href: string; children: ReactNode }) {
  return (
    <a href={href} target="_blank" rel="noopener noreferrer" className={styles.extLink}>
      {children} <ExternalLink size={12} />
    </a>
  )
}

type Step = { title: ReactNode; body?: ReactNode }

function Steps({ steps }: { steps: Step[] }) {
  return (
    <ol className={styles.stepList}>
      {steps.map((s, i) => (
        <li key={i} className={styles.stepItem}>
          <span className={styles.stepNum}>{i + 1}</span>
          <div className={styles.stepBody}>
            <div className={styles.stepTitle}>{s.title}</div>
            {s.body && <div className={styles.stepText}>{s.body}</div>}
          </div>
        </li>
      ))}
    </ol>
  )
}

function Requirements({ children }: { children: ReactNode }) {
  return (
    <div className={styles.reqBox}>
      <strong>Antes de empezar</strong>
      <div>{children}</div>
    </div>
  )
}

function Faq({ items }: { items: { q: string; a: ReactNode }[] }) {
  return (
    <details className={`${styles.details} ${styles.faq}`}>
      <summary>
        <ChevronRight size={16} className={styles.chevron} />
        <HelpCircle size={16} /> ¿Algo no funciona?
      </summary>
      <dl>
        {items.map((it) => (
          <div key={it.q} className={styles.faqItem}>
            <dt>{it.q}</dt>
            <dd>{it.a}</dd>
          </div>
        ))}
      </dl>
    </details>
  )
}

// Problemas comunes a los conectores con inicio de sesión (ChatGPT, Claude, Grok).
function oauthFaq(url: string) {
  return [
    {
      q: 'Al pulsar «Conectar» sale un error o no pasa nada',
      a: <>Revisa que la URL sea exactamente <code className={styles.inlineCode}>{url}</code>, sin espacios ni texto extra (usa el botón de copiar). Si el asistente te dejó un conector a medias, bórralo y créalo de nuevo.</>,
    },
    {
      q: 'Obertrack dice que mi rol no tiene habilitados los asistentes de IA',
      a: 'Tu rol todavía no tiene acceso. Pídeselo a un administrador de Obertrack; cuando lo active, vuelve a pulsar «Conectar».',
    },
    {
      q: 'La página de Obertrack dice que la autorización venció',
      a: 'Pasaron más de 10 minutos desde que se abrió. Cierra esa pestaña y vuelve a pulsar «Conectar» en el asistente.',
    },
    {
      q: 'Funcionaba y dejó de responder',
      a: 'Mira «Mis conexiones», más abajo. Si ya no aparece, la conexión se revocó o venció (duran 90 días): vuelve a pulsar «Conectar» en el asistente. Si responde que llegaste al límite de consultas, se reinicia cada día a las 00:00 UTC.',
    },
    {
      q: 'El asistente no usa Obertrack al preguntarle',
      a: 'Comprueba que el conector esté activado en ese chat y menciónalo en la pregunta: «Usa Obertrack para…».',
    },
  ]
}

function TestPrompt() {
  return (
    <div className={styles.verifyBox}>
      <MessageSquareText size={18} />
      <div>
        <strong>Comprueba que funciona.</strong> Abre un chat nuevo con Obertrack activado y pregunta:
        <div style={{ marginTop: 6 }}><CopyChip value={TEST_PROMPT} id="prompt" label="la pregunta de prueba" /></div>
        <span className={styles.sub} style={{ marginTop: 4 }}>Debe responder con tu nombre, tu rol y lo que puedes consultar.</span>
      </div>
    </div>
  )
}

export interface ConnectionGuideProps {
  client: ClientKey
  url: string
  connections: McpConnection[]
  personalTokens: McpConnection[]
  newToken: string | null
  creatingToken: boolean
  onCreateToken: () => void
}

export default function ConnectionGuide(props: ConnectionGuideProps) {
  const { client, url } = props
  const isLocal = !url.startsWith('https://')
  const nameChip = <CopyChip value={CONNECTOR_NAME} id="name" label="el nombre" />
  const urlChip = <CopyChip value={url} id="url-step" label="la URL" />

  const connected =
    client !== 'gemini' &&
    props.connections.find((c) => CLIENT_MATCH[client].some((m) => c.client_name.toLowerCase().includes(m)))

  return (
    <div>
      {connected && (
        <div className={styles.okBox}>
          <CheckCircle2 size={18} />
          <span>Ya tienes conectado «{connected.client_name}». Si te funciona, no tienes que hacer nada más.</span>
        </div>
      )}
      {isLocal && client !== 'gemini' && (
        <div className={styles.banner} style={{ marginBottom: 14 }}>
          <AlertTriangle size={18} />
          <span>
            Esta es una URL local de desarrollo. ChatGPT, Claude y Grok se conectan desde internet y no pueden llegar a ella:
            para probarlos usa la URL pública de producción (<code className={styles.inlineCode}>https://obertrack.com/api/mcp</code>).
          </span>
        </div>
      )}

      {client === 'claude' && (
        <>
          <Requirements>
            Una cuenta de Claude: funciona en todos los planes (en el gratuito, con un solo conector personalizado).
            Si tu Claude es de empresa (Team o Enterprise), mira el último punto de «¿Algo no funciona?».
          </Requirements>
          <Steps
            steps={[
              {
                title: <>Abre <ExtLink href="https://claude.ai/customize/connectors">Personalizar → Conectores</ExtLink> en claude.ai</>,
                body: <>También llegas desde tu nombre, abajo a la izquierda → <UI es="Personalizar" en="Customize" /> → <UI es="Conectores" en="Connectors" />.</>,
              },
              { title: <>Pulsa <UI es="+ Agregar" en="+ Add" /> y elige <UI es="Agregar conector personalizado" en="Add custom connector" /></> },
              {
                title: 'Completa los dos campos y pulsa Continuar',
                body: <>Nombre: {nameChip} · URL del servidor: {urlChip} → <UI es="Continuar" en="Continue" />.</>,
              },
              {
                title: <>Deja la autenticación como viene y pulsa <UI es="Agregar" en="Add" /></>,
                body: <>Claude detecta solo que Obertrack usa inicio de sesión. Si te pregunta, elige <UI es="Iniciar sesión ahora" en="Sign in now" />. No hace falta escribir cabeceras ni claves.</>,
              },
              {
                title: <>Se abre Obertrack: revisa y pulsa <strong>Autorizar</strong></>,
                body: 'Si no tienes la sesión abierta, primero inicia sesión con tu usuario de Obertrack de siempre. Al autorizar vuelves solo a Claude.',
              },
              {
                title: 'Actívalo en el chat',
                body: <>En un chat, pulsa el <strong>+</strong> de abajo a la izquierda → <UI es="Conectores" en="Connectors" /> → activa <strong>Obertrack</strong>.</>,
              },
            ]}
          />
          <TestPrompt />
          <Faq
            items={[
              ...oauthFaq(url),
              {
                q: 'Mi Claude es de empresa (Team o Enterprise) y no veo «Agregar conector personalizado»',
                a: <>Lo agrega una sola vez un propietario de la organización en <UI es="Configuración de la organización → Conectores" en="Organization settings → Connectors" /> → <UI es="Agregar" en="Add" /> → <UI es="Personalizado → Web" en="Custom → Web" />, con el mismo nombre y URL. Después cada persona va a Personalizar → Conectores, busca Obertrack y pulsa <UI es="Conectar" en="Connect" />.</>,
              },
            ]}
          />
        </>
      )}

      {client === 'chatgpt' && (
        <>
          <Requirements>
            Un plan de pago de ChatGPT. En planes de empresa (Business, Enterprise o Edu), un administrador tiene que permitir antes
            los conectores personalizados (Modo desarrollador). OpenAI cambia estos menús a menudo: si un nombre no coincide, busca
            en Configuración la palabra <strong>«MCP»</strong> o <strong>«Desarrollador»</strong>.
          </Requirements>
          <Steps
            steps={[
              {
                title: <>En <ExtLink href="https://chatgpt.com">chatgpt.com</ExtLink>, abre tu foto de perfil → <UI es="Configuración" en="Settings" /> → <UI es="Apps" en="Apps" /></>,
                body: <>En algunas cuentas la sección se llama <UI es="Aplicaciones y conectores" en="Apps & Connectors" />.</>,
              },
              {
                title: <>Activa el <UI es="Modo desarrollador" en="Developer mode" /></>,
                body: <>Está en <UI es="Configuración avanzada" en="Advanced settings" />, al final de la sección. Si ya ves un botón para crear un conector o servidor MCP, sáltate este paso.</>,
              },
              {
                title: <>Pulsa <UI es="Crear" en="Create" /> (o <strong>+</strong> → <UI es="Agregar servidor MCP personalizado" en="Add custom MCP server" />)</>,
                body: <>Nombre: {nameChip} · URL del servidor MCP: {urlChip} · Autenticación: <strong>OAuth</strong>. Marca la casilla de que confías en la aplicación y pulsa <UI es="Crear" en="Create" />.</>,
              },
              {
                title: <>Se abre Obertrack: revisa y pulsa <strong>Autorizar</strong></>,
                body: 'Si no tienes la sesión abierta, primero inicia sesión con tu usuario de Obertrack. Al autorizar vuelves solo a ChatGPT.',
              },
              {
                title: 'Actívalo en el chat',
                body: <>En un chat nuevo, pulsa <strong>+</strong> (o <UI es="Más" en="More" />) y elige <strong>Obertrack</strong> entre las herramientas.</>,
              },
            ]}
          />
          <TestPrompt />
          <Faq
            items={[
              {
                q: 'No veo «Modo desarrollador» ni ningún botón para crear',
                a: 'Tu plan no lo incluye o, en una cuenta de empresa, el administrador del espacio de trabajo aún no permitió crear conectores MCP. Pídeselo a quien administra tu ChatGPT.',
              },
              ...oauthFaq(url),
            ]}
          />
        </>
      )}

      {client === 'grok' && (
        <>
          <Requirements>
            Una cuenta de grok.com. En Grok Business o Enterprise, un administrador tiene que habilitar antes los conectores para el equipo.
          </Requirements>
          <Steps
            steps={[
              { title: <>Abre <ExtLink href="https://grok.com/connectors">grok.com/connectors</ExtLink></> },
              { title: <>Pulsa <UI es="Nuevo conector" en="New Connector" /> y elige <UI es="Personalizado" en="Custom" /></> },
              { title: 'Completa los campos y guarda', body: <>Nombre: {nameChip} · URL del servidor: {urlChip}.</> },
              {
                title: <>Se abre Obertrack: revisa y pulsa <strong>Autorizar</strong></>,
                body: 'Si no tienes la sesión abierta, primero inicia sesión con tu usuario de Obertrack. Al autorizar vuelves solo a Grok.',
              },
              { title: 'Pregunta en un chat', body: 'Grok usa Obertrack cuando la pregunta lo necesita. Para asegurarte, empieza con «Usa Obertrack para…».' },
            ]}
          />
          <TestPrompt />
          <Faq items={oauthFaq(url)} />
        </>
      )}

      {client === 'gemini' && (
        <GeminiGuide url={url} newToken={props.newToken} creating={props.creatingToken} onCreate={props.onCreateToken} hasTokens={props.personalTokens.length > 0} />
      )}
    </div>
  )
}

function GeminiGuide({ url, newToken, creating, onCreate, hasTokens }: {
  url: string
  newToken: string | null
  creating: boolean
  onCreate: () => void
  hasTokens: boolean
}) {
  const token = newToken ?? 'obs_pt_TU_TOKEN'
  const command = `gemini mcp add --scope user --transport http --header "Authorization: Bearer ${token}" obertrack ${url}`
  const json = `{
  "mcpServers": {
    "obertrack": {
      "httpUrl": "${url}",
      "headers": { "Authorization": "Bearer ${token}" }
    }
  }
}`
  return (
    <>
      <Requirements>
        Gemini CLI instalado en tu computadora (<code className={styles.inlineCode}>npm install -g @google/gemini-cli</code>).
        Gemini CLI no usa el inicio de sesión de Obertrack: se conecta con un token personal que generas aquí.
      </Requirements>
      <Steps
        steps={[
          {
            title: 'Genera tu token personal',
            body: newToken ? (
              <span className={styles.okInline}><CheckCircle2 size={15} /> Token creado y puesto en el comando de abajo. Cópialo ahora: no se vuelve a mostrar.</span>
            ) : (
              <>
                <span>Da el mismo acceso de solo lectura que tu usuario y vence a los 90 días.{hasTokens && ' Ya tienes alguno, pero por seguridad no se puede volver a ver: crea uno nuevo.'}</span>
                <div style={{ marginTop: 8 }}>
                  <Button size="sm" leftIcon={<KeyRound size={14} />} loading={creating} onClick={onCreate}>
                    Generar token para Gemini CLI
                  </Button>
                </div>
              </>
            ),
          },
          {
            title: 'Pega este comando en tu terminal y pulsa Enter',
            body: (
              <>
                {!newToken && <span className={styles.sub}>Genera primero el token: el comando se completa solo.</span>}
                <CodeWithCopy code={command} id="gemini-cmd" />
              </>
            ),
          },
          {
            title: 'Comprueba la conexión',
            body: <>Ejecuta <code className={styles.inlineCode}>gemini mcp list</code>: debe aparecer <strong>obertrack</strong> con ✓. Dentro de Gemini CLI también puedes escribir <code className={styles.inlineCode}>/mcp</code>.</>,
          },
        ]}
      />
      <TestPrompt />
      <Faq
        items={[
          {
            q: 'Prefiero editar el archivo de configuración a mano',
            a: <>Agrega esto a <code className={styles.inlineCode}>~/.gemini/settings.json</code> (si ya tiene otros servidores, añade solo el bloque «obertrack» dentro de «mcpServers»):<CodeWithCopy code={json} id="gemini-json" /></>,
          },
          {
            q: '«gemini» no se reconoce como comando',
            a: <>Gemini CLI no está instalado o no está en el PATH. Instálalo con <code className={styles.inlineCode}>npm install -g @google/gemini-cli</code> y abre una terminal nueva.</>,
          },
          {
            q: 'obertrack aparece con ✗ (desconectado)',
            a: <>Revisa que el token esté completo (empieza por obs_pt_) y que no lo hayas revocado en «Tokens personales». Si dudas, quita la entrada con <code className={styles.inlineCode}>gemini mcp remove --scope user obertrack</code>, genera un token nuevo aquí y vuelve a ejecutar el comando.</>,
          },
          {
            q: 'Responde que el token no es válido',
            a: <>Los tokens personales vencen a los 90 días. Quita la entrada con <code className={styles.inlineCode}>gemini mcp remove --scope user obertrack</code>, genera un token nuevo aquí y vuelve a ejecutar el comando.</>,
          },
        ]}
      />
    </>
  )
}
