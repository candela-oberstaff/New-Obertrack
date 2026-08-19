import { useState, useCallback, useMemo } from 'react'
import { useAuth } from '../../../context/AuthContext'
import { useNotification } from '../../../context/NotificationContext'
import { useConfirm } from '../../ui/ConfirmProvider'
import { useTutorials } from './useTutorials'
import { parseVideoUrl } from '../utils'
import type { Tutorial, CreateTutorialInput } from '../../../types'
import { EMPTY_TARGET } from '../../../types'

const EMPTY_FORM: CreateTutorialInput = {
  title: '',
  description: '',
  content_type: 'video',
  google_drive_url: '',
  image_url: '',
  body: '',
  icon_name: 'PlayCircle',
  category: 'General',
  audience: 'all',
  target: EMPTY_TARGET,
  duration_min: 0,
  order_index: 0,
  announce_days: 2,
  cta_label: '',
  cta_url: '',
  publish_at: null,
  expires_at: null,
  require_ack: false,
  is_active: true,
}

/**
 * Tarjetas o tabla. Se recuerda en el navegador porque es una preferencia de
 * cómo trabaja cada quien —el que publica quiere comparar, el que lee quiere
 * mirar— y volver a elegirla en cada visita sería un peaje diario.
 */
export type TutorialViewMode = 'cards' | 'table'

const VIEW_MODE_KEY = 'obertrack.novedades.view'

function readViewMode(): TutorialViewMode {
  try {
    return localStorage.getItem(VIEW_MODE_KEY) === 'table' ? 'table' : 'cards'
  } catch {
    return 'cards'
  }
}

export const ALL_CATEGORIES = '__all__'
export const ALL_AUDIENCES = '__all__'

