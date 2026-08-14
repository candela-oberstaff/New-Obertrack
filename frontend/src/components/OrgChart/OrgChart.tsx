import { useMemo, useState } from 'react'
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
import { Network, List, Building2 } from 'lucide-react'
import Avatar from '../Common/Avatar'
import { buildForest, dropRejection, teamSizes, type OrgPerson, type OrgTreeNode } from './orgTree'
import styles from './OrgChart.module.css'

export interface OrgChartProps {
  people: OrgPerson[]
  /** Sin esto el árbol se dibuja pero no se puede reordenar. */
  onReassign?: (userId: number, newManagerId: number | null) => Promise<void>
  hint?: string
}

type View = 'tree' | 'list'

export function OrgChart({ people, onReassign, hint }: OrgChartProps) {
  const [view, setView] = useState<View>('tree')
  const [dragId, setDragId] = useState<number | null>(null)
  const [message, setMessage] = useState<{ text: string; error: boolean } | null>(null)
  const [busy, setBusy] = useState(false)

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

  const handleDragEnd = async (event: DragEndEvent) => {
    const activeId = Number(event.active.id)
    setDragId(null)
    if (!onReassign || !event.over) return

    const droppedOn = Number(event.over.id)
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

  const shared = { people, sizes, dragId, editable: editable && !busy }

  return (
    <div className={styles.wrap}>
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
          <div className={styles.tree}>
            <ul className={styles.treeRoot}>
              {forest.map(node => <TreeNode key={node.person.user_id} node={node} {...shared} />)}
            </ul>
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
  node, people, sizes, dragId, editable, shape,
}: NodeProps & { shape: 'box' | 'row' }) {
  const { person } = node
  const id = person.user_id
  // La cuenta de empresa recibe gente pero no se mueve: es la cabeza del árbol.
  const canDrag = editable && !person.is_company

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
  ].filter(Boolean).join(' ')

  return (
    <div
      ref={el => { setDragRef(el); setDropRef(el) }}
      className={classes}
      title={person.is_company ? 'Suelta aquí a quien deba reportar directamente a la empresa.' : undefined}
      {...(canDrag ? { ...listeners, ...attributes } : {})}
    >
      {person.is_company
        ? <Building2 size={18} className={styles.companyIcon} />
        : <Avatar src={person.avatar} name={person.name} size="sm" />}
      <div className={styles.who}>
        <div className={styles.name}>{person.name}</div>
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
