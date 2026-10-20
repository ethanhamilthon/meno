package proc

import "time"

// SetGraceTime changes how long a cancelled process group may take to exit
// after SIGTERM before it is killed (default five seconds).
func SetGraceTime(d time.Duration) { graceTime.Store(int64(d)) }
