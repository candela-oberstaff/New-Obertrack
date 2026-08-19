import {
  useCallback, useEffect, useMemo, useRef, useState,
  type MutableRefObject, type PointerEvent as ReactPointerEvent,
} from 'react'
import {
  DndContext,
  DragOverlay,
  PointerSensor,
  useSensor,
  useSensors,
  useDraggable,
  useDroppable,
  pointerWithin,
  type DragEndEvent,
  type DragStartEvent,
} from '@dnd-kit/core'
import { Network, List, Building2, Maximize2, Minimize2, ZoomIn, ZoomOut, Scan } from 'lucide-react'
import Avatar from '../Common/Avatar'
import { buildForest, dropRejection, teamSizes, type OrgPerson, type OrgTreeNode } from './orgTree'
import styles from './OrgChart.module.css'

export interface OrgChartProps {
  people: OrgPerson[]
  /** Sin esto el árbol se dibuja pero no se puede reordenar. */
  onReassign?: (userId: number, newManagerId: number | null) => Promise<void>
  hint?: string
  /**
   * Enlace al perfil de cada persona. Lo decide quien monta el organigrama
   * porque la ficha vive en rutas distintas según quién mire —/admin/users/:id
   * para superadmin y customer success, /empresa/employees/:id para el
   * empleador— y un supervisor no tiene acceso a ninguna de las dos. Si esto no
   * viene, las tarjetas no se enlazan: mejor sin enlace que con uno que lleva a
   * una pantalla prohibida.
   */
  profileHref?: (person: OrgPerson) => string | null
}

type View = 'tree' | 'list'

