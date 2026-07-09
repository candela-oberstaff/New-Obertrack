import { useState, useCallback, useMemo } from 'react'
import { useAuth } from '../../../context/AuthContext'
import { useNotification } from '../../../context/NotificationContext'
import { useConfirm } from '../../ui/ConfirmProvider'
import { useTutorials } from './useTutorials'
import { parseVideoUrl } from '../utils'
import type { Tutorial, CreateTutorialInput } from '../../../types'

const EMPTY_FORM: CreateTutorialInput = {
  title: '',
  description: '',
  google_drive_url: '',
  icon_name: 'PlayCircle',
  category: 'General',
  audience: 'all',
  duration_min: 0,
  order_index: 0,
  is_active: true,
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
      google_drive_url: tutorial.google_drive_url,
      icon_name: tutorial.icon_name,
      category: tutorial.category || 'General',
      audience: tutorial.audience || 'all',
      duration_min: tutorial.duration_min,
      order_index: tutorial.order_index,
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
    if (!parseVideoUrl(formData.google_drive_url)) {
      error('Pega un link válido de Google Drive o YouTube')
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
