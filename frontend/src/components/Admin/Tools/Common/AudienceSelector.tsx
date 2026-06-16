import React, { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Search, Plus, X, Users, User, Mail } from 'lucide-react';
import { audienceService, AudienceGroup } from '../../../../services/audienceService';
import { emailService } from '../../../../services/emailService';
import styles from './AudienceSelector.module.css';

export interface ExpressContact {
  name: string;
  email: string;
}

export interface AudienceSelection {
  groupIds: number[];
  userIds: number[];
  expressContacts: ExpressContact[];
}

interface AudienceSelectorProps {
  value: AudienceSelection;
  onChange: (val: AudienceSelection) => void;
}

const AudienceSelector: React.FC<AudienceSelectorProps> = ({ value, onChange }) => {
  const [activeSubTab, setActiveSubTab] = useState<'groups' | 'individuals' | 'express'>('groups');
  const [userSearch, setUserSearch] = useState('');
  const [expressName, setExpressName] = useState('');
  const [expressEmail, setExpressEmail] = useState('');

  const groupIds = value?.groupIds || [];
  const userIds = value?.userIds || [];
  const expressContacts = value?.expressContacts || [];

  const { data: groups = [] } = useQuery<AudienceGroup[]>({
    queryKey: ['audienceGroups'],
    queryFn: audienceService.getGroups
  });

  const { data: users = [] } = useQuery<any[]>({
    queryKey: ['usersList'],
    queryFn: async () => (await emailService.getAvailableRecipients()).data || []
  });

  const toggleGroup = (groupId: number) => {
    const nextGroupIds = groupIds.includes(groupId)
      ? groupIds.filter(id => id !== groupId)
      : [...groupIds, groupId];
    onChange({ ...value, groupIds: nextGroupIds });
  };

  const toggleUser = (userId: number) => {
    const nextUserIds = userIds.includes(userId)
      ? userIds.filter(id => id !== userId)
      : [...userIds, userId];
    onChange({ ...value, userIds: nextUserIds });
  };

  const addExpressContact = (e: React.FormEvent) => {
    e.preventDefault();
    if (!expressName || !expressEmail) return;
    const exists = expressContacts.some(c => c.email.toLowerCase() === expressEmail.toLowerCase());
    if (exists) {
      alert('Este correo ya fue agregado.');
      return;
    }
    const nextExpress = [...expressContacts, { name: expressName, email: expressEmail }];
    onChange({ ...value, expressContacts: nextExpress });
    setExpressName('');
    setExpressEmail('');
  };

  const removeExpressContact = (email: string) => {
    const nextExpress = expressContacts.filter(c => c.email !== email);
    onChange({ ...value, expressContacts: nextExpress });
  };

  const filteredUsers = users.filter(u => 
    u.name.toLowerCase().includes(userSearch.toLowerCase()) || 
    u.email.toLowerCase().includes(userSearch.toLowerCase())
  );

  return (
    <div className={styles.selectorContainer}>
      <div className={styles.selectorTabs}>
        <button
          type="button"
          className={`${styles.tabBtn} ${activeSubTab === 'groups' ? styles.active : ''}`}
          onClick={() => setActiveSubTab('groups')}
        >
          <Users size={16} /> Grupos ({groups.length})
        </button>
        <button
          type="button"
          className={`${styles.tabBtn} ${activeSubTab === 'individuals' ? styles.active : ''}`}
          onClick={() => setActiveSubTab('individuals')}
        >
          <User size={16} /> Individuales
        </button>
        <button
          type="button"
          className={`${styles.tabBtn} ${activeSubTab === 'express' ? styles.active : ''}`}
          onClick={() => setActiveSubTab('express')}
        >
          <Mail size={16} /> Contacto Express
        </button>
      </div>

      <div className={styles.selectorContent}>
        {activeSubTab === 'groups' && (
          <div className={styles.groupsList}>
            {groups.length === 0 ? (
              <p className={styles.emptyText}>No tienes grupos creados. Administra tus grupos en la pestaña de métricas o de herramientas.</p>
            ) : (
              groups.map(group => (
                <label key={group.id} className={`${styles.itemLabel} ${groupIds.includes(group.id!) ? styles.selected : ''}`}>
                  <input
                    type="checkbox"
                    checked={groupIds.includes(group.id!)}
                    onChange={() => toggleGroup(group.id!)}
                  />
                  <div className={styles.itemInfo}>
                    <strong>{group.name}</strong>
                    <span>{group.members?.length || 0} miembros</span>
                  </div>
                </label>
              ))
            )}
          </div>
        )}

        {activeSubTab === 'individuals' && (
          <div className={styles.individualsSection}>
            <div className={styles.searchBar}>
              <Search size={16} />
              <input
                type="text"
                placeholder="Buscar usuarios..."
                value={userSearch}
                onChange={e => setUserSearch(e.target.value)}
              />
            </div>
            <div className={styles.usersList}>
              {filteredUsers.map(user => (
                <label key={user.id} className={`${styles.itemLabel} ${userIds.includes(user.id) ? styles.selected : ''}`}>
                  <input
                    type="checkbox"
                    checked={userIds.includes(user.id)}
                    onChange={() => toggleUser(user.id)}
                  />
                  <div className={styles.itemInfo}>
                    <strong>{user.name}</strong>
                    <span>{user.email}</span>
                  </div>
                </label>
              ))}
            </div>
          </div>
        )}

        {activeSubTab === 'express' && (
          <div className={styles.expressSection}>
            <form onSubmit={addExpressContact} className={styles.expressForm}>
              <input
                type="text"
                placeholder="Nombre"
                value={expressName}
                onChange={e => setExpressName(e.target.value)}
                required
              />
              <input
                type="email"
                placeholder="Correo Electrónico"
                value={expressEmail}
                onChange={e => setExpressEmail(e.target.value)}
                required
              />
              <button type="submit" className={styles.addExpressBtn}>
                <Plus size={16} /> Añadir
              </button>
            </form>

            <div className={styles.expressList}>
              {expressContacts.map(contact => (
                <div key={contact.email} className={styles.expressTag}>
                  <div className={styles.tagInfo}>
                    <strong>{contact.name}</strong>
                    <span>{contact.email}</span>
                  </div>
                  <button type="button" onClick={() => removeExpressContact(contact.email)}>
                    <X size={14} />
                  </button>
                </div>
              ))}
            </div>
          </div>
        )}
      </div>

      {/* Footer summarizing selections */}
      <div className={styles.selectorSummary}>
        <strong>Audiencia Seleccionada:</strong>
        <span>
          {groupIds.length} grupos, {userIds.length} individuales, {expressContacts.length} contactos express.
        </span>
      </div>
    </div>
  );
};

export default AudienceSelector;
