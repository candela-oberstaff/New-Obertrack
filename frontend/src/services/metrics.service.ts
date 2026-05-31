import api from './client';

export interface MetricsData {
  emails: {
    total_sent: number;
    total_opened: number;
    total_clicked: number;
    total_bounced: number;
    open_rate: number;
    click_rate: number;
    campaign_count: number;
    evolution: Array<{ date: string; event: string; count: number }>;
  };
  surveys: {
    total_surveys: number;
    total_responses: number;
    avg_satisfaction: number;
  };
  advanced: {
    segments: Array<{ name: string; engagement: number }>;
    devices: {
      desktop: number;
      mobile: number;
    };
  };
}

export const metricsService = {
  getGlobalMetrics: async (days: number = 30): Promise<MetricsData> => {
    const response = await api.get(`/metrics?days=${days}`);
    return response.data;
  }
};
