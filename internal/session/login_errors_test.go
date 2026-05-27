package session

import "testing"

func TestKratosErrorMessage(t *testing.T) {
	body := []byte(`{
		"id":"1dea324c-2ab9-48cc-82c0-da23cad2701e",
		"type":"api",
		"ui":{
			"messages":[{
				"id":4000006,
				"text":"The provided credentials are invalid, check for spelling mistakes in your password or username, email address, or phone number.",
				"type":"error"
			}]
		}
	}`)

	got := kratosErrorMessage(body)
	want := "The provided credentials are invalid, check for spelling mistakes in your password or username, email address, or phone number."
	if got != want {
		t.Fatalf("kratosErrorMessage() = %q, want %q", got, want)
	}
}

func TestKratosErrorMessageInvalidJSON(t *testing.T) {
	if got := kratosErrorMessage([]byte("not json")); got != "" {
		t.Fatalf("kratosErrorMessage() = %q, want empty", got)
	}
}
