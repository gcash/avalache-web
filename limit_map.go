package sherpa

type boundedSlice struct {
	limit int
	count int
	items map[int]notif
}

func newBoundedSlice(limit int) *boundedSlice {
	return &boundedSlice{
		limit: 8,
		items: make(map[int]notif, limit),
	}
}

func (m *boundedSlice) append(n notif) {
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

func (m *boundedSlice) values() []notif {
	values := make([]notif, len(m.items))
	for _, n := range m.items {
		values = append(values, n)
	}
	return values
}
