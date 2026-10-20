package spotify

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"os"
	"time"
)

// operations.json is the single registry of pathfinder operation name ->
// persisted query sha256 hash, extracted from the open.spotify.com bundles
// (see cmd/refresh-hashes). A file at the override path (config dir) wins per
// name, so a broken hash can be fixed without rebuilding.
//
//go:embed operations.json
var operationsJSON []byte

func loadOperations(overridePath string, log *slog.Logger) map[string]string {
	ops := map[string]string{}
	if err := json.Unmarshal(operationsJSON, &ops); err != nil {
		panic("bad embedded operations.json: " + err.Error())
	}
	if overridePath == "" {
		return ops
	}
	data, err := os.ReadFile(overridePath)
	if errors.Is(err, os.ErrNotExist) {
		return ops
	}
	over := map[string]string{}
	if err == nil {
		err = json.Unmarshal(data, &over)
	}
	if err != nil {
		log.Warn("ignoring operations override", "path", overridePath, "err", err)
		return ops
	}
	maps.Copy(ops, over)
	return ops
}

type gqlError struct {
	Message string `json:"message"`
}

// query runs a persisted pathfinder query and decodes `data` into out. A
// positive ttl caches the raw response in memory.
func (c *Client) query(ctx context.Context, op string, vars map[string]any, ttl time.Duration, out any) error {
	hash, ok := c.ops[op]
	if !ok {
		return &Error{Op: op, Err: errors.New("operation not in registry")}
	}
	reqBody, err := json.Marshal(map[string]any{
		"variables":     vars,
		"operationName": op,
		"extensions":    map[string]any{"persistedQuery": map[string]any{"version": 1, "sha256Hash": hash}},
	})
	if err != nil {
		return &Error{Op: op, Err: err}
	}

	key := "pf:" + op + ":" + string(reqBody)
	body, cached := c.cache.get(key)
	if !cached {
		body, err = c.do(ctx, request{
			op: op, method: "POST", url: c.pathfinderURL, body: reqBody,
			contentType: "application/json;charset=UTF-8", accept: "application/json",
			idempotent: true, // queries only; mutations go through mutate
		})
		if err != nil {
			return err
		}
	}

	var resp struct {
		Data   json.RawMessage `json:"data"`
		Errors []gqlError      `json:"errors"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return c.fail(op, body, fmt.Errorf("decoding response: %w", err))
	}
	// Partial data with errors is common (one shelf failing); only fail when
	// there is nothing to show.
	if len(resp.Errors) > 0 && (len(resp.Data) == 0 || string(resp.Data) == "null") {
		return c.fail(op, body, fmt.Errorf("graphql: %s", resp.Errors[0].Message))
	}
	if err := json.Unmarshal(resp.Data, out); err != nil {
		return c.fail(op, body, fmt.Errorf("decoding data: %w", err))
	}
	if !cached && ttl > 0 {
		c.cache.put(key, body, ttl)
	}
	return nil
}

// mutate runs a persisted mutation (never retried on 5xx, never cached).
func (c *Client) mutate(ctx context.Context, op string, vars map[string]any) error {
	return c.mutateInto(ctx, op, vars, nil)
}

// mutateInto is mutate that also decodes the response's data into out.
func (c *Client) mutateInto(ctx context.Context, op string, vars map[string]any, out any) error {
	hash, ok := c.ops[op]
	if !ok {
		return &Error{Op: op, Err: errors.New("operation not in registry")}
	}
	reqBody, _ := json.Marshal(map[string]any{
		"variables":     vars,
		"operationName": op,
		"extensions":    map[string]any{"persistedQuery": map[string]any{"version": 1, "sha256Hash": hash}},
	})
	body, err := c.do(ctx, request{
		op: op, method: "POST", url: c.pathfinderURL, body: reqBody,
		contentType: "application/json;charset=UTF-8", accept: "application/json",
	})
	if err != nil {
		return err
	}
	var resp struct {
		Data   json.RawMessage `json:"data"`
		Errors []gqlError      `json:"errors"`
	}
	if json.Unmarshal(body, &resp) == nil && len(resp.Errors) > 0 {
		return c.fail(op, body, fmt.Errorf("graphql: %s", resp.Errors[0].Message))
	}
	if out != nil {
		if err := json.Unmarshal(resp.Data, out); err != nil {
			return c.fail(op, body, fmt.Errorf("decoding data: %w", err))
		}
	}
	return nil
}

func (c *Client) fail(op string, body []byte, err error) error {
	e := &Error{Op: op, Status: 200, Body: excerpt(body), Err: err}
	c.log.Warn("pathfinder call failed", "op", op, "err", err, "body", e.Body)
	return e
}

// RawQuery runs a pathfinder operation and returns the whole response body
// (for cmd/spdev and fixture capture).
func (c *Client) RawQuery(ctx context.Context, op string, vars map[string]any) ([]byte, error) {
	hash, ok := c.ops[op]
	if !ok {
		return nil, &Error{Op: op, Err: errors.New("operation not in registry")}
	}
	body, _ := json.Marshal(map[string]any{
		"variables":     vars,
		"operationName": op,
		"extensions":    map[string]any{"persistedQuery": map[string]any{"version": 1, "sha256Hash": hash}},
	})
	return c.do(ctx, request{op: op, method: "POST", url: c.pathfinderURL, body: body,
		contentType: "application/json;charset=UTF-8", accept: "application/json"})
}

// RawGet GETs an spclient path and returns the body.
func (c *Client) RawGet(ctx context.Context, path, accept string) ([]byte, error) {
	return c.do(ctx, request{op: "GET " + path, method: "GET", url: c.spclientURL + path, accept: accept, idempotent: true})
}

// RawGetWith is RawGet with extra request headers (dev tooling).
func (c *Client) RawGetWith(ctx context.Context, path, accept string, h http.Header) ([]byte, error) {
	return c.do(ctx, request{op: "GET " + path, method: "GET", url: c.spclientURL + path, accept: accept, idempotent: true, headers: h})
}

// RawPostJSON POSTs a JSON body to an spclient path (dev tooling; only for read-style endpoints).
func (c *Client) RawPostJSON(ctx context.Context, path string, body []byte) ([]byte, error) {
	return c.do(ctx, request{op: "POST " + path, method: "POST", url: c.spclientURL + path, body: body, contentType: "application/json", accept: "application/json"})
}
