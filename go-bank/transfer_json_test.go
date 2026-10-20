package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTransferJSON(t *testing.T) {
	valid := `{"from":"checking","to":"savings","amount":"0.10"}`
	cases := []struct {
		name, body, memo string
		status           int
	}{
		{name: "optional memo", body: valid, status: http.StatusCreated},
		{name: "escaped fields", body: `{"fr\u006fm":"checking","to":"savings","amount":"0.\u00310","memo":"Caf\u00e9 \"fund\" \\ \uD83D\uDCB0"}`, memo: "Café \"fund\" \\ 💰", status: http.StatusCreated},
		{name: "case insensitive fields", body: `{"FROM":"checking","To":"savings","Amount":"0.10"}`, status: http.StatusCreated},
		{name: "last duplicate wins", body: `{"from":"checking","to":"savings","amount":"9.00","amount":"0.10"}`, status: http.StatusCreated},
		{name: "null leaves string unchanged", body: `{"from":"checking","to":"savings","amount":"0.10","amount":null,"memo":null}`, status: http.StatusCreated},
		{name: "size limit", body: valid + strings.Repeat(" ", 4096-len(valid)), status: http.StatusCreated},
		{name: "over size limit", body: valid + strings.Repeat(" ", 4097-len(valid)), status: http.StatusBadRequest},
		{name: "unknown field", body: `{"from":"checking","to":"savings","amount":"0.10","extra":null}`, status: http.StatusBadRequest},
		{name: "numeric amount", body: `{"from":"checking","to":"savings","amount":0.10}`, status: http.StatusBadRequest},
		{name: "object memo", body: `{"from":"checking","to":"savings","amount":"0.10","memo":{}}`, status: http.StatusBadRequest},
		{name: "array memo", body: `{"from":"checking","to":"savings","amount":"0.10","memo":[]}`, status: http.StatusBadRequest},
		{name: "boolean memo", body: `{"from":"checking","to":"savings","amount":"0.10","memo":true}`, status: http.StatusBadRequest},
		{name: "missing amount", body: `{"from":"checking","to":"savings"}`, status: http.StatusBadRequest},
		{name: "missing source", body: `{"to":"savings","amount":"0.10"}`, status: http.StatusBadRequest},
		{name: "missing destination", body: `{"from":"checking","amount":"0.10"}`, status: http.StatusBadRequest},
		{name: "multiple values", body: valid + `{}`, status: http.StatusBadRequest},
		{name: "trailing text", body: valid + `garbage`, status: http.StatusBadRequest},
		{name: "trailing comma", body: `{"from":"checking","to":"savings","amount":"0.10",}`, status: http.StatusBadRequest},
		{name: "invalid escape", body: `{"from":"checking","to":"savings","amount":"0.10","memo":"\x"}`, status: http.StatusBadRequest},
		{name: "truncated object", body: valid[:len(valid)-1], status: http.StatusBadRequest},
		{name: "array root", body: `[` + valid + `]`, status: http.StatusBadRequest},
		{name: "null root", body: `null`, status: http.StatusBadRequest},
		{name: "string root", body: `"transfer"`, status: http.StatusBadRequest},
		{name: "empty body", status: http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bank := newBank()
			before, err := json.Marshal(bank.snapshot())
			if err != nil {
				t.Fatal(err)
			}
			request := httptest.NewRequest(http.MethodPost, "/api/transfers", strings.NewReader(tc.body))
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			newHandler(bank).ServeHTTP(response, request)
			if response.Code != tc.status {
				t.Fatalf("got %d want %d: %s", response.Code, tc.status, response.Body.String())
			}
			if tc.status != http.StatusCreated {
				after, err := json.Marshal(bank.snapshot())
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(before, after) {
					t.Fatal("rejected request changed bank state")
				}
				return
			}
			var tx transaction
			if err := json.Unmarshal(response.Body.Bytes(), &tx); err != nil {
				t.Fatal(err)
			}
			wantMemo := tc.memo
			if wantMemo == "" {
				wantMemo = "Transfer to Goals Savings"
			}
			if tx.Account != "checking" || tx.ToAccount != "savings" || tx.Amount != "-0.10" || tx.Description != wantMemo {
				t.Fatalf("unexpected transaction: %+v", tx)
			}
			snapshot := bank.snapshot()
			if snapshot.Accounts[0].Balance != "8240.40" || snapshot.Accounts[1].Balance != "24500.10" || snapshot.TotalBalance != "32740.50" {
				t.Fatalf("unexpected balances: %+v", snapshot)
			}
		})
	}
}
