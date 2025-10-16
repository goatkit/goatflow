package notifications

import (
    "context"
    "sync"
    "time"
)

type PendingReminder struct {
    TicketID     int
    TicketNumber string
    Title        string
    QueueID      int
    QueueName    string
    PendingUntil time.Time
    StateName    string
}

type Hub interface {
    Dispatch(ctx context.Context, recipients []int, reminder PendingReminder) error
    Consume(userID int) []PendingReminder
}

type memoryHub struct {
    mu        sync.Mutex
    reminders map[int][]PendingReminder
}

func NewMemoryHub() Hub {
    return &memoryHub{reminders: make(map[int][]PendingReminder)}
}

func (m *memoryHub) Dispatch(_ context.Context, recipients []int, reminder PendingReminder) error {
    if len(recipients) == 0 {
        return nil
    }
    m.mu.Lock()
    defer m.mu.Unlock()
    for _, uid := range recipients {
        if uid <= 0 {
            continue
        }
        list := m.reminders[uid]
        replaced := false
        for i := range list {
            if list[i].TicketID == reminder.TicketID {
                list[i] = reminder
                replaced = true
                break
            }
        }
        if !replaced {
            list = append(list, reminder)
        }
        m.reminders[uid] = list
    }
    return nil
}

func (m *memoryHub) Consume(userID int) []PendingReminder {
    if userID <= 0 {
        return nil
    }
    m.mu.Lock()
    defer m.mu.Unlock()
    list := m.reminders[userID]
    if len(list) == 0 {
        delete(m.reminders, userID)
        return nil
    }
    // Return a shallow copy to avoid external mutation.
    out := make([]PendingReminder, len(list))
    copy(out, list)
    delete(m.reminders, userID)
    return out
}
