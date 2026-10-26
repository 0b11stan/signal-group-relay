package main

import (
	"context"
	"log"

	"github.com/0b11stan/signal2signal/internal/signalapi"
)

// relayer copies the messages you write in one linked group to all the other
// linked groups.
type relayer struct {
	client *signalapi.Client
	// groups are the linked groups, keyed by the id found in incoming messages.
	groups map[string]signalapi.Group
	dryRun bool
	// sent holds the timestamps of the messages we posted ourselves, so that
	// if one ever comes back as an incoming event it is not relayed again.
	sent map[int64]struct{}
}

func (r *relayer) handle(ctx context.Context, m message) {
	if !m.fromMe || m.group == nil {
		return
	}
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

	for id, g := range r.groups {
		if id == m.group.GroupID {
			continue
		}
		if r.dryRun {
			log.Printf("relay: [dry-run] would send to %q: %s", g.Name, m.text)
			continue
		}
		ts, err := r.client.Send(ctx, g.ID, m.text)
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
