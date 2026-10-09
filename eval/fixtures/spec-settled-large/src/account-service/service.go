package account

type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

func (s *Service) ChangeEmail(id, email string) error {
	account, err := s.store.Get(id)
	if err != nil {
		return err
	}
	account.Email = email
	return s.store.Put(account)
}

func (s *Service) Deposit(id string, amount int) error {
	account, err := s.store.Get(id)
	if err != nil {
		return err
	}
	account.Balance += amount
	return s.store.Put(account)
}
