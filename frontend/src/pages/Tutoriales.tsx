import { useState } from 'react'
import { Plus, BookOpen, Search, Compass, GraduationCap, LayoutGrid, List } from 'lucide-react'
import { startCurrentPageTour } from '../lib/tour'
import { DndContext, PointerSensor, useSensor, useSensors, closestCenter, type DragEndEvent } from '@dnd-kit/core'
import { SortableContext, rectSortingStrategy, arrayMove } from '@dnd-kit/sortable'
import { useTutorialsPageState, ALL_CATEGORIES, ALL_AUDIENCES } from '../components/Tutorials/hooks/useTutorialsPageState'
import { TutorialCard } from '../components/Tutorials/TutorialCard'
import { TutorialTable } from '../components/Tutorials/TutorialTable'
import { TutorialPlayerModal } from '../components/Tutorials/Modals/TutorialPlayerModal'
import { TutorialFormModal } from '../components/Tutorials/Modals/TutorialFormModal'
import { TutorialMetricsModal } from '../components/Tutorials/Modals/TutorialMetricsModal'
import { useAuth } from '../context/AuthContext'
import InductionSettings from '../components/Admin/InductionSettings'
import { Skeleton } from '../components/ui'
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
    audienceFilter,
    setAudienceFilter,
    searchQuery,
    setSearchQuery,
    viewedIds,
    isLoading,
    selectedTutorial,
    setSelectedTutorial,
    metricsTutorial,
    setMetricsTutorial,
    viewMode,
    setViewMode,
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

  // Pestaña principal. La de inducción solo existe para superadmin, así que
  // para el resto la página se comporta exactamente como antes.
  const [mainTab, setMainTab] = useState<'novedades' | 'induccion'>('novedades')
  const showNovedades = !isAdmin || mainTab === 'novedades'

  const canReorder = isAdmin && categoryFilter === ALL_CATEGORIES && audienceFilter === ALL_AUDIENCES && !searchQuery.trim()

  const audienceTabs = [
    { value: ALL_AUDIENCES, label: 'Todas las audiencias', count: tutorials.length },
    { value: 'empleador', label: 'Vista empresas', count: tutorials.filter(t => t.audience === 'all' || t.audience === 'empleador').length },
    { value: 'profesional', label: 'Vista profesionales', count: tutorials.filter(t => t.audience === 'all' || t.audience === 'profesional').length },
  ]

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
      <header className={styles['tutorials-header']} data-tour="tutoriales-header">
        <div>
          <h1>Novedades</h1>
          <p className={styles['tutorials-date']}>
            {today.charAt(0).toUpperCase() + today.slice(1)} · {greeting}, {user?.name?.split(' ')[0] || ''}
          </p>
          <p className={styles['tutorials-subtitle']}>
            Últimas novedades y anuncios de Obertrack.
          </p>
        </div>
        <div className={styles['tutorials-header-actions']}>
          <button type="button" className={styles['tutorials-tour-btn']} onClick={() => startCurrentPageTour('/novedades')} data-tour="tutoriales-current-tour">
            <Compass size={18} /> Recorrido guiado
          </button>
          {isAdmin && showNovedades && (
            <button type="button" className={styles['tutorials-create-btn']} onClick={openCreate} data-tour="tutoriales-create">
              <Plus size={18} /> Nueva novedad
            </button>
          )}
        </div>
      </header>

      {/* Pestañas principales: separan el contenido para el equipo de la
          configuración de la inducción, que solo toca el superadmin. */}
      {isAdmin && (
        <div className={styles['tutorials-tabs']}>
          <button
            type="button"
            className={`${styles['tutorials-tab']} ${mainTab === 'novedades' ? styles['active'] : ''}`}
            onClick={() => setMainTab('novedades')}
          >
            <BookOpen size={16} /> Novedades
            <span className={styles['tutorials-tab-count']}>{tutorials.length}</span>
          </button>
          <button
            type="button"
            className={`${styles['tutorials-tab']} ${mainTab === 'induccion' ? styles['active'] : ''}`}
            onClick={() => setMainTab('induccion')}
          >
            <GraduationCap size={16} /> Inducción
          </button>
        </div>
      )}

      {isAdmin && mainTab === 'induccion' && <InductionSettings />}

      {showNovedades && tutorials.length > 0 && (
        <div className={styles['tutorials-toolbar']}>
          <div className={styles['tutorials-tabs']} data-tour="tutoriales-tabs">
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
          {isAdmin && (
            <div className={styles['tutorials-tabs']} data-tour="tutoriales-audience-tabs">
              {audienceTabs.map((tab) => (
                <button
                  key={tab.value}
                  type="button"
                  className={`${styles['tutorials-tab']} ${audienceFilter === tab.value ? styles['active'] : ''}`}
                  onClick={() => setAudienceFilter(tab.value)}
                >
                  {tab.label}
                  <span className={styles['tutorials-tab-count']}>{tab.count}</span>
                </button>
              ))}
            </div>
          )}
          <div className={styles['tutorials-view-switch']} role="group" aria-label="Forma de ver las novedades">
            <button
              type="button"
              className={viewMode === 'cards' ? styles['active'] : ''}
              onClick={() => setViewMode('cards')}
              title="Ver en tarjetas"
              aria-pressed={viewMode === 'cards'}
            >
              <LayoutGrid size={16} /> Tarjetas
            </button>
            <button
              type="button"
              className={viewMode === 'table' ? styles['active'] : ''}
              onClick={() => setViewMode('table')}
              title="Ver en tabla"
              aria-pressed={viewMode === 'table'}
            >
              <List size={16} /> Tabla
            </button>
          </div>
          <div className={styles['tutorials-search']} data-tour="tutoriales-search">
            <Search size={16} />
            <input
              type="search"
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              placeholder="Buscar novedad..."
            />
          </div>
        </div>
      )}

      {!showNovedades ? null : isLoading ? (
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(260px, 1fr))', gap: '1rem' }}>
          {Array.from({ length: 6 }).map((_, i) => <Skeleton key={i} height={180} radius={16} />)}
        </div>
      ) : tutorials.length === 0 ? (
        <div className={styles['tutorials-empty']}>
          <BookOpen size={48} />
          <h2>Aún no hay novedades</h2>
          <p>
            {isAdmin
              ? 'Crea la primera para mantener a tu equipo al día.'
              : 'Tu administrador todavía no ha publicado novedades.'}
          </p>
          {isAdmin && (
            <button type="button" className={styles['tutorials-create-btn']} onClick={openCreate} data-tour="tutoriales-create">
              <Plus size={18} /> Crear primera novedad
            </button>
          )}
        </div>
      ) : filteredTutorials.length === 0 ? (
        <div className={styles['tutorials-empty']}>
          <Search size={40} />
          <h2>Sin resultados</h2>
          <p>Prueba a cambiar la categoría o el término de búsqueda.</p>
        </div>
      ) : viewMode === 'table' ? (
        <TutorialTable
          tutorials={filteredTutorials}
          isAdmin={isAdmin}
          viewedIds={viewedIds}
          onOpen={handleOpenTutorial}
          onEdit={openEdit}
          onDelete={handleDelete}
          onMetrics={setMetricsTutorial}
        />
      ) : (
        <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={handleDragEnd}>
          <SortableContext items={filteredTutorials.map(t => t.id)} strategy={rectSortingStrategy}>
            <div className={styles['tutorials-grid']} data-tour="tutoriales-grid">
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
                  onMetrics={setMetricsTutorial}
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

      <TutorialMetricsModal
        tutorial={metricsTutorial}
        onClose={() => setMetricsTutorial(null)}
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
