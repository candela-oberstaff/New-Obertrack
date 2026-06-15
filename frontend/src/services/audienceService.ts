import api from './client';

export interface AudienceGroup {
  id?: number;
  name: string;
  description: string;
  members?: Array<{ id: number; name: string; email: string }>;
  created_at?: string;
}

export const audienceService = {
  getGroups: async (): Promise<AudienceGroup[]> => {
    const response = await api.get('/audiences/groups');
    return response.data || [];
  },

  getGroupByID: async (id: number): Promise<AudienceGroup> => {
    const response = await api.get(`/audiences/groups/${id}`);
    return response.data;
  },

  createGroup: async (group: AudienceGroup): Promise<AudienceGroup> => {
    const response = await api.post('/audiences/groups', group);
    return response.data;
  },

  updateGroup: async (id: number, group: Partial<AudienceGroup>): Promise<AudienceGroup> => {
    const response = await api.put(`/audiences/groups/${id}`, group);
    return response.data;
  },

  deleteGroup: async (id: number): Promise<void> => {
    await api.delete(`/audiences/groups/${id}`);
  },

  addMember: async (groupId: number, userId: number): Promise<void> => {
    await api.post(`/audiences/groups/${groupId}/members`, { user_id: userId });
  },

  removeMember: async (groupId: number, userId: number): Promise<void> => {
    await api.delete(`/audiences/groups/${groupId}/members/${userId}`);
  }
};
