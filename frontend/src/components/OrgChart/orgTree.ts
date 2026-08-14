/** Una persona del organigrama, tal como la sirve GET /org-chart. */
export interface OrgPerson {
  user_id: number
  employment_id: number
  name: string
  email: string
  avatar?: string
  job_title?: string
  is_manager: boolean
  is_supervisor?: boolean
  is_active: boolean
  /** Manager principal. null = cuelga de la raíz de la vista. */
  manager_id: number | null
  /** Managers ADICIONALES al principal (el árbol solo dibuja el principal). */
  extra_managers?: number
  /** La cuenta de empresa: raíz del árbol, no se arrastra ni se le pone jefe. */
  is_company?: boolean
}

export interface OrgTreeNode {
  person: OrgPerson
  children: OrgTreeNode[]
}

/**
 * Arma el bosque a partir de la lista plana.
 *
 * Cuelga de la raíz a quien no tiene manager Y TAMBIÉN a quien apunta a alguien
 * que no está en la lista: pasa de verdad cuando un supervisor mira su rama (su
 * propio jefe queda fuera) o si los datos traen un manager que ya no trabaja en
 * la empresa. Sin ese caso, esa gente desaparecería del organigrama en vez de
 * verse suelta arriba, que es justo lo que hay que poder corregir.
 */
export function buildForest(people: OrgPerson[]): OrgTreeNode[] {
  const byId = new Map<number, OrgTreeNode>()
  for (const person of people) {
    byId.set(person.user_id, { person, children: [] })
  }

  const parentOf = new Map<number, OrgTreeNode>()
  const roots: OrgTreeNode[] = []
  for (const node of byId.values()) {
    const managerId = node.person.manager_id
    const parent = managerId != null ? byId.get(managerId) : undefined
    // Nadie puede ser su propio padre: si los datos vinieran así, a la raíz.
    if (parent && parent !== node) {
      parent.children.push(node)
      parentOf.set(node.person.user_id, parent)
    } else {
      roots.push(node)
    }
  }

  // Un ciclo heredado (A→B→A) deja a sus miembros fuera de toda raíz: no se
  // dibujarían, y son justo los que hay que ver para poder arreglarlos. Se
  // rescata al primero de cada ciclo CORTANDO su vínculo con el padre; si solo
  // se lo agregara como raíz, el árbol seguiría teniendo el ciclo y recorrerlo
  // no terminaría nunca.
  const reachable = new Set<number>()
  const mark = (node: OrgTreeNode) => {
    if (reachable.has(node.person.user_id)) return
    reachable.add(node.person.user_id)
    node.children.forEach(mark)
  }
  roots.forEach(mark)

  for (const node of byId.values()) {
    if (reachable.has(node.person.user_id)) continue
    const parent = parentOf.get(node.person.user_id)
    if (parent) {
      parent.children = parent.children.filter(child => child !== node)
    }
    roots.push(node)
    mark(node)
  }

  const byName = (a: OrgTreeNode, b: OrgTreeNode) =>
    (a.person.name || '').localeCompare(b.person.name || '', 'es', { sensitivity: 'base' })
  const sortDeep = (nodes: OrgTreeNode[]) => {
    nodes.sort(byName)
    nodes.forEach(n => sortDeep(n.children))
  }
  sortDeep(roots)

  return roots
}

/** IDs de todo lo que cuelga de userId, a cualquier profundidad (sin incluirlo). */
export function descendantIds(userId: number, people: OrgPerson[]): Set<number> {
  const childrenOf = new Map<number, number[]>()
  for (const p of people) {
    if (p.manager_id == null) continue
    const list = childrenOf.get(p.manager_id)
    if (list) list.push(p.user_id)
    else childrenOf.set(p.manager_id, [p.user_id])
  }

  const out = new Set<number>()
  const stack = [...(childrenOf.get(userId) ?? [])]
  while (stack.length > 0) {
    const id = stack.pop() as number
    // El guard de visitados es lo que evita colgarse si los datos traen un ciclo.
    if (out.has(id)) continue
    out.add(id)
    stack.push(...(childrenOf.get(id) ?? []))
  }
  return out
}

/**
 * Si `dragId` puede pasar a colgar de `dropId`. Devuelve el motivo del rechazo
 * para poder explicárselo a quien arrastra, en vez de que el nodo "rebote" sin
 * decir nada.
 *
 * El backend valida esto mismo (ensureNoManagerCycle); aquí se repite para no
 * hacer un viaje al servidor que ya sabemos que va a fallar.
 */
export function dropRejection(
  dragId: number,
  dropId: number | null,
  people: OrgPerson[],
): string | null {
  if (dropId == null) return null // soltar en la raíz: siempre válido
  if (dragId === dropId) return 'Nadie puede ser su propio manager.'

  const dragged = people.find(p => p.user_id === dragId)
  const target = people.find(p => p.user_id === dropId)
  if (!dragged || !target) return 'No se encontró a esa persona en el organigrama.'

  // La cuenta de empresa es la dueña del tenant: no se le pone jefe.
  if (dragged.is_company) return 'La cuenta de empresa es la cabeza del organigrama.'
  // Soltar sobre la empresa equivale a quitar el manager, así que siempre vale.
  if (target.is_company) return null

  if (dragged.manager_id === dropId) return null // ya está ahí: es un no-op

  if (descendantIds(dragId, people).has(dropId)) {
    return `${target.name} ya está dentro del equipo de ${dragged.name}: la asignación crearía un círculo.`
  }
  if (!target.is_active) return `${target.name} está inactivo y no puede recibir equipo.`
  return null
}

/** Cuántas personas cuelgan de cada uno, a cualquier profundidad. */
export function teamSizes(people: OrgPerson[]): Map<number, number> {
  const sizes = new Map<number, number>()
  for (const p of people) {
    sizes.set(p.user_id, descendantIds(p.user_id, people).size)
  }
  return sizes
}
