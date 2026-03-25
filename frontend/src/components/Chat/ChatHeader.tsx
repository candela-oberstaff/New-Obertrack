
import { Channel, Message } from '../../types/chat'

interface ChatHeaderProps {
  selectedChannel: Channel | null
  showMobileChannels: boolean
  setShowMobileChannels: (show: boolean) => void
  showSearch: boolean
  setShowSearch: (show: boolean) => void
  setSearchQuery: (query: string) => void
  setSearchResults: (results: Message[]) => void
  setShowStarred: (show: boolean) => void
  loadStarredMessages: () => void
  showPinnedMessages: boolean
  setShowPinnedMessages: (show: boolean) => void
  pinnedMessagesCount: number
  setShowAddMembers: (show: boolean) => void
  setShowChannelSettings: (show: boolean) => void
  leaveChannel: (channelId: number) => void
  setShowNewChannelModal: (show: boolean) => void
}

export function ChatHeader({
  selectedChannel,
  showMobileChannels,
  setShowMobileChannels,
  showSearch,
  setShowSearch,
  setSearchQuery,
  setSearchResults,
  setShowStarred,
  loadStarredMessages,
  setShowPinnedMessages,
  pinnedMessagesCount,
  setShowAddMembers,
  setShowChannelSettings,
  leaveChannel,
  setShowNewChannelModal,
}: ChatHeaderProps) {
  return (
    <div className="chat-header-bar">
      <button className="mobile-channels-toggle" onClick={() => setShowMobileChannels(!showMobileChannels)}>
        {selectedChannel ? `# ${selectedChannel.name}` : 'Seleccionar canal'}
      </button>
      
      <div className="channel-tabs">
        {selectedChannel && (
          <div className="channel-tab active">
            <span>{selectedChannel.type === 'private' ? '🔒' : '#'}</span>
            {selectedChannel.name}
          </div>
        )}
      </div>

      <div className="channel-actions">
        {selectedChannel && (
          <>
            <button onClick={() => { setShowSearch(!showSearch); setSearchQuery(''); setSearchResults([]); }} title="Buscar">
              ⌕
            </button>
            <button onClick={() => { setShowStarred(true); loadStarredMessages(); }} title="Mensajes starred">
              ☆
            </button>
            <button onClick={() => setShowPinnedMessages(true)} title="Mensajes fijados">
              • {pinnedMessagesCount > 0 && <span className="pin-count">{pinnedMessagesCount}</span>}
            </button>
            <button onClick={() => setShowAddMembers(true)} title="Añadir personas">
              ⊕
            </button>
            <button onClick={() => setShowChannelSettings(true)} title="Info del canal">
              ◈
            </button>
            <button onClick={() => leaveChannel(selectedChannel.id)} title="Salir">
              ⊗
            </button>
          </>
        )}
        <button onClick={() => setShowNewChannelModal(true)} title="Crear canal">
          +
        </button>
      </div>
    </div>
  )
}
