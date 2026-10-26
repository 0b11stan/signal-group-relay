// Command signal2signal connects to signal-cli-rest-api and prints the
// messages received by the account. It is the first step towards the bridge.
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/0b11stan/signal2signal/internal/signalapi"
)

func main() {
	apiURL := flag.String("api", "http://localhost:8081", "signal-cli-rest-api base URL")
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
	if len(ids) < 2 {
		log.Fatal("-linked-groups needs at least two group ids")
	}
	groups, err := resolveGroups(ctx, client, ids)
	if err != nil {
		log.Fatal(err)
	}
	r := &relayer{client: client, groups: groups, sent: make(map[int64]struct{})}
	log.Printf("relaying messages between %d linked groups", len(groups))

	log.Printf("listening for messages to %s (Ctrl+C to stop)", *number)
	err = client.Receive(ctx, func(in signalapi.Incoming) {
		m, ok := parseMessage(in)
		if !ok {
			return
		}
		if !r.linked(m) {
			return
		}
		m.print()
		r.handle(ctx, m)
	})
	if err != nil && !errors.Is(err, context.Canceled) {
		log.Fatal(err)
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
	if len(ids) == 0 {
		return nil, fmt.Errorf("%s contains no group id", path)
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
		g, ok := known[id]
		if !ok {
			return nil, fmt.Errorf("group id %q is not one of the account's groups (see -groups)", id)
		}
		found[id] = g
	}
	return found, nil
}

// message is a text message received by the account, either from someone
// else or typed on one of our own other devices.
type message struct {
	sender      string
	text        string
	timestamp   int64
	group       *signalapi.GroupInfo // nil for direct messages
	attachments int
	fromMe      bool
}

// parseMessage extracts a message from an incoming event. It returns false
// for receipts, typing indicators, reactions, deletions and the like.
func parseMessage(in signalapi.Incoming) (message, bool) {
	env := in.Envelope
	m := message{sender: env.SourceName}
	if m.sender == "" {
		m.sender = env.Source
	}

	switch {
	case env.DataMessage != nil:
		d := env.DataMessage
		m.text, m.timestamp, m.group, m.attachments = d.Message, d.Timestamp, d.GroupInfo, len(d.Attachments)
	case env.SyncMessage != nil && env.SyncMessage.SentMessage != nil:
		d := env.SyncMessage.SentMessage
		m.text, m.timestamp, m.group, m.attachments = d.Message, d.Timestamp, d.GroupInfo, len(d.Attachments)
		m.fromMe = true
		m.sender += " (me)"
	default:
		return message{}, false
	}
	if m.text == "" && m.attachments == 0 {
		return message{}, false
	}
	return m, true
}

func (m message) print() {
	where := "direct"
	if m.group != nil {
		where = fmt.Sprintf("group %q id=%s", m.group.GroupName, m.group.GroupID)
	}
	when := time.UnixMilli(m.timestamp).Format(time.DateTime)
	fmt.Printf("[%s] %s | %s: %s", when, where, m.sender, m.text)
	if m.attachments > 0 {
		fmt.Printf(" [+%d attachment(s)]", m.attachments)
	}
	fmt.Println()
}
