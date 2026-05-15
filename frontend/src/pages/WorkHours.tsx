import { useState } from 'react'
import { useAuth } from '../context/AuthContext'
import type { WorkHour } from '../types'
import { useWorkHours } from '../hooks'
import {
  Clock,
  ChevronLeft,
  ChevronRight,
} from 'lucide-react'

import { MONTHS_ES } from '../components/WorkHours/utils'
import { WorkHourCalendar } from '../components/WorkHours/WorkHourCalendar'
import { WorkHourStats } from '../components/WorkHours/WorkHourStats'
import { WorkHourList } from '../components/WorkHours/WorkHourList'
import { RegisterDayModal } from '../components/WorkHours/Modals/RegisterDayModal'
import { WorkHourDetailModal } from '../components/WorkHours/Modals/WorkHourDetailModal'

import styles from './WorkHours.module.css'

export default function WorkHours() {
  const { user } = useAuth()
  const {
    workHours,
    summary,
    isLoading,
    selectedDate,
    setSelectedDate,
    currentMonth,
    currentYear,
    setCurrentMonth,
    setCurrentYear,
    formData,
    setFormData,
    resetForm,
    createWorkHour,
    approveWorkHours,
    approveSingle,
    filteredHours,
    pendingForSelectedDate,
    weekHours,
    todayWork,
    canApprove,
  } = useWorkHours(user)

  const [showModal, setShowModal] = useState(false)
  const [selectedWorkHour, setSelectedWorkHour] = useState<WorkHour | null>(null)
  const [editingId, setEditingId] = useState<number | null>(null)

  const today = new Date().toISOString().split('T')[0]

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    try {
      if (editingId) {
        await createWorkHour({
          ...formData,
          id: editingId
        } as any)
      } else {
        await createWorkHour({
          work_date: formData.work_date,
          work_type: formData.work_type,
          activities: formData.activities,
          absence_reason: formData.absence_reason,
          absence_hours: formData.absence_hours,
        })
      }
      setShowModal(false)
      setEditingId(null)
      resetForm()
    } catch (error) {
      console.error('Error saving work hour:', error)
    }
  }

  const handleEdit = (wh: WorkHour) => {
    setFormData({
      work_date: wh.work_date.split('T')[0],
      work_type: wh.work_type as any,
      activities: wh.activities,
      absence_reason: wh.absence_reason || '',
      absence_hours: wh.absence_hours || 0,
    })
    setEditingId(wh.id)
    setShowModal(true)
    setSelectedWorkHour(null)
  }

  const handleBulkApprove = async () => {
    if (pendingForSelectedDate.length === 0) return
    try {
      await approveWorkHours(pendingForSelectedDate.map(wh => wh.id))
    } catch (error) {
      console.error('Error approving work hours:', error)
    }
  }

  const prevMonth = () => {
    if (currentMonth === 0) {
      setCurrentMonth(11)
      setCurrentYear(currentYear - 1)
    } else {
      setCurrentMonth(currentMonth - 1)
    }
  }

  const nextMonth = () => {
    if (currentMonth === 11) {
      setCurrentMonth(0)
      setCurrentYear(currentYear + 1)
    } else {
      setCurrentMonth(currentMonth + 1)
    }
  }

  if (isLoading) {
    return (
      <div className={styles['work-hours-loading']}>
        <div className={styles['spinner']} />
        <p>Cargando...</p>
      </div>
    )
  }

  return (
    <div className={styles['work-hours-page']}>
      <div className={styles['wh-page-header']}>
        <div className={styles['header-left']}>
          <h1><Clock size={28} style={{ verticalAlign: 'middle', marginRight: '8px' }} /> Mi Jornada</h1>
          <p className={styles['header-subtitle']}>Registra tu día laboral</p>
        </div>
        <button className={styles['btn-primary']} onClick={() => {
          setEditingId(null)
          resetForm()
          setShowModal(true)
        }}>
          + Registrar Día
        </button>
      </div>

      <WorkHourStats 
        todayWork={todayWork}
        weekHours={weekHours}
        summary={summary}
      />

      <div className={styles['work-hours-content']}>
        <div className={styles['calendar-section']}>
          <div className={styles['calendar-header']}>
            <button className={styles['nav-btn']} onClick={prevMonth}><ChevronLeft size={20} /></button>
            <h3>{MONTHS_ES[currentMonth]} {currentYear}</h3>
            <button className={styles['nav-btn']} onClick={nextMonth}><ChevronRight size={20} /></button>
          </div>
          <WorkHourCalendar
            workHours={workHours}
            onDayClick={setSelectedDate}
            selectedDate={selectedDate}
            currentMonth={currentMonth}
            currentYear={currentYear}
          />
          <div className={styles['calendar-legend']}>
            <span className={styles['legend-item']}><span className={`${styles['legend-dot']} ${styles['complete']}`}></span>Completa</span>
            <span className={styles['legend-item']}><span className={`${styles['legend-dot']} ${styles['absence']}`}></span>Ausencia</span>
          </div>
          {selectedDate && (
            <button className={styles['clear-filter']} onClick={() => setSelectedDate(null)}>
              Ver todos los días
            </button>
          )}
        </div>

        <WorkHourList 
          filteredHours={filteredHours}
          selectedDate={selectedDate}
          canApprove={canApprove}
          pendingForSelectedDate={pendingForSelectedDate}
          onBulkApprove={handleBulkApprove}
          onItemClick={setSelectedWorkHour}
        />
      </div>

      <RegisterDayModal 
        isOpen={showModal}
        onClose={() => {
          setShowModal(false)
          setEditingId(null)
        }}
        formData={formData}
        setFormData={setFormData}
        onSubmit={handleSubmit}
        today={today}
      />

      <WorkHourDetailModal 
        workHour={selectedWorkHour}
        onClose={() => setSelectedWorkHour(null)}
        canApprove={canApprove}
        onApprove={approveSingle}
        onEdit={handleEdit}
      />
    </div>
  )
}
