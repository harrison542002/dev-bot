package bot

type chunkedMessageAdapter struct {
	maxLen  int
	deliver func(string) error
	onError func(error)
}

func newChunkedMessageAdapter(maxLen int, deliver func(string) error, onError func(error)) *chunkedMessageAdapter {
	return &chunkedMessageAdapter{
		maxLen:  maxLen,
		deliver: deliver,
		onError: onError,
	}
}

func (a *chunkedMessageAdapter) Send(text string) {
	if a == nil || a.deliver == nil {
		return
	}
	for _, chunk := range splitMessage(text, a.maxLen) {
		if err := a.deliver(chunk); err != nil {
			if a.onError != nil {
				a.onError(err)
			}
			return
		}
	}
}

func splitMessage(text string, maxLen int) []string {
	if text == "" || maxLen <= 0 {
		return nil
	}

	runes := []rune(text)
	chunks := make([]string, 0, len(runes)/maxLen+1)
	for len(runes) > maxLen {
		splitAt := maxLen
		for i := maxLen - 1; i >= maxLen/2; i-- {
			if runes[i] == '\n' {
				splitAt = i + 1
				break
			}
		}
		chunks = append(chunks, string(runes[:splitAt]))
		runes = runes[splitAt:]
	}
	if len(runes) > 0 {
		chunks = append(chunks, string(runes))
	}
	return chunks
}
