import { useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { BellRing, MousePointerClick, ShieldCheck } from 'lucide-react'
import { Modal, Button, Skeleton } from '../../ui'
import { useConfirm } from '../../ui/ConfirmProvider'
import { useNotification } from '../../../context/NotificationContext'
import { tutorialService } from '../../../services/api'
import { isEmptyTarget } from '../../../types'
import type { Tutorial } from '../../../types'
import styles from './TutorialMetricsModal.module.css'

interface TutorialMetricsModalProps {
  tutorial: Tutorial | null
  onClose: () => void
}

const USER_TYPE_LABELS: Record<string, string> = {
  empleador: 'Empresas',
  profesional: 'Profesionales',
  manager: 'Managers',
}

/** Resume el público acotado en una frase legible. */
function describeTarget(target: Tutorial['target']): string {
  const parts: string[] = []
  if (target.company_ids?.length) {
    parts.push(`${target.company_ids.length} ${target.company_ids.length === 1 ? 'empresa' : 'empresas'}`)
  }
  if (target.countries?.length) parts.push(target.countries.join(', '))
  if (target.group_ids?.length) {
    parts.push(`${target.group_ids.length} ${target.group_ids.length === 1 ? 'grupo' : 'grupos'}`)
  }
  if (target.managers_only) parts.push('solo con equipo a cargo')
  return parts.join(' · ')
}

function formatDate(value: string) {
  const date = new Date(value)
  return date.toLocaleDateString('es-ES', { day: '2-digit', month: '2-digit' }) +
    ' · ' + date.toLocaleTimeString('es-ES', { hour: '2-digit', minute: '2-digit' })
}

/**
 * Desempeño de una novedad. La pregunta que responde no es "cuántas vistas
 * tuvo" sino "a cuánta de la gente a la que iba dirigida le llegó", así que
 * todo se lee contra el alcance y no en números sueltos.
 */
export function TutorialMetricsModal({ tutorial, onClose }: TutorialMetricsModalProps) {
  const qc = useQueryClient()
  const confirm = useConfirm()
  const { success, error } = useNotification()
  const [reminding, setReminding] = useState(false)

  const { data: metrics, isLoading } = useQuery({
    queryKey: ['tutorial-metrics', tutorial?.id],
    queryFn: () => tutorialService.getMetrics(tutorial!.id),
    enabled: !!tutorial,
    staleTime: 30_000,
  })

  if (!tutorial) return null

  // Recordar es mandar otro aviso a media empresa: se confirma antes, y el
  // servidor además frena dos envíos seguidos.
  const handleRemind = async () => {
    if (!metrics) return
    const ok = await confirm({
      title: 'Recordar a los pendientes',
      message: `Se les avisará de nuevo a las ${metrics.pending} personas que aún no la han visto, y el aviso a pantalla completa volverá a salirles al entrar.`,
      confirmLabel: 'Recordar',
    })
    if (!ok) return
    setReminding(true)
    try {
      const reminded = await tutorialService.remindPending(tutorial.id)
      success(reminded === 1 ? 'Se recordó a 1 persona' : `Se recordó a ${reminded} personas`)
      await qc.invalidateQueries({ queryKey: ['tutorial-metrics', tutorial.id] })
      await qc.invalidateQueries({ queryKey: ['tutorials'] })
    } catch (err: any) {
      error(err?.response?.data?.error || 'No se pudo enviar el recordatorio')
    } finally {
      setReminding(false)
    }
  }

  const rate = metrics?.reach ? Math.min(100, metrics.view_rate) : 0
  const totalViews = metrics?.views ?? 0
  const announcementShare = totalViews ? ((metrics!.from_announcement / totalViews) * 100) : 0

  return (
    <Modal
      isOpen
      onClose={onClose}
      size="lg"
      title={`Métricas · ${tutorial.title}`}
      footer={
        <>
          {!!metrics && metrics.pending > 0 && (
            <Button variant="secondary" onClick={handleRemind} loading={reminding}>
              <BellRing size={15} /> Recordar a {metrics.pending} pendientes
            </Button>
          )}
          <Button variant="secondary" onClick={onClose}>Cerrar</Button>
        </>
      }
    >
      {isLoading || !metrics ? (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
          <Skeleton height={90} />
          <Skeleton height={70} />
          <Skeleton height={160} />
        </div>
      ) : metrics.reach === 0 ? (
        <div className={styles['empty']}>
          Ninguna cuenta activa entra en el público de esta novedad. Revisa a quién va dirigida.
        </div>
      ) : (
        <div className={styles['metrics']}>
          <div className={styles['headline']}>
            <div className={styles['hero']}>
              <span className={styles['hero-value']}>{rate}%</span>
              <span className={styles['hero-label']}>
                la vieron · {metrics.views} de {metrics.reach} personas
              </span>
            </div>
            <div
              className={styles['meter']}
              role="meter"
              aria-valuenow={rate}
              aria-valuemin={0}
              aria-valuemax={100}
              aria-label="Porcentaje de la audiencia que vio la novedad"
            >
              <div className={styles['meter-fill']} style={{ width: `${rate}%` }} />
            </div>
          </div>

          <div className={styles['tiles']}>
            <div className={styles['tile']}>
              <span className={styles['tile-label']}>Alcance</span>
              <span className={styles['tile-value']}>{metrics.reach}</span>
            </div>
            <div className={styles['tile']}>
              <span className={styles['tile-label']}>La vieron</span>
              <span className={styles['tile-value']}>{metrics.views}</span>
            </div>
            <div className={styles['tile']}>
              <span className={styles['tile-label']}>Les falta</span>
              <span className={styles['tile-value']}>{metrics.pending}</span>
            </div>
          </div>

          {(tutorial.cta_label || metrics.require_ack) && (
            <div className={styles['tiles']}>
              {tutorial.cta_label && (
                <div className={styles['tile']}>
                  <span className={styles['tile-label']}>
                    <MousePointerClick size={12} /> Pulsaron "{tutorial.cta_label}"
                  </span>
                  <span className={styles['tile-value']}>
                    {metrics.clicks}
                    {/* El clic se mide sobre quienes la vieron: contra el
                        alcance castigaría a la novedad por gente que ni
                        siquiera la abrió. */}
                    <small> · {metrics.click_rate}% de quienes la vieron</small>
                  </span>
                </div>
              )}
              {metrics.require_ack && (
                <div className={styles['tile']}>
                  <span className={styles['tile-label']}>
                    <ShieldCheck size={12} /> Confirmaron la lectura
                  </span>
                  <span className={styles['tile-value']}>
                    {metrics.acknowledged}
                    <small> de {metrics.reach}</small>
                  </span>
                </div>
              )}
            </div>
          )}

          {totalViews > 0 && (
            <div className={styles['section']}>
              <div className={styles['section-title']}>Cómo la vieron</div>
              {/* Dos clases: el color identifica y el número está siempre
                  escrito al lado, así que nada depende solo del color. */}
              <div className={styles['split']}>
                <div
                  className={`${styles['split-segment']} ${styles['split-anuncio']}`}
                  style={{ width: `${announcementShare}%` }}
                />
                {/* El segundo segmento toma el resto en vez de un ancho
                    calculado: así la separación de 2px sale del total y no
                    empuja el final de la barra fuera de la caja. */}
                <div className={`${styles['split-segment']} ${styles['split-seccion']}`} style={{ flex: 1 }} />
              </div>
              <div className={styles['legend']}>
                <span className={styles['legend-item']}>
                  <span className={styles['legend-dot']} style={{ background: 'var(--series-anuncio)' }} />
                  Aviso a pantalla completa
                  <span className={styles['legend-value']}>{metrics.from_announcement}</span>
                </span>
                <span className={styles['legend-item']}>
                  <span className={styles['legend-dot']} style={{ background: 'var(--series-seccion)' }} />
                  Desde la sección
                  <span className={styles['legend-value']}>{metrics.from_section}</span>
                </span>
              </div>
            </div>
          )}

          {metrics.by_audience.length > 1 && (
            <div className={styles['section']}>
              <div className={styles['section-title']}>Por tipo de cuenta</div>
              {metrics.by_audience.map((row) => {
                const pct = row.reach ? Math.round((row.views / row.reach) * 100) : 0
                return (
                  <div key={row.user_type} className={styles['breakdown-row']}>
                    <span>{USER_TYPE_LABELS[row.user_type] ?? row.user_type}</span>
                    <span className={styles['meter']}>
                      <span className={styles['meter-fill']} style={{ width: `${pct}%`, display: 'block' }} />
                    </span>
                    <span className={styles['breakdown-count']}>{row.views}/{row.reach}</span>
                  </div>
                )
              })}
            </div>
          )}

          <div className={styles['section']}>
            <div className={styles['section-title']}>Últimas personas que la vieron</div>
            {metrics.recent_viewers.length === 0 ? (
              <div className={styles['empty']}>Todavía no la ha visto nadie.</div>
            ) : (
              <div className={styles['viewers']}>
                {metrics.recent_viewers.map((viewer) => (
                  <div key={viewer.user_id} className={styles['viewer']}>
                    <span className={styles['viewer-name']}>
                      {viewer.name}
                      <small className={styles['viewer-email']}>{viewer.email}</small>
                    </span>
                    <span className={styles['viewer-source']}>
                      {viewer.acknowledged && (
                        <ShieldCheck size={12} className={styles['viewer-ack']} aria-label="Confirmó la lectura" />
                      )}
                      {viewer.clicked && (
                        <MousePointerClick size={12} className={styles['viewer-click']} aria-label="Pulsó el botón" />
                      )}
                      <span
                        className={styles['legend-dot']}
                        style={{
                          background: viewer.source === 'anuncio'
                            ? 'var(--series-anuncio)'
                            : 'var(--series-seccion)',
                        }}
                      />
                      {viewer.source === 'anuncio' ? 'Aviso' : 'Sección'}
                    </span>
                    <span className={styles['viewer-date']}>{formatDate(viewer.viewed_at)}</span>
                  </div>
                ))}
              </div>
            )}
          </div>

          {!isEmptyTarget(tutorial.target) && (
            <p className={styles['note']}>
              Público acotado: {describeTarget(tutorial.target)}. El alcance de arriba ya lo tiene en cuenta.
            </p>
          )}

          {metrics.reminded_at && (
            <p className={styles['note']}>
              Último recordatorio: {formatDate(metrics.reminded_at)}.
            </p>
          )}

          <p className={styles['note']}>
            {metrics.announced_at
              ? metrics.announce_open
                ? 'El aviso a pantalla completa sigue activo: los pendientes todavía pueden verlo al entrar.'
                : 'La ventana del aviso ya se cerró. Los pendientes solo la verán si entran a la sección.'
              : 'Esta novedad aún no se ha publicado, así que no se ha anunciado a nadie.'}
          </p>
        </div>
      )}
    </Modal>
  )
}
