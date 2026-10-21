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
	return c.follow(ctx, id, mirror, onSnapshot, receive, nil)
}

// Follow delivers only accepted immutable projection updates.
func (c *Client) Follow(ctx context.Context, id string, receive func(ProjectionUpdate) error) error {
	return c.follow(ctx, id, &Mirror{}, nil, nil, receive)
}

func (c *Client) follow(ctx context.Context, id string, mirror *Mirror, onSnapshot func(proto.Snapshot), raw func(proto.Event) error, receive func(ProjectionUpdate) error) error {
	if mirror == nil {
		mirror = &Mirror{}
	}
	for {
		h, err := c.Hello(ctx)
		if err != nil {
			return err
		}
		sn, err := c.Snapshot(ctx, id)
		if err != nil {
			return err
		}
		update, accepted := mirror.LoadUpdate(sn)
		if accepted {
			if onSnapshot != nil {
				onSnapshot(sn)
			}
			if receive != nil {
				if err = receive(update); err != nil {
					return err
				}
			}
		}
		err = c.Stream(ctx, id, sn.Seq, h.BootID, func(ev proto.Event) error {
			if ev.Type == proto.EventHello {
				return nil
			}
			u, ok := mirror.ApplyUpdate(ev)
			if mirror.NeedsResync() {
				return errResync
			}
			if !ok {
				return nil
			}
			if raw != nil {
				if e := raw(ev); e != nil {
					return e
				}
			}
			if receive != nil {
				return receive(u)
			}
			return nil
		})
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != errResync {
			return err
		}
	}
}
