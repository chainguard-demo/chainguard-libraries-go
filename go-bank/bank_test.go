package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
)

func TestTransferExactArithmetic(t *testing.T) {
	bank := newBank()
	initial := bank.snapshot()
	tx, err := bank.transfer(transferInput{From: "checking", To: "savings", Amount: "0.10", Memo: " A dime "})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := uuid.Parse(tx.ID); err != nil {
		t.Fatalf("invalid transfer ID: %v", err)
	}
	if tx.Amount != "-0.10" || tx.Description != "A dime" {
		t.Fatalf("unexpected transaction: %+v", tx)
	}
	got := bank.snapshot()
	if got.Accounts[0].Balance != "8240.40" || got.Accounts[1].Balance != "24500.10" {
		t.Fatalf("unexpected balances: %+v", got.Accounts)
	}
	if got.TotalBalance != initial.TotalBalance {
		t.Fatal("transfer changed total holdings")
	}
	if len(got.Transactions) != len(initial.Transactions)+1 {
		t.Fatal("missing transaction")
	}
	if _, err := bank.transfer(transferInput{From: "savings", To: "checking", Amount: "0.10"}); err != nil {
		t.Fatal(err)
	}
	if bank.snapshot().Accounts[0].Balance != initial.Accounts[0].Balance {
		t.Fatal("round trip lost precision")
	}
}

func TestInvalidTransfersDoNotMutateState(t *testing.T) {
	cases := []transferInput{
		{"checking", "savings", "0", ""}, {"checking", "savings", "-1", ""},
		{"checking", "savings", "1.001", ""}, {"checking", "savings", "1e2", ""},
		{"checking", "savings", "NaN", ""}, {"checking", "savings", "8240.51", ""},
		{"checking", "checking", "1", ""}, {"missing", "savings", "1", ""},
		{"checking", "missing", "1", ""}, {"checking", "savings", "1", strings.Repeat("a", 121)},
	}
	for i, input := range cases {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			bank := newBank()
			before, _ := json.Marshal(bank.snapshot())
			if _, err := bank.transfer(input); err == nil {
				t.Fatal("invalid transfer accepted")
			}
			after, _ := json.Marshal(bank.snapshot())
			if !bytes.Equal(before, after) {
				t.Fatal("rejected transfer changed state")
			}
		})
	}
}

func TestConcurrentTransfersCannotOverdraw(t *testing.T) {
	bank := newBank()
	var wg sync.WaitGroup
	var successes atomic.Int32
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := bank.transfer(transferInput{From: "checking", To: "savings", Amount: "1000"}); err == nil {
				successes.Add(1)
			}
			_ = bank.snapshot()
		}()
	}
	wg.Wait()
	got := bank.snapshot()
	if successes.Load() != 8 || got.Accounts[0].Balance != "240.50" || got.TotalBalance != "32740.50" {
		t.Fatalf("non-atomic transfers: %+v, successes=%d", got, successes.Load())
	}
}

func TestHTTPAPI(t *testing.T) {
	handler := newHandler(newBank())
	cases := []struct {
		method, path, body, contentType string
		status                          int
	}{
		{"GET", "/", "", "", 200}, {"GET", "/styles.css", "", "", 200},
		{"GET", "/health", "", "", 200}, {"GET", "/api/dashboard", "", "", 200},
		{"GET", "/api/dependencies", "", "", 200}, {"GET", "/api/go-mod", "", "", 200},
		{"GET", "/.netrc", "", "", 404}, {"GET", "/.goenv", "", "", 404},
		{"GET", "/api/missing", "", "", 404}, {"GET", "/api/transfers", "", "", 405},
		{"POST", "/api/transfers", `{"from":"checking","to":"savings","amount":"10.25"}`, "application/json", 201},
		{"POST", "/api/transfers", `{}`, "text/plain", 415},
		{"POST", "/api/transfers", `{"amount":10}`, "application/json", 400},
		{"POST", "/api/transfers", `{} {}`, "application/json", 400},
		{"POST", "/api/transfers", `{"unknown":true}`, "application/json", 400},
		{"POST", "/api/transfers", `{"memo":"` + strings.Repeat("a", 5000) + `"}`, "application/json", 400},
	}
	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path+" "+tc.body[:min(len(tc.body), 25)], func(t *testing.T) {
			request := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			if tc.contentType != "" {
				request.Header.Set("Content-Type", tc.contentType)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != tc.status {
				t.Fatalf("got %d want %d: %s", response.Code, tc.status, response.Body.String())
			}
			if response.Header().Get("X-Content-Type-Options") != "nosniff" {
				t.Fatal("missing content type protection")
			}
		})
	}
}

func TestSnapshotDoesNotShareState(t *testing.T) {
	bank := newBank()
	snapshot := bank.snapshot()
	snapshot.Accounts[0].Balance = "0.00"
	snapshot.Transactions[0].Description = "mutated"
	if bank.snapshot().Accounts[0].Balance != "8240.50" || bank.snapshot().Transactions[0].Description == "mutated" {
		t.Fatal("snapshot exposes mutable state")
	}
}
