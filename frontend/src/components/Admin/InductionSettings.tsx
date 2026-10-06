import { useCallback, useEffect, useMemo, useState } from 'react'
import { GraduationCap, Save, AlertTriangle } from 'lucide-react'

import { useNotification } from '../../context/NotificationContext'
import {
  inductionService,
  type InductionBlock,
  type InductionConfig,
  type InductionProgram,
  type InductionVideo,
} from '../../services/induction.service'
import { tutorialService } from '../../services/tutorial.service'
import type { TutorialAudienceOption } from '../../types/tutorials'
import InductionProgramList from './InductionProgramList'
import InductionBlockLibrary from './InductionBlockLibrary'
import CertificateTemplateList from '../Certificates/CertificateTemplateList'
import { draftFromProgram, isReady, programIssues } from './inductionReadiness'
import styles from './InductionSettings.module.css'

type Section = 'programs' | 'blocks' | 'certificates'

interface Step {
  key: Section
  label: string
  /** Estado en una línea: qué hay o qué falta. */
  sub: string
}

/**
 * Configuración de la inducción del profesional recién contratado.
 *
 * Tres piezas: el interruptor global (con la vigencia del enlace), la
 * biblioteca de bloques (video de su biblioteca + cuestionario propio) y los
 * programas (secuencias de bloques asignadas por empresa).
 *
 * Solo debe renderizarse para superadmin: es quien puede guardar en el backend.
 */
