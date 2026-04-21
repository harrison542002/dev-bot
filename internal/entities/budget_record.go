package entities

type BudgetRecord struct {
	Provider     string
	InputTokens  int64
	OutputTokens int64
	CostUSD      float64
}
