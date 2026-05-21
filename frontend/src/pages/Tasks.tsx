import { useTasksPageState } from '../components/Tasks/hooks/useTasksPageState'
import type { User, Board } from '../types'

import { TasksBoard } from '../components/Tasks/components/TasksBoard'
import { TaskDetailPanel } from '../components/Tasks/TaskDetailPanel'
import { NewTaskModal } from '../components/Tasks/Modals/NewTaskModal'
import { BoardModal } from '../components/Tasks/Modals/BoardModal'
import { BoardMembersModal } from '../components/Tasks/Modals/BoardMembersModal'
import { JoinBoardModal } from '../components/Tasks/Modals/JoinBoardModal'
import {
  Plus,
  UserPlus,
  Trash2,
  CheckSquare
} from 'lucide-react'
import styles from './Tasks.module.css'

export default function Tasks() {
  const {
    user,
    visibleUsers,
    assigneeSearch,
    setAssigneeSearch,
    showNewTaskModal,
    setShowNewTaskModal,
    showBoardModal,
    setShowBoardModal,
    showBoardMembersModal,
    setShowBoardMembersModal,
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

  // Empty state
  if (boards.length === 0) {
    return (
      <div className={styles['tasks-page']}>
        <div className={styles['page-header']}>
          <h1>Tareas</h1>
        </div>
        <div className={styles['tasks-loading']}>
          <h2>No tienes tableros</h2>
          <p>Crea tu primer tablero para organizar tus tareas</p>
          <div style={{ display: 'flex', gap: '12px' }}>
            <button className={styles['btn-primary']} onClick={openBoardModal}>
              <Plus size={18} /> Crear Tablero
            </button>
            <button 
              className={styles['btn-secondary'] || 'btn-secondary'} 
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
      <div className={styles['page-header']}>
        <div className={styles['header-left']}>
          <h1>Tareas</h1>
          <div className={styles['board-selector']}>
            {boards.length > 0 && (
              <>
                <div className={styles['board-select-container']}>
                  <select
                    value={selectedBoard?.id || ''}
                    onChange={(e) => {
                      if (e.target.value === '') {
                        setSelectedBoard(null)
                        return
                      }
                      const board = boards.find((b: Board) => b.id === Number(e.target.value))
                      if (board) setSelectedBoard(board)
                    }}
                    style={{ borderLeftColor: selectedBoard?.color || 'transparent' }}
                  >
                    <option value="">Seleccione un tablero...</option>
                    {boards.map((board: Board) => (
                      <option key={board.id} value={board.id}>{board.name}</option>
                    ))}
                  </select>
                </div>
                <button className={styles['btn-icon']} onClick={openBoardModal} title="Crear tablero">
                  <Plus size={18} />
                </button>
                <button
                  className={`${styles['btn-secondary'] || 'btn-secondary'} ${styles['btn-sm'] || 'btn-sm'}`}
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
                      onClick={() => {
                        setOptimisticMembers(selectedBoard.members?.map((m: User) => m.id) || [])
                        setShowBoardMembersModal(true)
                      }}
                      title="Gestionar miembros"
                      style={{ marginLeft: '4px' }}
                    >
                      <UserPlus size={18} />
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
        <button className={styles['btn-primary']} onClick={handleOpenNewTaskModal}>
          + Nueva Tarea
        </button>
      </div>

      {!selectedBoard ? (
        <div className={styles['tasks-loading']} style={{ background: 'transparent' }}>
          <div className={styles['empty-state-glass'] || styles['dashboard-card']}>
            <CheckSquare size={64} style={{ color: 'var(--primary)', marginBottom: '24px', opacity: 0.6 }} />
            <h2 style={{ fontSize: '24px', fontWeight: '700', color: 'var(--black)', marginBottom: '12px' }}>
              Comienza seleccionando un tablero
            </h2>
            <p style={{ color: '#64748b', fontSize: '16px', maxWidth: '400px', margin: '0 auto 32px' }}>
              Elige uno de tus tableros en el menú superior o crea uno nuevo para empezar a gestionar tus tareas.
            </p>
            <div style={{ display: 'flex', gap: '12px', justifyContent: 'center' }}>
              <button className={styles['btn-primary']} onClick={openBoardModal}>
                <Plus size={18} /> Crear Nuevo Tablero
              </button>
              <button 
                className={styles['btn-secondary'] || 'btn-secondary'} 
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
      ) : (
        <TasksBoard
          tasks={tasks}
          selectedBoard={selectedBoard}
          onTaskClick={setSelectedTask}
          onUpdateTask={handleUpdateTask}
          onMovePhaseLeft={handleMovePhaseLeft}
          onMovePhaseRight={handleMovePhaseRight}
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
    </div>
  )
}
