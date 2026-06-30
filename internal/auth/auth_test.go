package auth

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
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

func TestMakeJWT(t *testing.T) {
	userID := uuid.New()
	//parsedUserID, _ := uuid.Parse(userID)
	token, err := MakeJWT(userID, "test", 5*time.Minute)
	//fmt.Println("token: ", token)
	if err != nil || token == "" {
		t.Errorf("Expected idk, got %s", err)
	}

}

func TestValidateJWT(t *testing.T) {
	userID := uuid.New()
	token, err := MakeJWT(userID, "test", 5*time.Minute)
	validToken, err := ValidateJWT(token, "test")
	//fmt.Println("token: ", validToken.String())
	if err != nil || validToken.String() != userID.String() {
		t.Errorf("error got %s", err)
	}
}

func TestValidateExpiredJWT(t *testing.T) {
	userID := uuid.New()
	token, err := MakeJWT(userID, "test", 1*time.Second)
	if err != nil {
		t.Fatalf("error creating token: %v", err)
	}
	time.Sleep(5 * time.Second)
	_, err = ValidateJWT(token, "test")
	//fmt.Println("VALID TOKEN: ", validToken)
	if err == nil {
		t.Errorf("expected token to be expired but got no error")
	}
}

func TestValidateJWTWrongKey(t *testing.T) {
	userID := uuid.New()
	token, err := MakeJWT(userID, "test", 5*time.Minute)
	if err != nil {
		t.Fatalf("error creating token: %v", err)
	}
	_, err = ValidateJWT(token, "Wrong")
	if err == nil {
		t.Errorf("expected token to be invalid but got no errors")
	}
}

func TestGetBearerToken(t *testing.T) {
	userID := uuid.New()
	token, err := MakeJWT(userID, "test", 5*time.Minute)
	if err != nil {
		t.Fatalf("error creating token: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/hello", nil)

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	headerToken, err := GetBearerToken(req.Header)
	_, err = ValidateJWT(headerToken, "test")

	if err != nil {
		t.Errorf("expected no error got %s", err)
	}
}

func TestGetBearerTokenNoHeader(t *testing.T) {

	req := httptest.NewRequest(http.MethodPost, "/hello", nil)

	_, err := GetBearerToken(req.Header)

	if err == nil {
		t.Errorf("expected error got no error %v", err)
	}
}
