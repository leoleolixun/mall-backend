package jwt

import (
	"testing"
	"time"

	gojwt "github.com/golang-jwt/jwt/v5"
)

func TestAccessTokenRoundTrip(t *testing.T) {
	token, err := GenerateAccessToken(7, "buyer-secret", time.Hour)
	if err != nil {
		t.Fatalf("generate buyer token: %v", err)
	}
	userID, err := ParseAccessToken(token, "buyer-secret")
	if err != nil {
		t.Fatalf("parse buyer token: %v", err)
	}
	if userID != 7 {
		t.Fatalf("unexpected user id: %d", userID)
	}
}

func TestAccessTokenRejectsNonHS256Algorithm(t *testing.T) {
	claims := Claims{
		UserID: 7,
		RegisteredClaims: gojwt.RegisteredClaims{
			ExpiresAt: gojwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
	token, err := gojwt.NewWithClaims(gojwt.SigningMethodHS384, claims).SignedString([]byte("buyer-secret"))
	if err != nil {
		t.Fatalf("generate HS384 token: %v", err)
	}
	if _, err := ParseAccessToken(token, "buyer-secret"); err == nil {
		t.Fatal("expected HS384 token to be rejected")
	}
}

func TestAccessTokenRejectsMissingSecretAndInvalidUser(t *testing.T) {
	if _, err := GenerateAccessToken(7, "", time.Hour); err == nil {
		t.Fatal("expected empty generation secret to be rejected")
	}
	if _, err := ParseAccessToken("token", ""); err == nil {
		t.Fatal("expected empty parsing secret to be rejected")
	}

	token, err := GenerateAccessToken(0, "buyer-secret", time.Hour)
	if err != nil {
		t.Fatalf("generate invalid-user token: %v", err)
	}
	if _, err := ParseAccessToken(token, "buyer-secret"); err == nil {
		t.Fatal("expected non-positive user id to be rejected")
	}
}
