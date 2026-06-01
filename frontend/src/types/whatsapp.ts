export interface WhatsAppChatTicket {
  zoho_id: string
  contact_name: string
  contact_phone: string
  subject: string
  status: string
  assignee_id?: string
  modified_time: string
}

export interface WhatsAppMessageDTO {
  id: string
  content: string
  direction: 'incoming' | 'outgoing'
  author_name: string
  author_type: string
  created_time: string
}