export function OrgChart({ people, onReassign, hint, profileHref }: OrgChartProps) {
  const [view, setView] = useState<View>('tree')
  const [dragId, setDragId] = useState<number | null>(null)
  const [message, setMessage] = useState<{ text: string; error: boolean } | null>(null)
  const [busy, setBusy] = useState(false)
  const [fullscreen, setFullscreen] = useState(false)

  // Soltar una tarjeta dispara un clic sobre ella justo después. Sin esta marca,
  // cada vez que alguien reorganiza el equipo se le abriría además el perfil de
  // la persona que acaba de mover.
  const justDragged = useRef(false)

  // Escape cierra la vista ampliada. Es lo que espera cualquiera que la haya
  // abierto, y evita quedar atrapado si el botón queda fuera de la pantalla en
  // un árbol ancho.
  useEffect(() => {
    if (!fullscreen) return
    const onKey = (e: KeyboardEvent) => { if (e.key === 'Escape') setFullscreen(false) }
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [fullscreen])

  // Con el organigrama ocupando toda la pantalla, el fondo no debe seguir
  // desplazándose por detrás.
  useEffect(() => {
    if (!fullscreen) return
    const previous = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    return () => { document.body.style.overflow = previous }
  }, [fullscreen])

  const forest = useMemo(() => buildForest(people), [people])
  const sizes = useMemo(() => teamSizes(people), [people])
  const editable = !!onReassign
  // Con la empresa de raíz, soltar sobre su tarjeta ya ES "quitar el manager".
  //
  // Y donde NO hay cuenta de empresa (la vista del supervisor, que es raíz de su
  // propia rama) tampoco debe ofrecerse: dejar a alguien sin manager lo sacaría
  // del árbol del supervisor, que dejaría de poder tocarlo. Puerta de una sola
  // dirección. Por eso no hay zona de "quitar el manager" en ningún caso.
  const companyNode = people.find(p => p.is_company)

  // Arrancar el arrastre pide 5px de movimiento, igual que el tablero de tareas:
  // así un clic para leer una tarjeta no dispara un cambio de organigrama.
  const sensors = useSensors(useSensor(PointerSensor, { activationConstraint: { distance: 5 } }))

  const dragged = dragId != null ? people.find(p => p.user_id === dragId) ?? null : null

  // ── Lienzo ────────────────────────────────────────────────────────────────
  // El árbol se mueve trasladándolo, no desplazando el scroll del contenedor.
  // La diferencia importa: con scroll solo se puede mover lo que desborda, así
  // que en cuanto el árbol entraba entero —al alejarlo, por ejemplo— se quedaba
  // clavado. Una pizarra se mueve siempre.
  //
  // Trasladar es seguro para el arrastre de personas: las tarjetas no aplican
  // el transform de dnd-kit (el fantasma lo dibuja DragOverlay), y la detección
  // es pointerWithin, que compara la posición del puntero contra rectángulos ya
  // trasladados. Ambos viven en coordenadas de la ventana, así que siguen
  // coincidiendo.
  const panRef = useRef<HTMLDivElement | null>(null)
  const contentRef = useRef<HTMLUListElement | null>(null)
  const [panning, setPanning] = useState(false)
  const [offset, setOffset] = useState({ x: 0, y: 0 })
  const panFrom = useRef({ x: 0, y: 0, ox: 0, oy: 0 })

  // No se deja empujar el árbol fuera de la vista: siempre queda un asomo por
  // el que volver a agarrarlo. Sin esto es fácil perderlo de un manotazo.
  const clampOffset = (x: number, y: number) => {
    const box = panRef.current
    const el = contentRef.current
    if (!box || !el) return { x, y }
    const w = el.getBoundingClientRect().width
    const h = el.getBoundingClientRect().height
    const KEEP = 120
    return {
      x: Math.min(box.clientWidth - KEEP, Math.max(KEEP - w, x)),
      y: Math.min(box.clientHeight - KEEP, Math.max(KEEP - h, y)),
    }
  }

  // ── Zoom ──────────────────────────────────────────────────────────────────
  // Con 30 o 40 personas el árbol es más ancho que cualquier pantalla, y
  // desplazarse no alcanza: hay que poder alejarse para verlo entero.
  //
  // Se usa la propiedad 'zoom' y no 'transform: scale' porque zoom SÍ afecta al
  // layout: el área desplazable pasa a medir el árbol ya reducido, en vez de
  // conservar el tamaño original y dejar un vacío alrededor. Y como
  // getBoundingClientRect devuelve las medidas ya ajustadas, dnd-kit sigue
  // ubicando bien las zonas de soltado —que era el motivo para no usar scale—.
  const [zoom, setZoom] = useState(1)
  const MIN_ZOOM = 0.3
  const MAX_ZOOM = 1.2

  const naturalWidth = () => {
    const el = contentRef.current
    if (!el) return 0
    // El rect ya viene multiplicado por el zoom; se deshace para saber cuánto
    // mide el árbol de verdad.
    return el.getBoundingClientRect().width / zoom
  }

  // Deja la raíz del árbol centrada y arriba, que es desde donde se lee.
  const centerView = useCallback(() => {
    const box = panRef.current
    const el = contentRef.current
    if (!box || !el) return
    const w = el.getBoundingClientRect().width
    setOffset({ x: Math.round((box.clientWidth - w) / 2), y: 16 })
  }, [])

  const fitToWidth = useCallback(() => {
    const box = panRef.current
    const natural = naturalWidth()
    if (!box || !natural) return
    const available = box.clientWidth - 24
    const next = Math.min(1, Math.max(MIN_ZOOM, available / natural))
    setZoom(Math.round(next * 100) / 100)
  }, [zoom])

  const stepZoom = (delta: number) =>
    setZoom(z => Math.min(MAX_ZOOM, Math.max(MIN_ZOOM, Math.round((z + delta) * 100) / 100)))

  // Al abrir la vista ampliada se ajusta solo para que entre el árbol entero:
  // ampliar y seguir sin ver a nadie no ayudaba a nadie.
  useEffect(() => {
    if (view !== 'tree') return
    const id = requestAnimationFrame(() => { if (fullscreen) fitToWidth(); centerView() })
    return () => cancelAnimationFrame(id)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [fullscreen, view])

  // Cambiar el zoom mueve el punto de referencia; se vuelve a centrar para no
  // perder de vista dónde estabas.
  useEffect(() => {
    if (view !== 'tree') return
    const id = requestAnimationFrame(centerView)
    return () => cancelAnimationFrame(id)
  }, [zoom, view, centerView])

  // La rueda mueve el lienzo y con Ctrl (o Cmd) acerca y aleja, como en
  // cualquier pizarra. Va con addEventListener y passive:false porque React
  // registra onWheel como pasivo y ahí preventDefault no surte efecto: sin eso
  // la rueda desplazaría la página entera por detrás.
  useEffect(() => {
    const box = panRef.current
    if (!box || view !== 'tree') return
    const onWheel = (e: WheelEvent) => {
      e.preventDefault()
      if (e.ctrlKey || e.metaKey) {
        stepZoom(e.deltaY > 0 ? -0.1 : 0.1)
        return
      }
      setOffset(o => clampOffset(o.x - e.deltaX, o.y - e.deltaY))
    }
    box.addEventListener('wheel', onWheel, { passive: false })
    return () => box.removeEventListener('wheel', onWheel)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [view, zoom])

  const startPan = (e: ReactPointerEvent<HTMLDivElement>) => {
    if (e.button !== 0) return
    // Arrancar sobre una tarjeta es mover a esa persona, no el lienzo.
    if ((e.target as HTMLElement).closest('[data-org-person]')) return
    const el = panRef.current
    if (!el) return
    panFrom.current = { x: e.clientX, y: e.clientY, ox: offset.x, oy: offset.y }
    setPanning(true)
    el.setPointerCapture(e.pointerId)
  }

  const movePan = (e: ReactPointerEvent<HTMLDivElement>) => {
    if (!panning) return
    const from = panFrom.current
    setOffset(clampOffset(
      from.ox + (e.clientX - from.x),
      from.oy + (e.clientY - from.y),
    ))
  }

  const endPan = (e: ReactPointerEvent<HTMLDivElement>) => {
    if (!panning) return
    setPanning(false)
    panRef.current?.releasePointerCapture(e.pointerId)
  }

  // El clic que sigue a soltar llega en el mismo ciclo; se libera en el
  // siguiente para no tragarse clics legítimos posteriores.
  const swallowNextClick = () => {
    justDragged.current = true
    setTimeout(() => { justDragged.current = false }, 0)
  }

  const handleDragEnd = async (event: DragEndEvent) => {
    const activeId = Number(event.active.id)
    setDragId(null)
    swallowNextClick()
    if (!onReassign || !event.over) return

    const droppedOn = Number(event.over.id)
    // Soltar a alguien sobre sí mismo es no haber movido nada, no un error.
    // Ocurre al empujar de más una tarjeta yendo a hacerle clic, y contestaba
    // "Nadie puede ser su propio manager", que suena a que hiciste algo mal.
    if (droppedOn === activeId) return
    // Soltar sobre la cuenta de empresa es "quitarle el manager": la empresa no
    // es un manager de verdad y el backend rechazaría asignarla como tal.
    const droppedOnCompany = !!people.find(p => p.user_id === droppedOn)?.is_company
    const newManagerId = droppedOnCompany ? null : droppedOn

    const current = people.find(p => p.user_id === activeId)?.manager_id ?? null
    const currentIsCompany = people.find(p => p.user_id === current)?.is_company
    if (current === newManagerId || (currentIsCompany && newManagerId === null)) return

    const rejection = dropRejection(activeId, droppedOn, people)
    if (rejection) {
      setMessage({ text: rejection, error: true })
      return
    }

    setBusy(true)
    setMessage(null)
    try {
      await onReassign(activeId, newManagerId)
      const who = people.find(p => p.user_id === activeId)?.name ?? 'La persona'
      if (newManagerId == null) {
        setMessage({
          text: companyNode
            ? `${who} ahora reporta directamente a ${companyNode.name}.`
            : `${who} quedó sin manager.`,
          error: false,
        })
      } else {
        const target = people.find(p => p.user_id === newManagerId)
        const moved = people.find(p => p.user_id === activeId)
        // Se avisa del ascenso porque es un cambio de permisos que el usuario no
        // pidió explícitamente: lo dedujo el organigrama de lo que acaba de mover.
        let promoted = ''
        if (target && moved?.is_manager && !target.is_supervisor) {
          promoted = ` ${target.name} pasó a ser supervisor: ahora tiene managers a su cargo.`
        } else if (target && !target.is_manager) {
          promoted = ` ${target.name} pasó a ser manager.`
        }
        setMessage({ text: `${who} ahora reporta a ${target?.name ?? 'su nuevo manager'}.${promoted}`, error: false })
      }
    } catch (err: any) {
      setMessage({
        text: err?.response?.data?.error ?? 'No se pudo mover a esa persona.',
        error: true,
      })
    } finally {
      setBusy(false)
    }
  }

  if (people.length === 0) {
    return <div className={styles.wrap}><p className={styles.empty}>Esta empresa todavía no tiene profesionales activos.</p></div>
  }

  const shared = { people, sizes, dragId, editable: editable && !busy, profileHref, justDragged }

  return (
    <div className={`${styles.wrap} ${fullscreen ? styles.fullscreen : ''}`}>
      <div className={styles.toolbar}>
        <p className={styles.hint}>
          {hint ?? (editable
            ? 'Arrastra a una persona sobre otra para cambiar su manager. Se lleva a su equipo con ella.'
            : 'Estructura de la organización.')}
        </p>
        <div className={styles.views} role="tablist" aria-label="Vista del organigrama">
          <button
            type="button"
            role="tab"
            aria-selected={view === 'tree'}
            className={`${styles.viewBtn} ${view === 'tree' ? styles.viewActive : ''}`}
            onClick={() => setView('tree')}
          >
            <Network size={14} /> Organigrama
          </button>
          <button
            type="button"
            role="tab"
            aria-selected={view === 'list'}
            className={`${styles.viewBtn} ${view === 'list' ? styles.viewActive : ''}`}
            onClick={() => setView('list')}
          >
            <List size={14} /> Lista
          </button>
        </div>
        {view === 'tree' && (
          <div className={styles.zoomBar}>
            <button
              type="button"
              className={styles.zoomBtn}
              onClick={() => stepZoom(-0.1)}
              disabled={zoom <= MIN_ZOOM}
              title="Alejar"
              aria-label="Alejar"
            >
              <ZoomOut size={14} />
            </button>
            <button
              type="button"
              className={styles.zoomLevel}
              onClick={fitToWidth}
              title="Ajustar el árbol al ancho disponible"
            >
              <Scan size={13} /> {Math.round(zoom * 100)}%
            </button>
            <button
              type="button"
              className={styles.zoomBtn}
              onClick={() => stepZoom(0.1)}
              disabled={zoom >= MAX_ZOOM}
              title="Acercar"
              aria-label="Acercar"
            >
              <ZoomIn size={14} />
            </button>
          </div>
        )}
        <button
          type="button"
          className={styles.expandBtn}
          onClick={() => setFullscreen(v => !v)}
          title={fullscreen ? 'Salir de la vista ampliada (Esc)' : 'Ver el organigrama a pantalla completa'}
          aria-pressed={fullscreen}
        >
          {fullscreen ? <Minimize2 size={14} /> : <Maximize2 size={14} />}
          {fullscreen ? 'Reducir' : 'Ampliar'}
        </button>
      </div>

      {message && (
        <p className={`${styles.message} ${message.error ? styles.messageError : styles.messageOk}`}>
          {message.text}
        </p>
      )}

      <DndContext
        sensors={sensors}
        collisionDetection={pointerWithin}
        onDragStart={(e: DragStartEvent) => setDragId(Number(e.active.id))}
        onDragCancel={() => setDragId(null)}
        onDragEnd={handleDragEnd}
      >
        {view === 'tree' ? (
          <div
            ref={panRef}
            className={`${styles.tree} ${panning ? styles.panning : ''}`}
            onPointerDown={startPan}
            onPointerMove={movePan}
            onPointerUp={endPan}
            onPointerCancel={endPan}
          >
            {/* El zoom va en el árbol y la traslación en la capa de afuera: si
                compartieran elemento, el zoom multiplicaría el desplazamiento y
                el lienzo no seguiría al puntero. */}
            <div
              className={styles.canvas}
              data-org-canvas=""
              style={{ transform: `translate3d(${offset.x}px, ${offset.y}px, 0)` }}
            >
              <ul ref={contentRef} className={styles.treeRoot} style={{ zoom }}>
                {forest.map(node => <TreeNode key={node.person.user_id} node={node} {...shared} />)}
              </ul>
            </div>
          </div>
        ) : (
          <div className={`${styles.branch} ${styles.branchRoot}`}>
            {forest.map(node => <ListNode key={node.person.user_id} node={node} {...shared} />)}
          </div>
        )}

        <DragOverlay>
          {dragged && (
            <div className={styles.overlayCard}>
              <Avatar src={dragged.avatar} name={dragged.name} size="sm" />
              {dragged.name}
            </div>
          )}
        </DragOverlay>
      </DndContext>
    </div>
  )
}

interface NodeProps {
  node: OrgTreeNode
  people: OrgPerson[]
  sizes: Map<number, number>
  dragId: number | null
  editable: boolean
  profileHref?: (person: OrgPerson) => string | null
  justDragged: MutableRefObject<boolean>
}

function TreeNode({ node, ...rest }: NodeProps) {
  return (
    <li>
      <Person node={node} shape="box" {...rest} />
      {node.children.length > 0 && (
        <ul>
          {node.children.map(child => <TreeNode key={child.person.user_id} node={child} {...rest} />)}
        </ul>
      )}
    </li>
  )
}

function ListNode({ node, ...rest }: NodeProps) {
  return (
    <div>
      <Person node={node} shape="row" {...rest} />
      {node.children.length > 0 && (
        <div className={styles.branch}>
          {node.children.map(child => <ListNode key={child.person.user_id} node={child} {...rest} />)}
        </div>
      )}
    </div>
  )
}

function Person({
  node, people, sizes, dragId, editable, shape, profileHref, justDragged,
}: NodeProps & { shape: 'box' | 'row' }) {
  const { person } = node
  const id = person.user_id
  // La cuenta de empresa recibe gente pero no se mueve: es la cabeza del árbol.
  const canDrag = editable && !person.is_company
  // La empresa no tiene ficha de persona que abrir.
  const href = person.is_company ? null : profileHref?.(person) ?? null

  const openProfile = () => {
    if (!href || justDragged.current) return
    // noopener: la pestaña nueva no debe poder tocar a la que la abrió.
    window.open(href, '_blank', 'noopener,noreferrer')
  }

  const { attributes, listeners, setNodeRef: setDragRef, isDragging } = useDraggable({ id, disabled: !canDrag })
  const { setNodeRef: setDropRef, isOver } = useDroppable({ id, disabled: !editable })

  // Mientras se arrastra, cada destino se pinta según pueda o no recibir a quien
  // viene: se ve ANTES de soltar, en vez de descubrirlo con un error después.
  const blocked = dragId != null && dragId !== id && dropRejection(dragId, id, people) !== null

  const team = sizes.get(id) ?? 0
  const classes = [
    shape === 'box' ? styles.box : styles.row,
    person.is_company ? styles.company : '',
    canDrag ? styles.draggable : '',
    isDragging ? styles.dragging : '',
    isOver && !blocked ? styles.over : '',
    blocked ? styles.blocked : '',
    person.is_active ? '' : styles.inactive,
    href ? styles.linked : '',
  ].filter(Boolean).join(' ')

  const title = person.is_company
    ? 'Suelta aquí a quien deba reportar directamente a la empresa.'
    : href
      ? `Abrir el perfil de ${person.name} en una pestaña nueva`
      : undefined

  return (
    <div
      ref={el => { setDragRef(el); setDropRef(el) }}
      className={classes}
      title={title}
      onClick={openProfile}
      // Marca para que agarrar una tarjeta mueva a la persona y no el lienzo.
      data-org-person=""
      {...(canDrag ? { ...listeners, ...attributes } : {})}
    >
      {person.is_company
        ? <Building2 size={18} className={styles.companyIcon} />
        : <Avatar src={person.avatar} name={person.name} size="sm" />}
      <div className={styles.who}>
        {/* El nombre es un enlace de verdad y no solo un div con onClick: así
            funcionan ctrl+clic, el clic con la rueda y el "abrir en pestaña
            nueva" del menú contextual, que es como mucha gente navega. No se
            frena el pointerdown a propósito —arrastrar agarrando el nombre
            sigue moviendo la tarjeta, que es el gesto natural—; lo único que se
            corta es la propagación del clic, para que no se abran dos pestañas
            (la del enlace y la de la tarjeta). */}
        {href ? (
          <a
            href={href}
            target="_blank"
            rel="noopener noreferrer"
            className={`${styles.name} ${styles.nameLink}`}
            draggable={false}
            onClick={e => {
              e.stopPropagation()
              if (justDragged.current) e.preventDefault()
            }}
          >
            {person.name}
          </a>
        ) : (
          <div className={styles.name}>{person.name}</div>
        )}
        <div className={styles.role}>{person.job_title || 'Profesional'}</div>
      </div>
      {/* Una sola insignia por persona: el nivel más alto que tenga. El tamaño
          del equipo ya se ve en el propio dibujo, así que en la vista de cajas
          no se repite; en la lista sí, porque ahí las ramas se pueden plegar
          visualmente y el número orienta. */}
      <div className={styles.badges}>
        {person.is_supervisor
          ? <span className={`${styles.badge} ${styles.badgeSupervisor}`}>Supervisor</span>
          : person.is_manager
            ? <span className={`${styles.badge} ${styles.badgeManager}`}>Manager</span>
            : null}
        {shape === 'row' && team > 0 && (
          <span className={`${styles.badge} ${styles.badgeTeam}`}>{team}</span>
        )}
        {/* El árbol solo dibuja al manager PRINCIPAL. Quien además responde a
            otros lo lleva marcado: sin esto, mover a alguien parecía quitarle el
            manager anterior cuando en realidad sigue ahí y sigue aprobándole. */}
        {(person.extra_managers ?? 0) > 0 && (
          <span
            className={`${styles.badge} ${styles.badgeExtra}`}
            title="Además del principal responde a otros managers, que no se dibujan en el árbol."
          >
            +{person.extra_managers}
          </span>
        )}
      </div>
    </div>
  )
}
