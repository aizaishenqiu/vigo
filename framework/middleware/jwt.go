package middleware

import (
	"errors"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"vigo/config"
	"vigo/framework/mvc"

	"github.com/golang-jwt/jwt/v5"
)

// JWTClaims JWT 声明
type JWTClaims struct {
	UserID   uint   `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// JWTService JWT 服务
type JWTService struct {
	secret []byte
	expire time.Duration
	issuer string
}

var jwtService *JWTService

// InitJWT 初始化 JWT 服务
func InitJWT() {
	jwtService = &JWTService{
		secret: []byte(config.App.Security.JWT.Secret),
		expire: time.Duration(config.App.Security.JWT.Expire) * time.Second,
		issuer: config.App.Security.JWT.Issuer,
	}
}

// GetJWTService 获取 JWT 服务实例
func GetJWTService() *JWTService {
	if jwtService == nil {
		InitJWT()
	}
	return jwtService
}

// GenerateToken 生成 JWT Token
func (s *JWTService) GenerateToken(userID uint, username, role string) (string, error) {
	claims := JWTClaims{
		UserID:   userID,
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.expire)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    s.issuer,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secret)
}

// ParseToken 解析 JWT Token
func (s *JWTService) ParseToken(tokenString string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return s.secret, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*JWTClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token")
}

// RefreshToken 刷新 Token
func (s *JWTService) RefreshToken(tokenString string) (string, error) {
	claims, err := s.ParseToken(tokenString)
	if err != nil {
		return "", err
	}

	return s.GenerateToken(claims.UserID, claims.Username, claims.Role)
}

// JWTBlacklist JWT 黑名单
var JWTBlacklist = &sync.Map{}

// JWTAuth JWT 认证中间件
func JWTAuth() mvc.HandlerFunc {
	return func(c *mvc.Context) {
		if isInternalPath(c.Request.URL.Path) {
			c.Next()
			return
		}

		authHeader := c.Request.Header.Get("Authorization")
		if authHeader == "" {
			c.Error(http.StatusUnauthorized, "Missing Authorization header")
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.Error(http.StatusUnauthorized, "Invalid Authorization header format")
			c.Abort()
			return
		}

		tokenString := parts[1]

		// 检查是否在黑名单中
		if isTokenRevoked(tokenString) {
			c.Error(http.StatusUnauthorized, "Token has been revoked")
			c.Abort()
			return
		}

		claims, err := GetJWTService().ParseToken(tokenString)
		if err != nil {
			if errors.Is(err, jwt.ErrTokenExpired) {
				c.Error(http.StatusUnauthorized, "Token expired")
			} else {
				c.Error(http.StatusUnauthorized, "Invalid token")
			}
			c.Abort()
			return
		}

		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("role", claims.Role)
		c.Set("claims", claims)

		c.Next()
	}
}

// RevokeToken 将 token 加入黑名单
func RevokeToken(tokenString string, expireDuration time.Duration) error {
	JWTBlacklist.Store(tokenString, time.Now().Add(expireDuration))
	return nil
}

// isTokenRevoked 检查 token 是否已被撤销
func isTokenRevoked(tokenString string) bool {
	if val, exists := JWTBlacklist.Load(tokenString); exists {
		if expiry, ok := val.(time.Time); ok {
			if time.Now().After(expiry) {
				// 已过期，从黑名单中移除
				JWTBlacklist.Delete(tokenString)
				return false
			}
			return true
		}
	}
	return false
}

// OptionalJWT 可选 JWT 认证中间件（不强制要求登录）
func OptionalJWT() mvc.HandlerFunc {
	return func(c *mvc.Context) {
		authHeader := c.Request.Header.Get("Authorization")
		if authHeader == "" {
			c.Next()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.Next()
			return
		}

		tokenString := parts[1]
		claims, err := GetJWTService().ParseToken(tokenString)
		if err == nil {
			c.Set("user_id", claims.UserID)
			c.Set("username", claims.Username)
			c.Set("role", claims.Role)
			c.Set("claims", claims)
		}

		c.Next()
	}
}

// RoleAuth 角色权限中间件
func RoleAuth(allowedRoles ...string) mvc.HandlerFunc {
	return func(c *mvc.Context) {
		role, exists := c.Get("role")
		if !exists {
			c.Error(http.StatusUnauthorized, "Unauthorized")
			c.Abort()
			return
		}

		userRole := role.(string)
		allowed := false
		for _, r := range allowedRoles {
			if r == userRole || r == "*" {
				allowed = true
				break
			}
		}

		if !allowed {
			log.Printf("[Auth] 权限不足: role=%s, required=%v", userRole, allowedRoles)
			c.Error(http.StatusForbidden, "Permission denied")
			c.Abort()
			return
		}

		c.Next()
	}
}

// GetCurrentUser 获取当前登录用户信息
func GetCurrentUser(c *mvc.Context) (uint, string, string, bool) {
	userID, exists := c.Get("user_id")
	if !exists {
		return 0, "", "", false
	}
	username, _ := c.Get("username")
	role, _ := c.Get("role")

	return userID.(uint), username.(string), role.(string), true
}

// MustGetCurrentUser 获取当前登录用户信息（必须登录）
func MustGetCurrentUser(c *mvc.Context) (uint, string, string) {
	userID, username, role, ok := GetCurrentUser(c)
	if !ok {
		panic("user not authenticated")
	}
	return userID, username, role
}
