package person

import (
	"testing"
)

func TestValidateCreateRequest_ValidComplete(t *testing.T) {
	body := []byte(`{
		"apelido": "josé",
		"nome": "José Roberto",
		"nascimento": "2000-10-01",
		"stack": ["C#", "Node", "Oracle"]
	}`)
	p, err := ParseAndValidate(body)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if p.Apelido != "josé" {
		t.Errorf("expected apelido 'josé', got '%s'", p.Apelido)
	}
	if p.Nome != "José Roberto" {
		t.Errorf("expected nome 'José Roberto', got '%s'", p.Nome)
	}
	if p.Nascimento != "2000-10-01" {
		t.Errorf("expected nascimento '2000-10-01', got '%s'", p.Nascimento)
	}
	if len(p.Stack) != 3 || p.Stack[0] != "C#" {
		t.Errorf("expected stack [C#, Node, Oracle], got %v", p.Stack)
	}
}

func TestValidateCreateRequest_ValidNullStack(t *testing.T) {
	body := []byte(`{
		"apelido": "ana",
		"nome": "Ana Barbosa",
		"nascimento": "1985-09-23",
		"stack": null
	}`)
	p, err := ParseAndValidate(body)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if p.Stack != nil {
		t.Errorf("expected nil stack, got %v", p.Stack)
	}
}

func TestValidateCreateRequest_MissingApelido(t *testing.T) {
	body := []byte(`{
		"nome": "Ana Barbosa",
		"nascimento": "1985-09-23",
		"stack": null
	}`)
	_, err := ParseAndValidate(body)
	if err == nil {
		t.Fatal("expected error for missing apelido")
	}
	ve, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("expected *ValidationError, got %T", err)
	}
	if ve.StatusCode != 422 {
		t.Errorf("expected 422, got %d", ve.StatusCode)
	}
}

func TestValidateCreateRequest_NullNome(t *testing.T) {
	body := []byte(`{
		"apelido": "ana",
		"nome": null,
		"nascimento": "1985-09-23",
		"stack": null
	}`)
	_, err := ParseAndValidate(body)
	if err == nil {
		t.Fatal("expected error for null nome")
	}
	ve, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("expected *ValidationError, got %T", err)
	}
	if ve.StatusCode != 422 {
		t.Errorf("expected 422, got %d", ve.StatusCode)
	}
}

func TestValidateCreateRequest_NomeIsNumber(t *testing.T) {
	body := []byte(`{
		"apelido": "apelido",
		"nome": 1,
		"nascimento": "1985-01-01",
		"stack": null
	}`)
	_, err := ParseAndValidate(body)
	if err == nil {
		t.Fatal("expected error for numeric nome")
	}
	ve, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("expected *ValidationError, got %T", err)
	}
	if ve.StatusCode != 400 {
		t.Errorf("expected 400, got %d", ve.StatusCode)
	}
}

func TestValidateCreateRequest_StackWithNumber(t *testing.T) {
	body := []byte(`{
		"apelido": "apelido",
		"nome": "nome",
		"nascimento": "1985-01-01",
		"stack": [1, "PHP"]
	}`)
	_, err := ParseAndValidate(body)
	if err == nil {
		t.Fatal("expected error for non-string stack element")
	}
	ve, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("expected *ValidationError, got %T", err)
	}
	if ve.StatusCode != 400 {
		t.Errorf("expected 400, got %d", ve.StatusCode)
	}
}

func TestValidateCreateRequest_ApelidoTooLong(t *testing.T) {
	longApelido := make([]byte, 33)
	for i := range longApelido {
		longApelido[i] = 'a'
	}
	body := []byte(`{
		"apelido": "` + string(longApelido) + `",
		"nome": "Nome",
		"nascimento": "1985-01-01",
		"stack": null
	}`)
	_, err := ParseAndValidate(body)
	if err == nil {
		t.Fatal("expected error for apelido too long")
	}
	ve, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("expected *ValidationError, got %T", err)
	}
	if ve.StatusCode != 422 {
		t.Errorf("expected 422, got %d", ve.StatusCode)
	}
}

func TestValidateCreateRequest_InvalidDateFormat(t *testing.T) {
	body := []byte(`{
		"apelido": "apelido",
		"nome": "Nome",
		"nascimento": "01-01-1985",
		"stack": null
	}`)
	_, err := ParseAndValidate(body)
	if err == nil {
		t.Fatal("expected error for invalid date format")
	}
}

func TestBuildSearchField(t *testing.T) {
	p := &Person{
		Apelido: "José",
		Nome:    "José Roberto",
		Stack:   []string{"C#", "Node"},
	}
	got := p.BuildSearchField()
	if got != "josé josé roberto c# node" {
		t.Errorf("expected 'josé josé roberto c# node', got '%s'", got)
	}
}

func TestBuildSearchField_NilStack(t *testing.T) {
	p := &Person{
		Apelido: "ana",
		Nome:    "Ana Barbosa",
		Stack:   nil,
	}
	got := p.BuildSearchField()
	if got != "ana ana barbosa" {
		t.Errorf("expected 'ana ana barbosa', got '%s'", got)
	}
}
