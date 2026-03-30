import api from './client'

export interface UploadResponse {
  url: string
  filename: string
  size: number
  type: string
}

export const uploadService = {
  upload: async (file: File | Blob): Promise<UploadResponse> => {
    const formData = new FormData()
    formData.append('file', file)
    const { data } = await api.post<UploadResponse>('/uploads', formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
    })
    return data
  },
}
