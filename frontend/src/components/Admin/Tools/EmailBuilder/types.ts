export type BlockType = 'text' | 'image' | 'button' | 'divider' | 'spacer' | 'social' | 'columns' | 'settings';

export interface Block {
  id: string;
  type: BlockType;
  content: any;
  style: any;
}

export interface EmailBuilderProps {
  onBack: () => void;
  onSave?: (data: { title: string, subject: string, blocks: Block[] }) => void;
  onSend?: (data: { title: string, subject: string, blocks: Block[], recipientIds: any }) => void;
  onSchedule?: (data: { title: string, subject: string, blocks: Block[], date: string, recipientIds: any }) => void;
  availableRecipients?: any[];
  initialBlocks?: Block[];
  initialTitle?: string;
  initialSubject?: string;
  initialScheduledAt?: string;
  initialRecipientIds?: any;
}

