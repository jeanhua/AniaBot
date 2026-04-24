package aichat

type messageWindow struct {
	messages   []Message
	windowSize int
}

func newMessageWindow(windowSize int) *messageWindow {
	return &messageWindow{windowSize: windowSize}
}

func (w *messageWindow) append(msgs ...Message) {
	w.messages = append(w.messages, msgs...)
	w.trim()
}

func (w *messageWindow) trim() {
	if w.windowSize <= 0 {
		return
	}

	humanCount := 0
	for _, m := range w.messages {
		if m.Role == RoleUser {
			humanCount++
		}
	}

	for humanCount > w.windowSize {
		firstHuman := -1
		for i, m := range w.messages {
			if m.Role == RoleUser {
				firstHuman = i
				break
			}
		}
		if firstHuman < 0 {
			break
		}
		nextHuman := -1
		for i := firstHuman + 1; i < len(w.messages); i++ {
			if w.messages[i].Role == RoleUser {
				nextHuman = i
				break
			}
		}
		if nextHuman < 0 {
			break
		}
		w.messages = w.messages[nextHuman:]
		humanCount--
	}
}

func (w *messageWindow) history() []Message {
	return w.messages
}

func (w *messageWindow) clear() {
	w.messages = nil
}
