package account

type Calculator struct{}

func (Calculator) Add(left, right int) int {
	return left + right
}

func Retry(attempts int, operation func() error) error {
	var err error
	for range attempts {
		if err = operation(); err == nil {
			return nil
		}
	}
	return err
}
