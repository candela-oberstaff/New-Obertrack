export type BlockType = 'text' | 'image' | 'button' | 'divider' | 'spacer' | 'social';

export interface Block {
  id: string;
  type: BlockType;
  content: any;
  style: any;
}

export interface EmailBuilderProps {
  onBack: () => void;
  onSave?: (data: { title: string, blocks: Block[] }) => void;
  onSend?: (data: { title: string, blocks: Block[], recipientIds: number[] }) => void;
  onSchedule?: (data: { title: string, blocks: Block[], date: string, recipientIds: number[] }) => void;
  availableRecipients?: any[];
  initialBlocks?: Block[];
  initialTitle?: string;
  initialScheduledAt?: string;
  initialRecipientIds?: number[];
}
