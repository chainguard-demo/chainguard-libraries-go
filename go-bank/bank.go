package main

import (
	"errors"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type account struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Number  string `json:"number"`
	Balance string `json:"balance"`
}

type transaction struct {
	ID          string    `json:"id"`
	Description string    `json:"description"`
	Category    string    `json:"category"`
	Account     string    `json:"account"`
	ToAccount   string    `json:"toAccount,omitempty"`
	Amount      string    `json:"amount"`
	Date        time.Time `json:"date"`
}

type dashboard struct {
	Customer     string        `json:"customer"`
	Currency     string        `json:"currency"`
	Accounts     []account     `json:"accounts"`
	Transactions []transaction `json:"transactions"`
	TotalBalance string        `json:"totalBalance"`
}

type bankStore struct {
	mu           sync.Mutex
	accounts     []account
	transactions []transaction
}

func newBank() *bankStore {
	now := time.Now().UTC()
	return &bankStore{
		accounts: []account{
			{ID: "checking", Name: "Everyday Checking", Number: "4821", Balance: "8240.50"},
			{ID: "savings", Name: "Goals Savings", Number: "7903", Balance: "24500.00"},
		},
		transactions: []transaction{
			{ID: "demo-1", Description: "Monthly payroll", Category: "Income", Account: "checking", Amount: "4250.00", Date: now.Add(-2 * time.Hour)},
			{ID: "demo-2", Description: "Market Street Grocer", Category: "Shopping", Account: "checking", Amount: "-86.42", Date: now.Add(-6 * time.Hour)},
			{ID: "demo-3", Description: "Savings contribution", Category: "Transfer", Account: "checking", ToAccount: "savings", Amount: "-500.00", Date: now.Add(-24 * time.Hour)},
			{ID: "demo-4", Description: "Northside Coffee", Category: "Food & drink", Account: "checking", Amount: "-6.80", Date: now.Add(-27 * time.Hour)},
			{ID: "demo-5", Description: "City Utilities", Category: "Bills", Account: "checking", Amount: "-124.00", Date: now.Add(-48 * time.Hour)},
		},
	}
}

func (b *bankStore) snapshot() dashboard {
	b.mu.Lock()
	defer b.mu.Unlock()
	total := decimal.Zero
	for _, a := range b.accounts {
		total = total.Add(decimal.RequireFromString(a.Balance))
	}
	return dashboard{Customer: "Alex Morgan", Currency: "USD", Accounts: append([]account{}, b.accounts...), Transactions: append([]transaction{}, b.transactions...), TotalBalance: total.StringFixed(2)}
}

type transferInput struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Amount string `json:"amount"`
	Memo   string `json:"memo"`
}

var currencyAmount = regexp.MustCompile(`^[0-9]{1,9}(\.[0-9]{1,2})?$`)

func (b *bankStore) transfer(input transferInput) (transaction, error) {
	if !currencyAmount.MatchString(input.Amount) {
		return transaction{}, errors.New("Enter an amount with no more than two decimal places.")
	}
	amount, err := decimal.NewFromString(input.Amount)
	if err != nil || !amount.IsPositive() {
		return transaction{}, errors.New("The transfer amount must be greater than zero.")
	}
	if input.From == input.To {
		return transaction{}, errors.New("Choose two different accounts.")
	}
	memo := strings.TrimSpace(input.Memo)
	if len([]rune(memo)) > 120 {
		return transaction{}, errors.New("Keep your note to 120 characters or fewer.")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	from, to := -1, -1
	for i, a := range b.accounts {
		if a.ID == input.From {
			from = i
		}
		if a.ID == input.To {
			to = i
		}
	}
	if from < 0 || to < 0 {
		return transaction{}, errors.New("Choose an available account.")
	}
	fromBalance := decimal.RequireFromString(b.accounts[from].Balance)
	toBalance := decimal.RequireFromString(b.accounts[to].Balance)
	if fromBalance.LessThan(amount) {
		return transaction{}, errors.New("This account has insufficient funds for the transfer.")
	}
	id, err := uuid.NewRandom()
	if err != nil {
		return transaction{}, errors.New("Could not create a transfer ID. Please try again.")
	}
	if memo == "" {
		memo = "Transfer to " + b.accounts[to].Name
	}
	t := transaction{ID: id.String(), Description: memo, Category: "Transfer", Account: input.From, ToAccount: input.To, Amount: amount.Neg().StringFixed(2), Date: time.Now().UTC()}
	b.accounts[from].Balance = fromBalance.Sub(amount).StringFixed(2)
	b.accounts[to].Balance = toBalance.Add(amount).StringFixed(2)
	b.transactions = append([]transaction{t}, b.transactions...)
	if len(b.transactions) > 100 {
		b.transactions = b.transactions[:100]
	}
	return t, nil
}