export function useTutorialsPageState() {
  const { user } = useAuth()
  const { success, error } = useNotification()
  const confirm = useConfirm()
  const isAdmin = !!user?.is_superadmin

  const {
    tutorials,
    setTutorials,
    viewedIds,
    isLoading,
    createTutorial,
    updateTutorial,
    deleteTutorial,
    reorderTutorials,
    recordView,
  } = useTutorials()

  const [selectedTutorial, setSelectedTutorial] = useState<Tutorial | null>(null)
  const [metricsTutorial, setMetricsTutorial] = useState<Tutorial | null>(null)
  const [viewMode, setViewModeState] = useState<TutorialViewMode>(readViewMode)

  const setViewMode = useCallback((mode: TutorialViewMode) => {
    setViewModeState(mode)
    try { localStorage.setItem(VIEW_MODE_KEY, mode) } catch { /* modo incógnito */ }
  }, [])
  const [showFormModal, setShowFormModal] = useState(false)
  const [editingTutorial, setEditingTutorial] = useState<Tutorial | null>(null)
  const [formData, setFormData] = useState<CreateTutorialInput>(EMPTY_FORM)
  const [isSaving, setIsSaving] = useState(false)

  const [categoryFilter, setCategoryFilter] = useState<string>(ALL_CATEGORIES)
  // Solo para superadmin: previsualizar qué ve cada audiencia (empresas / profesionales).
  const [audienceFilter, setAudienceFilter] = useState<string>(ALL_AUDIENCES)
  const [searchQuery, setSearchQuery] = useState('')

  const availableCategories = useMemo(() => {
    const set = new Set<string>()
    tutorials.forEach(t => {
      if (t.category?.trim()) set.add(t.category.trim())
    })
    return Array.from(set).sort((a, b) => a.localeCompare(b))
  }, [tutorials])

  const filteredTutorials = useMemo(() => {
    const query = searchQuery.trim().toLowerCase()
    return tutorials.filter(t => {
      if (categoryFilter !== ALL_CATEGORIES && t.category !== categoryFilter) return false
      if (audienceFilter !== ALL_AUDIENCES && t.audience !== 'all' && t.audience !== audienceFilter) return false
      if (!query) return true
      return (
        t.title.toLowerCase().includes(query) ||
        t.description.toLowerCase().includes(query) ||
        t.category.toLowerCase().includes(query)
      )
    })
  }, [tutorials, categoryFilter, audienceFilter, searchQuery])

  const openCreate = useCallback(() => {
    setEditingTutorial(null)
    setFormData({ ...EMPTY_FORM, category: categoryFilter !== ALL_CATEGORIES ? categoryFilter : 'General' })
    setShowFormModal(true)
  }, [categoryFilter])

  const openEdit = useCallback((tutorial: Tutorial) => {
    setEditingTutorial(tutorial)
    setFormData({
      title: tutorial.title,
      description: tutorial.description,
      content_type: tutorial.content_type || 'video',
      google_drive_url: tutorial.google_drive_url,
      image_url: tutorial.image_url || '',
      body: tutorial.body || '',
      icon_name: tutorial.icon_name,
      category: tutorial.category || 'General',
      audience: tutorial.audience || 'all',
      target: {
        company_ids: tutorial.target?.company_ids ?? [],
        countries: tutorial.target?.countries ?? [],
        group_ids: tutorial.target?.group_ids ?? [],
        managers_only: tutorial.target?.managers_only ?? false,
      },
      duration_min: tutorial.duration_min,
      order_index: tutorial.order_index,
      announce_days: tutorial.announce_days ?? 2,
      cta_label: tutorial.cta_label || '',
      cta_url: tutorial.cta_url || '',
      publish_at: tutorial.publish_at ?? null,
      expires_at: tutorial.expires_at ?? null,
      require_ack: tutorial.require_ack ?? false,
      is_active: tutorial.is_active,
    })
    setShowFormModal(true)
  }, [])

  const closeForm = useCallback(() => {
    setShowFormModal(false)
    setEditingTutorial(null)
    setFormData(EMPTY_FORM)
  }, [])

  const handleSubmit = useCallback(async (e: React.FormEvent) => {
    e.preventDefault()

    if (!formData.title.trim()) {
      error('El título es obligatorio')
      return
    }
    // Cada tipo de novedad exige su propio contenido: se valida el que manda,
    // no los tres.
    if (formData.content_type === 'video' && !parseVideoUrl(formData.google_drive_url)) {
      error('Pega un link válido de Google Drive o YouTube')
      return
    }
    if (formData.content_type === 'imagen' && !formData.image_url.trim()) {
      error('Sube la imagen de la novedad')
      return
    }
    if (formData.content_type === 'texto' && !formData.body.replace(/<[^>]*>/g, '').trim()) {
      error('Escribe el contenido de la novedad')
      return
    }

    if (formData.cta_label.trim() && !formData.cta_url.trim()) {
      error('El botón de acción necesita un destino')
      return
    }
    if (formData.cta_url.trim() && !formData.cta_label.trim()) {
      error('El botón de acción necesita un texto')
      return
    }

    setIsSaving(true)
    try {
      if (editingTutorial) {
        await updateTutorial(editingTutorial.id, formData)
        success('Novedad actualizada')
      } else {
        await createTutorial(formData)
        success('Novedad creada')
      }
      closeForm()
    } catch (err: any) {
      error(err?.response?.data?.error || 'No se pudo guardar la novedad')
    } finally {
      setIsSaving(false)
    }
  }, [formData, editingTutorial, createTutorial, updateTutorial, success, error, closeForm])

  const handleDelete = useCallback(async (tutorial: Tutorial) => {
    const ok = await confirm({
      title: 'Eliminar novedad',
      message: `¿Eliminar la novedad "${tutorial.title}"?`,
      confirmLabel: 'Eliminar',
      variant: 'danger',
    })
    if (!ok) return
    try {
      await deleteTutorial(tutorial.id)
      success('Novedad eliminada')
      if (selectedTutorial?.id === tutorial.id) {
        setSelectedTutorial(null)
      }
    } catch (err: any) {
      error(err?.response?.data?.error || 'No se pudo eliminar la novedad')
    }
  }, [deleteTutorial, selectedTutorial, success, error, confirm])

  const handleReorder = useCallback(async (orderedIds: number[]) => {
    try {
      await reorderTutorials(orderedIds)
    } catch (err: any) {
      error(err?.response?.data?.error || 'No se pudo reordenar')
    }
  }, [reorderTutorials, error])

  const handleOpenTutorial = useCallback((tutorial: Tutorial) => {
    setSelectedTutorial(tutorial)
    recordView(tutorial.id)
  }, [recordView])

  return {
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
  }
}
