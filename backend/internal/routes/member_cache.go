package routes

import (
	"sync"
	"time"

	"github.com/obertrack/backend/internal/repository"
)

// memberCacheTTL is the staleness bound for channel membership used by the
// WebSocket hub's MemberResolver. A member added (or removed) is reflected in
// live broadcast/typing routing within at most this window; the full history is
// always available over REST regardless, so the worst case is a freshly added
// member missing up-to-TTL of live events. Kept short on purpose.
const memberCacheTTL = 30 * time.Second

// memberCacheEntry holds a channel's member set and when it expires.
type memberCacheEntry struct {
	members map[uint]bool
	expiry  time.Time
}

// memberCache caches channel membership (the JOIN-to-users GetMembers query) so
// the hub does not hit the database on every broadcast and every typing frame.
// Safe for concurrent use.
type memberCache struct {
	repo repository.ChannelRepository
	mu   sync.RWMutex
	data map[uint]memberCacheEntry
}

func newMemberCache(repo repository.ChannelRepository) *memberCache {
	return &memberCache{
		repo: repo,
		data: make(map[uint]memberCacheEntry),
	}
}

// Members returns the member-id set for a channel, serving a non-expired cached
// entry when available and otherwise loading it from the repo and caching it.
//
// It ALWAYS returns a fresh copy of the set, never the map stored in c.data.
// The WebSocket hub reads this result without holding any lock, so handing back
// the cached map directly would be a data race the moment a concurrent refresh
// (or any future mutation) touched the same underlying map. Returning a copy
// makes the result owned solely by the caller and immune to cache churn.
func (c *memberCache) Members(channelID uint) map[uint]bool {
	now := time.Now()

	c.mu.RLock()
	entry, ok := c.data[channelID]
	c.mu.RUnlock()
	if ok && now.Before(entry.expiry) {
		return cloneMemberSet(entry.members)
	}

	members, err := c.repo.GetMembers(channelID)
	if err != nil {
		// On error, fall back to whatever we had cached (even if expired) rather
		// than dropping all routing for the channel; otherwise empty set.
		if ok {
			return cloneMemberSet(entry.members)
		}
		return map[uint]bool{}
	}

	set := make(map[uint]bool, len(members))
	for _, m := range members {
		set[m.ID] = true
	}

	c.mu.Lock()
	c.data[channelID] = memberCacheEntry{members: set, expiry: now.Add(memberCacheTTL)}
	c.mu.Unlock()

	// Return a copy so the caller never shares the map we just stored in the
	// cache (a later refresh replaces the entry; callers must not observe that).
	return cloneMemberSet(set)
}

// cloneMemberSet returns an independent copy of a member-id set.
func cloneMemberSet(src map[uint]bool) map[uint]bool {
	dst := make(map[uint]bool, len(src))
	for id := range src {
		dst[id] = true
	}
	return dst
}

// Invalidate drops the cached entry for a channel so the next lookup reloads it.
func (c *memberCache) Invalidate(channelID uint) {
	c.mu.Lock()
	delete(c.data, channelID)
	c.mu.Unlock()
}
