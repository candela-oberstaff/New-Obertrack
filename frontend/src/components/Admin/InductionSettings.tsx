import { useCallback, useEffect, useMemo, useState } from 'react'
import { GraduationCap, Save, AlertTriangle, Route, Layers } from 'lucide-react'

import { useNotification } from '../../context/NotificationContext'
import {
  inductionService,
  type InductionBlock,
  type InductionConfig,
  type InductionProgram,
} from '../../services/induction.service'
import { tutorialService } from '../../services/tutorial.service'
import type { Tutorial, TutorialAudienceOption } from '../../types/tutorials'
import InductionProgramList from './InductionProgramList'
import InductionBlockLibrary from './InductionBlockLibrary'
import styles from './InductionSettings.module.css'

type Section = 'programs' | 'blocks'

/**
 * Configuración de la inducción del profesional recién contratado.
 *
 * Tres piezas: el interruptor global (con la vigencia del enlace), la
 * biblioteca de bloques (video de Novedades + cuestionario propio) y los
 * programas (secuencias de bloques asignadas por empresa).
 *
 * Solo debe renderizarse para superadmin: es quien puede guardar en el backend.
 */
export default function InductionSettings() {
  const { success, error: showError } = useNotification()

  const [config, setConfig] = useState<InductionConfig | null>(null)
  const [programs, setPrograms] = useState<InductionProgram[]>([])
  const [blocks, setBlocks] = useState<InductionBlock[]>([])
  const [tutorials, setTutorials] = useState<Tutorial[]>([])
  const [companies, setCompanies] = useState<TutorialAudienceOption[]>([])
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [section, setSection] = useState<Section>('programs')

  const refresh = useCallback(async () => {
    const [programList, blockList] = await Promise.all([
      inductionService.listPrograms().catch(() => [] as InductionProgram[]),
      inductionService.listBlocks().catch(() => [] as InductionBlock[]),
    ])
    setPrograms(programList)
    setBlocks(blockList)
  }, [])

  useEffect(() => {
    const load = async () => {
      try {
        const [cfg, tutorialList, audience] = await Promise.all([
          inductionService.getConfig(),
          tutorialService.getAll().catch(() => [] as Tutorial[]),
          tutorialService.getAudienceOptions().catch(() => null),
        ])
        setConfig(cfg)
        setTutorials(Array.isArray(tutorialList) ? tutorialList : [])
        setCompanies(audience?.companies ?? [])
        await refresh()
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
        <div className={styles.subTabs}>
          <button
            type="button"
            className={section === 'programs' ? styles.subTabActive : styles.subTab}
            onClick={() => setSection('programs')}
          >
            <Route size={15} /> Programas <span className={styles.count}>{programs.length}</span>
          </button>
          <button
            type="button"
            className={section === 'blocks' ? styles.subTabActive : styles.subTab}
            onClick={() => setSection('blocks')}
          >
            <Layers size={15} /> Bloques <span className={styles.count}>{blocks.length}</span>
          </button>
        </div>

        {section === 'programs' ? (
          <InductionProgramList
            programs={programs}
            library={blocks}
            companies={companies}
            onChanged={refresh}
            onGoToBlocks={() => setSection('blocks')}
          />
        ) : (
          <InductionBlockLibrary
            blocks={blocks}
            tutorials={tutorials}
            fallbackPassingScore={defaultProgram?.default_passing_score ?? 70}
            programs={programs}
            onChanged={refresh}
          />
        )}
      </div>
    </div>
  )
}
