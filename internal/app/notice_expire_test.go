package app

import (
	"testing"
	"time"
)

func TestNoticesExpire(t *testing.T) {
	a, _ := newTestApp(t)
	base := time.Now()
	noticeNow = func() time.Time { return base }
	defer func() { noticeNow = time.Now }()
	a.notify("info")
	a.notifyErr("bad")
	noticeNow = func() time.Time { return base.Add(6 * time.Second) }
	if !a.pruneNotices() || len(a.st.Notices) != 1 || a.st.Notices[0].Text != "bad" {
		t.Fatalf("after 6s: %+v", a.st.Notices)
	}
	noticeNow = func() time.Time { return base.Add(11 * time.Second) }
	if !a.pruneNotices() || len(a.st.Notices) != 0 {
		t.Fatalf("after 11s: %+v", a.st.Notices)
	}
}
