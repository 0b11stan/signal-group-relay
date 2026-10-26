package main

import (
	"context"
	"fmt"
	"log"

	"github.com/0b11stan/signal2signal/internal/signalapi"
)

// relayer copies the messages posted in one linked group to all the other
// linked groups. Messages written by other members are prefixed with the
// origin group; your own are sent as they are.
type relayer struct {
	client *signalapi.Client
	// groups are the linked groups, keyed by the id found in incoming messages.
	groups map[string]signalapi.Group
	// sent holds the timestamps of the messages we posted ourselves, so that
	// if one ever comes back as an incoming event it is not relayed again.
	sent map[int64]struct{}
}

// linked reports whether m was posted in one of the linked groups.
func (r *relayer) linked(m message) bool {
	if m.group == nil {
		return false
	}
	_, ok := r.groups[m.group.GroupID]
	return ok
}

// handle relays m to the other linked groups. m must be linked.
func (r *relayer) handle(ctx context.Context, m message) {
	if _, ours := r.sent[m.timestamp]; ours {
		return
	}
	if m.text == "" {
		log.Printf("relay: skipping message without text (attachments are not supported yet)")
		return
	}
	if m.attachments > 0 {
		log.Printf("relay: %d attachment(s) not relayed (not supported yet)", m.attachments)
	}

	text := m.text
	if !m.fromMe {
		text = fmt.Sprintf("[%s] %s", r.groups[m.group.GroupID].Name, m.text)
	}

	for id, g := range r.groups {
		if id == m.group.GroupID {
			continue
		}
		ts, err := r.client.Send(ctx, g.ID, text)
		if err != nil {
			log.Printf("relay: send to %q failed: %v", g.Name, err)
			continue
		}
		if ts != 0 {
			r.sent[ts] = struct{}{}
		}
		log.Printf("relay: sent to %q", g.Name)
	}
}
