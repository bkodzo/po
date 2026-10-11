package mood

import "fmt"

// NewReader builds the reader called name: auto, decision, ollama or keyword.
// Auto uses the decision server and falls back to keywords if it fails.
func NewReader(name, url, model string) (Reader, error) {
	switch name {
	case "keyword":
		return Keyword{}, nil
	case "decision":
		return NewDecision(url, model), nil
	case "ollama":
		return NewOllama(url, model), nil
	case "auto":
		return Fallback{Primary: NewDecision(url, model), Backup: Keyword{}}, nil
	}
	return nil, fmt.Errorf("unknown reader %q", name)
}
