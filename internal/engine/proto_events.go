package engine

import "meno/internal/proto"

// publish delegates sequencing to the daemon hub, which owns boot-scoped cursors.
var _ = proto.Event{}
