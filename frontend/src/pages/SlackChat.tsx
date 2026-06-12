import { useState, useEffect, useRef } from 'react'
import { useSearchParams } from 'react-router-dom'
import { useAuth } from '../context/AuthContext'
import { channelService, uploadService, adminService } from '../services/api'
import { Select } from '../components/ui/Select'
import type { User } from '../types'
import type { Message } from '../types/chat'
import { useConfirm } from '../components/ui/ConfirmProvider'
import { useNotification } from '../context/NotificationContext'
import { Sidebar } from '../components/Chat/Sidebar'
import { ChatHeader } from '../components/Chat/ChatHeader'
import { MessageList } from '../components/Chat/MessageList'
import { MessageInput } from '../components/Chat/MessageInput'
import { canEditModule } from '../lib/permissions'
import { ThreadPanel } from '../components/Chat/ThreadPanel'
import { NewChannelModal } from '../components/Chat/Modals/NewChannelModal'
import { ChannelSettingsModal } from '../components/Chat/Modals/ChannelSettingsModal'
import { AddMembersModal } from '../components/Chat/Modals/AddMembersModal'
import { NewDmModal } from '../components/Chat/Modals/NewDmModal'
import { PinnedMessagesModal } from '../components/Chat/Modals/PinnedMessagesModal'
import { SearchModal } from '../components/Chat/Modals/SearchModal'
import { StarredMessagesModal } from '../components/Chat/Modals/StarredMessagesModal'
import { useSlackChat } from '../hooks/useSlackChat'
import { formatTime, highlightMentions, getUserColor } from '../components/Chat/ChatUtils'
import styles from './SlackChat.module.css'

interface CompanyOption { id: number; company_name: string }

