package storage

type Collection struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Requests []Request `json:"requests"`
}

type Request struct {
	ID      string            `json:"id"`
	Name    string            `json:"name"`
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body"`
}

type Environment struct {
	Name      string            `json:"name"`
	Variables map[string]string `json:"variables"`
}

type EnvFile struct {
	Active string        `json:"active"`
	Envs   []Environment `json:"envs"`
}

type Response struct {
	StatusCode  int
	Status      string
	Headers     map[string][]string
	Body        string
	ContentType string
	DurationMs  int64
	SizeBytes   int64
	Error       string
}
