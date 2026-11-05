package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/0b11stan/signal-group-relay/internal/signalapi"
)

// relayer copies the messages posted in one linked group to all the other
// linked groups. Messages written by other members are prefixed with the
// origin group; your own are sent as they are.
type relayer struct {
	client *signalapi.Client
	// groups are the linked groups, keyed by the id found in incoming messages.
	groups map[string]signalapi.Group
	// sent holds the timestamps of the messages we posted ourselves, with the
	// time we sent them, so that if one ever comes back as an incoming event
	// it is not relayed again. An echo would arrive within seconds, so entries
	// older than sentTTL are dropped.
	sent map[int64]time.Time
}

const sentTTL = time.Hour

// linked reports whether m was posted in one of the linked groups.
func (r *relayer) linked(m message) bool {
	if m.GroupInfo == nil {
		return false
	}
	_, ok := r.groups[m.GroupInfo.GroupID]
	return ok
}

// handle relays m to the other linked groups. m must be linked.
func (r *relayer) handle(ctx context.Context, m message) {
	if _, ours := r.sent[m.Timestamp]; ours {
		return
	}
	files := r.images(ctx, m)
	if m.Message == "" && len(files) == 0 {
		return
	}

	origin := m.GroupInfo.GroupID
	text := m.Message
	if !m.fromMe {
		text = strings.TrimSpace(fmt.Sprintf("[%s] %s", r.groups[origin].Name, m.Message))
	}

	for id, g := range r.groups {
		if id == origin {
			continue
		}
		ts, err := r.client.Send(ctx, g.ID, text, files...)
		if err != nil {
			log.Printf("relay: %v", err)
			continue
		}
		if ts != 0 {
			r.remember(ts)
		}
	}
}

// images downloads the image attachments of m. Other kinds of attachment are
// not relayed yet. Images that fail to download are left out.
func (r *relayer) images(ctx context.Context, m message) []signalapi.File {
	var files []signalapi.File
	for _, a := range m.Attachments {
		if !strings.HasPrefix(a.ContentType, "image/") {
			continue
		}
		data, err := r.client.Attachment(ctx, a.ID)
		if err != nil {
			log.Printf("relay: %v", err)
			continue
		}
		files = append(files, signalapi.File{ContentType: a.ContentType, Data: data})
	}
	return files
}

// remember records ts as one of our own messages and forgets the ones older
// than sentTTL.
func (r *relayer) remember(ts int64) {
	now := time.Now()
	for old, at := range r.sent {
		if now.Sub(at) > sentTTL {
			delete(r.sent, old)
		}
	}
	r.sent[ts] = now
}
