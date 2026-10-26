// Package signalapi is a minimal client for signal-cli-rest-api running in
// json-rpc mode.
package signalapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/gorilla/websocket"
)

// Client talks to a signal-cli-rest-api instance on behalf of one account.
type Client struct {
	baseURL *url.URL
	number  string
	http    *http.Client
}

// New returns a client for the API at baseURL (e.g. http://localhost:8081)
// acting as the account registered or linked under number (e.g. +33600000000).
func New(baseURL, number string) (*Client, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse api url: %w", err)
	}
	return &Client{baseURL: u, number: number, http: http.DefaultClient}, nil
}

// Group is a Signal group the account is a member of.
type Group struct {
	// ID is the API's identifier, used when sending to the group.
	ID string `json:"id"`
	// InternalID is the base64 id found in incoming messages' GroupInfo.
	InternalID string   `json:"internal_id"`
	Name       string   `json:"name"`
	Members    []string `json:"members"`
}

// Groups lists the groups the account is a member of.
func (c *Client) Groups(ctx context.Context) ([]Group, error) {
	u := c.baseURL.JoinPath("v1", "groups", c.number)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("list groups: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list groups: unexpected status %s", resp.Status)
	}
	var groups []Group
	if err := json.NewDecoder(resp.Body).Decode(&groups); err != nil {
		return nil, fmt.Errorf("decode groups: %w", err)
	}
	return groups, nil
}

// Receive connects to the receive websocket and calls handle for every
// incoming event until ctx is cancelled or the connection fails.
func (c *Client) Receive(ctx context.Context, handle func(Incoming)) error {
	u := *c.baseURL.JoinPath("v1", "receive", c.number)
	switch u.Scheme {
	case "https":
		u.Scheme = "wss"
	default:
		u.Scheme = "ws"
	}

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, u.String(), nil)
	if err != nil {
		return fmt.Errorf("connect to %s: %w", u.String(), err)
	}
	defer conn.Close()

	// Unblock ReadMessage when the context is cancelled.
	go func() {
		<-ctx.Done()
		conn.Close()
	}()

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("read: %w", err)
		}
		var in Incoming
		if err := json.Unmarshal(data, &in); err != nil {
			return fmt.Errorf("decode event %q: %w", strings.TrimSpace(string(data)), err)
		}
		handle(in)
	}
}
