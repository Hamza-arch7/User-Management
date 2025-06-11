package components

import "time"

// User model
type User struct {
	ID        string
	Username  string
	Email     string
	Type      string // "admin" or "regular"
	Scope     *Scope // admin only
	CreatedAt time.Time
}

type Scope struct {
	ConsoleAccess, LogsAccess bool
}
