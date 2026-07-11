package jwt

import (
	"testing"
	"time"
)

func TestMerchantAccessToken(t *testing.T) {
	token, err := GenerateMerchantAccessToken(11, 3, "owner", "merchant-secret", time.Hour)
	if err != nil {
		t.Fatalf("generate merchant token: %v", err)
	}

	claims, err := ParseMerchantAccessToken(token, "merchant-secret")
	if err != nil {
		t.Fatalf("parse merchant token: %v", err)
	}
	if claims.AccountID != 11 || claims.MerchantID != 3 || claims.Role != "owner" || claims.TokenType != TokenTypeMerchant {
		t.Fatalf("unexpected merchant claims: %+v", claims)
	}
}

func TestMerchantAccessTokenRejectsBuyerToken(t *testing.T) {
	buyerToken, err := GenerateAccessToken(7, "merchant-secret", time.Hour)
	if err != nil {
		t.Fatalf("generate buyer token: %v", err)
	}
	if _, err := ParseMerchantAccessToken(buyerToken, "merchant-secret"); err == nil {
		t.Fatal("expected buyer token to be rejected")
	}
}

func TestMerchantAccessTokenRejectsWrongSecret(t *testing.T) {
	token, err := GenerateMerchantAccessToken(11, 3, "owner", "merchant-secret", time.Hour)
	if err != nil {
		t.Fatalf("generate merchant token: %v", err)
	}
	if _, err := ParseMerchantAccessToken(token, "wrong-secret"); err == nil {
		t.Fatal("expected wrong secret to be rejected")
	}
}
