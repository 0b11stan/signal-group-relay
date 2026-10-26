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
	linkedFile := flag.String("linked-groups", "", "file with one group id per line (as printed by -groups); only messages from these groups are shown")
	flag.Parse()

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

	var linked map[string]struct{}
	if *linkedFile != "" {
		linked, err = loadGroupIDs(*linkedFile)
		if err != nil {
			log.Fatal(err)
		}
		if err := checkGroupsExist(ctx, client, linked); err != nil {
			log.Fatal(err)
		}
		log.Printf("linked groups: %d", len(linked))
	}

	log.Printf("listening for messages to %s (Ctrl+C to stop)", *number)
	err = client.Receive(ctx, func(in signalapi.Incoming) { printMessage(in, linked) })
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

// checkGroupsExist fails if one of ids is not a group the account belongs to,
// which would otherwise silently match nothing.
func checkGroupsExist(ctx context.Context, client *signalapi.Client, ids map[string]struct{}) error {
	groups, err := client.Groups(ctx)
	if err != nil {
		return err
	}
	known := make(map[string]struct{}, len(groups))
	for _, g := range groups {
		known[g.InternalID] = struct{}{}
	}
	for id := range ids {
		if _, ok := known[id]; !ok {
			return fmt.Errorf("group id %q is not one of the account's groups (see -groups)", id)
		}
	}
	return nil
}

// printMessage prints text messages, whether sent by others or by our own
// other devices, and ignores receipts and typing indicators. If linked is not
// nil, only messages posted in one of those groups are printed.
func printMessage(in signalapi.Incoming, linked map[string]struct{}) {
	env := in.Envelope
	sender := env.SourceName
	if sender == "" {
		sender = env.Source
	}

	var (
		text  string
		ts    int64
		group *signalapi.GroupInfo
		n     int
	)
	switch {
	case env.DataMessage != nil:
		m := env.DataMessage
		text, ts, group, n = m.Message, m.Timestamp, m.GroupInfo, len(m.Attachments)
	case env.SyncMessage != nil && env.SyncMessage.SentMessage != nil:
		m := env.SyncMessage.SentMessage
		text, ts, group, n = m.Message, m.Timestamp, m.GroupInfo, len(m.Attachments)
		sender += " (me)"
	default:
		return
	}
	if text == "" && n == 0 {
		return // reactions, deletions, profile updates...
	}
	if linked != nil {
		if group == nil {
			return
		}
		if _, ok := linked[group.GroupID]; !ok {
			return
		}
	}

	where := "direct"
	if group != nil {
		where = fmt.Sprintf("group %q id=%s", group.GroupName, group.GroupID)
	}
	when := time.UnixMilli(ts).Format(time.DateTime)
	fmt.Printf("[%s] %s | %s: %s", when, where, sender, text)
	if n > 0 {
		fmt.Printf(" [+%d attachment(s)]", n)
	}
	fmt.Println()
}
