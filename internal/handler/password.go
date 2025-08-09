package handler

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// hashPassword 使用bcrypt加密密码
func hashPassword(password string) (string, error) {
	// 使用bcrypt生成密码哈希，cost=12为推荐值
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}
	return string(hash), nil
}

// verifyPassword 验证密码
func verifyPassword(password, hash string) (bool, error) {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if err != nil {
		if err == bcrypt.ErrMismatchedHashAndPassword {
			return false, nil // 密码不匹配，但这不是系统错误
		}
		return false, fmt.Errorf("password verification failed: %w", err)
	}
	return true, nil
}