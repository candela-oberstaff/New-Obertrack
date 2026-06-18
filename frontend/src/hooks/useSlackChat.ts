import { useState, useEffect, useRef, useCallback, useMemo } from 'react'
import { channelService, uploadService } from '../services/api'
import type { User } from '../types'
import type { Channel, Message, ChannelMember, MessageReaction, UserStatus } from '../types/chat'
import { playNotificationSound, newTempId } from '../components/Chat/ChatUtils'
import { useConfirm } from '../components/ui/ConfirmProvider'
import { useNotification } from '../context/NotificationContext'

export function useSlackChat(user: User | null, companyId: number | null = null) {
  const [channels, setChannels] = useState<Channel[]>([])
  const [selectedChannel, setSelectedChannel] = useState<Channel | null>(null)
  const [messages, setMessages] = useState<Message[]>([])
  const [hasMoreMessages, setHasMoreMessages] = useState(false)
  const [loadingOlder, setLoadingOlder] = useState(false)
  const [pinnedMessages, setPinnedMessages] = useState<Message[]>([])
  const [newMessage, setNewMessage] = useState('')
  const [channelMembers, setChannelMembers] = useState<ChannelMember[]>([])
  const [allUsers, setAllUsers] = useState<User[]>([])
  const [isConnected, setIsConnected] = useState(false)
  const [typingUsers, setTypingUsers] = useState<Map<number, string>>(new Map())
  const [isRecording, setIsRecording] = useState(false)
  const [isPaused, setIsPaused] = useState(false)
  const [recordingStream, setRecordingStream] = useState<MediaStream | null>(null)
  const [recordedBlob, setRecordedBlob] = useState<Blob | null>(null)
  const [isUploading, setIsUploading] = useState(false)
  const [showThread, setShowThread] = useState<Message | null>(null)
  const [threadReplies, setThreadReplies] = useState<Message[]>([])
  const [starredMessages, setStarredMessages] = useState<Message[]>([])
  const [starredIds, setStarredIds] = useState<Set<number>>(new Set())
  const [userStatuses, setUserStatuses] = useState<Map<number, UserStatus['status']>>(new Map())
  
  const confirm = useConfirm()
  const { error: showError } = useNotification()

  const wsRef = useRef<WebSocket | null>(null)
  const reconnectAttemptsRef = useRef(0)
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const closedByUsRef = useRef(false)
  const [connectionLost, setConnectionLost] = useState(false)
  const typingTimeoutRef = useRef<Map<number, ReturnType<typeof setTimeout>>>(new Map())
  const mediaRecorderRef = useRef<MediaRecorder | null>(null)
  const audioChunksRef = useRef<Blob[]>([])
  const recordingCancelledRef = useRef(false)
  const selectedChannelIdRef = useRef<number | null>(null)
  const userIdRef = useRef<number | null>(null)
  const messagesRef = useRef<Message[]>([])
  const threadRepliesRef = useRef<Message[]>([])
  // companyId lives in a ref so the WS socket closure can read the current
  // scope without `companyId` being in connectWebSocket's deps (which would
  // recreate the socket on every company switch — see A-2).
  const companyIdRef = useRef<number | null>(companyId)
  // Timer that promotes the connection to "stable" ~10s after onopen; only then
  // do we reset the backoff counter, so a chronically-dropped client keeps
  // escalating its backoff instead of hammering the API every ~1s (reconnect-loop fix).
  const stableTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  // Debounce guard for A-4.3: when messages arrive for channels not yet in the
  // list, coalesce the catch-up fetchChannels() into a single call instead of
  // one per message (avoids N simultaneous fetches and any refetch loop).
  const channelsRefetchTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  // Keep the company ref current. This runs before the connection effect's
  // onopen can fire for a new socket, so the re-sync reads the right scope.
  useEffect(() => {
    companyIdRef.current = companyId
  }, [companyId])

  // Global unread badge is DERIVED from the per-channel server counts (single
  // source of truth) instead of an independent counter — removes the reconnect
  // double-count/loss race (A-4).
  const unreadCount = useMemo(
    () => channels.reduce((sum, c) => sum + (c.unread_count || 0), 0),
    [channels]
  )

  useEffect(() => {
    messagesRef.current = messages
  }, [messages])

  useEffect(() => {
    threadRepliesRef.current = threadReplies
  }, [threadReplies])

  useEffect(() => {
    selectedChannelIdRef.current = selectedChannel?.id ?? null
  }, [selectedChannel?.id])

  useEffect(() => {
    userIdRef.current = user?.id ?? null
  }, [user?.id])

  // Reads companyId from the ref so the callback is stable (deps []), letting
  // the WS socket call it on re-sync without depending on companyId.
  const fetchChannels = useCallback(async () => {
    try {
      const data = await channelService.getChannels(companyIdRef.current)
      setChannels(data || [])
    } catch (error) {
      console.error('Error fetching channels:', error)
    }
  }, [])

  // A-4.3: debounced fetchChannels. Only triggered when an incoming message
  // references a channel not present in the current list, and coalesces a burst
  // into one fetch (~300ms). The guard (timer already pending) prevents both a
  // refetch loop and N simultaneous fetches. Stable (deps []) — reads
  // fetchChannels via ref so it doesn't churn.
  const fetchChannelsRef = useRef(fetchChannels)
  useEffect(() => { fetchChannelsRef.current = fetchChannels }, [fetchChannels])
  const refetchChannelsSoon = useCallback(() => {
    if (channelsRefetchTimerRef.current) return
    channelsRefetchTimerRef.current = setTimeout(() => {
      channelsRefetchTimerRef.current = null
      fetchChannelsRef.current()
    }, 300)
  }, [])

  const fetchAllUsers = useCallback(async () => {
    try {
      const data = await channelService.getAllUsers(companyIdRef.current)
      setAllUsers(data || [])
    } catch (error) {
      console.error('Error fetching users:', error)
    }
  }, [])

  const PAGE_SIZE = 100

  const fetchMessages = useCallback(async (channelId: number) => {
    try {
      const data = await channelService.getMessages(channelId)
      setMessages(data || [])
      setHasMoreMessages((data || []).length >= PAGE_SIZE)
    } catch (error) {
      console.error('Error fetching messages:', error)
    }
  }, [])

  // Page into older history: fetch messages before the oldest one on screen.
  const loadOlderMessages = useCallback(async () => {
    const channelId = selectedChannelIdRef.current
    if (!channelId || loadingOlder) return
    setLoadingOlder(true)
    try {
      const oldestId = messagesRef.current.find(m => m.id > 0)?.id ?? 0
      // No real message on screen yet — nothing to page before. The `finally`
      // below still resets loadingOlder; hasMoreMessages is left untouched (it
      // reflects the last fetchMessages result), so no inconsistent state.
      if (!oldestId) return
      const older = await channelService.getMessages(channelId, oldestId)
      if (selectedChannelIdRef.current !== channelId) return
      const batch = older || []
      // M-1: client-side robustness without a backend has_more flag.
      // - Empty batch: there is nothing older. Turn off "load more" and prepend
      //   nothing (avoids an extra empty network round-trip on the next click).
      // - Non-empty batch: keep paging only when the server filled a full page;
      //   a partial page means we've reached the start of history.
      if (batch.length === 0) {
        setHasMoreMessages(false)
        return
      }
      setMessages(prev => [...batch, ...prev])
      setHasMoreMessages(batch.length >= PAGE_SIZE)
    } catch (error) {
      console.error('Error loading older messages:', error)
    } finally {
      setLoadingOlder(false)
    }
  }, [loadingOlder])

  const fetchPinnedMessages = useCallback(async (channelId: number) => {
    try {
      const data = await channelService.getPinnedMessages(channelId)
      setPinnedMessages(data || [])
    } catch (error) {
      console.error('Error fetching pinned messages:', error)
    }
  }, [])

  const fetchChannelMembers = useCallback(async (channelId: number) => {
    try {
      const data = await channelService.getMembers(channelId)
      setChannelMembers(data || [])
    } catch (error) {
      console.error('Error fetching members:', error)
    }
  }, [])

  const fetchStarredMessages = useCallback(async () => {
    try {
      const data = await channelService.getStarredMessages()
      setStarredMessages(data || [])
      setStarredIds(new Set((data || []).map(m => m.id)))
    } catch (error) {
      console.error('Error fetching starred messages:', error)
    }
  }, [])

  // Apply a reaction change to EVERY list that may hold a copy of the message:
  // the main list, the thread panel, and the pinned/starred modals (B-2). These
  // lists are small, so mapping each on every reaction WS event is cheap, and it
  // keeps the modals live instead of stale-until-reopen. (searchResults lives in
  // SlackChat.tsx and is ephemeral — reset on open/close and re-fetched per
  // search — so it's intentionally not synced here; reopening refetches it.)
  const applyReactionAdded = useCallback((messageId: number, reaction: MessageReaction) => {
    const update = (list: Message[]) => list.map(m => {
      if (m.id !== messageId) return m
      const exists = (m.reactions || []).some(r => r.user_id === reaction.user_id && r.emoji === reaction.emoji)
      return exists ? m : { ...m, reactions: [...(m.reactions || []), reaction] }
    })
    setMessages(update)
    setThreadReplies(update)
    setPinnedMessages(update)
    setStarredMessages(update)
  }, [])

  const applyReactionRemoved = useCallback((messageId: number, userId: number, emoji: string) => {
    const update = (list: Message[]) => list.map(m =>
      m.id !== messageId
        ? m
        : { ...m, reactions: (m.reactions || []).filter(r => !(r.user_id === userId && r.emoji === emoji)) }
    )
    setMessages(update)
    setThreadReplies(update)
    setPinnedMessages(update)
    setStarredMessages(update)
  }, [])

  const handleTyping = useCallback((userId: number, userName: string, channelId: number) => {
    if (channelId !== selectedChannelIdRef.current || userId === userIdRef.current) return
    setTypingUsers(prev => new Map(prev).set(userId, userName))
    if (typingTimeoutRef.current.has(userId)) clearTimeout(typingTimeoutRef.current.get(userId)!)
    const timeout = setTimeout(() => {
      setTypingUsers(prev => {
        const newMap = new Map(prev); newMap.delete(userId); return newMap
      })
    }, 3000)
    typingTimeoutRef.current.set(userId, timeout)
  }, [])

  const connectWebSocket = useCallback(() => {
    if (wsRef.current && wsRef.current.readyState !== WebSocket.CLOSED) return
    closedByUsRef.current = false

    // Auth travels via the httpOnly cookie on the same-origin WS handshake.
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const wsUrl = `${protocol}//${window.location.host}/ws/channels`
    const ws = new WebSocket(wsUrl)

    ws.onopen = () => {
      setIsConnected(true)
      setConnectionLost(false)
      if (reconnectAttemptsRef.current > 0) {
        // Recovered after a drop: re-sync whatever we missed while offline.
        // fetchChannels reads the current companyId from companyIdRef and
        // setChannels feeds the derived unread badge (A-4), so the badge is
        // refreshed from the server's per-channel counts automatically.
        fetchChannels()
        const channelId = selectedChannelIdRef.current
        if (channelId) {
          fetchMessages(channelId)
          fetchPinnedMessages(channelId)
        }
      }
      // Reconnect-loop fix: do NOT reset the backoff counter immediately. A
      // chronically-slow client that the server keeps dropping would otherwise
      // reconnect every ~1s with a full HTTP re-sync. Only reset after the
      // connection has stayed stable for ~10s.
      if (stableTimerRef.current) clearTimeout(stableTimerRef.current)
      stableTimerRef.current = setTimeout(() => {
        reconnectAttemptsRef.current = 0
        stableTimerRef.current = null
      }, 10000)
    }
    ws.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data)
        const selectedChannelId = selectedChannelIdRef.current
        const currentUserId = userIdRef.current

        if (selectedChannelId !== null && msg.channel_id === selectedChannelId) {
          if (msg.type === 'message') {
            // Always add the message (fixes multi-tab: a second tab of the sender
            // must see its own message). Dedup by real id, and drop the optimistic
            // placeholder matching the echoed temp_id so we never duplicate.
            setMessages(prev => {
              // id wins: if the real message is already here (e.g. sendMessage's
              // own .then already appended it, tempId preserved), don't dup it.
              if (prev.some(m => m.id === msg.data.id)) return prev
              // M-3: when this echo reconciles our own optimistic message, carry
              // the optimistic tempId onto the real one so the React key
              // (tempId ?? id) stays stable and MessageItem isn't remounted.
              const optimistic = msg.temp_id ? prev.find(m => m.tempId === msg.temp_id) : undefined
              const filtered = msg.temp_id ? prev.filter(m => m.tempId !== msg.temp_id) : prev
              const reconciled = optimistic ? { ...msg.data, tempId: optimistic.tempId } : msg.data
              return [...filtered, reconciled]
            })
            // Only notify/mark-read for messages from OTHER users; the sender's own
            // message must not ring or mark the channel read.
            if (msg.user_id !== currentUserId) {
              playNotificationSound()
              // A-3: only auto-mark-read if the tab is actually visible. If the
              // document is hidden the user hasn't seen the message, so leave it
              // counted as unread (bump this channel's count); the visibilitychange
              // listener will mark it read when the tab regains focus.
              if (!document.hidden) {
                // Don't refetch the whole channel list per incoming message; the
                // local setChannels below already zeroes the active channel's badge.
                channelService.markAsRead(selectedChannelId).then(() => {
                  window.dispatchEvent(new CustomEvent('chat-unread-updated'))
                })
                setChannels(prev => prev.map(c => c.id === selectedChannelId ? { ...c, unread_count: 0 } : c))
              } else {
                setChannels(prev => prev.map(c => c.id === selectedChannelId ? { ...c, unread_count: (c.unread_count || 0) + 1 } : c))
                window.dispatchEvent(new CustomEvent('chat-unread-updated'))
              }
            }
          } else if (msg.type === 'typing') {
            handleTyping(msg.user_id, msg.user_name, msg.channel_id)
          } else if (msg.type === 'message_pinned') {
            setMessages(prev => prev.map(m => m.id === msg.data.id ? { ...m, is_pinned: true } : m))
            fetchPinnedMessages(selectedChannelId)
          } else if (msg.type === 'message_unpinned') {
            setMessages(prev => prev.map(m => m.id === msg.data.id ? { ...m, is_pinned: false } : m))
            fetchPinnedMessages(selectedChannelId)
          } else if (msg.type === 'reaction_added') {
            applyReactionAdded(msg.data.message_id, msg.data.reaction)
          } else if (msg.type === 'reaction_removed') {
            applyReactionRemoved(msg.data.message_id, msg.data.user_id, msg.data.emoji)
          } else if (msg.type === 'message_edited') {
            setMessages(prev => prev.map(m => m.id === msg.data.id ? { ...m, ...msg.data, is_edited: true } : m))
          } else if (msg.type === 'message_deleted') {
            setMessages(prev => prev.map(m => m.id === msg.data.id ? { ...m, is_deleted: true, content: '[Mensaje eliminado]' } : m))
          } else if (msg.type === 'thread_reply') {
            // Mirror the main-list 'message' handler exactly (M-3): id wins for
            // dedup, then carry the optimistic tempId onto the reconciled reply so
            // the thread key (tempId ?? id) stays stable and no transient duplicate
            // (optimistic id:0 + real) survives even if the WS echo beats the HTTP
            // response. (Backend doesn't echo temp_id for thread_reply today, so the
            // tempId branch is a no-op there, but the pattern matches the main list.)
            setThreadReplies(prev => {
              if (prev.some(r => r.id === msg.data.id)) return prev
              const optimistic = msg.temp_id ? prev.find(r => r.tempId === msg.temp_id) : undefined
              const filtered = msg.temp_id ? prev.filter(r => r.tempId !== msg.temp_id) : prev
              const reconciled = optimistic ? { ...msg.data, tempId: optimistic.tempId } : msg.data
              return [...filtered, reconciled]
            })
            if (msg.data.parent_id) {
              setMessages(prev => prev.map(m =>
                m.id === msg.data.parent_id
                  ? { ...m, reply_count: (m.reply_count || 0) + 1 }
                  : m
              ))
            }
          }
        } else if (msg.type === 'message' && msg.user_id !== currentUserId) {
          // A-4: bump only THIS channel's per-channel count; the global badge is
          // a useMemo over channels, so it re-derives automatically.
          playNotificationSound()
          setChannels(prev => {
            // A-4.3: if the channel isn't in the list yet (new incoming DM or a
            // channel added since the last fetch) the .map() can't bump it and the
            // increment would be lost. Detect the no-match here and schedule a
            // fetchChannels() to pull the new channel WITH its server unread_count,
            // so it appears in the sidebar/badge. The fetch is guarded/debounced
            // (see refetchChannelsSoon) so a burst of messages from several new
            // channels doesn't fire N simultaneous fetches.
            const exists = prev.some(c => c.id === msg.channel_id)
            if (!exists) {
              refetchChannelsSoon()
              return prev
            }
            return prev.map(c => c.id === msg.channel_id ? { ...c, unread_count: (c.unread_count || 0) + 1 } : c)
          })
          window.dispatchEvent(new CustomEvent('chat-unread-updated'))
        }
      } catch (e) { console.error('Error parsing message:', e) }
    }
    ws.onclose = () => {
      setIsConnected(false)
      // Cancel the pending "stable" promotion so a connection that drops before
      // the 10s mark doesn't reset the backoff counter (reconnect-loop fix).
      if (stableTimerRef.current) { clearTimeout(stableTimerRef.current); stableTimerRef.current = null }
      if (closedByUsRef.current) return
      // Reconnect with exponential backoff: 1s, 2s, 4s, 8s, 16s, then 30s.
      setConnectionLost(true)
      const attempt = reconnectAttemptsRef.current++
      const delay = Math.min(30000, 1000 * 2 ** Math.min(attempt, 5))
      reconnectTimerRef.current = setTimeout(connectWebSocket, delay)
    }
    ws.onerror = () => setIsConnected(false)
    wsRef.current = ws
    // Deps are [] so this callback is stable: the socket PERSISTS across company
    // switches (A-2). companyId is read via companyIdRef; fetchChannels/
    // fetchMessages/fetchPinnedMessages/handleTyping/applyReaction* are all stable
    // (deps [] or company-independent), and connectWebSocket references itself
    // for reconnect, which is safe because it's stable.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  // A-2: When the company scope changes (superadmin), reset the open
  // conversation and reload channels/users by REST. The WS socket is NOT
  // recreated — companyIdRef (kept current above) feeds the persistent socket's
  // re-sync. fetchChannels/fetchAllUsers read companyIdRef, so they fetch the
  // new scope. (skip is unnecessary: on mount this also does the initial load.)
  useEffect(() => {
    setSelectedChannel(null)
    setMessages([])
    fetchChannels()
    fetchAllUsers()
  }, [companyId, fetchChannels, fetchAllUsers])

  // Open the persistent WS socket once on mount and tear it down on unmount.
  // The socket survives company switches (A-2) because connectWebSocket is stable.
  useEffect(() => {
    fetchStarredMessages()
    connectWebSocket()
    return () => {
      closedByUsRef.current = true
      if (reconnectTimerRef.current) clearTimeout(reconnectTimerRef.current)
      if (stableTimerRef.current) clearTimeout(stableTimerRef.current)
      if (channelsRefetchTimerRef.current) clearTimeout(channelsRefetchTimerRef.current)
      if (wsRef.current) wsRef.current.close()
    }
  }, [fetchStarredMessages, connectWebSocket])

  // Presence: report online/away/offline based on page visibility and chat lifetime.
  useEffect(() => {
    if (!user?.id) return
    channelService.updateStatus('online').catch(() => {})
    const onVisibility = () => {
      channelService.updateStatus(document.hidden ? 'away' : 'online').catch(() => {})
    }
    // Abrupt tab/window close doesn't run the effect cleanup, so visibility-based
    // updates leave the user "online" forever. Report offline best-effort during
    // unload via sendBeacon (auth is the httpOnly same-origin cookie, which the
    // beacon carries). Falls back to fetch+keepalive when beacon isn't available.
    const reportOfflineOnUnload = () => {
      const url = '/api/channels/status'
      const payload = JSON.stringify({ status: 'offline' })
      try {
        if (navigator.sendBeacon) {
          navigator.sendBeacon(url, new Blob([payload], { type: 'application/json' }))
          return
        }
      } catch { /* fall through to fetch */ }
      try {
        fetch(url, {
          method: 'POST',
          body: payload,
          headers: { 'Content-Type': 'application/json' },
          keepalive: true,
          credentials: 'include',
        }).catch(() => {})
      } catch { /* best-effort */ }
    }
    document.addEventListener('visibilitychange', onVisibility)
    window.addEventListener('pagehide', reportOfflineOnUnload)
    window.addEventListener('beforeunload', reportOfflineOnUnload)
    return () => {
      document.removeEventListener('visibilitychange', onVisibility)
      window.removeEventListener('pagehide', reportOfflineOnUnload)
      window.removeEventListener('beforeunload', reportOfflineOnUnload)
      channelService.updateStatus('offline').catch(() => {})
    }
  }, [user?.id])

  // Poll statuses of everyone visible in the chat (DM list, members, mentions).
  useEffect(() => {
    if (allUsers.length === 0) return
    const ids = allUsers.map(u => u.id)
    let active = true
    const load = async () => {
      try {
        const statuses = await channelService.getStatuses(ids)
        if (!active) return
        setUserStatuses(new Map((statuses || []).map(s => [s.user_id, s.status])))
      } catch (error) {
        console.error('Error fetching statuses:', error)
      }
    }
    load()
    const interval = setInterval(load, 30000)
    return () => { active = false; clearInterval(interval) }
  }, [allUsers])

  useEffect(() => {
    const selectedChannelId = selectedChannel?.id
    if (selectedChannelId) {
      fetchMessages(selectedChannelId)
      fetchChannelMembers(selectedChannelId)
      fetchPinnedMessages(selectedChannelId)
      channelService.markAsRead(selectedChannelId).then(() => {
        window.dispatchEvent(new CustomEvent('chat-unread-updated'))
        fetchChannels()
      })
      setChannels(prev => prev.map(c => c.id === selectedChannelId ? { ...c, unread_count: 0 } : c))
    }
  }, [selectedChannel?.id, fetchMessages, fetchChannelMembers, fetchPinnedMessages, fetchChannels])

  // A-3: when the tab regains visibility and a channel is open, mark it read.
  // While hidden, incoming messages in the active channel are left unread (see
  // onmessage), so this catches up the read state once the user is actually looking.
  // A-3.3: some browsers / split-screen setups don't emit visibilitychange when
  // the window merely regains FOCUS (document.hidden was already false, or the
  // event is suppressed). A message that arrived while hidden would then stay
  // unread even though the channel is open and visible. Bind the SAME handler to
  // window 'focus' as well, so refocusing the window also catches up the read
  // state. The handler is a no-op when document.hidden is still true, so the
  // focus path can't mark a genuinely-hidden tab as read.
  useEffect(() => {
    const markActiveChannelRead = () => {
      if (document.hidden) return
      const channelId = selectedChannelIdRef.current
      if (!channelId) return
      channelService.markAsRead(channelId).then(() => {
        setChannels(prev => prev.map(c => c.id === channelId ? { ...c, unread_count: 0 } : c))
        window.dispatchEvent(new CustomEvent('chat-unread-updated'))
      }).catch(err => console.error('Error marking as read on visibility/focus:', err))
    }
    document.addEventListener('visibilitychange', markActiveChannelRead)
    window.addEventListener('focus', markActiveChannelRead)
    return () => {
      document.removeEventListener('visibilitychange', markActiveChannelRead)
      window.removeEventListener('focus', markActiveChannelRead)
    }
  }, [])

  const sendMessage = useCallback((attachment?: { url: string; filename: string }, tempId?: string, contentOverride?: string, fileType?: string) => {
    const textToSend = contentOverride !== undefined ? contentOverride : newMessage
    if ((!textToSend.trim() && !attachment) || !selectedChannel) return
    const content = attachment ? `${textToSend} [Archivo: ${attachment.filename}]` : textToSend
    channelService.sendMessage(selectedChannel.id, { content, attachment: attachment?.url, file_name: attachment?.filename, file_type: fileType, temp_id: tempId })
      .then(msg => {
        // Dedup by both tempId (optimistic placeholder) and id (in case the WS
        // echo already appended the real message in this same tab).
        // M-3: carry the optimistic tempId onto the reconciled real message so
        // the React key (tempId ?? id) doesn't change in the optimistic->real
        // transition and MessageItem isn't remounted (keeps emoji picker / edit
        // focus). The filter still removes any prior copy of this id, so no dup.
        setMessages(prev => prev.filter(m => m.tempId !== tempId && m.id !== msg.id).concat([{ ...msg, tempId }]))
        setNewMessage('')
      }).catch(err => {
        console.error('Error sending message:', err)
        setMessages(prev => prev.filter(m => m.tempId !== tempId))
      })
  }, [selectedChannel, newMessage])

  const sendTypingIndicator = useCallback(() => {
    if (!wsRef.current || !isConnected || !selectedChannel) return
    wsRef.current.send(JSON.stringify({ type: 'typing', channel_id: selectedChannel.id, user_name: user?.name }))
  }, [isConnected, selectedChannel, user?.name])

  const startRecording = async () => {
    try {
      const stream = await navigator.mediaDevices.getUserMedia({ audio: true })
      const mediaRecorder = new MediaRecorder(stream)
      audioChunksRef.current = []
      recordingCancelledRef.current = false
      setRecordedBlob(null)
      mediaRecorder.ondataavailable = (e) => { if (e.data.size > 0) audioChunksRef.current.push(e.data) }
      // Stopping no longer sends: it produces a blob the user can preview first.
      mediaRecorder.onstop = () => {
        const audioBlob = new Blob(audioChunksRef.current, { type: 'audio/webm' })
        stream.getTracks().forEach(track => track.stop())
        setRecordingStream(null)
        setIsPaused(false)
        if (recordingCancelledRef.current) { recordingCancelledRef.current = false; return }
        setRecordedBlob(audioBlob)
      }
      mediaRecorderRef.current = mediaRecorder
      mediaRecorder.start()
      setRecordingStream(stream)
      setIsRecording(true)
      setIsPaused(false)
    } catch (e) { console.error('Error starting recording:', e) }
  }

  const pauseRecording = () => {
    if (mediaRecorderRef.current && isRecording && !isPaused) {
      mediaRecorderRef.current.pause()
      setIsPaused(true)
    }
  }

  const resumeRecording = () => {
    if (mediaRecorderRef.current && isRecording && isPaused) {
      mediaRecorderRef.current.resume()
      setIsPaused(false)
    }
  }

  const stopRecording = () => { if (mediaRecorderRef.current && isRecording) { mediaRecorderRef.current.stop(); setIsRecording(false) } }

  const cancelRecording = () => {
    if (mediaRecorderRef.current && isRecording) {
      recordingCancelledRef.current = true
      mediaRecorderRef.current.stop()
      setIsRecording(false)
    }
    setRecordedBlob(null)
  }

  const discardRecording = () => setRecordedBlob(null)

  const sendRecording = async () => {
    if (!recordedBlob || !selectedChannel || !user) return
    const blob = recordedBlob
    setRecordedBlob(null)
    try {
      setIsUploading(true)
      const result = await uploadService.upload(blob as any)
      const tempId = newTempId()
      const optimisticMsg: Message = { id: 0, channel_id: selectedChannel.id, user_id: user.id, content: '[Nota de voz]', tempId, attachment: result.url, file_name: 'Nota de voz', file_type: 'audio/webm', created_at: new Date().toISOString(), user }
      setMessages(prev => [...prev, optimisticMsg])
      sendMessage({ url: result.url, filename: 'Nota de voz' }, tempId, undefined, 'audio/webm')
    } catch (error) {
      console.error('Error uploading voice note:', error)
      showError('No se pudo enviar la nota de voz. Intenta de nuevo.')
    }
    finally { setIsUploading(false) }
  }

  const editMessage = async (id: number, content: string) => {
    if (!selectedChannel) return
    // Snapshot for rollback if the request fails.
    const prevMessages = messagesRef.current
    const prevThreadReplies = threadRepliesRef.current
    const apply = (list: Message[]) => list.map(m => m.id === id ? { ...m, content, is_edited: true } : m)
    setMessages(apply)
    setThreadReplies(apply)
    try {
      await channelService.editMessage(selectedChannel.id, id, content)
    } catch (e) {
      console.error(e)
      setMessages(prevMessages)
      setThreadReplies(prevThreadReplies)
      showError('No se pudo editar el mensaje. Intenta de nuevo.')
    }
  }

  const deleteMessage = async (id: number) => {
    if (!selectedChannel) return
    const ok = await confirm({
      title: 'Eliminar mensaje',
      message: '¿Eliminar este mensaje?',
      confirmLabel: 'Eliminar',
      variant: 'danger',
    })
    if (!ok) return
    // Snapshot for rollback if the request fails.
    const prevMessages = messagesRef.current
    const prevThreadReplies = threadRepliesRef.current
    const apply = (list: Message[]) => list.map(m => m.id === id ? { ...m, is_deleted: true, content: '[Mensaje eliminado]' } : m)
    setMessages(apply)
    setThreadReplies(apply)
    try {
      await channelService.deleteMessage(selectedChannel.id, id)
    } catch (e) {
      console.error(e)
      setMessages(prevMessages)
      setThreadReplies(prevThreadReplies)
      showError('No se pudo eliminar el mensaje. Intenta de nuevo.')
    }
  }

  // Restore ONLY the affected message's reactions array (in both lists).
  // We intentionally don't restore the whole `messages` snapshot: reactions for
  // OTHER messages (or other emojis on this one) may have arrived via WS between
  // the optimistic change and the failed request, and a global restore would
  // clobber them.
  const restoreReactions = useCallback((messageId: number, reactions: MessageReaction[]) => {
    const update = (list: Message[]) => list.map(m =>
      m.id === messageId ? { ...m, reactions } : m
    )
    setMessages(update)
    setThreadReplies(update)
  }, [])

  const toggleReaction = async (message: Message, emoji: string) => {
    if (!selectedChannel || !user) return
    const mine = (message.reactions || []).some(r => r.user_id === user.id && r.emoji === emoji)
    // M-6: snapshot ONLY this message's reactions array for a targeted rollback.
    const reactionsSnapshot = [...(message.reactions || [])]
    if (mine) {
      // Optimistic remove, then roll back this message's reactions if it fails.
      applyReactionRemoved(message.id, user.id, emoji)
      try {
        await channelService.removeReaction(selectedChannel.id, message.id, emoji)
      } catch (e) {
        console.error('Error removing reaction:', e)
        restoreReactions(message.id, reactionsSnapshot)
        showError('No se pudo quitar la reacción. Intenta de nuevo.')
      }
    } else {
      // Symmetric optimistic add. Build the local reaction (the WS echo / server
      // response will reconcile by user_id+emoji, so no duplicate is created).
      applyReactionAdded(message.id, { message_id: message.id, user_id: user.id, emoji, user } as MessageReaction)
      try {
        await channelService.addReaction(selectedChannel.id, message.id, emoji)
      } catch (e) {
        console.error('Error adding reaction:', e)
        restoreReactions(message.id, reactionsSnapshot)
        showError('No se pudo agregar la reacción. Intenta de nuevo.')
      }
    }
  }

  const starMessage = async (id: number) => {
    try {
      await channelService.starMessage(id)
      setStarredIds(prev => new Set(prev).add(id))
      fetchStarredMessages()
    } catch (e) { console.error('Error starring message:', e) }
  }

  const unstarMessage = async (id: number) => {
    try {
      await channelService.unstarMessage(id)
      setStarredIds(prev => { const next = new Set(prev); next.delete(id); return next })
      setStarredMessages(prev => prev.filter(m => m.id !== id))
    } catch (e) { console.error('Error unstarring message:', e) }
  }

  const pinMessage = async (id: number) => {
    if (selectedChannel) {
      try { await channelService.pinMessage(selectedChannel.id, id); setMessages(prev => prev.map(m => m.id === id ? { ...m, is_pinned: true } : m)); fetchPinnedMessages(selectedChannel.id) } catch (e) { console.error(e) }
    }
  }

  const unpinMessage = async (id: number) => {
    if (selectedChannel) {
      try { await channelService.unpinMessage(selectedChannel.id, id); setMessages(prev => prev.map(m => m.id === id ? { ...m, is_pinned: false } : m)); fetchPinnedMessages(selectedChannel.id) } catch (e) { console.error(e) }
    }
  }

  return {
    channels, selectedChannel, setSelectedChannel,
    messages, setMessages, pinnedMessages,
    hasMoreMessages, loadingOlder, loadOlderMessages,
    newMessage, setNewMessage,
    channelMembers, fetchChannelMembers,
    allUsers, isConnected, connectionLost, unreadCount,
    typingUsers, isRecording, isPaused, recordingStream, recordedBlob,
    pauseRecording, resumeRecording, cancelRecording, discardRecording, sendRecording,
    isUploading, setIsUploading,
    showThread, setShowThread, threadReplies, setThreadReplies,
    sendMessage, sendTypingIndicator, startRecording, stopRecording,
    editMessage, deleteMessage, pinMessage, unpinMessage, fetchChannels, fetchAllUsers,
    toggleReaction, starMessage, unstarMessage, starredMessages, starredIds, fetchStarredMessages,
    userStatuses
  }
}
