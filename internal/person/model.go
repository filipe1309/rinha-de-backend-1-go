package person

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Person represents a person record.
type Person struct {
	ID         uuid.UUID `json:"id"`
	Apelido    string    `json:"apelido"`
	Nome       string    `json:"nome"`
	Nascimento string    `json:"nascimento"`
	Stack      []string  `json:"stack"`
}

// ValidationError represents a validation failure with HTTP status code.
type ValidationError struct {
	StatusCode int
	Message    string
}

func (e *ValidationError) Error() string {
	return e.Message
}

// ParseAndValidate parses JSON body and validates all fields.
// Returns 400 for type errors, 422 for missing/null required fields or constraint violations.
func ParseAndValidate(body []byte) (*Person, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, &ValidationError{StatusCode: 400, Message: "invalid JSON"}
	}

	apelidoRaw, ok := raw["apelido"]
	if !ok || string(apelidoRaw) == "null" {
		return nil, &ValidationError{StatusCode: 422, Message: "apelido is required"}
	}
	var apelido string
	if err := json.Unmarshal(apelidoRaw, &apelido); err != nil {
		return nil, &ValidationError{StatusCode: 400, Message: "apelido must be a string"}
	}
	if len(apelido) > 32 {
		return nil, &ValidationError{StatusCode: 422, Message: "apelido must be at most 32 characters"}
	}

	nomeRaw, ok := raw["nome"]
	if !ok || string(nomeRaw) == "null" {
		return nil, &ValidationError{StatusCode: 422, Message: "nome is required"}
	}
	var nome string
	if err := json.Unmarshal(nomeRaw, &nome); err != nil {
		return nil, &ValidationError{StatusCode: 400, Message: "nome must be a string"}
	}
	if len(nome) > 100 {
		return nil, &ValidationError{StatusCode: 422, Message: "nome must be at most 100 characters"}
	}

	nascimentoRaw, ok := raw["nascimento"]
	if !ok || string(nascimentoRaw) == "null" {
		return nil, &ValidationError{StatusCode: 422, Message: "nascimento is required"}
	}
	var nascimento string
	if err := json.Unmarshal(nascimentoRaw, &nascimento); err != nil {
		return nil, &ValidationError{StatusCode: 400, Message: "nascimento must be a string"}
	}
	if _, err := time.Parse("2006-01-02", nascimento); err != nil {
		return nil, &ValidationError{StatusCode: 422, Message: "nascimento must be in YYYY-MM-DD format"}
	}

	var stack []string
	stackRaw, ok := raw["stack"]
	if ok && string(stackRaw) != "null" {
		var rawStack []json.RawMessage
		if err := json.Unmarshal(stackRaw, &rawStack); err != nil {
			return nil, &ValidationError{StatusCode: 400, Message: "stack must be an array of strings"}
		}
		stack = make([]string, 0, len(rawStack))
		for _, item := range rawStack {
			var s string
			if err := json.Unmarshal(item, &s); err != nil {
				return nil, &ValidationError{StatusCode: 400, Message: "each stack element must be a string"}
			}
			if len(s) > 32 {
				return nil, &ValidationError{StatusCode: 422, Message: "each stack element must be at most 32 characters"}
			}
			stack = append(stack, s)
		}
	}

	p := &Person{
		ID:         uuid.New(),
		Apelido:    apelido,
		Nome:       nome,
		Nascimento: nascimento,
		Stack:      stack,
	}

	return p, nil
}

// BuildSearchField builds the lowercase searchable text for pg_trgm index.
func (p *Person) BuildSearchField() string {
	parts := []string{p.Apelido, p.Nome}
	parts = append(parts, p.Stack...)
	return strings.ToLower(strings.Join(parts, " "))
}

// StackAsText returns stack as comma-separated string for DB storage.
func (p *Person) StackAsText() *string {
	if p.Stack == nil {
		return nil
	}
	s := strings.Join(p.Stack, ",")
	return &s
}

// StackFromText parses comma-separated string back to []string.
func StackFromText(s *string) []string {
	if s == nil || *s == "" {
		return nil
	}
	return strings.Split(*s, ",")
}

// PersonResponse is used for JSON serialization to match the API spec.
type PersonResponse struct {
	ID         string   `json:"id"`
	Apelido    string   `json:"apelido"`
	Nome       string   `json:"nome"`
	Nascimento string   `json:"nascimento"`
	Stack      []string `json:"stack"`
}

// ToResponse converts a Person to a PersonResponse.
func (p *Person) ToResponse() PersonResponse {
	return PersonResponse{
		ID:         p.ID.String(),
		Apelido:    p.Apelido,
		Nome:       p.Nome,
		Nascimento: p.Nascimento,
		Stack:      p.Stack,
	}
}
