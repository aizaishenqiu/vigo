package controller

import (
	"time"

	"vigo/config"
	"vigo/framework/middleware"
	"vigo/framework/mvc"
	"vigo/framework/validate"
)

// AuthController 认证控制器
type AuthController struct {
	BaseController
}

// LoginRequest 登录请求
type LoginRequest struct {
	Username string `json:"username" form:"username"`
	Password string `json:"password" form:"password"`
}

// LoginResponse 登录响应
type LoginResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	User      UserInfo  `json:"user"`
}

// UserInfo 用户信息
type UserInfo struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

// Login 登录接口
// @Summary 用户登录
// @Description 验证用户名和密码并返回JWT Token
// @Tags 认证
// @Accept  json
// @Produce  json
// @Param request body LoginRequest true "登录信息"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Router /api/login [post]
func (c *AuthController) Login(ctx *mvc.Context) {
	c.Init(ctx)

	var req LoginRequest
	if err := ctx.BindJSON(&req); err != nil {
		c.Error(400, "Invalid request format")
		return
	}

	v := validate.New()
	v.Required("username", req.Username)
	v.Required("password", req.Password)
	v.Min("password", req.Password, config.App.Security.Password.MinLength)

	if !v.IsValid() {
		c.Error(400, "Validation failed")
		return
	}

	user, err := authenticateUser(req.Username, req.Password)
	if err != nil {
		c.Error(401, "Invalid username or password")
		return
	}

	token, err := middleware.GetJWTService().GenerateToken(user.ID, user.Username, user.Role)
	if err != nil {
		c.Error(500, "Failed to generate token")
		return
	}

	expiresAt := time.Now().Add(time.Duration(config.App.Security.JWT.Expire) * time.Second)

	c.Success(LoginResponse{
		Token:     token,
		ExpiresAt: expiresAt,
		User: UserInfo{
			ID:       user.ID,
			Username: user.Username,
			Role:     user.Role,
		},
	})
}

// RefreshToken 刷新 Token
// @Summary 刷新Token
// @Description 使用当前Token刷新获取新Token
// @Tags 认证
// @Accept  json
// @Produce  json
// @Security Bearer
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Router /api/refresh [post]
func (c *AuthController) RefreshToken(ctx *mvc.Context) {
	c.Init(ctx)

	authHeader := ctx.Request.Header.Get("Authorization")
	if authHeader == "" {
		c.Error(401, "Missing Authorization header")
		return
	}

	token := extractToken(authHeader)
	if token == "" {
		c.Error(401, "Invalid Authorization header")
		return
	}

	newToken, err := middleware.GetJWTService().RefreshToken(token)
	if err != nil {
		c.Error(401, "Failed to refresh token")
		return
	}

	expiresAt := time.Now().Add(time.Duration(config.App.Security.JWT.Expire) * time.Second)

	c.Success(map[string]interface{}{
		"token":      newToken,
		"expires_at": expiresAt,
	})
}

// Logout 登出接口
// @Summary 用户登出
// @Description 登出当前用户（客户端需删除Token）
// @Tags 认证
// @Accept  json
// @Produce  json
// @Security Bearer
// @Success 200 {object} map[string]interface{}
// @Router /api/logout [post]
func (c *AuthController) Logout(ctx *mvc.Context) {
	c.Init(ctx)
	c.Success(map[string]string{"message": "Logged out successfully"})
}

// GetCurrentUser 获取当前用户信息
// @Summary 获取当前用户
// @Description 获取当前登录用户的详细信息
// @Tags 认证
// @Accept  json
// @Produce  json
// @Security Bearer
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Router /api/user [get]
func (c *AuthController) GetCurrentUser(ctx *mvc.Context) {
	c.Init(ctx)

	userID, username, role, ok := middleware.GetCurrentUser(ctx)
	if !ok {
		c.Error(401, "Unauthorized")
		return
	}

	c.Success(UserInfo{
		ID:       userID,
		Username: username,
		Role:     role,
	})
}

// authenticateUser 验证用户凭据
// 注意：这是一个示例实现，实际应用中应该从数据库查询用户并验证密码
func authenticateUser(username, password string) (*UserInfo, error) {
	// TODO: 实现数据库用户验证
	// 示例：从数据库查询用户
	// user, err := userModel.FindByUsername(username)
	// if err != nil {
	//     return nil, err
	// }
	// if !middleware.PasswordVerifyHash(user.Password, password) {
	//     return nil, errors.New("invalid password")
	// }
	// return &UserInfo{ID: user.ID, Username: user.Username, Role: user.Role}, nil

	// 开发环境：禁止使用硬编码密码
	// 生产环境必须实现数据库验证
	return nil, ErrUserNotFound
}

// ErrUserNotFound 用户未找到错误
var ErrUserNotFound = &AuthError{Message: "user not found"}

// AuthError 认证错误
type AuthError struct {
	Message string
}

func (e *AuthError) Error() string {
	return e.Message
}

// extractToken 从 Authorization header 提取 token
func extractToken(authHeader string) string {
	if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
		return authHeader[7:]
	}
	return ""
}
