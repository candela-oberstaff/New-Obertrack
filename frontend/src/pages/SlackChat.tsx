import { useState, useEffect, useRef } from 'react'
import { useAuth } from '../context/AuthContext'
import { channelService, uploadService } from '../services/api'
import type { User } from '../types'
import type { Message } from '../types/chat'
import { Sidebar } from '../components/Chat/Sidebar'
import { ChatHeader } from '../components/Chat/ChatHeader'
import { MessageList } from '../components/Chat/MessageList'
import { MessageInput } from '../components/Chat/MessageInput'
import { ThreadPanel } from '../components/Chat/ThreadPanel'
import { NewChannelModal } from '../components/Chat/Modals/NewChannelModal'
import { ChannelSettingsModal } from '../components/Chat/Modals/ChannelSettingsModal'
import { AddMembersModal } from '../components/Chat/Modals/AddMembersModal'
import { NewDmModal } from '../components/Chat/Modals/NewDmModal'
import { PinnedMessagesModal } from '../components/Chat/Modals/PinnedMessagesModal'
import { useSlackChat } from '../hooks/useSlackChat'
import { formatTime, highlightMentions, getUserColor } from '../components/Chat/ChatUtils'
import styles from './SlackChat.module.css'

export default function SlackChat() {
  const { user } = useAuth()
  const {
    channels, selectedChannel, setSelectedChannel,
    messages, setMessages, pinnedMessages,
    newMessage, setNewMessage,
    channelMembers, fetchChannelMembers,
    allUsers, unreadCount,
    typingUsers, isRecording, isUploading, setIsUploading,
    showThread, setShowThread, threadReplies, setThreadReplies,
    sendMessage, sendTypingIndicator, startRecording, stopRecording,
    editMessage, deleteMessage, pinMessage, unpinMessage, fetchChannels
  } = useSlackChat(user as any)

  const [showNewChannelModal, setShowNewChannelModal] = useState(false)
  const [showChannelSettings, setShowChannelSettings] = useState(false)
  const [showAddMembers, setShowAddMembers] = useState(false)
  const [showNewDmModal, setShowNewDmModal] = useState(false)
  const [showPinnedMessages, setShowPinnedMessages] = useState(false)
  const [showMobileChannels, setShowMobileChannels] = useState(false)
  const [chatSidebarCollapsed, setChatSidebarCollapsed] = useState(false)
  const [chatSidebarWidth, setChatSidebarWidth] = useState(220)
  const [isResizing, setIsResizing] = useState(false)
  const [playingAudio, setPlayingAudio] = useState<string | null>(null)
  const messagesEndRef = useRef<HTMLDivElement>(null)
  const audioRef = useRef<HTMLAudioElement | null>(null)
  const [newChannel, setNewChannel] = useState({ name: '', description: '', type: 'public' as 'public' | 'private' })
  const [editingMessageId, setEditingMessageId] = useState<number | null>(null)
  const [editContent, setEditContent] = useState('')
  const [showMentionDropdown, setShowMentionDropdown] = useState(false)
  const [mentionFilterUsers, setMentionFilterUsers] = useState<User[]>([])
  const [originalFavicon, setOriginalFavicon] = useState<string | null>(null)

  useEffect(() => {
    const icon = document.querySelector("link[rel*='icon']") as HTMLLinkElement
    if (icon) setOriginalFavicon(icon.href)
  }, [])

  useEffect(() => { scrollToBottom() }, [messages])

  useEffect(() => {
    const icon = document.querySelector("link[rel*='icon']") as HTMLLinkElement
    if (!icon) return
    if (unreadCount > 0) {
      icon.href = `data:image/svg+xml,<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 100 100'><text y='.9em' font-size='90'>🔴</text></svg>`
    } else if (originalFavicon) {
      icon.href = originalFavicon
    }
  }, [unreadCount, originalFavicon])

  const scrollToBottom = () => messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' })

  const handleInputKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      if (newMessage.trim()) {
        const tempId = `temp-${Date.now()}`
        const optimisticMsg: Message = { id: 0, channel_id: selectedChannel!.id, user_id: user!.id, content: newMessage, tempId, created_at: new Date().toISOString(), user: user! as any }
        setMessages(prev => [...prev, optimisticMsg])
        sendMessage(undefined, tempId)
      }
    }
    if (e.key === '@') { setMentionFilterUsers(allUsers); setShowMentionDropdown(true) }
    else if (showMentionDropdown && (e.key === 'Escape' || e.key === ' ')) { setShowMentionDropdown(false) }
  }

  const handleFileUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return
    setIsUploading(true)
    try {
      const result = await uploadService.upload(file)
      const tempId = `temp-${Date.now()}`
      const optimisticMsg: Message = { id: 0, channel_id: selectedChannel!.id, user_id: user!.id, content: `[Archivo: ${file.name}]`, tempId, created_at: new Date().toISOString(), user: user! as any }
      setMessages(prev => [...prev, optimisticMsg])
      sendMessage({ url: result.url, filename: file.name }, tempId)
    } catch (error) { console.error('Error uploading file:', error) }
    finally { setIsUploading(false); e.target.value = '' }
  }

  const togglePlayAudio = (url: string) => {
    if (playingAudio === url) { audioRef.current?.pause(); setPlayingAudio(null) }
    else { if (audioRef.current) audioRef.current.pause(); audioRef.current = new Audio(url); audioRef.current.onended = () => setPlayingAudio(null); audioRef.current.play(); setPlayingAudio(url) }
  }

  const handleSaveEdit = async () => {
    if (!selectedChannel || !editingMessageId || !editContent.trim()) return
    await editMessage(editingMessageId, editContent)
    setEditingMessageId(null); setEditContent('')
  }

  const leaveChannel = async (id: number) => { try { await channelService.leaveChannel(id); if (selectedChannel?.id === id) setSelectedChannel(null); fetchChannels() } catch (e) { console.error(e) } }
  const addMember = async (id: number) => { if (selectedChannel) try { await channelService.addMember(selectedChannel.id, id); fetchChannelMembers(selectedChannel.id) } catch (e) { console.error(e) } }
  const removeMember = async (id: number) => { if (selectedChannel) try { await channelService.removeMember(selectedChannel.id, id); fetchChannelMembers(selectedChannel.id) } catch (e) { console.error(e) } }
  const handleUpdateChannel = async (id: number, updates: { name: string; description: string }) => {
    try {
      const updated = await channelService.updateChannel(id, updates)
      setSelectedChannel(updated as any)
      fetchChannels()
    } catch (e) {
      console.error('Error updating channel:', e)
    }
  }

  const handleMouseDown = (e: React.MouseEvent) => {
    e.preventDefault(); setIsResizing(true)
    const onMouseMove = (e: MouseEvent) => { setChatSidebarWidth(Math.max(150, Math.min(400, e.clientX))) }
    const onMouseUp = () => { setIsResizing(false); document.removeEventListener('mousemove', onMouseMove); document.removeEventListener('mouseup', onMouseUp) }
    document.addEventListener('mousemove', onMouseMove); document.addEventListener('mouseup', onMouseUp)
  }

  const openThread = async (msg: Message) => {
    if (!selectedChannel) return
    setShowThread(msg)
    try {
      const replies = await channelService.getThreadReplies(selectedChannel.id, msg.id)
      setThreadReplies(replies || [])
    } catch (e) { console.error(e) }
  }

  const sendThreadReply = async (content: string) => {
    if (selectedChannel && showThread && content.trim()) {
      try {
        // Don't add optimistically - WS delivers it to all clients including sender
        await channelService.sendThreadReply(selectedChannel.id, showThread.id, content)
      } catch (e) { console.error(e) }
    }
  }

  return (
    <div className={styles['chat-page']}>
      <ChatHeader
        selectedChannel={selectedChannel}
        showMobileChannels={showMobileChannels}
        setShowMobileChannels={setShowMobileChannels}
        showPinnedMessages={showPinnedMessages}
        setShowPinnedMessages={setShowPinnedMessages}
        pinnedMessagesCount={pinnedMessages.length}
        setShowAddMembers={setShowAddMembers}
        setShowChannelSettings={setShowChannelSettings}
        leaveChannel={leaveChannel}
      />

      <div className={styles['chat-body']}>
        <Sidebar
          channels={channels}
          selectedChannel={selectedChannel}
          setSelectedChannel={setSelectedChannel}
          showMobileChannels={showMobileChannels}
          setShowMobileChannels={setShowMobileChannels}
          chatSidebarCollapsed={chatSidebarCollapsed}
          setChatSidebarCollapsed={setChatSidebarCollapsed}
          chatSidebarWidth={chatSidebarWidth}
          setShowNewChannelModal={setShowNewChannelModal}
          setShowNewDmModal={setShowNewDmModal}
          fetchAllUsers={fetchAllUsers}
          onMouseDownResize={handleMouseDown}
          isResizing={isResizing}
        />

        <div className={styles['chat-main']}>
          {selectedChannel ? (
            <>
              <MessageList
                messages={messages}
                currentUser={user as any}
                editingMessageId={editingMessageId}
                editContent={editContent}
                setEditContent={setEditContent}
                onSaveEdit={handleSaveEdit}
                onCancelEdit={() => { setEditingMessageId(null); setEditContent('') }}
                onStartEdit={(msg) => { setEditingMessageId(msg.id); setEditContent(msg.content) }}
                onDelete={deleteMessage}
                onPin={pinMessage}
                onUnpin={unpinMessage}
                onReply={openThread}
                playingAudio={playingAudio}
                togglePlayAudio={togglePlayAudio}
                formatTime={formatTime}
                highlightMentions={(content) => highlightMentions(content, allUsers)}
                messagesEndRef={messagesEndRef}
                typingArray={Array.from(typingUsers.values())}
              />

              <MessageInput
                newMessage={newMessage}
                setNewMessage={setNewMessage}
                onKeyDown={handleInputKeyDown}
                onFileUpload={handleFileUpload}
                isUploading={isUploading}
                isRecording={isRecording}
                startRecording={startRecording}
                stopRecording={stopRecording}
                sendTypingIndicator={sendTypingIndicator}
                onSend={() => {
                  if (newMessage.trim()) {
                    const content = newMessage.trim()
                    const tempId = `temp-${Date.now()}`
                    const optimisticMsg: Message = { id: 0, channel_id: selectedChannel!.id, user_id: user!.id, content, tempId, created_at: new Date().toISOString(), user: user! as any }
                    setMessages(prev => [...prev, optimisticMsg])
                    setNewMessage('')
                    sendMessage(undefined, tempId, content)
                  }
                }}
              />

              {showMentionDropdown && (
                <div className={styles['mention-dropdown']}>
                  {mentionFilterUsers.map(u => (
                    <div key={u.id} className={styles['mention-option']} onClick={() => {
                      const lastAt = newMessage.lastIndexOf('@')
                      setNewMessage(newMessage.slice(0, lastAt + 1) + u.name + ' ')
                      setShowMentionDropdown(false)
                    }}>
                      <span 
                        className={styles['mention-user-avatar']}
                        style={{ background: getUserColor(u.name) }}
                      >
                        {u.name.charAt(0).toUpperCase()}
                      </span>
                      <span className={styles['mention-user-name']}>{u.name}</span>
                    </div>
                  ))}
                </div>
              )}
            </>
          ) : (
            <div className={styles['no-channel-selected']}>
              <div className={styles['no-channel-icon']}>#</div>
              <h2>Selecciona un canal</h2>
              <p>Elige un canal de la lista para empezar a chatear</p>
              <button onClick={() => setShowNewChannelModal(true)} className={styles['create-first-channel']}>+ Crear tu primer canal</button>
            </div>
          )}
        </div>
      </div>

      {showNewChannelModal && (
        <NewChannelModal
          newChannel={newChannel}
          setNewChannel={setNewChannel}
          onClose={() => setShowNewChannelModal(false)}
          onCreate={async () => {
            if (!newChannel.name.trim()) return
            try { await channelService.createChannel(newChannel); setShowNewChannelModal(false); setNewChannel({ name: '', description: '', type: 'public' }); fetchChannels() } catch (e) { console.error(e) }
          }}
        />
      )}

      {showNewDmModal && (
        <NewDmModal
          allUsers={allUsers}
          onSelectUser={async (recipientId) => {
            try {
              const dm = await channelService.createDM(recipientId)
              await fetchChannels()
              setSelectedChannel(dm as any)
              setShowNewDmModal(false)
            } catch (e) {
              console.error('Error creating DM:', e)
            }
          }}
          onClose={() => setShowNewDmModal(false)}
          currentUser={user as any}
        />
      )}

      {showChannelSettings && selectedChannel && (
        <ChannelSettingsModal
          selectedChannel={selectedChannel}
          channelMembers={channelMembers}
          currentUser={user as any}
          onClose={() => setShowChannelSettings(false)}
          onRemoveMember={removeMember}
          onLeaveChannel={leaveChannel}
          onShowAddMembers={() => setShowAddMembers(true)}
          onUpdateChannel={handleUpdateChannel}
        />
      )}

      {showAddMembers && (
        <AddMembersModal
          allUsers={allUsers}
          isMember={(id) => channelMembers.some(m => m.id === id)}
          onAddMember={addMember}
          onClose={() => setShowAddMembers(false)}
          currentUser={user as any}
        />
      )}

      {showPinnedMessages && (
        <PinnedMessagesModal
          pinnedMessages={pinnedMessages}
          currentUser={user as any}
          onUnpin={unpinMessage}
          onClose={() => setShowPinnedMessages(false)}
          formatTime={formatTime}
        />
      )}

      <ThreadPanel
        showThread={showThread}
        threadReplies={threadReplies}
        onClose={() => { setShowThread(null); setThreadReplies([]) }}
        onSendReply={sendThreadReply}
        formatTime={formatTime}
      />
    </div>
  )
}
