/**
 * Obervoice — página de telefonía.
 *
 * Comportamiento según rol:
 *  - superadmin / customer_success → vista de administración Obervoice + accesos directos
 *  - empleador con datos          → credenciales SIP (con copiado fácil) + Obervoice
 *  - empleador sin datos          → publicidad interactiva + solicitud (ticket + mail a Lucía)
 *  - profesional                  → aviso de servicio corporativo
 */
import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import {
  Phone, User as UserIcon, Mail, Hash, KeyRound,
  Building2, ExternalLink, CheckCircle2,
  Copy, Check, ShieldCheck, HelpCircle, PhoneCall,
  Sparkles, Globe, Cpu, Users, Shield, Zap
} from 'lucide-react'
import { useAuth } from '../context/AuthContext'
import styles from './Obervoice.module.css'

const WEBRTC_URL = 'https://voice.oberstaff.com/webrtc/login'

// ─────────────────────────────────────────────────────────────────────────────

export default function Obervoice() {
  const { user } = useAuth()
  const navigate = useNavigate()

  const isAdmin = user?.is_superadmin || user?.user_type === 'customer_success'
  const isEmployer = user?.user_type === 'empleador'
  const isProfessional = user?.user_type === 'profesional'

  const hasCredentials = !!(
    user?.obervoice_username ||
    user?.obervoice_extension ||
    (user as any)?.obervoice_phone
  )

  // ── Superadmin / CS → Vista de Administración Obervoice ─────────────────
  if (isAdmin) {
    return (
      <div className={styles.page}>
        <div className={styles.header}>
          <div className={styles.headerTitleGroup}>
            <div className={styles.headerIconBadge}>
              <Phone size={26} />
            </div>
            <div>
              <h1>Obervoice — Panel de Control</h1>
              <p>Gestión de telefonía virtual y acceso a Obervoice.</p>
            </div>
          </div>
        </div>

        <div className={styles.mainGrid}>
          <div className={styles.leftCol}>
            <div className={styles.card}>
              <div className={styles.cardHeader}>
                <h3 className={styles.cardTitle}>
                  <ShieldCheck size={18} style={{ color: '#cc33cc' }} /> Acceso Administrador
                </h3>
              </div>
              <p style={{ fontSize: 14, color: '#64748b', marginBottom: 24, lineHeight: 1.6 }}>
                Como administrador tienes acceso completo para gestionar las extensiones y credenciales SIP de todas las empresas y profesionales activos en la plataforma.
              </p>
              <div style={{ display: 'flex', gap: 14, flexWrap: 'wrap', justifyContent: 'center' }}>
                <button
                  onClick={() => navigate('/admin?tab=obervoice')}
                  className={styles.softphonePrimaryBtn}
                >
                   Gestionar usuarios en Panel Admin
                </button>
              </div>
            </div>
          </div>

          <div className={styles.rightCol}>
            <div className={styles.softphoneHeroCard}>
              <h3 className={styles.softphoneHeroTitle}>Obervoice</h3>
              <p className={styles.softphoneHeroDesc}>
                Realizá y recibí llamadas directamente desde tu navegador sin instalar ningún software adicional.
              </p>
              <a
                href={WEBRTC_URL}
                target="_blank"
                rel="noopener noreferrer"
                className={styles.softphonePrimaryBtn}
              >
                <PhoneCall size={18} /> Iniciar Obervoice <ExternalLink size={14} />
              </a>
            </div>
          </div>
        </div>
      </div>
    )
  }

  // ── Profesional → No disponible ─────────────────────────────────────────
  if (isProfessional) {
    return (
      <div className={styles.page}>
        <div className={styles.header}>
          <div className={styles.headerTitleGroup}>
            <div className={styles.headerIconBadge}>
              <Phone size={26} />
            </div>
            <div>
              <h1>Obervoice</h1>
              <p>Servicio de telefonía para empresas.</p>
            </div>
          </div>
        </div>
        <div className={styles.card} style={{ maxWidth: 520, textAlign: 'center', padding: '3rem 2rem', margin: '20px auto' }}>
          <div style={{ fontSize: 48, marginBottom: 16 }}>📵</div>
          <h2 style={{ margin: '0 0 10px', fontSize: 20, fontWeight: 800, color: '#0f172a' }}>
            Servicio no disponible
          </h2>
          <p style={{ fontSize: 14, color: '#64748b', lineHeight: 1.6, margin: 0 }}>
            Obervoice es un servicio exclusivo de telefonía corporativa para empresas. Tu cuenta de profesional no cuenta con asignación de línea.
          </p>
        </div>
      </div>
    )
  }

  // ── Empleador sin datos → Solicitud / Publicidad ──────────────────────────
  if (isEmployer && !hasCredentials) {
    return <ObervoiceAd user={user} />
  }

  // ── Empleador con datos → Mis credenciales y Softphone ───────────────────
  return <ObervoiceCredentials user={user} />
}

