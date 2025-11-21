package main

// The ticket plugin adapts the in-process Jira provider to OpsOrch Core's
// JSON-RPC plugin contract. Core spawns this binary locally, writes request
// objects (method/config/payload) to stdin, and reads responses from stdout.
// Each request includes the decrypted adapter config so secrets never leave the
// host. The plugin lazily constructs a provider instance using that config and
// reuses it for subsequent calls to avoid re-initialization overhead.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/opsorch/opsorch-core/schema"
	coreticket "github.com/opsorch/opsorch-core/ticket"
	adapter "github.com/opsorch/opsorch-jira-adapter/ticket"
)

type rpcRequest struct {
	Method  string          `json:"method"`
	Config  map[string]any  `json:"config"`
	Payload json.RawMessage `json:"payload"`
}

type rpcResponse struct {
	Result any    `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
}

var provider coreticket.Provider

func main() {
	dec := json.NewDecoder(os.Stdin)
	enc := json.NewEncoder(os.Stdout)

	for {
		var req rpcRequest
		if err := dec.Decode(&req); err != nil {
			if errors.Is(err, io.EOF) {
				return
			}
			writeErr(enc, err)
			return
		}

		prov, err := ensureProvider(req.Config)
		if err != nil {
			writeErr(enc, err)
			continue
		}

		ctx := context.Background()
		switch req.Method {
		case "ticket.query":
			var query schema.TicketQuery
			if err := json.Unmarshal(req.Payload, &query); err != nil {
				writeErr(enc, err)
				continue
			}
			res, err := prov.Query(ctx, query)
			write(enc, res, err)
		case "ticket.get":
			var payload struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(req.Payload, &payload); err != nil {
				writeErr(enc, err)
				continue
			}
			res, err := prov.Get(ctx, payload.ID)
			write(enc, res, err)
		case "ticket.create":
			var in schema.CreateTicketInput
			if err := json.Unmarshal(req.Payload, &in); err != nil {
				writeErr(enc, err)
				continue
			}
			res, err := prov.Create(ctx, in)
			write(enc, res, err)
		case "ticket.update":
			var payload struct {
				ID    string                   `json:"id"`
				Input schema.UpdateTicketInput `json:"input"`
			}
			if err := json.Unmarshal(req.Payload, &payload); err != nil {
				writeErr(enc, err)
				continue
			}
			res, err := prov.Update(ctx, payload.ID, payload.Input)
			write(enc, res, err)
		default:
			writeErr(enc, fmt.Errorf("unknown method: %s", req.Method))
		}
	}
}

func ensureProvider(cfg map[string]any) (coreticket.Provider, error) {
	if provider != nil {
		return provider, nil
	}
	prov, err := adapter.New(cfg)
	if err != nil {
		return nil, err
	}
	provider = prov
	return provider, nil
}

func write(enc *json.Encoder, result any, err error) {
	if err != nil {
		writeErr(enc, err)
		return
	}
	_ = enc.Encode(rpcResponse{Result: result})
}

func writeErr(enc *json.Encoder, err error) {
	_ = enc.Encode(rpcResponse{Error: err.Error()})
}
