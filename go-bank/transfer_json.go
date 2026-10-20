package main

import (
	"bytes"
	"fmt"
	"log"
	"strings"

	"github.com/buger/jsonparser"
)

// redactCredentials returns a copy of a raw transfer request with any
// client-supplied secret PIN removed. Some clients include a
// {"secret":{"pin":"…"}} block to authorize a transfer; that credential must
// never reach the audit trail, so jsonparser.Delete strips it in place.
func redactCredentials(raw []byte) []byte {
	return jsonparser.Delete(raw, "secret", "pin")
}

// recordTransferAudit writes a redacted copy of a transfer request to the
// audit log. It is invoked as fire-and-forget logging on its own goroutine so
// a slow audit sink never blocks a customer's transfer.
func recordTransferAudit(raw []byte) {
	log.Printf("audit: transfer request %s", redactCredentials(raw))
}

// UnmarshalJSON reads transfer fields with jsonparser. The encoding/json decoder
// checks JSON syntax before it calls this method.
func (input *transferInput) UnmarshalJSON(data []byte) error {
	// Match encoding/json: null leaves the destination unchanged.
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return nil
	}
	return jsonparser.ObjectEach(data, func(key, value []byte, kind jsonparser.ValueType, _ int) error {
		var field *string
		switch strings.ToLower(string(key)) {
		case "from":
			field = &input.From
		case "to":
			field = &input.To
		case "amount":
			field = &input.Amount
		case "memo":
			field = &input.Memo
		default:
			return fmt.Errorf("unknown transfer field %q", key)
		}
		if kind == jsonparser.Null {
			return nil
		}
		if kind != jsonparser.String {
			return fmt.Errorf("transfer field %q must be a string", key)
		}
		text, err := jsonparser.ParseString(value)
		if err != nil {
			return err
		}
		*field = text
		return nil
	})
}
