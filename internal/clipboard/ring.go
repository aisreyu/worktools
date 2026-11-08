package clipboard

import "time"

// Item is an in-memory clipboard capture held by a Ring before (or instead of)
// being persisted to the vault.
type Item struct {
	ID      int64
	Text    string
	Preview string
	Kind    string
	Pinned  bool
	Data    []byte // PNG bytes for image captures
	Mime    string
	At      time.Time
}

// Ring is a bounded, newest-first, de-duplicating in-memory clipboard buffer.
// Re-adding text that already exists refreshes its position instead of
// creating a duplicate, and pinned items survive eviction and Clear.
type Ring struct {
	max   int
	next  int64
	items []Item
}

// New creates a Ring holding at most max entries (minimum 1).
func New(max int) *Ring {
	if max < 1 {
		max = 1
	}
	return &Ring{max: max, next: 1}
}

// Add inserts text at the front of the ring. When an identical capture already
// exists it is moved to the front and returned with its original ID; otherwise
// a new entry is created. Oldest unpinned entries are evicted past the limit.
func (r *Ring) Add(text string) Item {
	for i, it := range r.items {
		if it.Text == text {
			it.Preview = Preview(text)
			it.Kind = DetectKind(text)
			it.At = time.Now()
			r.items = append(r.items[:i], r.items[i+1:]...)
			r.items = append([]Item{it}, r.items...)
			r.evict()
			return it
		}
	}
	it := Item{
		ID:      r.next,
		Text:    text,
		Preview: Preview(text),
		Kind:    DetectKind(text),
		At:      time.Now(),
	}
	r.next++
	r.items = append([]Item{it}, r.items...)
	r.evict()
	return it
}

// evict drops the oldest unpinned entries until the ring is within its limit.
func (r *Ring) evict() {
	for len(r.items) > r.max {
		idx := -1
		for i := len(r.items) - 1; i >= 0; i-- {
			if !r.items[i].Pinned {
				idx = i
				break
			}
		}
		if idx < 0 {
			break
		}
		r.items = append(r.items[:idx], r.items[idx+1:]...)
	}
}

// List returns a copy of the ring's entries, newest first.
func (r *Ring) List() []Item {
	out := make([]Item, len(r.items))
	copy(out, r.items)
	return out
}

// SetPinned pins or unpins an entry, reporting whether the ID was found.
func (r *Ring) SetPinned(id int64, pinned bool) bool {
	for i := range r.items {
		if r.items[i].ID == id {
			r.items[i].Pinned = pinned
			return true
		}
	}
	return false
}

// Delete removes an entry, reporting whether the ID was found.
func (r *Ring) Delete(id int64) bool {
	for i := range r.items {
		if r.items[i].ID == id {
			r.items = append(r.items[:i], r.items[i+1:]...)
			return true
		}
	}
	return false
}

// Clear removes all unpinned entries and returns how many were removed.
func (r *Ring) Clear() int {
	kept := r.items[:0]
	removed := 0
	for _, it := range r.items {
		if it.Pinned {
			kept = append(kept, it)
		} else {
			removed++
		}
	}
	r.items = kept
	return removed
}
