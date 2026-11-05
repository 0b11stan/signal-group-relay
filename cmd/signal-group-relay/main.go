// Command signal-group-relay relays the messages posted in one of several
// linked Signal groups to all the others, through signal-cli-rest-api.
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/0b11stan/signal-group-relay/internal/signalapi"
)

func main() {
	apiURL := flag.String("api", "http://localhost:8080", "signal-cli-rest-api base URL")
	number := flag.String("number", os.Getenv("SIGNAL_NUMBER"), "account phone number, e.g. +33600000000 (or $SIGNAL_NUMBER)")
	listGroups := flag.Bool("groups", false, "list the account's groups and exit")
	linkedFile := flag.String("linked-groups", "", "file with one group id per line (as printed by -groups); messages posted in one of these groups are relayed to all the others")
	flag.Parse()

	if *linkedFile == "" && !*listGroups {
		flag.Usage()
		os.Exit(2)
	}
	if *number == "" {
		log.Fatal("missing account number: pass -number or set SIGNAL_NUMBER")
	}

	client, err := signalapi.New(*apiURL, *number)
	if err != nil {
		log.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if *listGroups {
		groups, err := client.Groups(ctx)
		if err != nil {
			log.Fatal(err)
		}
		for _, g := range groups {
			fmt.Printf("%-40s members=%-5d id=%s\n", g.Name, len(g.Members), g.InternalID)
		}
		return
	}

	ids, err := loadGroupIDs(*linkedFile)
	if err != nil {
		log.Fatal(err)
	}
	groups, err := resolveGroups(ctx, client, ids)
	if err != nil {
		log.Fatal(err)
	}
	r := &relayer{client: client, groups: groups, sent: make(map[int64]time.Time)}
	log.Printf("relaying messages between %d linked groups (Ctrl+C to stop)", len(groups))
	receiveForever(ctx, client, func(env signalapi.Envelope) {
		m, ok := parseMessage(env)
		if ok && r.linked(m) {
			r.handle(ctx, m)
		}
	})
}

// receiveForever feeds handle with incoming events until ctx is cancelled,
// reconnecting with an increasing delay whenever the connection drops (API
// restarting, network hiccup...). Messages received by the API while we are
// disconnected are not replayed.
func receiveForever(ctx context.Context, client *signalapi.Client, handle func(signalapi.Envelope)) {
	const (
		minDelay = time.Second
		maxDelay = time.Minute
	)
	delay := minDelay
	for {
		start := time.Now()
		err := client.Receive(ctx, handle)
		if ctx.Err() != nil {
			return
		}
		// A connection that held for a while was healthy: start over.
		if time.Since(start) > maxDelay {
			delay = minDelay
		}
		log.Printf("receive: %v; reconnecting in %s", err, delay)
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
		delay = min(delay*2, maxDelay)
	}
}

// loadGroupIDs reads one group id per line from path. Blank lines and lines
// starting with # are ignored.
func loadGroupIDs(path string) (map[string]struct{}, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open linked groups file: %w", err)
	}
	defer f.Close()

	ids := make(map[string]struct{})
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		ids[line] = struct{}{}
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if len(ids) < 2 {
		return nil, fmt.Errorf("%s must contain at least two group ids", path)
	}
	return ids, nil
}

// resolveGroups returns the account's groups matching ids, keyed by id. It
// fails if one of ids is not a group the account belongs to, which would
// otherwise silently match nothing.
func resolveGroups(ctx context.Context, client *signalapi.Client, ids map[string]struct{}) (map[string]signalapi.Group, error) {
	groups, err := client.Groups(ctx)
	if err != nil {
		return nil, err
	}
	known := make(map[string]signalapi.Group, len(groups))
	for _, g := range groups {
		known[g.InternalID] = g
	}
	found := make(map[string]signalapi.Group, len(ids))
	for id := range ids {
		if g, ok := known[id]; ok {
			found[id] = g
		}
	}
	if unknown := len(ids) - len(found); unknown > 0 {
		return nil, fmt.Errorf("%d linked group id(s) are not among the account's groups (see -groups)", unknown)
	}
	return found, nil
}

// message is a message received by the account, either from someone else or
// typed on one of our own other devices.
type message struct {
	signalapi.DataMessage
	fromMe bool
}

// parseMessage extracts a message from an incoming event. It returns false
// for receipts, typing indicators, reactions, deletions and the like.
func parseMessage(env signalapi.Envelope) (message, bool) {
	d, fromMe := env.DataMessage, false
	if d == nil && env.SyncMessage != nil {
		d, fromMe = env.SyncMessage.SentMessage, true
	}
	if d == nil || (d.Message == "" && len(d.Attachments) == 0) {
		return message{}, false
	}
	return message{DataMessage: *d, fromMe: fromMe}, true
}
