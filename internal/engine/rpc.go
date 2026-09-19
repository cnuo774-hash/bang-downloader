package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"
)

type rpcClient struct {
	url    string
	secret string
	client *http.Client
	gate   chan struct{}
	nextID atomic.Uint64
}

type rpcRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      uint64        `json:"id"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params,omitempty"`
}

type rpcResponse struct {
	Result json.RawMessage `json:"result"`
	Error  *rpcError       `json:"error"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *rpcError) Error() string { return fmt.Sprintf("aria2 RPC %d: %s", e.Code, e.Message) }

func newRPC(port int, secret string) *rpcClient {
	return &rpcClient{
		url:    fmt.Sprintf("http://127.0.0.1:%d/jsonrpc", port),
		secret: secret,
		client: &http.Client{Timeout: 8 * time.Second},
		gate:   make(chan struct{}, 1),
	}
}

func (c *rpcClient) call(ctx context.Context, method string, params []interface{}, result interface{}) error {
	select {
	case c.gate <- struct{}{}:
		defer func() { <-c.gate }()
	case <-ctx.Done():
		return ctx.Err()
	}
	secured := make([]interface{}, 0, len(params)+1)
	secured = append(secured, "token:"+c.secret)
	secured = append(secured, params...)
	body, err := json.Marshal(rpcRequest{JSONRPC: "2.0", ID: c.nextID.Add(1), Method: method, Params: secured})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("aria2 RPC HTTP %s", resp.Status)
	}
	var envelope rpcResponse
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return err
	}
	if envelope.Error != nil {
		return envelope.Error
	}
	if result == nil || len(envelope.Result) == 0 {
		return nil
	}
	return json.Unmarshal(envelope.Result, result)
}
