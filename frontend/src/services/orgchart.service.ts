import api from './client'
import type { OrgPerson } from '../components/OrgChart/orgTree'

export const orgChartService = {
  /**
   * Organigrama de una empresa. El superadmin indica cuál con companyId; el
   * empleador y el supervisor lo omiten y el backend resuelve la suya (y le
   * recorta lo que puede ver).
   */
  get: async (companyId?: number): Promise<OrgPerson[]> => {
    const { data } = await api.get('/org-chart', {
      params: companyId ? { company_id: companyId } : undefined,
    })
    return data?.data ?? []
  },
}