// ─────────────────────────────────────────────────────────────────────────────

function ObervoiceCredentials({ user }: { user: any }) {
  const [copied, setCopied] = useState<string | null>(null)

  const sipUsername  = user?.obervoice_username  || user?.email || '—'
  const sipExtension = user?.obervoice_extension || '—'
  const sipPrefix    = user?.obervoice_prefix    || '—'
  const sipPhone     = user?.obervoice_phone     || '—'

  const copyToClipboard = (text: string, label: string) => {
    if (!text || text === '—') return
    navigator.clipboard.writeText(text)
    setCopied(label)
    setTimeout(() => setCopied(null), 2000)
  }

  return (
    <div className={styles.page}>
      {/* Encabezado */}
      <div className={styles.header}>
        <div className={styles.headerTitleGroup}>
          <div className={styles.headerIconBadge}>
            <Phone size={26} />
          </div>
          <div>
            <h1>Obervoice — Telefonía Virtual</h1>
            <p>Tus credenciales SIP y acceso directo al softphone corporativo.</p>
          </div>
        </div>

        <div className={styles.topActions}>
          <a
            href={WEBRTC_URL}
            target="_blank"
            rel="noopener noreferrer"
            className={styles.softphonePrimaryBtn}
          >
            <PhoneCall size={16} /> Abrir Obervoice <ExternalLink size={14} style={{ opacity: 0.8 }} />
          </a>
        </div>
      </div>

      {/* Grid Principal de 2 columnas */}
      <div className={styles.mainGrid}>
        
        {/* Columna Izquierda: Datos y Credenciales */}
        <div className={styles.leftCol}>
          
          {/* Card: Credenciales SIP */}
          <div className={styles.card}>
            <div className={styles.cardHeader}>
              <h3 className={styles.cardTitle}>
                <KeyRound size={18} style={{ color: '#cc33cc' }} /> Credenciales de Telefonía SIP
              </h3>
              <span style={{ fontSize: 12, fontWeight: 700, color: '#059669', background: 'rgba(16,185,129,0.1)', padding: '4px 10px', borderRadius: 99 }}>
                ✓ Línea Activa
              </span>
            </div>

            <div className={styles.grid2Col}>
              <CredBlock
                icon={<Mail size={13} />}
                label="Usuario SIP"
                value={sipUsername}
                onCopy={() => copyToClipboard(sipUsername, 'usuario')}
                isCopied={copied === 'usuario'}
              />
              <CredBlock
                icon={<Hash size={13} />}
                label="Extensión"
                value={sipExtension}
                onCopy={() => copyToClipboard(sipExtension, 'extension')}
                isCopied={copied === 'extension'}
              />
              <CredBlock
                icon={<Hash size={13} />}
                label="Prefijo de salida"
                value={sipPrefix}
                onCopy={() => copyToClipboard(sipPrefix, 'prefijo')}
                isCopied={copied === 'prefijo'}
              />
              <CredBlock
                icon={<Phone size={13} />}
                label="Teléfono Asignado"
                value={sipPhone}
                onCopy={() => copyToClipboard(sipPhone, 'telefono')}
                isCopied={copied === 'telefono'}
              />
              <CredBlock
                icon={<KeyRound size={13} />}
                label="Servidor SIP"
                value="voice.oberstaff.com"
                onCopy={() => copyToClipboard('voice.oberstaff.com', 'servidor')}
                isCopied={copied === 'servidor'}
              />
              <CredBlock
                icon={<Hash size={13} />}
                label="Puerto SIP / WebRTC"
                value="5060 / 8089"
                onCopy={() => copyToClipboard('5060', 'puerto')}
                isCopied={copied === 'puerto'}
              />
            </div>

            <div style={{ marginTop: 20, padding: '14px 16px', background: '#f8fafc', borderRadius: 12, border: '1px solid #eef2f7', display: 'flex', alignItems: 'center', gap: 10 }}>
              <HelpCircle size={16} style={{ color: '#cc33cc', flexShrink: 0 }} />
              <span style={{ fontSize: 12.5, color: '#64748b', lineHeight: 1.4 }}>
                Puedes configurar estas credenciales en cualquier cliente SIP estándar como MicroSIP, Zoiper, Linphone o usar el softphone web oficial.
              </span>
            </div>
          </div>

          {/* Card: Información de la Cuenta */}
          <div className={styles.card}>
            <div className={styles.cardHeader}>
              <h3 className={styles.cardTitle}>
                <UserIcon size={18} style={{ color: '#cc33cc' }} /> Información del Titular
              </h3>
            </div>

            <div className={styles.grid2Col}>
              <InfoBlock icon={<UserIcon size={13} />} label="Nombre completo" value={user?.name} />
              <InfoBlock icon={<Mail size={13} />} label="Correo de cuenta" value={user?.email} />
              <InfoBlock icon={<Phone size={13} />} label="Teléfono de contacto" value={user?.phone_number} />
              <InfoBlock icon={<Building2 size={13} />} label="Empresa vinculada" value={user?.company_name} />
            </div>
          </div>

        </div>

        {/* Columna Derecha: Softphone Hero & Guía Rápida */}
        <div className={styles.rightCol}>
          
          {/* Card Hero: WebRTC Softphone */}
          <div className={styles.softphoneHeroCard}>
            <span className={styles.badgeGlow}>Conexión WebRTC en Vivo</span>
            <h3 className={styles.softphoneHeroTitle}>Teléfono Virtual Obervoice</h3>
            <p className={styles.softphoneHeroDesc}>
              Haz y recibe llamadas corporativas en tiempo real directamente desde tu navegador con la mejor calidad de audio.
            </p>
            <a
              href={WEBRTC_URL}
              target="_blank"
              rel="noopener noreferrer"
              className={styles.softphonePrimaryBtn}
            >
              <PhoneCall size={18} /> Abrir Teléfono <ExternalLink size={15} />
            </a>
            <span style={{ fontSize: 11, color: 'rgba(255,255,255,0.5)', textAlign: 'center' }}>
              URL: voice.oberstaff.com/webrtc/login
            </span>
          </div>

          {/* Card: Guía Rápida */}
          <div className={styles.guideCard}>
            <h4 style={{ margin: '0 0 16px', fontSize: 14, fontWeight: 800, color: '#0f172a' }}>
              ¿Cómo conectarte en 3 pasos?
            </h4>

            <div className={styles.guideStep}>
              <div className={styles.guideStepNumber}>1</div>
              <div className={styles.guideStepContent}>
                <strong>Abre el Obervoice</strong>
                <span>Haz clic en el botón superior para ingresar al portal de telefonía.</span>
              </div>
            </div>

            <div className={styles.guideStep}>
              <div className={styles.guideStepNumber}>2</div>
              <div className={styles.guideStepContent}>
                <strong>Ingresa tu Usuario SIP y Contraseña</strong>
                <span>Copia tu usuario SIP asignado e ingresa la clave provista por el administrador.</span>
              </div>
            </div>

            <div className={styles.guideStep}>
              <div className={styles.guideStepNumber}>3</div>
              <div className={styles.guideStepContent}>
                <strong>Permití acceso al Micrófono</strong>
                <span>Concede el permiso en tu navegador para realizar llamadas claras.</span>
              </div>
            </div>
          </div>

        </div>

      </div>
    </div>
  )
}