export default function InductionSettings() {
  const { success, error: showError } = useNotification()

  const [config, setConfig] = useState<InductionConfig | null>(null)
  const [programs, setPrograms] = useState<InductionProgram[]>([])
  const [blocks, setBlocks] = useState<InductionBlock[]>([])
  const [videos, setVideos] = useState<InductionVideo[]>([])
  const [companies, setCompanies] = useState<TutorialAudienceOption[]>([])
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [section, setSection] = useState<Section>('programs')

  const refresh = useCallback(async () => {
    const [programList, blockList, videoList] = await Promise.all([
      inductionService.listPrograms().catch(() => [] as InductionProgram[]),
      inductionService.listBlocks().catch(() => [] as InductionBlock[]),
      inductionService.listVideos().catch(() => [] as InductionVideo[]),
    ])
    setPrograms(programList)
    setBlocks(blockList)
    setVideos(videoList)
    return blockList
  }, [])
  // Para los hijos, que solo esperan que la lista se recargue.
  const reload = useCallback(async () => {
    await refresh()
  }, [refresh])

  useEffect(() => {
    const load = async () => {
      try {
        const [cfg, audience] = await Promise.all([
          inductionService.getConfig(),
          tutorialService.getAudienceOptions().catch(() => null),
        ])
        setConfig(cfg)
        setCompanies(audience?.companies ?? [])
        // Sin bloques no hay con qué armar un programa: se empieza por el paso 1.
        const blockList = await refresh()
        if (blockList.length === 0) setSection('blocks')
      } catch {
        showError('No se pudo cargar la configuración de inducción.')
      } finally {
        setLoading(false)
      }
    }
    void load()
  }, [refresh, showError])

  const defaultProgram = useMemo(() => programs.find((p) => p.is_default) ?? null, [programs])
  // Usable = activo y con bloques: lo mismo que exige el backend para encender.
  const canActivate = !!defaultProgram && defaultProgram.is_active && defaultProgram.block_count > 0

  // El armado tiene un orden natural: bloques, luego el programa que los
  // ordena, luego (opcional) el certificado que emite. Las pestañas lo
  // muestran como pasos numerados con su estado.
  const steps = useMemo<Step[]>(() => {
    const withQuestions = blocks.filter((b) => b.question_count > 0).length
    const pendingPrograms = programs.filter((p) => p.is_active && !isReady(programIssues(draftFromProgram(p)))).length
    const certifying = programs.filter((p) => !!p.certificate_template_id).length
    return [
      {
        key: 'blocks',
        label: 'Bloques',
        sub:
          blocks.length === 0
            ? 'Video + cuestionario'
            : `${blocks.length} ${blocks.length === 1 ? 'bloque' : 'bloques'}${
                withQuestions < blocks.length ? ` · ${blocks.length - withQuestions} sin preguntas` : ''
              }`,
      },
      {
        key: 'programs',
        label: 'Programas',
        sub:
          programs.length === 0
            ? 'Ordena los bloques'
            : pendingPrograms > 0
              ? `${pendingPrograms} con pendientes`
              : `${programs.length} ${programs.length === 1 ? 'programa listo' : 'programas listos'}`,
      },
      {
        key: 'certificates',
        label: 'Certificados',
        sub: certifying > 0 ? `En ${certifying} ${certifying === 1 ? 'programa' : 'programas'}` : 'Opcional',
      },
    ]
  }, [blocks, programs])

  const handleSaveConfig = async () => {
    if (!config) return
    setSaving(true)
    try {
      const saved = await inductionService.saveConfig({
        invite_ttl_days: config.invite_ttl_days,
        is_active: config.is_active,
      })
      setConfig(saved)
      success(saved.is_active ? 'Inducción encendida.' : 'Configuración guardada.')
    } catch (err: any) {
      showError(err?.response?.data?.error ?? 'No se pudo guardar la configuración.')
    } finally {
      setSaving(false)
    }
  }

  if (loading || !config) {
    return (
      <div className={styles.panel}>
        <p className={styles.muted}>Cargando inducción...</p>
      </div>
    )
  }

  return (
    <div className={styles.panel}>
      <div className={styles.head}>
        <div className={styles.icon}>
          <GraduationCap size={22} />
        </div>
        <div>
          <h2 className={styles.title}>Inducción de nuevos profesionales</h2>
          <p className={styles.intro}>
            Quien llega contratado recibe un enlace y recorre los bloques de su programa: en cada
            uno ve un video y responde su cuestionario. Si aprueba todos, se le habilita el acceso;
            si agota los intentos de un bloque, se abre una alerta en Soporte.
          </p>
        </div>
      </div>

      {!canActivate && (
        <div className={styles.warn}>
          <AlertTriangle size={16} style={{ flexShrink: 0, marginTop: 1 }} />
          <span>
            {defaultProgram
              ? 'El programa por defecto no tiene bloques (o está apagado). Agrégale al menos un bloque para poder activar la inducción.'
              : 'Crea un programa por defecto con al menos un bloque para poder activar la inducción.'}{' '}
            Mientras esté apagada, los profesionales contratados reciben acceso directo, como hasta ahora.
          </span>
        </div>
      )}

      <div className={styles.configCard}>
        <div className={styles.configMain}>
          <label className={styles.switch} title={config.is_active ? 'Apagar inducción' : 'Encender inducción'}>
            <input
              type="checkbox"
              checked={config.is_active}
              disabled={!canActivate && !config.is_active}
              onChange={(e) => setConfig({ ...config, is_active: e.target.checked })}
            />
            <span className={styles.slider} />
          </label>
          <div className={styles.configText}>
            <span className={styles.configTitle}>
              Inducción obligatoria
              <span className={config.is_active ? styles.pillOn : styles.pillOff}>
                {config.is_active ? 'Encendida' : 'Apagada'}
              </span>
            </span>
            <span className={styles.toggleHint}>
              {config.is_active
                ? 'Ningún profesional nuevo entra sin aprobar su programa.'
                : 'Los profesionales contratados reciben acceso directo.'}
            </span>
          </div>
        </div>
        <div className={styles.configRight}>
          <div className={styles.inlineField}>
            <label htmlFor="induction-ttl">Vigencia del enlace</label>
            <input
              id="induction-ttl"
              type="number"
              min={1}
              max={365}
              value={config.invite_ttl_days}
              onChange={(e) => setConfig({ ...config, invite_ttl_days: Number(e.target.value) })}
              aria-describedby="induction-ttl-hint"
            />
          </div>
          <button
            type="button"
            className={`${styles.saveBtn} ${styles.saveBtnSm}`}
            disabled={saving}
            onClick={handleSaveConfig}
          >
            <Save size={16} /> {saving ? 'Guardando...' : 'Guardar'}
          </button>
        </div>
      </div>
      <p id="induction-ttl-hint" className={styles.summaryLine}>
        {defaultProgram ? (
          <>
            <span>
              Programa por defecto: <strong>{defaultProgram.name}</strong> · {defaultProgram.block_count}{' '}
              {defaultProgram.block_count === 1 ? 'bloque' : 'bloques'} · mínimo {defaultProgram.default_passing_score}% ·{' '}
              {defaultProgram.max_attempts} intentos por bloque.
            </span>
            <span>El enlace del correo vence a los {config.invite_ttl_days} días.</span>
          </>
        ) : (
          <span>Sin programa por defecto. El enlace del correo vence a los {config.invite_ttl_days} días.</span>
        )}
      </p>

      <div className={styles.keySection}>
        <div className={styles.steps} role="tablist">
          {steps.map((step, i) => (
            <button
              key={step.key}
              type="button"
              role="tab"
              aria-selected={section === step.key}
              className={section === step.key ? styles.stepActive : styles.step}
              onClick={() => setSection(step.key)}
            >
              <span className={styles.stepNum}>{i + 1}</span>
              <span className={styles.stepText}>
                <span className={styles.stepLabel}>{step.label}</span>
                <span className={styles.stepSub}>{step.sub}</span>
              </span>
            </button>
          ))}
        </div>

        {section === 'certificates' ? (
          <CertificateTemplateList onChanged={reload} />
        ) : section === 'programs' ? (
          <InductionProgramList
            programs={programs}
            library={blocks}
            companies={companies}
            onChanged={reload}
            onGoToBlocks={() => setSection('blocks')}
          />
        ) : (
          <InductionBlockLibrary
            blocks={blocks}
            videos={videos}
            fallbackPassingScore={defaultProgram?.default_passing_score ?? 70}
            programs={programs}
            onChanged={reload}
          />
        )}
      </div>
    </div>
  )
}
