package api

type notificationCache struct {
	limit int
	count int
	items map[int]notif
}

func newNotificationCache(limit int) *notificationCache {
	return &notificationCache{
		limit: 8,
		items: make(map[int]notif, limit),
	}
}

func (m *notificationCache) append(n notif) {
	var toDelete []int
	if len(m.items) >= m.limit {
		for i := range m.items {
			if i < m.count+1-m.limit {
				toDelete = append(toDelete, i)
			}
		}
	}
	for _, i := range toDelete {
		delete(m.items, i)
	}
	m.items[m.count+1] = n
	m.count++
}

func (m *notificationCache) values() []notif {
	values := make([]notif, len(m.items))
	for _, n := range m.items {
		values = append(values, n)
	}
	return values
}
