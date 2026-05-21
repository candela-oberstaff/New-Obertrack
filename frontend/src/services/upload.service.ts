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
    // Ensure the file name is sent with the form data. For Blob objects, provide a default name.
    const fileName = (file as File).name || 'audio.webm'
    formData.append('file', file, fileName)
    const { data } = await api.post<UploadResponse>('/uploads', formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
    })
    return data
  },
}
