package account

import "errors"

var ErrNotFound = errors.New("account not found")

type Account struct {
	ID      string
	Email   string
	Balance int
}

type Store interface {
	Get(id string) (Account, error)
	Put(account Account) error
}
