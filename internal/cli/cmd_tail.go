package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

const tailUsage = "usage: meno tail ID... [--children-of ID|me] [--events TYPE,...] [--from SEQ [--boot BOOT]]"

// runTail prints one JSON object per event. --json changes nothing: the
// output is always JSON lines, so it can be read while the session runs.
func runTail(args []string, s IO, a API, _ bool) (int, error) {
	values, _, pos, err := parseFlags(args, map[string]bool{"children-of": true, "events": true, "from": true, "boot": true}, nil)
	if err != nil {
		return 0, err
	}
	req := TailRequest{IDs: pos, ChildrenOf: values["children-of"], Boot: values["boot"]}
	switch {
	case len(pos) == 0 && req.ChildrenOf == "":
		return 0, usageError(tailUsage)
	case req.ChildrenOf != "" && len(pos) > 0:
		return 0, usageError("--children-of cannot be combined with session ids")
	}
	if v := values["events"]; v != "" {
		req.Events = strings.Split(v, ",")
	}
	if v := values["from"]; v != "" {
		n, perr := strconv.ParseUint(v, 10, 64)
		if perr != nil {
			return 0, usageError("--from must be a sequence number")
		}
		if len(pos) != 1 {
			return 0, usageError("--from continues one session: give exactly one session id")
		}
		req.From, req.HasFrom = n, true
	}
	if req.Boot != "" && !req.HasFrom {
		return 0, usageError("--boot belongs to --from")
	}
	if err = apiRequired(a); err != nil {
		return 0, err
	}
	enc := json.NewEncoder(s.Out)
	enc.SetEscapeHTML(false)
	err = a.Tail(req, func(e Event) error {
		if err := enc.Encode(e); err != nil {
			return fmt.Errorf("write: %w", err)
		}
		return nil
	})
	return 0, err
}
