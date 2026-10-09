package account

type MemoryStore struct {
	accounts map[string]Account
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{accounts: map[string]Account{}}
}

func (s *MemoryStore) Get(id string) (Account, error) {
	account, ok := s.accounts[id]
	if !ok {
		return Account{}, ErrNotFound
	}
	return account, nil
}

func (s *MemoryStore) Put(account Account) error {
	s.accounts[account.ID] = account
	return nil
}
