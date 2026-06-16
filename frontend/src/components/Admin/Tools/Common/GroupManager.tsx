import React, { useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { X, Plus, Trash2, Users, UserMinus, UserPlus, Search } from 'lucide-react';
import { audienceService, AudienceGroup } from '../../../../services/audienceService';
import { emailService } from '../../../../services/emailService';
import styles from './GroupManager.module.css';

interface GroupManagerProps {
  onClose: () => void;
}

interface User {
  id: number;
  name: string;
  email: string;
  is_superadmin?: boolean;
  is_manager?: boolean;
  user_type?: 'profesional' | 'empleador' | 'customer_success' | 'superadmin' | 'analista_it';
}


const GroupManager: React.FC<GroupManagerProps> = ({ onClose }) => {
  const qc = useQueryClient();
  const [activeGroupId, setActiveGroupId] = useState<number | null>(null);
  const [newGroupName, setNewGroupName] = useState('');
  const [newGroupDesc, setNewGroupDesc] = useState('');
  const [userSearch, setUserSearch] = useState('');
  const [userTypeFilter, setUserTypeFilter] = useState('all');

  const { data: groups = [] } = useQuery<AudienceGroup[]>({
    queryKey: ['audienceGroups'],
    queryFn: audienceService.getGroups
  });

  const { data: users = [] } = useQuery<User[]>({
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
    if (isMember) return false;

    const matchesSearch = u.name.toLowerCase().includes(userSearch.toLowerCase()) || 
                          u.email.toLowerCase().includes(userSearch.toLowerCase());
    if (!matchesSearch) return false;

    if (userTypeFilter !== 'all') {
      if (userTypeFilter === 'superadmin') return u.is_superadmin || u.user_type === 'superadmin';
      if (userTypeFilter === 'manager') return u.is_manager;
      return u.user_type === userTypeFilter;
    }

    return true;
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
                              <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                                <strong>{member.name}</strong>
                                {member.is_superadmin && <span style={{ fontSize: 9, background: '#fee2e2', color: '#ef4444', padding: '1px 5px', borderRadius: 4, fontWeight: 700 }}>Admin</span>}
                                {member.user_type === 'empleador' && <span style={{ fontSize: 9, background: '#dbeafe', color: '#2563eb', padding: '1px 5px', borderRadius: 4, fontWeight: 700 }}>Empresa</span>}
                                {member.user_type === 'profesional' && <span style={{ fontSize: 9, background: '#f0fdf4', color: '#16a34a', padding: '1px 5px', borderRadius: 4, fontWeight: 700 }}>Profesional</span>}
                                {member.user_type === 'customer_success' && <span style={{ fontSize: 9, background: '#f5f3ff', color: '#7c3aed', padding: '1px 5px', borderRadius: 4, fontWeight: 700 }}>CS</span>}
                              </div>
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
                    <div style={{ display: 'flex', flexDirection: 'column', gap: 8, marginBottom: 12 }}>
                      <div className={styles.searchBar} style={{ marginBottom: 0 }}>
                        <Search size={14} />
                        <input
                          type="text"
                          placeholder="Buscar usuarios..."
                          value={userSearch}
                          onChange={e => setUserSearch(e.target.value)}
                        />
                      </div>
                      <select
                        style={{
                          padding: '6px 10px',
                          border: '1px solid #e2e8f0',
                          borderRadius: 6,
                          fontSize: 12,
                          outline: 'none',
                          background: 'white',
                          cursor: 'pointer',
                        }}
                        value={userTypeFilter}
                        onChange={e => setUserTypeFilter(e.target.value)}
                      >
                        <option value="all">Todos los tipos de usuario</option>
                        <option value="profesional">Profesionales</option>
                        <option value="empleador">Empresas (Empleadores)</option>
                        <option value="customer_success">Customer Success</option>
                        <option value="manager">Managers</option>
                        <option value="superadmin">Administradores</option>
                      </select>
                    </div>
                    <div className={styles.columnList}>
                      {filteredUsers.length === 0 ? (
                        <p className={styles.emptyText}>No hay más usuarios que coincidan.</p>
                      ) : (
                        filteredUsers.map(user => (
                          <div key={user.id} className={styles.memberRow}>
                            <div>
                              <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                                <strong>{user.name}</strong>
                                {user.is_superadmin && <span style={{ fontSize: 9, background: '#fee2e2', color: '#ef4444', padding: '1px 5px', borderRadius: 4, fontWeight: 700 }}>Admin</span>}
                                {user.user_type === 'empleador' && <span style={{ fontSize: 9, background: '#dbeafe', color: '#2563eb', padding: '1px 5px', borderRadius: 4, fontWeight: 700 }}>Empresa</span>}
                                {user.user_type === 'profesional' && <span style={{ fontSize: 9, background: '#f0fdf4', color: '#16a34a', padding: '1px 5px', borderRadius: 4, fontWeight: 700 }}>Profesional</span>}
                                {user.user_type === 'customer_success' && <span style={{ fontSize: 9, background: '#f5f3ff', color: '#7c3aed', padding: '1px 5px', borderRadius: 4, fontWeight: 700 }}>CS</span>}
                              </div>
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