// ─────────────────────────────────────────────────────────────────────────────

function InfoBlock({ icon, label, value }: { icon: React.ReactNode; label: string; value?: string | null }) {
  return (
    <div className={styles.infoBlock}>
      <span className={styles.infoLabel}>{icon} {label}</span>
      {value
        ? <span className={styles.infoValue}>{value}</span>
        : <span className={styles.infoValueEmpty}>Sin datos</span>
      }
    </div>
  )
}

function CredBlock({
  icon, label, value, onCopy, isCopied
}: {
  icon: React.ReactNode; label: string; value: string; onCopy: () => void; isCopied: boolean
}) {
  return (
    <div className={styles.infoBlock}>
      <div className={styles.credRowHeader}>
        <span className={styles.infoLabel}>{icon} {label}</span>
        {value && value !== '—' && (
          <button type="button" onClick={onCopy} className={styles.copyBtn} title="Copiar al portapapeles">
            {isCopied ? <Check size={12} style={{ color: '#059669' }} /> : <Copy size={12} />}
            {isCopied ? '¡Copiado!' : 'Copiar'}
          </button>
        )}
      </div>
      <span className={styles.credValue}>{value}</span>
    </div>
  )
}

// ─────────────────────────────────────────────────────────────────────────────

/** Vista Publicitaria para empresas que aún no contrataron Obervoice */
function ObervoiceAd({ user }: { user: any }) {
  const [sent, setSent] = useState(false)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const handleRequest = async () => {
    setLoading(true); setError(null)
    try {
      const res = await fetch('/api/tickets/internal/obervoice-request', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({
          company_name:     user?.company_name || user?.name || 'Empresa',
          requester_email:  user?.email || '',
          employer_id:      user?.id || 0,
        }),
      })
      if (!res.ok) {
        const d = await res.json().catch(() => ({}))
        throw new Error(d.error || `Error ${res.status}`)
      }
      setSent(true)
    } catch (e: any) {
      setError(e?.message || 'No se pudo enviar la solicitud.')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className={styles.page}>
      <div className={styles.promoContainer}>
        
        {/* ── Hero Banner Principal ── */}
        <div className={styles.promoHeroCard}>
          <div className={styles.promoHeroGrid}>
            <div>
              <div className={styles.promoBadges}>
                <span className={styles.promoBadge}>
                  <Sparkles size={13} /> Telefonía IP de Nueva Generación
                </span>
                <span className={styles.promoBadge}>
                  <Zap size={13} /> 100% Integrado en Obertrack
                </span>
              </div>
              <h1 className={styles.promoHeroTitle}>
                La Telefonía Virtual Inteligente para tu Empresa
              </h1>
              <p className={styles.promoHeroSubtitle}>
                Conecta a todo tu equipo con un conmutador cloud WebRTC. Realiza llamadas HD directamente desde el navegador, asigna extensiones a tus profesionales y gestiona la telefonía de tu organización de forma centralizada sin instalaciones ni equipos físicos.
              </p>
              
              {!sent && (
                <button
                  onClick={handleRequest}
                  disabled={loading}
                  className={styles.softphonePrimaryBtn}
                  style={{ padding: '14px 28px', fontSize: 15, opacity: loading ? 0.7 : 1 }}
                >
                  <PhoneCall size={18} />
                  {loading ? 'Enviando solicitud…' : 'Solicitar Obervoice para mi Empresa'}
                </button>
              )}
            </div>

            <div className={styles.heroImageWrapper}>
              <img
                src="/obervoice_hero.jpg"
                alt="Obervoice Virtual Telephony System"
                className={styles.heroImage}
              />
            </div>
          </div>
        </div>

        {/* ── Beneficios Principales (Grid 3x2) ── */}
        <div>
          <div style={{ textAlign: 'center', marginBottom: 28 }}>
            <h2 style={{ fontSize: 24, fontWeight: 900, color: '#0f172a', margin: '0 0 6px 0', letterSpacing: '-0.02em' }}>
              ¿Por qué activar Obervoice en tu organización?
            </h2>
            <p style={{ fontSize: 14.5, color: '#64748b', margin: 0 }}>
              Potencia la comunicación de tu equipo con herramientas avanzadas diseñadas para la gestión moderna.
            </p>
          </div>

          <div className={styles.promoFeaturesGrid}>
            <div className={styles.promoFeatureCard}>
              <div className={styles.promoFeatureIcon}>
                <Globe size={22} />
              </div>
              <div>
                <h3 className={styles.promoFeatureTitle}>Obervoice Integrado</h3>
                <p className={styles.promoFeatureDesc}>
                  Llamadas HD desde cualquier navegador web sin necesidad de instalar clientes pesados ni hardware telefónico.
                </p>
              </div>
            </div>

            <div className={styles.promoFeatureCard}>
              <div className={styles.promoFeatureIcon}>
                <Users size={22} />
              </div>
              <div>
                <h3 className={styles.promoFeatureTitle}>Extensiones por Profesional</h3>
                <p className={styles.promoFeatureDesc}>
                  Asignación de números cortos internos a cada miembro del equipo para transferencias y comunicación fluida.
                </p>
              </div>
            </div>

            <div className={styles.promoFeatureCard}>
              <div className={styles.promoFeatureIcon}>
                <Cpu size={22} />
              </div>
              <div>
                <h3 className={styles.promoFeatureTitle}>Compatibilidad SIP Universal</h3>
                <p className={styles.promoFeatureDesc}>
                  Usá tus credenciales en aplicaciones SIP estándar (MicroSIP, Zoiper, Linphone) o teléfonos IP físicos.
                </p>
              </div>
            </div>

            <div className={styles.promoFeatureCard}>
              <div className={styles.promoFeatureIcon}>
                <Zap size={22} />
              </div>
              <div>
                <h3 className={styles.promoFeatureTitle}>Alta Inmediata de Credenciales</h3>
                <p className={styles.promoFeatureDesc}>
                  Configuración rápida y envío automático de accesos al correo corporativo de cada profesional.
                </p>
              </div>
            </div>

            <div className={styles.promoFeatureCard}>
              <div className={styles.promoFeatureIcon}>
                <Shield size={22} />
              </div>
              <div>
                <h3 className={styles.promoFeatureTitle}>Control & Gestión Centralizada</h3>
                <p className={styles.promoFeatureDesc}>
                  Administra líneas, contraseñas SIP y estado de conexión desde la misma plataforma de Obertrack.
                </p>
              </div>
            </div>

            <div className={styles.promoFeatureCard}>
              <div className={styles.promoFeatureIcon}>
                <Sparkles size={22} />
              </div>
              <div>
                <h3 className={styles.promoFeatureTitle}>Atención & Setup Personalizado</h3>
                <p className={styles.promoFeatureDesc}>
                  Nuestro equipo de Customer Success configura la central telefónica según la estructura de tu empresa.
                </p>
              </div>
            </div>
          </div>
        </div>

        {/* ── Banner Final CTA ── */}
        <div className={styles.promoCtaCard}>
          <div className={styles.promoCtaText}>
            <h3>Empieza a llamar con Obervoice</h3>
            <p>
              Haz clic en el botón para solicitar la contratación. Se generará un ticket de alta y notificaremos a nuestro equipo de atención para ponerse en contacto contigo.
            </p>
          </div>

          <div>
            {sent ? (
              <div style={{
                display: 'flex', alignItems: 'center', gap: 10, background: 'rgba(255, 255, 255, 0.95)',
                color: '#059669', padding: '16px 24px', borderRadius: 14, fontWeight: 800, fontSize: 15,
                boxShadow: '0 8px 20px rgba(0,0,0,0.15)'
              }}>
                <CheckCircle2 size={22} />
                ¡Solicitud enviada a nuestro equipo! Te contactaremos a la brevedad.
              </div>
            ) : (
              <button
                onClick={handleRequest}
                disabled={loading}
                className={styles.promoCtaBtn}
                style={{ opacity: loading ? 0.7 : 1 }}
              >
                <PhoneCall size={18} />
                {loading ? 'Enviando solicitud…' : 'Solicitar Obervoice para mi Empresa'}
              </button>
            )}
            {error && (
              <p style={{ marginTop: 8, fontSize: 13, color: '#fee2e2', textAlign: 'center' }}>{error}</p>
            )}
          </div>
        </div>

      </div>
    </div>
  )
}