export default function SlackChat() {
  const { user } = useAuth()
  const confirm = useConfirm()
  const { error: showError } = useNotification()
  const [searchParams, setSearchParams] = useSearchParams()
  const userIdParam = searchParams.get('userId')

  const isSuperadmin = !!user?.is_superadmin
  // Permiso del rol sobre el módulo Chat: con "Ver" el chat queda de solo
  // lectura (el backend igual bloquea los envíos con 403).
  const canEditChat = canEditModule(user, 'chat')

  // Superadmin scope: pick a company so channels/DMs from different tenants never mix.
  const [companies, setCompanies] = useState<CompanyOption[]>([])
  const [selectedCompanyId, setSelectedCompanyIdState] = useState<number | null>(() => {
    const stored = localStorage.getItem('preferred_company_id')
    return stored ? Number(stored) : null
  })
  const setSelectedCompanyId = (id: number | null) => {
    setSelectedCompanyIdState(id)
    if (id) localStorage.setItem('preferred_company_id', String(id))
    else localStorage.removeItem('preferred_company_id')
  }

  useEffect(() => {
    if (!isSuperadmin) return
    let active = true
    adminService.getTenants()
      .then((res: any) => {
        if (!active) return
        setCompanies((res || []).map((t: any) => ({
          id: t.id,
          company_name: t.company_name || t.owner_name || `Empresa ${t.id}`,
        })))
      })
      .catch((err) => console.error('Error fetching companies:', err))
    return () => { active = false }
  }, [isSuperadmin])

  const {
    channels, selectedChannel, setSelectedChannel,
    messages, setMessages, pinnedMessages,
    hasMoreMessages, loadingOlder, loadOlderMessages,
    newMessage, setNewMessage,
    channelMembers, fetchChannelMembers,
    allUsers, connectionLost, unreadCount,
    typingUsers, isRecording, isPaused, recordingStream, recordedBlob,
    pauseRecording, resumeRecording, cancelRecording, discardRecording, sendRecording,
    isUploading, setIsUploading,
    showThread, setShowThread, threadReplies, setThreadReplies,
    sendMessage, sendTypingIndicator, startRecording, stopRecording,
    editMessage, deleteMessage, pinMessage, unpinMessage, fetchChannels, fetchAllUsers,
    toggleReaction, starMessage, unstarMessage, starredMessages, starredIds, fetchStarredMessages,
    userStatuses
  } = useSlackChat(user as any, isSuperadmin ? selectedCompanyId : null)

  const [showNewChannelModal, setShowNewChannelModal] = useState(false)
  const [showChannelSettings, setShowChannelSettings] = useState(false)
  const [showAddMembers, setShowAddMembers] = useState(false)
  const [showNewDmModal, setShowNewDmModal] = useState(false)
  const [showPinnedMessages, setShowPinnedMessages] = useState(false)
  const [showSearchModal, setShowSearchModal] = useState(false)
  const [searchQuery, setSearchQuery] = useState('')
  const [searchResults, setSearchResults] = useState<Message[]>([])
  const [showStarredModal, setShowStarredModal] = useState(false)
  const [showMobileChannels, setShowMobileChannels] = useState(false)
  const [chatSidebarCollapsed, setChatSidebarCollapsed] = useState(false)
  const [chatSidebarWidth, setChatSidebarWidth] = useState(220)
  const [isResizing, setIsResizing] = useState(false)
  const [playingAudio, setPlayingAudio] = useState<string | null>(null)
  const messagesEndRef = useRef<HTMLDivElement>(null)
  const audioRef = useRef<HTMLAudioElement | null>(null)
  const [newChannel, setNewChannel] = useState({ name: '', description: '', type: 'public' as 'public' | 'private', member_ids: [] as number[] })
  const [editingMessageId, setEditingMessageId] = useState<number | null>(null)
  const [editContent, setEditContent] = useState('')
  const [showMentionDropdown, setShowMentionDropdown] = useState(false)
  const [mentionFilterUsers, setMentionFilterUsers] = useState<User[]>([])
  const [mentionDismissedAt, setMentionDismissedAt] = useState<number | null>(null)
  const [originalFavicon, setOriginalFavicon] = useState<string | null>(null)
  const [highlightMessageId, setHighlightMessageId] = useState<number | null>(null)

  useEffect(() => {
    const icon = document.querySelector("link[rel*='icon']") as HTMLLinkElement
    if (icon) setOriginalFavicon(icon.href)
  }, [])

  // Deep link from notifications: /chat?channel=<id>&message=<id> opens the
  // channel and scrolls to the mentioned message.
  useEffect(() => {
    const channelParam = searchParams.get('channel')
    if (!channelParam || channels.length === 0) return
    const target = channels.find(c => c.id === parseInt(channelParam))
    if (!target) return
    const messageParam = searchParams.get('message')
    if (messageParam) setHighlightMessageId(parseInt(messageParam))
    setSelectedChannel(target)
    setSearchParams({}, { replace: true })
  }, [searchParams, channels, setSelectedChannel, setSearchParams])

  useEffect(() => {
    if (!highlightMessageId || messages.length === 0) return
    const el = document.getElementById(`message-${highlightMessageId}`)
    if (!el) return
    el.scrollIntoView({ behavior: 'smooth', block: 'center' })
    const timeout = setTimeout(() => setHighlightMessageId(null), 3000)
    return () => clearTimeout(timeout)
  }, [highlightMessageId, messages])

  useEffect(() => {
    if (userIdParam && allUsers.length > 0) {
      const recipientId = parseInt(userIdParam)
      if (recipientId && recipientId !== user?.id) {
        // Try to find if a DM with this user already exists in channels
        const existingDm = channels.find(c => 
          c.type === 'direct' && c.recipient?.id === recipientId
        )
        if (existingDm) {
          setSelectedChannel(existingDm)
          setSearchParams({}, { replace: true })
        } else {
          // Check if the recipient exists in allUsers
          const recipientExists = allUsers.some(u => u.id === recipientId)
          if (recipientExists) {
            channelService.createDM(recipientId, isSuperadmin ? selectedCompanyId : null).then(async (dm) => {
              await fetchChannels()
              setSelectedChannel(dm as any)
              setSearchParams({}, { replace: true })
            }).catch(e => console.error(e))
          }
        }
      }
    }
  }, [userIdParam, allUsers, channels, user?.id, setSearchParams, setSelectedChannel, fetchChannels])

  // Scroll to bottom only when the newest message changes (sending/receiving),
  // not when older history is prepended by the pagination.
  const lastMessageKeyRef = useRef<string | null>(null)
  useEffect(() => {
    const last = messages[messages.length - 1]
    const key = last ? `${last.id}-${last.tempId ?? ''}` : null
    if (key !== lastMessageKeyRef.current) {
      lastMessageKeyRef.current = key
      scrollToBottom()
    }
  }, [messages])

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

  // Accent/case-insensitive comparison for mention matching ("mend" matches "Méndez").
  const normalizeText = (s: string) => s.normalize('NFD').replace(/[̀-ͯ]/g, '').toLowerCase()

  // Live mention suggestions: filter as the user types after the last "@".
  useEffect(() => {
    const lastAt = newMessage.lastIndexOf('@')
    if (lastAt === -1 || mentionDismissedAt === lastAt) {
      setShowMentionDropdown(false)
      return
    }
    const query = normalizeText(newMessage.slice(lastAt + 1))
    if (query.length > 30) {
      setShowMentionDropdown(false)
      return
    }
    const startsWith: User[] = []
    const contains: User[] = []
    for (const u of allUsers) {
      if (u.id === user?.id) continue
      const name = normalizeText(u.name || '')
      if (name.startsWith(query)) startsWith.push(u)
      else if (query && name.includes(query)) contains.push(u)
    }
    const matches = [...startsWith, ...contains]
    setMentionFilterUsers(matches)
    setShowMentionDropdown(matches.length > 0)
  }, [newMessage, allUsers, mentionDismissedAt, user?.id])

  const insertMention = (u: User) => {
    const lastAt = newMessage.lastIndexOf('@')
    setNewMessage(newMessage.slice(0, lastAt + 1) + u.name + ' ')
    setShowMentionDropdown(false)
  }

  const handleInputKeyDown = (e: React.KeyboardEvent) => {
    if (showMentionDropdown && mentionFilterUsers.length > 0 && (e.key === 'Enter' || e.key === 'Tab') && !e.shiftKey) {
      e.preventDefault()
      insertMention(mentionFilterUsers[0])
      return
    }
    if (e.key === 'Escape' && showMentionDropdown) {
      setMentionDismissedAt(newMessage.lastIndexOf('@'))
      setShowMentionDropdown(false)
      return
    }
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      if (newMessage.trim()) {
        const tempId = `temp-${Date.now()}`
        const optimisticMsg: Message = { id: 0, channel_id: selectedChannel!.id, user_id: user!.id, content: newMessage, tempId, created_at: new Date().toISOString(), user: user! as any }
        setMessages(prev => [...prev, optimisticMsg])
        sendMessage(undefined, tempId)
      }
    }
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

  const leaveChannel = async (id: number) => {
    const ok = await confirm({
      title: 'Salir del canal',
      message: '¿Seguro que quieres salir de este canal? Dejarás de ver sus mensajes.',
      confirmLabel: 'Salir',
      variant: 'danger',
    })
    if (!ok) return
    try {
      await channelService.leaveChannel(id)
      if (selectedChannel?.id === id) setSelectedChannel(null)
      fetchChannels()
    } catch (e) {
      console.error(e)
      showError('No se pudo salir del canal. Intenta de nuevo.')
    }
  }
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

  const handleSearch = async () => {
    if (!selectedChannel || !searchQuery.trim()) return
    try {
      const results = await channelService.searchMessages(selectedChannel.id, searchQuery.trim())
      setSearchResults(results || [])
    } catch (e) {
      console.error('Error searching messages:', e)
      showError('No se pudo realizar la búsqueda.')
    }
  }

  // Public channels are visible without explicit membership, but unread tracking
  // and the member list need it — offer joining before participating.
  const isMemberOfSelected = !!user && channelMembers.some(m => m.id === user.id)
  const needsJoin = selectedChannel?.type === 'public' && channelMembers.length > 0 && !isMemberOfSelected

  const handleJoinChannel = async () => {
    if (!selectedChannel) return
    try {
      await channelService.joinChannel(selectedChannel.id)
      fetchChannelMembers(selectedChannel.id)
      fetchChannels()
    } catch (e) {
      console.error('Error joining channel:', e)
      showError('No se pudo unir al canal.')
    }
  }

  const handleSendGif = (url: string, title?: string) => {
    if (!selectedChannel || !user) return
    const tempId = `temp-${Date.now()}`
    const optimisticMsg: Message = { id: 0, channel_id: selectedChannel.id, user_id: user.id, content: `[Archivo: ${title || 'GIF'}]`, tempId, attachment: url, file_name: title || 'GIF', file_type: 'image/gif', created_at: new Date().toISOString(), user: user as any }
    setMessages(prev => [...prev, optimisticMsg])
    sendMessage({ url, filename: title || 'GIF' }, tempId, '')
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
      {connectionLost && (
        <div className={styles['connection-banner']}>
          ⚠️ Conexión perdida — reconectando...
        </div>
      )}
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
        onShowSearch={() => { setSearchQuery(''); setSearchResults([]); setShowSearchModal(true) }}
        onShowStarred={() => { fetchStarredMessages(); setShowStarredModal(true) }}
        recipientStatus={selectedChannel?.type === 'direct' && selectedChannel.recipient
          ? (userStatuses.get(selectedChannel.recipient.id) || 'offline')
          : undefined}
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
          userStatuses={userStatuses}
          headerExtra={isSuperadmin ? (
            <Select
              value={selectedCompanyId ?? ''}
              onChange={(v) => setSelectedCompanyId(v ? Number(v) : null)}
              clearable
              placeholder="Seleccione una empresa..."
              options={companies.map(c => ({ value: c.id, label: c.company_name }))}
            />
          ) : undefined}
        />

        <div className={styles['chat-main']}>
          {isSuperadmin && !selectedCompanyId ? (
            <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', height: '100%', textAlign: 'center', color: '#64748b', padding: '24px' }}>
              <h2 style={{ fontSize: '20px', fontWeight: 700, color: 'var(--black)', marginBottom: '8px' }}>Selecciona una empresa</h2>
              <p style={{ maxWidth: '380px' }}>
                Elige una empresa para ver sus canales y mensajes directos. La información de cada empresa se mantiene aislada.
              </p>
            </div>
          ) : selectedChannel ? (
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
                onToggleReaction={toggleReaction}
                starredIds={starredIds}
                onStar={starMessage}
                onUnstar={unstarMessage}
                playingAudio={playingAudio}
                togglePlayAudio={togglePlayAudio}
                formatTime={formatTime}
                highlightMentions={(content) => highlightMentions(content, allUsers, user?.name)}
                messagesEndRef={messagesEndRef}
                typingArray={Array.from(typingUsers.values())}
                highlightedMessageId={highlightMessageId}
                hasMoreMessages={hasMoreMessages}
                loadingOlder={loadingOlder}
                onLoadOlder={loadOlderMessages}
              />

              {!canEditChat ? (
                <div className={styles['join-banner']}>
                  <p>Tu rol tiene acceso de solo lectura en el Chat.</p>
                </div>
              ) : needsJoin ? (
                <div className={styles['join-banner']}>
                  <p>Estás viendo <b>#{selectedChannel.name}</b>. Únete al canal para participar y recibir notificaciones.</p>
                  <button onClick={handleJoinChannel}>Unirse al canal</button>
                </div>
              ) : (
                <MessageInput
                  newMessage={newMessage}
                  setNewMessage={setNewMessage}
                  onKeyDown={handleInputKeyDown}
                  onFileUpload={handleFileUpload}
                  isUploading={isUploading}
                  isRecording={isRecording}
                  isPaused={isPaused}
                  recordingStream={recordingStream}
                  recordedBlob={recordedBlob}
                  startRecording={startRecording}
                  stopRecording={stopRecording}
                  pauseRecording={pauseRecording}
                  resumeRecording={resumeRecording}
                  cancelRecording={cancelRecording}
                  discardRecording={discardRecording}
                  sendRecording={sendRecording}
                  sendTypingIndicator={sendTypingIndicator}
                  onSendGif={handleSendGif}
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
              )}

              {showMentionDropdown && (
                <div className={styles['mention-dropdown']}>
                  {mentionFilterUsers.map((u, i) => (
                    <div key={u.id} className={`${styles['mention-option']} ${i === 0 ? styles['active'] : ''}`} onClick={() => insertMention(u)}>
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
              {channels.length > 0 ? (
                <>
                  <h2>Selecciona un canal</h2>
                  <p>Elige un canal de la lista para empezar a chatear</p>
                </>
              ) : (
                <>
                  <h2>Aún no hay canales</h2>
                  <p>Crea el primer canal de tu empresa para empezar a chatear</p>
                  <button onClick={() => setShowNewChannelModal(true)} className={styles['create-first-channel']}>+ Crear tu primer canal</button>
                </>
              )}
            </div>
          )}
        </div>
      </div>

      {showNewChannelModal && (
        <NewChannelModal
          newChannel={newChannel}
          setNewChannel={setNewChannel}
          allUsers={allUsers}
          currentUser={user as any}
          onClose={() => setShowNewChannelModal(false)}
          onCreate={async () => {
            if (!newChannel.name.trim()) return
            try { await channelService.createChannel(newChannel, isSuperadmin ? selectedCompanyId : null); setShowNewChannelModal(false); setNewChannel({ name: '', description: '', type: 'public', member_ids: [] }); fetchChannels() } catch (e) { console.error(e); showError('No se pudo crear el canal.') }
          }}
        />
      )}

      {showNewDmModal && (
        <NewDmModal
          allUsers={allUsers}
          onSelectUser={async (recipientId) => {
            try {
              const dm = await channelService.createDM(recipientId, isSuperadmin ? selectedCompanyId : null)
              await fetchChannels()
              setSelectedChannel(dm as any)
              setShowNewDmModal(false)
            } catch (e) {
              console.error('Error creating DM:', e)
              showError('No se pudo iniciar la conversación.')
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

      {showSearchModal && selectedChannel && (
        <SearchModal
          searchQuery={searchQuery}
          setSearchQuery={setSearchQuery}
          searchResults={searchResults}
          onSearch={handleSearch}
          onClose={() => { setShowSearchModal(false); setSearchQuery(''); setSearchResults([]) }}
          formatTime={formatTime}
        />
      )}

      {showStarredModal && (
        <StarredMessagesModal
          starredMessages={starredMessages}
          onUnstar={unstarMessage}
          onClose={() => setShowStarredModal(false)}
          formatTime={formatTime}
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
