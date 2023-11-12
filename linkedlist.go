package eventsourcing

import "github.com/0x9ef/eventsourcing-go/event"

type node struct {
	value event.Eventer
	next  *node
}

type linkedList struct {
	len  int
	head *node
}

func (l *linkedList) add(value event.Eventer) {
	initNode := &node{
		value: value,
	}
	if l.head == nil {
		l.head = initNode
		l.len++
		return
	}

	current := l.head
	for current.next != nil {
		current = current.next
	}
	current.next = initNode
	l.len++
}

func (l *linkedList) remove(value event.Eventer) {
	var prev *node
	current := l.head
	for current != nil {
		if current.value.GetAggregateId() == value.GetAggregateId() &&
			current.value.GetVersion() == value.GetVersion() {
			if prev == nil {
				l.head = current.next
			} else {
				prev.next = current.next
			}
		} else {
			prev = current
		}
		current = current.next
	}
	l.len--
}

func (l *linkedList) traverse(f func(value event.Eventer) error) error {
	current := l.head
	for current != nil {
		err := f(current.value)
		if err != nil {
			return err
		}
		current = current.next
	}
	return nil
}
