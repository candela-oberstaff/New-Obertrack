import React, { useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { X, Plus, Trash2, Users, UserMinus, UserPlus, Search } from 'lucide-react';
import { audienceService, AudienceGroup } from '../../../../services/audienceService';
import { emailService } from '../../../../services/emailService';
import styles from './GroupManager.module.css';

interface GroupManagerProps {
  onClose: () => void;
}

const GroupManager: React.FC<GroupManagerProps> = ({ onClose }) => {
  const qc = useQueryClient();
  const [activeGroupId, setActiveGroupId] = useState<number | null>(null);
  const [newGroupName, setNewGroupName] = useState('');
  const [newGroupDesc, setNewGroupDesc] = useState('');
  const [userSearch, setUserSearch] = useState('');

  const { data: groups = [] } = useQuery<AudienceGroup[]>({
    queryKey: ['audienceGroups'],
    queryFn: audienceService.getGroups
  });

  const { data: users = [] } = useQuery<any[]>({
    queryKey: ['usersList'],
    queryFn: async () => (await emailService.getAvailableRecipients()).data || []
  });

  const activeGroup = groups.find(g => g.id === activeGroupId);

  const handleCreateGroup = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newGroupName) return;
    try {
      await audienceService.createGroup({ name: newGroupName, description: newGroupDesc });
      setNewGroupName('');
      setNewGroupDesc('');
      qc.invalidateQueries({ queryKey: ['audienceGroups'] });
      alert('Grupo creado exitosamente');
    } catch {
      alert('Error al crear el grupo');
    }
  };

  const handleDeleteGroup = async (id: number) => {
    if (!window.confirm('¿Estás seguro de que deseas eliminar este grupo?')) return;
    try {
      await audienceService.deleteGroup(id);
      if (activeGroupId === id) setActiveGroupId(null);
      qc.invalidateQueries({ queryKey: ['audienceGroups'] });
    } catch {
      alert('Error al eliminar el grupo');
    }
  };

  const handleAddMember = async (userId: number) => {
    if (!activeGroupId) return;
    try {
      await audienceService.addMember(activeGroupId, userId);
      qc.invalidateQueries({ queryKey: ['audienceGroups'] });
    } catch {
      alert('Error al agregar miembro');
    }
  };

  const handleRemoveMember = async (userId: number) => {
    if (!activeGroupId) return;
    try {
      await audienceService.removeMember(activeGroupId, userId);
      qc.invalidateQueries({ queryKey: ['audienceGroups'] });
    } catch {
      alert('Error al remover miembro');
    }
  };

  const filteredUsers = users.filter(u => {
    // Exclude users already in activeGroup
    const isMember = activeGroup?.members?.some(m => m.id === u.id);
    const matchesSearch = u.name.toLowerCase().includes(userSearch.toLowerCase()) || 
                          u.email.toLowerCase().includes(userSearch.toLowerCase());
    return !isMember && matchesSearch;
  });

  return (
    <div className={styles.modalOverlay}>
      <div className={styles.modalContainer}>
        <header className={styles.modalHeader}>
          <h3><Users size={20} /> Gestor de Grupos de Audiencia</h3>
          <button className={styles.closeBtn} onClick={onClose}><X size={20} /></button>
        </header>

        <div className={styles.modalBody}>
          {/* Left panel: List and Create Groups */}
          <div className={styles.groupsPanel}>
            <form onSubmit={handleCreateGroup} className={styles.createForm}>
              <h4>Crear Nuevo Grupo</h4>
              <input
                type="text"
                placeholder="Nombre del grupo"
                value={newGroupName}
                onChange={e => setNewGroupName(e.target.value)}
                required
              />
              <input
                type="text"
                placeholder="Descripción (opcional)"
                value={newGroupDesc}
                onChange={e => setNewGroupDesc(e.target.value)}
              />
              <button type="submit" className={styles.submitBtn}>
                <Plus size={16} /> Crear Grupo
              </button>
            </form>

            <div className={styles.groupsList}>
              <h4>Tus Grupos</h4>
              {groups.length === 0 ? (
                <p className={styles.emptyText}>No hay grupos creados.</p>
              ) : (
                groups.map(group => (
                  <div
                    key={group.id}
                    className={`${styles.groupRow} ${activeGroupId === group.id ? styles.active : ''}`}
                    onClick={() => setActiveGroupId(group.id!)}
                  >
                    <div className={styles.groupInfo}>
                      <strong>{group.name}</strong>
                      <span>{group.members?.length || 0} miembros</span>
                    </div>
                    <button
                      className={styles.deleteGroupBtn}
                      onClick={(e) => { e.stopPropagation(); handleDeleteGroup(group.id!); }}
                    >
                      <Trash2 size={14} />
                    </button>
                  </div>
                ))
              )}
            </div>
          </div>

          {/* Right panel: Members management */}
          <div className={styles.membersPanel}>
            {activeGroup ? (
              <>
                <div className={styles.membersHeader}>
                  <h4>Miembros de "{activeGroup.name}"</h4>
                  <p>{activeGroup.description}</p>
                </div>

                <div className={styles.membersColumns}>
                  {/* Current Members */}
                  <div className={styles.columnSection}>
                    <h5>Miembros Actuales ({activeGroup.members?.length || 0})</h5>
                    <div className={styles.columnList}>
                      {(!activeGroup.members || activeGroup.members.length === 0) ? (
                        <p className={styles.emptyText}>Este grupo no tiene miembros.</p>
                      ) : (
                        activeGroup.members.map(member => (
                          <div key={member.id} className={styles.memberRow}>
                            <div>
                              <strong>{member.name}</strong>
                              <span>{member.email}</span>
                            </div>
                            <button
                              className={styles.removeMemberBtn}
                              onClick={() => handleRemoveMember(member.id)}
                              title="Remover miembro"
                            >
                              <UserMinus size={14} />
                            </button>
                          </div>
                        ))
                      )}
                    </div>
                  </div>

                  {/* Add Members */}
                  <div className={styles.columnSection}>
                    <h5>Añadir Miembros</h5>
                    <div className={styles.searchBar}>
                      <Search size={14} />
                      <input
                        type="text"
                        placeholder="Buscar usuarios..."
                        value={userSearch}
                        onChange={e => setUserSearch(e.target.value)}
                      />
                    </div>
                    <div className={styles.columnList}>
                      {filteredUsers.length === 0 ? (
                        <p className={styles.emptyText}>No hay más usuarios que coincidan.</p>
                      ) : (
                        filteredUsers.map(user => (
                          <div key={user.id} className={styles.memberRow}>
                            <div>
                              <strong>{user.name}</strong>
                              <span>{user.email}</span>
                            </div>
                            <button
                              className={styles.addMemberBtn}
                              onClick={() => handleAddMember(user.id)}
                              title="Añadir miembro"
                            >
                              <UserPlus size={14} />
                            </button>
                          </div>
                        ))
                      )}
                    </div>
                  </div>
                </div>
              </>
            ) : (
              <div className={styles.noGroupSelected}>
                <Users size={40} />
                <p>Selecciona un grupo de la lista para ver y gestionar sus miembros.</p>
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
};

export default GroupManager;
