import { useTasksPageState } from '../components/Tasks/hooks/useTasksPageState'
import type { User } from '../types'

import { TasksBoard } from '../components/Tasks/components/TasksBoard'
import { Select } from '../components/ui/Select'
import { TaskDetailPanel } from '../components/Tasks/TaskDetailPanel'
import { NewTaskModal } from '../components/Tasks/Modals/NewTaskModal'
import { BoardModal } from '../components/Tasks/Modals/BoardModal'
import { BoardMembersModal } from '../components/Tasks/Modals/BoardMembersModal'
import { JoinBoardModal } from '../components/Tasks/Modals/JoinBoardModal'
import { PhasesModal } from '../components/Tasks/Modals/PhasesModal'
import {
  Plus,
  UserPlus,
  Trash2,
  CheckSquare,
  Columns3
} from 'lucide-react'
import styles from './Tasks.module.css'

export default function Tasks() {
  const {
    user,
    isSuperadmin,
    companies,
    selectedCompanyId,
    setSelectedCompanyId,
    visibleUsers,
    assigneeSearch,
    setAssigneeSearch,
    showNewTaskModal,
    setShowNewTaskModal,
    showBoardModal,
    setShowBoardModal,
    showBoardMembersModal,
    setShowBoardMembersModal,
    showPhasesModal,
    setShowPhasesModal,
    isSavingPhase,
    showJoinBoardModal,
    setShowJoinBoardModal,
    optimisticMembers,
    setOptimisticMembers,
    isCreatingTask,
    isDeletingBoard,
    isJoiningBoard,
    newTaskData,
    setNewTaskData,
    boardFormData,
    setBoardFormData,
    newBoardPhaseSearch,
    setNewBoardPhaseSearch,
    boards,
    boardTaskCounts,
    selectedBoard,
    setSelectedBoard,
    publicBoards,
    isLoadingBoards,
    isCreatingBoard,
    tasks,
    selectedTask,
    setSelectedTask,
    potentialMemberUsers,
    assignableUsers,
    handleDeleteBoard,
    handleBoardSubmit,
    handleMovePhaseLeft,
    handleMovePhaseRight,
    handleReorderPhaseByIndex,
    draggingPhase,
    dragOverIdx,
    isDraggingPhasesRef,
    handlePhaseDragStart,
    handlePhaseDragEnter,
    handlePhaseDragEnd,
    handleAddPhase,
    handleRemovePhase,
    handleOpenNewTaskModal,
    handleCreateTask,
    handleUpdateTask,
    handleDeleteTask,
    handleJoinBoard,
    getCurrentColumns,
    openBoardModal,
    fetchPublicBoards,
    updateBoardMembers,
  } = useTasksPageState()

  // Company selector (superadmin only) — scopes the whole view to one tenant.
  const companySelector = isSuperadmin ? (
    <div className={styles['board-selector']}>
      <Select
        value={selectedCompanyId ?? ''}
        onChange={(v) => {
          setSelectedCompanyId(v ? Number(v) : null)
          setSelectedBoard(null)
        }}
        clearable
        placeholder="Seleccione una empresa..."
        options={companies.map(c => ({ value: c.id, label: c.company_name }))}
      />
    </div>
  ) : null

  // Loading state
  if (isLoadingBoards && boards.length === 0) {
    return (
      <div className={styles['tasks-page']}>
        <div className={styles['tasks-loading']}>
          <div className={styles['spinner']} />
          <p>Cargando...</p>
        </div>
      </div>
    )
  }

  // Superadmin must pick a company before any board/task is shown, so tenants
  // never get mixed in the same view.
  if (isSuperadmin && !selectedCompanyId) {
    return (
      <div className={styles['tasks-page']}>
        <div className={styles['page-header']} data-tour="tasks-header">
          <div className={styles['header-left']}>
            <h1>Tareas</h1>
            {companySelector}
          </div>
        </div>
        <div className={styles['tasks-loading']} style={{ background: 'transparent' }}>
          <div className={styles['empty-state-glass'] || styles['dashboard-card']}>
            <CheckSquare size={64} style={{ color: 'var(--primary)', marginBottom: '24px', opacity: 0.6 }} />
            <h2 style={{ fontSize: '24px', fontWeight: '700', color: 'var(--black)', marginBottom: '12px' }}>
              Selecciona una empresa
            </h2>
            <p style={{ color: '#64748b', fontSize: '16px', maxWidth: '420px', margin: '0 auto' }}>
              Elige una empresa para ver y gestionar sus tableros y tareas. La información de cada empresa se mantiene aislada.
            </p>
          </div>
        </div>
      </div>
    )
  }

  // Empty state
  if (boards.length === 0) {
    return (
      <div className={styles['tasks-page']}>
        <div className={styles['page-header']} data-tour="tasks-header">
          <div className={styles['header-left']}>
            <h1>Tareas</h1>
            {companySelector}
          </div>
        </div>
        <div className={styles['tasks-loading']} data-tour="tasks-empty">
          <h2>No tienes tableros</h2>
          <p>Crea tu primer tablero para organizar tus tareas</p>
          <div style={{ display: 'flex', gap: '12px' }}>
            <button className={styles['btn-primary']} onClick={openBoardModal} data-tour="tasks-create-board">
              <Plus size={18} /> Crear Tablero
            </button>
            <button 
              className={styles['btn-secondary'] || 'btn-secondary'} 
              data-tour="tasks-join-board"
              onClick={() => {
                fetchPublicBoards()
                setShowJoinBoardModal(true)
              }}
            >
              Unirse a Tablero
            </button>
          </div>
        </div>
        <BoardModal
          isOpen={showBoardModal}
          onClose={() => setShowBoardModal(false)}
          newBoardData={boardFormData}
          setNewBoardData={setBoardFormData}
          assigneeSearch={assigneeSearch}
          setAssigneeSearch={setAssigneeSearch}
          filteredUsers={potentialMemberUsers}
          newBoardPhaseSearch={newBoardPhaseSearch}
          setNewBoardPhaseSearch={setNewBoardPhaseSearch}
          onSubmit={handleBoardSubmit}
          isCreatingBoard={isCreatingBoard}
        />
      </div>
    )
  }

  return (
    <div className={styles['tasks-page']}>
      <div className={styles['page-header']} data-tour="tasks-header">
        <div className={styles['header-left']}>
          <h1>Tareas</h1>
          {companySelector}
          <div className={styles['board-selector']} data-tour="tasks-board-selector">
            {boards.length > 0 && (
              <>
                <Select
                  value={selectedBoard?.id ?? ''}
                  onChange={(v) => setSelectedBoard(v ? boards.find(b => b.id === Number(v)) ?? null : null)}
                  clearable
                  placeholder="Seleccione un tablero..."
                  options={boards.map(b => ({ value: b.id, label: b.name, color: b.color || 'var(--primary)' }))}
                />
                <button className={styles['btn-icon']} onClick={openBoardModal} title="Crear tablero" data-tour="tasks-create-board">
                  <Plus size={18} />
                </button>
                <button
                  className={`${styles['btn-secondary'] || 'btn-secondary'} ${styles['btn-sm'] || 'btn-sm'}`}
                  data-tour="tasks-join-board"
                  onClick={() => {
                    fetchPublicBoards()
                    setShowJoinBoardModal(true)
                  }}
                  style={{ marginLeft: '8px', fontSize: '12px', padding: '6px 10px' }}
                >
                  Unirse a Tablero
                </button>
                {selectedBoard && (
                  <>
                    <button
                      className={`${styles['btn-icon']} ${styles['members-btn'] || 'members-btn'}`}
                      data-tour="tasks-members"
                      onClick={() => {
                        setOptimisticMembers(selectedBoard.members?.map((m: User) => m.id) || [])
                        setShowBoardMembersModal(true)
                      }}
                      title="Gestionar miembros"
                      style={{ marginLeft: '4px' }}
                    >
                      <UserPlus size={18} />
                    </button>
                    <button
                      className={styles['btn-icon']}
                      data-tour="tasks-phases"
                      onClick={() => setShowPhasesModal(true)}
                      title="Gestionar fases"
                      style={{ marginLeft: '4px' }}
                    >
                      <Columns3 size={18} />
                    </button>
                    {user?.id === selectedBoard.created_by && (
                      <button
                        className={`${styles['btn-icon']} ${styles['delete-board-btn'] || 'delete-board-btn'}`}
                        onClick={() => handleDeleteBoard(selectedBoard.id)}
                        title={isDeletingBoard ? "Eliminando..." : "Eliminar tablero"}
                        disabled={isDeletingBoard}
                        style={{ marginLeft: '4px', color: '#ef4444' }}
                      >
                        {isDeletingBoard ? '...' : <Trash2 size={18} />}
                      </button>
                    )}
                  </>
                )}
              </>
            )}
          </div>
        </div>
        <button className={styles['btn-primary']} onClick={handleOpenNewTaskModal} data-tour="tasks-new-task">
          + Nueva Tarea
        </button>
      </div>

      {!selectedBoard ? (
        boards.length > 0 ? (
          <div className={styles['board-picker']} data-tour="tasks-board-picker">
            <div className={styles['board-picker-header']}>
              <h2>Selecciona un tablero</h2>
              <p>Elige un tablero para gestionar sus tareas.</p>
            </div>
            <div className={styles['board-picker-grid']}>
              {boards.map((b) => {
                const counts = boardTaskCounts[b.id] || {}
                const phases = b.phases && b.phases.length ? b.phases : []
                const phaseData = phases.map((p) => ({
                  id: p.id,
                  name: p.name,
                  color: p.color || 'var(--primary)',
                  count: counts[p.status || p.name.toLowerCase().replace(/\s+/g, '_')] || 0,
                }))
                const total = Object.values(counts).reduce((sum, n) => sum + n, 0)
                return (
                  <button
                    key={b.id}
                    type="button"
                    className={styles['board-picker-card']}
                    style={{ ['--board-color' as any]: b.color || 'var(--primary)' }}
                    onClick={() => setSelectedBoard(b)}
                  >
                    <div className={styles['board-picker-card-head']}>
                      <h3>{b.name}</h3>
                      <span className={styles['board-picker-total']}>
                        {total} {total === 1 ? 'tarea' : 'tareas'}
                      </span>
                    </div>
                    {b.description && <p>{b.description}</p>}
                    {phases.length > 0 && (
                      <>
                        <div className={styles['board-picker-bar']}>
                          {total > 0 ? (
                            phaseData
                              .filter((p) => p.count > 0)
                              .map((p) => (
                                <span
                                  key={p.id}
                                  className={styles['board-picker-bar-seg']}
                                  style={{ flexGrow: p.count, background: p.color }}
                                />
                              ))
                          ) : (
                            <span className={styles['board-picker-bar-empty']} />
                          )}
                        </div>
                        <div className={styles['board-picker-phases']}>
                          {phaseData.map((p) => (
                            <span key={p.id} className={styles['board-picker-phase']}>
                              <span className={styles['board-picker-phase-dot']} style={{ background: p.color }} />
                              {p.name}
                              <strong>{p.count}</strong>
                            </span>
                          ))}
                        </div>
                      </>
                    )}
                    <span className={styles['board-picker-meta']}>
                      {(b.members?.length || 0)} miembro{(b.members?.length || 0) === 1 ? '' : 's'}
                    </span>
                  </button>
                )
              })}
              <button
                type="button"
                className={`${styles['board-picker-card']} ${styles['board-picker-card-new']}`}
                onClick={openBoardModal}
                data-tour="tasks-create-board"
              >
                <Plus size={24} />
                Crear nuevo tablero
              </button>
            </div>
          </div>
        ) : (
          <div className={styles['tasks-loading']} style={{ background: 'transparent' }} data-tour="tasks-empty">
            <div className={styles['empty-state-glass'] || styles['dashboard-card']}>
              <CheckSquare size={64} style={{ color: 'var(--primary)', marginBottom: '24px', opacity: 0.6 }} />
              <h2 style={{ fontSize: '24px', fontWeight: '700', color: 'var(--black)', marginBottom: '12px' }}>
                No hay tableros
              </h2>
              <p style={{ color: '#64748b', fontSize: '16px', maxWidth: '400px', margin: '0 auto 32px' }}>
                Crea un nuevo tablero para empezar a gestionar las tareas de esta empresa.
              </p>
              <div style={{ display: 'flex', gap: '12px', justifyContent: 'center' }}>
                <button className={styles['btn-primary']} onClick={openBoardModal} data-tour="tasks-create-board">
                  <Plus size={18} /> Crear Nuevo Tablero
                </button>
                <button
                  className={styles['btn-secondary'] || 'btn-secondary'}
                  data-tour="tasks-join-board"
                  onClick={() => {
                    fetchPublicBoards()
                    setShowJoinBoardModal(true)
                  }}
                >
                  Unirse a Tablero
                </button>
              </div>
            </div>
          </div>
        )
      ) : (
        <TasksBoard
          tasks={tasks}
          selectedBoard={selectedBoard}
          onTaskClick={setSelectedTask}
          onUpdateTask={handleUpdateTask}
          onMovePhaseLeft={handleMovePhaseLeft}
          onMovePhaseRight={handleMovePhaseRight}
          onReorderPhase={handleReorderPhaseByIndex}
          onAddPhase={handleAddPhase}
          isSavingPhase={isSavingPhase}
        />
      )}

      {selectedTask && (
        <TaskDetailPanel
          task={selectedTask}
          users={visibleUsers}
          onClose={() => setSelectedTask(null)}
          onUpdate={handleUpdateTask}
          onDelete={handleDeleteTask}
          columns={getCurrentColumns()}
        />
      )}

      <NewTaskModal
        isOpen={showNewTaskModal}
        onClose={() => {
          setShowNewTaskModal(false)
          setNewTaskData({
            title: '',
            description: '',
            priority: 'medium',
            end_date: '',
            assignees: [],
            attachments: [],
          })
        }}
        newTaskData={newTaskData}
        setNewTaskData={setNewTaskData}
        isCreatingTask={isCreatingTask}
        onSubmit={handleCreateTask}
        assigneeSearch={assigneeSearch}
        setAssigneeSearch={setAssigneeSearch}
        filteredUsers={assignableUsers}
      />

      <BoardModal
        isOpen={showBoardModal}
        onClose={() => {
          setShowBoardModal(false)
          setAssigneeSearch('')
          setNewBoardPhaseSearch('')
        }}
        newBoardData={boardFormData}
        setNewBoardData={setBoardFormData}
        assigneeSearch={assigneeSearch}
        setAssigneeSearch={setAssigneeSearch}
        filteredUsers={potentialMemberUsers}
        newBoardPhaseSearch={newBoardPhaseSearch}
        setNewBoardPhaseSearch={setNewBoardPhaseSearch}
        onSubmit={handleBoardSubmit}
        isCreatingBoard={isCreatingBoard}
      />

      <BoardMembersModal
        isOpen={showBoardMembersModal}
        onClose={() => {
          setShowBoardMembersModal(false)
          setOptimisticMembers(selectedBoard?.members?.map((m: User) => m.id) || [])
        }}
        selectedBoard={selectedBoard}
        optimisticMembers={optimisticMembers}
        setOptimisticMembers={setOptimisticMembers}
        users={visibleUsers}
        onUpdateMembers={(newMembers: number[]) => selectedBoard ? updateBoardMembers(selectedBoard.id, newMembers) : Promise.resolve()}
      />

      <JoinBoardModal
        isOpen={showJoinBoardModal}
        onClose={() => setShowJoinBoardModal(false)}
        publicBoards={publicBoards}
        handleJoinBoard={handleJoinBoard}
        isJoiningBoard={isJoiningBoard}
      />

      <PhasesModal
        isOpen={showPhasesModal}
        onClose={() => setShowPhasesModal(false)}
        selectedBoard={selectedBoard}
        draggingPhase={draggingPhase}
        dragOverIdx={dragOverIdx}
        handlePhaseDragStart={handlePhaseDragStart}
        handlePhaseDragEnter={handlePhaseDragEnter}
        handlePhaseDragEnd={handlePhaseDragEnd}
        isDraggingPhasesRef={isDraggingPhasesRef}
        onAddPhase={handleAddPhase}
        onRemovePhase={handleRemovePhase}
        isSavingPhase={isSavingPhase}
      />
    </div>
  )
}
