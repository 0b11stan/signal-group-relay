// Package signalapi is a minimal client for signal-cli-rest-api running in
// json-rpc mode.
package signalapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
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

// File is an attachment to send.
type File struct {
	ContentType string
	Data        []byte
}

// Attachment downloads a received attachment by its id.
func (c *Client) Attachment(ctx context.Context, id string) ([]byte, error) {
	u := c.baseURL.JoinPath("v1", "attachments", id)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download attachment: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download attachment: unexpected status %s", resp.Status)
	}
	return io.ReadAll(resp.Body)
}

// Send posts a message with optional attachments to a group and returns the
// message timestamp. groupID is the API's Group.ID (not the InternalID).
func (c *Client) Send(ctx context.Context, groupID, text string, files ...File) (int64, error) {
	payload := map[string]any{
		"message":    text,
		"number":     c.number,
		"recipients": []string{groupID},
	}
	if len(files) > 0 {
		attachments := make([]string, len(files))
		for i, f := range files {
			attachments[i] = "data:" + f.ContentType + ";base64," + base64.StdEncoding.EncodeToString(f.Data)
		}
		payload["base64_attachments"] = attachments
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return 0, err
	}
	u := c.baseURL.JoinPath("v2", "send")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, fmt.Errorf("send: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return 0, fmt.Errorf("send: unexpected status %s: %s", resp.Status, strings.TrimSpace(string(msg)))
	}
	return parseSendTimestamp(resp.Body)
}

// parseSendTimestamp extracts the message timestamp from a send response.
// Depending on the API version the response is either an object or an array
// of objects carrying a "timestamp" (a number or a numeric string). It returns
// 0 if no timestamp is present: the message was still sent.
func parseSendTimestamp(r io.Reader) (int64, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return 0, fmt.Errorf("read send response: %w", err)
	}
	var entries []struct {
		Timestamp json.Number `json:"timestamp"`
	}
	if trimmed := bytes.TrimSpace(data); len(trimmed) > 0 && trimmed[0] == '[' {
		err = json.Unmarshal(trimmed, &entries)
	} else {
		entries = make([]struct {
			Timestamp json.Number `json:"timestamp"`
		}, 1)
		err = json.Unmarshal(trimmed, &entries[0])
	}
	if err != nil {
		return 0, fmt.Errorf("decode send response %q: %w", data, err)
	}
	for _, e := range entries {
		if ts, err := strconv.ParseInt(e.Timestamp.String(), 10, 64); err == nil {
			return ts, nil
		}
	}
	return 0, nil
}
