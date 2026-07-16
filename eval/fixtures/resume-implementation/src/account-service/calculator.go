package account

type Calculator struct{}

func (Calculator) Add(left, right int) int {
	return left + right
}

func (Calculator) Health() string {
	return "ok"
}
