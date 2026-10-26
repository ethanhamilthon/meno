package tui

import "meno/internal/render"

type layoutKey struct {
	width   int
	opts    render.JSOpts
	palette render.Palette
}
type turnKey struct {
	turn render.Turn
	run  render.RunStats
	cost float64
}
type cachedTurn struct {
	key   turnKey
	lines []render.Line
}

// One current configuration retains at most one layout per saved ID.
type lineCache struct {
	key layoutKey
	m   map[string]cachedTurn
}

func layoutFor(w int, js render.JSOpts) layoutKey {
	js.Elapsed, js.Spin, js.Exec, js.Running, js.Streaming = 0, 0, false, false, false
	return layoutKey{w, js, render.CurrentPalette()}
}
func turnIdentity(t render.Turn) turnKey {
	k := turnKey{turn: t}
	if t.Run != nil {
		k.run = *t.Run
		if t.Run.Cost != nil {
			k.cost = *t.Run.Cost
		}
		k.run.Cost = nil
	}
	k.turn.Run = nil
	return k
}
func (c *lineCache) turnLines(t render.Turn, w int, js render.JSOpts) []render.Line {
	key := layoutFor(w, js)
	if c.m == nil || key != c.key {
		c.key = key
		c.m = map[string]cachedTurn{}
	}
	if t.ID == "" {
		return render.BuildLines([]render.Turn{t}, w, js)
	}
	sig := turnIdentity(t)
	if old, ok := c.m[t.ID]; ok && old.key == sig {
		return old.lines
	}
	lines := render.BuildLines([]render.Turn{t}, w, js)
	c.m[t.ID] = cachedTurn{sig, lines}
	return lines
}
