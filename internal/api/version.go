package api

// SessionVersion is the compare boundary for session state. A client keeps the
// highest version it has applied and rejects any snapshot or refresh that is
// older. Revision restarts from zero when the daemon boots, so BootID decides
// whether two versions are comparable at all.
type SessionVersion struct {
	BootID   string `json:"boot_id"`
	Revision uint64 `json:"revision"`
}

// Newer reports whether v is strictly newer than o. Versions from different
// boots are not ordered, so they are never reported as newer or older.
func (v SessionVersion) Newer(o SessionVersion) bool {
	return v.BootID == o.BootID && v.Revision > o.Revision
}
