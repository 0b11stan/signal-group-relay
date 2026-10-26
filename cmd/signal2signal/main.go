// Command signal2signal connects to signal-cli-rest-api and prints the
// messages received by the account. It is the first step towards the bridge.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/0b11stan/signal2signal/internal/signalapi"
)

func main() {
	apiURL := flag.String("api", "http://localhost:8081", "signal-cli-rest-api base URL")
	number := flag.String("number", os.Getenv("SIGNAL_NUMBER"), "account phone number, e.g. +33600000000 (or $SIGNAL_NUMBER)")
	listGroups := flag.Bool("groups", false, "list the account's groups and exit")
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

	log.Printf("listening for messages to %s (Ctrl+C to stop)", *number)
	err = client.Receive(ctx, printMessage)
	if err != nil && !errors.Is(err, context.Canceled) {
		log.Fatal(err)
	}
}

// printMessage prints text messages, whether sent by others or by our own
// other devices, and ignores receipts and typing indicators.
func printMessage(in signalapi.Incoming) {
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
