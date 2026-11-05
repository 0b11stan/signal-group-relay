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
	"slices"
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

// New returns a client for the API at baseURL (e.g. http://localhost:8080)
// acting as the account registered or linked under number (e.g. +33600000000).
func New(baseURL, number string) (*Client, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse api url: %w", err)
	}
	return &Client{baseURL: u, number: number, http: http.DefaultClient}, nil
}

// do sends a request to the API endpoint made of path, with payload encoded
// as the JSON body if it is not nil. Non-2xx responses are turned into an
// error; otherwise the caller must close the response body.
//
// Errors are meant to be logged, so they never carry the URL (it can hold the
// account number) or the API's error body (it can quote numbers or contents).
func (c *Client) do(ctx context.Context, method string, payload any, path ...string) (*http.Response, error) {
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL.JoinPath(path...).String(), body)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		if ue, ok := err.(*url.Error); ok {
			err = ue.Err
		}
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		resp.Body.Close()
		return nil, fmt.Errorf("unexpected status %s", resp.Status)
	}
	return resp, nil
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
	resp, err := c.do(ctx, http.MethodGet, nil, "v1", "groups", c.number)
	if err != nil {
		return nil, fmt.Errorf("list groups: %w", err)
	}
	defer resp.Body.Close()
	var groups []Group
	if err := json.NewDecoder(resp.Body).Decode(&groups); err != nil {
		return nil, fmt.Errorf("decode groups: %w", err)
	}
	return groups, nil
}

// Receive connects to the receive websocket and calls handle for every
// incoming event until ctx is cancelled or the connection fails.
func (c *Client) Receive(ctx context.Context, handle func(Envelope)) error {
	u := *c.baseURL.JoinPath("v1", "receive", c.number)
	switch u.Scheme {
	case "https":
		u.Scheme = "wss"
	default:
		u.Scheme = "ws"
	}

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, u.String(), nil)
	if err != nil {
		// Not wrapped with the URL: it holds the account number.
		return fmt.Errorf("connect: %w", err)
	}
	defer conn.Close()
	// Unblock ReadMessage when the context is cancelled.
	defer context.AfterFunc(ctx, func() { conn.Close() })()

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("read: %w", err)
		}
		var in struct {
			Envelope Envelope `json:"envelope"`
		}
		if err := json.Unmarshal(data, &in); err != nil {
			// The event is not quoted: it holds message contents and numbers.
			return fmt.Errorf("decode event: %w", err)
		}
		handle(in.Envelope)
	}
}

// File is an attachment to send.
type File struct {
	// ContentType is a bare MIME type such as image/png, without parameters.
	ContentType string
	Data        []byte
}

// validContentType reports whether ct is a bare MIME type (type/subtype made
// of lowercase letters, digits and .+-). Anything else could add parameters
// to the data: URI the API parses, which it trusts (e.g. the filename it
// writes the attachment to).
func validContentType(ct string) bool {
	typ, sub, ok := strings.Cut(ct, "/")
	if !ok || typ == "" || sub == "" {
		return false
	}
	for _, c := range typ + sub {
		if !('a' <= c && c <= 'z' || '0' <= c && c <= '9' || strings.ContainsRune(".+-", c)) {
			return false
		}
	}
	return true
}

// Attachment downloads a received attachment by its id.
func (c *Client) Attachment(ctx context.Context, id string) ([]byte, error) {
	resp, err := c.do(ctx, http.MethodGet, nil, "v1", "attachments", id)
	if err != nil {
		return nil, fmt.Errorf("download attachment: %w", err)
	}
	defer resp.Body.Close()
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
			if !validContentType(f.ContentType) {
				return 0, fmt.Errorf("send: invalid attachment content type")
			}
			attachments[i] = "data:" + f.ContentType + ";base64," + base64.StdEncoding.EncodeToString(f.Data)
		}
		payload["base64_attachments"] = attachments
	}
	resp, err := c.do(ctx, http.MethodPost, payload, "v2", "send")
	if err != nil {
		return 0, fmt.Errorf("send: %w", err)
	}
	defer resp.Body.Close()
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
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return 0, nil
	}
	if data[0] != '[' {
		data = slices.Concat([]byte("["), data, []byte("]"))
	}
	var results []struct {
		Timestamp json.Number `json:"timestamp"`
	}
	if err := json.Unmarshal(data, &results); err != nil {
		return 0, fmt.Errorf("decode send response: %w", err)
	}
	for _, res := range results {
		if ts, err := strconv.ParseInt(res.Timestamp.String(), 10, 64); err == nil {
			return ts, nil
		}
	}
	return 0, nil
}
