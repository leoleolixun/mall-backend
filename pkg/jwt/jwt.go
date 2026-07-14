package jwt

import (
	"fmt"
	"time"

	gojwt "github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID int64 `json:"user_id"`
	gojwt.RegisteredClaims
}

const TokenTypeMerchant = "merchant"

type MerchantClaims struct {
	AccountID      int64  `json:"account_id"`
	MerchantID     int64  `json:"merchant_id"`
	Role           string `json:"role"`
	SessionVersion int64  `json:"session_version"`
	TokenType      string `json:"token_type"`
	gojwt.RegisteredClaims
}

// 生成访问令牌
func GenerateAccessToken(userID int64, secret string, ttl time.Duration) (string, error) {
	claims := Claims{
		UserID: userID,
		RegisteredClaims: gojwt.RegisteredClaims{
			ExpiresAt: gojwt.NewNumericDate(time.Now().Add(ttl)),
			IssuedAt:  gojwt.NewNumericDate(time.Now()),
		},
	}

	token := gojwt.NewWithClaims(gojwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))

}

// 解析访问令牌
func ParseAccessToken(tokenString string, secret string) (int64, error) {
	token, err := gojwt.ParseWithClaims(tokenString, &Claims{}, func(token *gojwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*gojwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("无效的签定方法")
		}
		return []byte(secret), nil
	})
	if err != nil {
		return 0, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return 0, fmt.Errorf("token 无效")
	}
	return claims.UserID, nil
}

func GenerateMerchantAccessToken(
	accountID int64,
	merchantID int64,
	role string,
	sessionVersion int64,
	secret string,
	ttl time.Duration,
) (string, error) {
	if secret == "" {
		return "", fmt.Errorf("商家 JWT 密钥未配置")
	}
	claims := MerchantClaims{
		AccountID:      accountID,
		MerchantID:     merchantID,
		Role:           role,
		SessionVersion: sessionVersion,
		TokenType:      TokenTypeMerchant,
		RegisteredClaims: gojwt.RegisteredClaims{
			ExpiresAt: gojwt.NewNumericDate(time.Now().Add(ttl)),
			IssuedAt:  gojwt.NewNumericDate(time.Now()),
		},
	}

	token := gojwt.NewWithClaims(gojwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func ParseMerchantAccessToken(tokenString string, secret string) (*MerchantClaims, error) {
	if secret == "" {
		return nil, fmt.Errorf("商家 JWT 密钥未配置")
	}
	token, err := gojwt.ParseWithClaims(
		tokenString,
		&MerchantClaims{},
		func(token *gojwt.Token) (interface{}, error) {
			if token.Method != gojwt.SigningMethodHS256 {
				return nil, fmt.Errorf("无效的签名方法")
			}
			return []byte(secret), nil
		},
		gojwt.WithValidMethods([]string{gojwt.SigningMethodHS256.Alg()}),
	)
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*MerchantClaims)
	if !ok || !token.Valid || claims.TokenType != TokenTypeMerchant || claims.AccountID <= 0 || claims.MerchantID <= 0 {
		return nil, fmt.Errorf("token 无效")
	}
	return claims, nil
}
