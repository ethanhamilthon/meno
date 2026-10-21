package client

import (
	"context"
	"meno/internal/proto"
)

// Attach takes a snapshot, then resumes events without gaps or duplicates.
// It reloads the snapshot when the event ring or daemon boot requires resync.
func (c *Client) Attach(ctx context.Context, id string, mirror *Mirror, receive func(proto.Event) error) error {
	return c.AttachWith(ctx, id, mirror, nil, receive)
}

// AttachWith is Attach that also hands every snapshot it takes (the first one
// and each one after a resync) to onSnapshot.
func (c *Client) AttachWith(ctx context.Context, id string, mirror *Mirror, onSnapshot func(proto.Snapshot), receive func(proto.Event) error) error {
	var after uint64
	var boot string
	for {
		h, e := c.Hello(ctx)
		if e != nil {
			return e
		}
		boot = h.BootID
		snap, e := c.Snapshot(ctx, id)
		if e != nil {
			return e
		}
		if mirror != nil {
			mirror.Load(snap)
		}
		if onSnapshot != nil {
			onSnapshot(snap)
		}
		after = snap.Seq
		e = c.Stream(ctx, id, after, boot, func(ev proto.Event) error {
			if ev.Type != proto.EventHello && ev.Seq > after {
				after = ev.Seq
			}
			if ev.Type == proto.EventHello {
				return nil
			}
			if mirror != nil {
				mirror.Apply(ev)
			}
			if receive != nil {
				if err := receive(ev); err != nil {
					return err
				}
			}
			// The mirror cannot rebuild some changes from the event alone (a
			// gap, a tasks or session change): take a new snapshot.
			if mirror != nil && mirror.Snapshot().NeedsResync {
				return errResync
			}
			return nil
		})
		if e != nil && e != errResync {
			return e
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
}
