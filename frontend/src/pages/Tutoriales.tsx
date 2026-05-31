import { Plus, BookOpen, Search, Compass } from 'lucide-react'
import { startSystemTour } from '../lib/tour'
import { DndContext, PointerSensor, useSensor, useSensors, closestCenter, type DragEndEvent } from '@dnd-kit/core'
import { SortableContext, rectSortingStrategy, arrayMove } from '@dnd-kit/sortable'
import { useTutorialsPageState, ALL_CATEGORIES } from '../components/Tutorials/hooks/useTutorialsPageState'
import { TutorialCard } from '../components/Tutorials/TutorialCard'
import { TutorialPlayerModal } from '../components/Tutorials/Modals/TutorialPlayerModal'
import { TutorialFormModal } from '../components/Tutorials/Modals/TutorialFormModal'
import { useAuth } from '../context/AuthContext'
import styles from './Tutoriales.module.css'

export default function Tutoriales() {
  const { user } = useAuth()
  const {
    isAdmin,
    tutorials,
    setTutorials,
    filteredTutorials,
    availableCategories,
    categoryFilter,
    setCategoryFilter,
    searchQuery,
    setSearchQuery,
    viewedIds,
    isLoading,
    selectedTutorial,
    setSelectedTutorial,
    handleOpenTutorial,
    showFormModal,
    editingTutorial,
    formData,
    setFormData,
    isSaving,
    openCreate,
    openEdit,
    closeForm,
    handleSubmit,
    handleDelete,
    handleReorder,
  } = useTutorialsPageState()

  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 6 } })
  )

  const canReorder = isAdmin && categoryFilter === ALL_CATEGORIES && !searchQuery.trim()

  const greeting = (() => {
    const hour = new Date().getHours()
    if (hour < 12) return 'Buenos días'
    if (hour < 19) return 'Buenas tardes'
    return 'Buenas noches'
  })()

  const today = new Date().toLocaleDateString('es-ES', {
    weekday: 'long',
    day: 'numeric',
    month: 'long',
  })

  const handleDragEnd = (event: DragEndEvent) => {
    const { active, over } = event
    if (!over || active.id === over.id) return

    const oldIndex = tutorials.findIndex(t => t.id === active.id)
    const newIndex = tutorials.findIndex(t => t.id === over.id)
    if (oldIndex < 0 || newIndex < 0) return

    const reordered = arrayMove(tutorials, oldIndex, newIndex)
    setTutorials(reordered)
    handleReorder(reordered.map(t => t.id))
  }

  return (
    <div className={styles['tutorials-page']}>
      <header className={styles['tutorials-header']}>
        <div>
          <h1>Tutoriales</h1>
          <p className={styles['tutorials-date']}>
            {today.charAt(0).toUpperCase() + today.slice(1)} · {greeting}, {user?.name?.split(' ')[0] || ''}
          </p>
          <p className={styles['tutorials-subtitle']}>
            Guías rápidas para sacar el máximo provecho de Obertrack.
          </p>
        </div>
        <div className={styles['tutorials-header-actions']}>
          <button type="button" className={styles['tutorials-tour-btn']} onClick={startSystemTour}>
            <Compass size={18} /> Recorrido guiado
          </button>
          {isAdmin && (
            <button type="button" className={styles['tutorials-create-btn']} onClick={openCreate}>
              <Plus size={18} /> Nuevo tutorial
            </button>
          )}
        </div>
      </header>

      {tutorials.length > 0 && (
        <div className={styles['tutorials-toolbar']}>
          <div className={styles['tutorials-tabs']}>
            <button
              type="button"
              className={`${styles['tutorials-tab']} ${categoryFilter === ALL_CATEGORIES ? styles['active'] : ''}`}
              onClick={() => setCategoryFilter(ALL_CATEGORIES)}
            >
              Todos
              <span className={styles['tutorials-tab-count']}>{tutorials.length}</span>
            </button>
            {availableCategories.map((cat) => {
              const count = tutorials.filter(t => t.category === cat).length
              return (
                <button
                  key={cat}
                  type="button"
                  className={`${styles['tutorials-tab']} ${categoryFilter === cat ? styles['active'] : ''}`}
                  onClick={() => setCategoryFilter(cat)}
                >
                  {cat}
                  <span className={styles['tutorials-tab-count']}>{count}</span>
                </button>
              )
            })}
          </div>
          <div className={styles['tutorials-search']}>
            <Search size={16} />
            <input
              type="search"
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              placeholder="Buscar tutorial..."
            />
          </div>
        </div>
      )}

      {isLoading ? (
        <div className={styles['tutorials-loading']}>Cargando tutoriales...</div>
      ) : tutorials.length === 0 ? (
        <div className={styles['tutorials-empty']}>
          <BookOpen size={48} />
          <h2>Aún no hay tutoriales</h2>
          <p>
            {isAdmin
              ? 'Crea el primero para que tu equipo aprenda a usar la plataforma.'
              : 'Tu administrador todavía no ha publicado tutoriales.'}
          </p>
          {isAdmin && (
            <button type="button" className={styles['tutorials-create-btn']} onClick={openCreate}>
              <Plus size={18} /> Crear primer tutorial
            </button>
          )}
        </div>
      ) : filteredTutorials.length === 0 ? (
        <div className={styles['tutorials-empty']}>
          <Search size={40} />
          <h2>Sin resultados</h2>
          <p>Prueba a cambiar la categoría o el término de búsqueda.</p>
        </div>
      ) : (
        <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={handleDragEnd}>
          <SortableContext items={filteredTutorials.map(t => t.id)} strategy={rectSortingStrategy}>
            <div className={styles['tutorials-grid']}>
              {filteredTutorials.map((tutorial) => (
                <TutorialCard
                  key={tutorial.id}
                  tutorial={tutorial}
                  isAdmin={isAdmin}
                  isViewed={viewedIds.has(tutorial.id)}
                  sortable={canReorder}
                  onOpen={handleOpenTutorial}
                  onEdit={openEdit}
                  onDelete={handleDelete}
                />
              ))}
            </div>
          </SortableContext>
        </DndContext>
      )}

      <TutorialPlayerModal
        tutorial={selectedTutorial}
        onClose={() => setSelectedTutorial(null)}
      />

      <TutorialFormModal
        isOpen={showFormModal}
        isEditing={!!editingTutorial}
        isSaving={isSaving}
        formData={formData}
        setFormData={setFormData}
        availableCategories={availableCategories}
        onClose={closeForm}
        onSubmit={handleSubmit}
      />
    </div>
  )
}
