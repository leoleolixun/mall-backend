package password

import "golang.org/x/crypto/bcrypt"

// 使用 bcrypt 对密码进行哈希处理
func HashPassword(raw string) (string, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(raw), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashed), nil
}

// 检查原始密码与哈希密码是否匹配
func CheckPassword(hash string, raw string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(raw)) == nil
}
