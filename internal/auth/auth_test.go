package auth

import (
	"testing"
)

// TestHelloName calls greetings.Hello with a name, checking
// for a valid return value.
func TestHashPassword(t *testing.T) {
	password := "Gladys"
	hash, err := HashPassword("Gladys")
	match, err := CheckPasswordHash(password, hash)
	want := match == true
	if !want || err != nil {
		t.Errorf(`Expected match to be true, got %s %t`, err, match)
	}
}
