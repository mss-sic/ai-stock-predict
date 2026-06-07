package handler

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"strings"
	"strconv"
	"time"

	"github.com/ai-stock-predict/server/internal/db"
	"github.com/ai-stock-predict/server/internal/model"
	"github.com/ai-stock-predict/server/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var jwtSecret []byte

func init() {
	s := os.Getenv("JWT_SECRET")
	if s == "" {
		s = "ai-stock-predict-jwt-secret-key-change-in-production"
	}
	jwtSecret = []byte(s)
}

// ── JWT Helpers ──

type Claims struct {
	UserID    uint   `json:"uid"`
	Username  string `json:"uname"`
	Role      string `json:"role"`
	SessionID uint   `json:"sid"`
	jwt.RegisteredClaims
}

func generateAccessToken(user *model.User, sessionID uint) (string, error) {
	claims := Claims{
		UserID:    user.ID,
		Username:  user.Username,
		Role:      user.Role,
		SessionID: sessionID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(2 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "ai-stock-predict",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

func generateRefreshToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func deviceFingerprint(c *gin.Context) string {
	ua := c.GetHeader("User-Agent")
	ip := c.ClientIP()
	h := sha256.Sum256([]byte(ua + "|" + ip))
	return hex.EncodeToString(h[:])
}

func parseToken(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		return jwtSecret, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	return claims, nil
}

// ── Middleware ──

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := c.GetHeader("Authorization")
		if auth == "" {
			// Also check query param for SSE EventSource which can't set headers
			auth = c.Query("token")
			if auth != "" {
				auth = "Bearer " + auth
			}
		}
		if auth == "" {
			response.Unauthorized(c, "未登录")
			return
		}
		tokenStr := strings.TrimPrefix(auth, "Bearer ")
		claims, err := parseToken(tokenStr)
		if err != nil {
			response.Unauthorized(c, "登录已过期，请重新登录")
			return
		}
		// Verify session is still active (not kicked)
		var session model.Session
		sid := claims.SessionID
		if sid == 0 {
			// Legacy token without session ID — fallback to access_token lookup
			if err := db.MySQL.Where("access_token = ? AND is_active = true", tokenStr).First(&session).Error; err != nil {
				response.Unauthorized(c, "会话已失效，请重新登录")
				return
			}
		} else {
			if err := db.MySQL.Where("id = ? AND is_active = true", sid).First(&session).Error; err != nil {
				response.Unauthorized(c, "会话已失效，请重新登录")
				return
			}
		}

		c.Set("userId", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("role", claims.Role)

		// Update heartbeat using session ID from claims
		go func() {
			now := time.Now()
			if claims.SessionID > 0 {
				db.MySQL.Model(&model.Session{}).
					Where("id = ? AND is_active = true", claims.SessionID).
					Updates(map[string]interface{}{"is_online": true, "last_heartbeat": &now})
			} else {
				// Legacy fallback
				db.MySQL.Model(&model.Session{}).
					Where("access_token = ? AND is_active = true", tokenStr).
					Updates(map[string]interface{}{"is_online": true, "last_heartbeat": &now})
			}
		}()

		c.Next()
	}
}

func AdminMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, _ := c.Get("role")
		if role != "admin" {
			response.Forbidden(c, "仅管理员可操作")
			return
		}
		c.Next()
	}
}

// ── Auth Handler ──

type AuthHandler struct{}

func NewAuthHandler() *AuthHandler { return &AuthHandler{} }

func (h *AuthHandler) Login(c *gin.Context) {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Username == "" || body.Password == "" {
		response.BadRequest(c, "请输入用户名和密码")
		return
	}

	var user model.User
	if err := db.MySQL.Where("username = ? AND is_active = true", body.Username).First(&user).Error; err != nil {
		recordLoginLog(0, body.Username, "failed", c.ClientIP(), c.GetHeader("User-Agent"), deviceFingerprint(c), false, "用户不存在或已停用")
		response.Unauthorized(c, "用户名或密码错误")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(body.Password)); err != nil {
		recordLoginLog(user.ID, body.Username, "failed", c.ClientIP(), c.GetHeader("User-Agent"), deviceFingerprint(c), false, "密码错误")
		response.Unauthorized(c, "用户名或密码错误")
		return
	}

	// generate initial accessToken, will be regenerated after session creation
	accessToken, err := generateAccessToken(&user, 0)
	if err != nil {
		response.InternalError(c, "生成token失败")
		return
	}

	refreshToken, err := generateRefreshToken()
	if err != nil {
		response.InternalError(c, "生成refresh token失败")
		return
	}

	fp := deviceFingerprint(c)
	deviceInfo := c.GetHeader("User-Agent")
	if len(deviceInfo) > 200 {
		deviceInfo = deviceInfo[:200]
	}

	session := model.Session{
		UserID:       user.ID,
		RefreshToken: refreshToken,
		AccessToken:  "",  // will be set after we generate token with session ID
		DeviceFp:     fp,
		DeviceInfo:   deviceInfo,
		IPAddress:    c.ClientIP(),
		IsActive:     true,
		ExpiresAt:    time.Now().Add(7 * 24 * time.Hour),
	}
	if err := db.MySQL.Create(&session).Error; err != nil {
		log.Printf("[auth] create session error: %v", err)
		response.InternalError(c, "创建会话失败")
		return
	}

	// Regenerate token with real session ID
	accessToken, err = generateAccessToken(&user, session.ID)
	if err != nil {
		response.InternalError(c, "生成token失败")
		return
	}
	// Store the final access token in session
	db.MySQL.Model(&session).Update("access_token", accessToken)

	// Deactivate previous sessions from the same device
	db.MySQL.Model(&model.Session{}).
		Where("user_id = ? AND device_fp = ? AND id != ? AND is_active = true", user.ID, fp, session.ID).
		Updates(map[string]interface{}{"is_active": false, "is_online": false})

	// Record login log
	recordLoginLog(user.ID, user.Username, "login", c.ClientIP(), deviceInfo, fp, true, "")

	now := time.Now()
	changed := false
	if user.LastDeviceFp != "" && user.LastDeviceFp != fp {
		changed = true
	}
	db.MySQL.Model(&user).Updates(map[string]interface{}{
		"last_login_at":  &now,
		"last_login_ip":  c.ClientIP(),
		"last_device_fp": fp,
	})

	response.Success(c, gin.H{
		"accessToken":    accessToken,
		"refreshToken":   refreshToken,
		"user":           gin.H{"id": user.ID, "username": user.Username, "nickname": user.Nickname, "role": user.Role},
		"deviceChanged":  changed,
		"requires2FA":    user.Require2FA,
	})
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	var body struct {
		RefreshToken string `json:"refreshToken"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.RefreshToken == "" {
		response.BadRequest(c, "缺少refreshToken")
		return
	}

	var session model.Session
	if err := db.MySQL.Where("refresh_token = ? AND is_active = true", body.RefreshToken).First(&session).Error; err != nil {
		response.Unauthorized(c, "无效的refresh token")
		return
	}

	if time.Now().After(session.ExpiresAt) {
		db.MySQL.Model(&session).Update("is_active", false)
		response.Unauthorized(c, "refresh token已过期，请重新登录")
		return
	}

	var user model.User
	if err := db.MySQL.First(&user, session.UserID).Error; err != nil {
		response.Unauthorized(c, "用户不存在")
		return
	}

	accessToken, _ := generateAccessToken(&user, session.ID)
	newRefreshToken, _ := generateRefreshToken()

	session.RefreshToken = newRefreshToken
	session.AccessToken = accessToken
	session.DeviceFp = deviceFingerprint(c)
	session.IPAddress = c.ClientIP()
	session.ExpiresAt = time.Now().Add(7 * 24 * time.Hour)
	db.MySQL.Save(&session)

	response.Success(c, gin.H{
		"accessToken":  accessToken,
		"refreshToken": newRefreshToken,
	})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	uid, _ := c.Get("userId")
	uname, _ := c.Get("username")
	var body struct {
		RefreshToken string `json:"refreshToken"`
	}
	c.ShouldBindJSON(&body)
	if body.RefreshToken != "" {
		db.MySQL.Model(&model.Session{}).Where("refresh_token = ?", body.RefreshToken).Update("is_active", false)
	}
	recordLoginLog(uid.(uint), uname.(string), "logout", c.ClientIP(), c.GetHeader("User-Agent"), deviceFingerprint(c), true, "")
	response.SuccessMsg(c, "ok")
}

func (h *AuthHandler) KickUser(c *gin.Context) {
	var body struct {
		UserId uint `json:"userId"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	// Revoke all active sessions for this user
	db.MySQL.Model(&model.Session{}).Where("user_id = ? AND is_active = true", body.UserId).Updates(map[string]interface{}{
		"is_active": false,
		"is_online": false,
	})
	// Record kick log
	var kickedUser model.User
	if err := db.MySQL.First(&kickedUser, body.UserId).Error; err == nil {
		adminName, _ := c.Get("username")
		recordLoginLog(body.UserId, kickedUser.Username, "kicked", c.ClientIP(), c.GetHeader("User-Agent"), "", true, "被管理员 "+adminName.(string)+" 强制下线")
	}
	response.SuccessMsg(c, "用户已被强制下线")
}



func (h *AuthHandler) Me(c *gin.Context) {
	uid, _ := c.Get("userId")
	var user model.User
	if err := db.MySQL.First(&user, uid).Error; err != nil {
		response.NotFound(c, "用户不存在")
		return
	}
	response.Success(c, gin.H{
		"id":           user.ID,
		"username":     user.Username,
		"nickname":     user.Nickname,
		"role":         user.Role,
		"require2fa":   user.Require2FA,
		"lastLoginAt":  user.LastLoginAt,
		"lastLoginIp":  user.LastLoginIP,
	})
}

func (h *AuthHandler) ChangePassword(c *gin.Context) {
	uid, _ := c.Get("userId")
	var body struct {
		OldPassword string `json:"oldPassword"`
		NewPassword string `json:"newPassword"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.OldPassword == "" || body.NewPassword == "" {
		response.BadRequest(c, "请输入新旧密码")
		return
	}
	if len(body.NewPassword) < 6 {
		response.BadRequest(c, "新密码至少6位")
		return
	}

	var user model.User
	if err := db.MySQL.First(&user, uid).Error; err != nil {
		response.NotFound(c, "用户不存在")
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(body.OldPassword)); err != nil {
		response.BadRequest(c, "旧密码错误")
		return
	}

	hash, _ := bcrypt.GenerateFromPassword([]byte(body.NewPassword), bcrypt.DefaultCost)
	db.MySQL.Model(&user).Update("password_hash", string(hash))

	// Invalidate all other sessions
	db.MySQL.Model(&model.Session{}).Where("user_id = ? AND id != ?", uid, 0).Update("is_active", false)

	response.SuccessMsg(c, "密码修改成功，其他设备已下线")
}





func (h *AuthHandler) UpdateProfile(c *gin.Context) {
	uid, _ := c.Get("userId")
	var body struct {
		Nickname string `json:"nickname"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}

	if body.Nickname == "" {
		response.BadRequest(c, "昵称不能为空")
		return
	}
	if len(body.Nickname) > 20 {
		response.BadRequest(c, "昵称长度不能超过20个字符")
		return
	}

	var user model.User
	if err := db.MySQL.First(&user, uid).Error; err != nil {
		response.NotFound(c, "用户不存在")
		return
	}

	db.MySQL.Model(&user).Update("nickname", body.Nickname)
	response.Success(c, gin.H{"nickname": body.Nickname})
}
// ── Login Log Helpers ──

func recordLoginLog(userID uint, username, action, ip, deviceInfo, deviceFp string, success bool, failReason string) {
	log := model.LoginLog{
		UserID:     userID,
		Username:   username,
		Action:     action,
		Success:    success,
		IPAddress:  ip,
		DeviceInfo: deviceInfo,
		DeviceFp:   deviceFp,
		FailReason: failReason,
	}
	db.MySQL.Create(&log)
}
// ── Heartbeat ──

func (h *AuthHandler) Heartbeat(c *gin.Context) {
	uid, _ := c.Get("userId")
	authHeader := c.GetHeader("Authorization")
	accessToken := strings.TrimPrefix(authHeader, "Bearer ")

	now := time.Now()
	// Update session heartbeat
	db.MySQL.Model(&model.Session{}).
		Where("access_token = ? AND user_id = ? AND is_active = true", accessToken, uid).
		Updates(map[string]interface{}{"is_online": true, "last_heartbeat": &now})

	response.SuccessMsg(c, "ok")
}

// ── Admin: User Management ──

type UserWithStatus struct {
	model.User
	IsOnline      bool       `json:"isOnline"`
	LastHeartbeat *time.Time `json:"lastHeartbeat"`
	DeviceInfo    string     `json:"deviceInfo"`
	SessionIP     string     `json:"sessionIp"`
	SessionCount  int        `json:"sessionCount"`
}

func (h *AuthHandler) ListUsers(c *gin.Context) {
	var users []model.User
	db.MySQL.Select("id", "username", "role", "is_active", "last_login_at", "last_login_ip", "created_at", "updated_at").
		Order("id").Find(&users)

	role, _ := c.Get("role")
	uid, _ := c.Get("userId")

	result := make([]UserWithStatus, 0, len(users))
	for _, u := range users {
		// Regular users can only see themselves
		if role != "admin" && u.ID != uid.(uint) {
			continue
		}

		us := UserWithStatus{User: u}
		// Get latest active session for this user
		var sess model.Session
		err := db.MySQL.Where("user_id = ? AND is_active = true", u.ID).
			Order("CASE WHEN last_heartbeat IS NOT NULL THEN 0 ELSE 1 END, last_heartbeat DESC, created_at DESC").First(&sess).Error
		if err == nil {
			us.IsOnline = sess.IsOnline
			us.LastHeartbeat = sess.LastHeartbeat
			us.DeviceInfo = sess.DeviceInfo
			us.SessionIP = sess.IPAddress
		}
		// Count active sessions
		var count int64
		db.MySQL.Model(&model.Session{}).Where("user_id = ? AND is_active = true", u.ID).Count(&count)
		us.SessionCount = int(count)
		result = append(result, us)
	}

	response.Success(c, result)
}

func (h *AuthHandler) CreateUser(c *gin.Context) {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Username == "" || body.Password == "" {
		response.BadRequest(c, "用户名和密码必填")
		return
	}
	if len(body.Password) < 6 {
		response.BadRequest(c, "密码至少6位")
		return
	}
	if body.Role == "" {
		body.Role = "user"
	}

	var existing model.User
	if db.MySQL.Where("username = ?", body.Username).First(&existing).Error == nil {
		response.Conflict(c, "用户名已存在")
		return
	}

	hash, _ := bcrypt.GenerateFromPassword([]byte(body.Password), bcrypt.DefaultCost)
	user := model.User{
		Username:     body.Username,
		PasswordHash: string(hash),
		Role:         body.Role,
		IsActive:     true,
	}
	db.MySQL.Create(&user)

	response.Created(c, gin.H{"id": user.ID, "username": user.Username, "role": user.Role})
}

func (h *AuthHandler) ResetPassword(c *gin.Context) {
	var body struct {
		UserId      uint   `json:"userId"`
		NewPassword string `json:"newPassword"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if len(body.NewPassword) < 6 {
		response.BadRequest(c, "新密码至少6位")
		return
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte(body.NewPassword), bcrypt.DefaultCost)
	db.MySQL.Model(&model.User{}).Where("id = ?", body.UserId).Updates(map[string]interface{}{
		"password_hash": string(hash),
	})
	// Invalidate all sessions for this user
	db.MySQL.Model(&model.Session{}).Where("user_id = ?", body.UserId).Update("is_active", false)
	response.SuccessMsg(c, "密码已重置")
}

func (h *AuthHandler) ToggleUser(c *gin.Context) {
	var body struct {
		UserId   uint `json:"userId"`
		IsActive bool `json:"isActive"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	db.MySQL.Model(&model.User{}).Where("id = ?", body.UserId).Update("is_active", body.IsActive)
	if !body.IsActive {
		db.MySQL.Model(&model.Session{}).Where("user_id = ?", body.UserId).Update("is_active", false)
	}
	response.SuccessMsg(c, "ok")
}

// ── 2FA / Device Verification ──

func (h *AuthHandler) GetSessions(c *gin.Context) {
	uid, _ := c.Get("userId")
	var sessions []model.Session
	db.MySQL.Where("user_id = ? AND is_active = true", uid).Order("created_at DESC").Limit(20).Find(&sessions)
	response.Success(c, sessions)
}

func (h *AuthHandler) RevokeSession(c *gin.Context) {
	uid, _ := c.Get("userId")
	sid := c.Param("id")
	db.MySQL.Model(&model.Session{}).Where("id = ? AND user_id = ?", sid, uid).Update("is_active", false)
	response.SuccessMsg(c, "ok")
}


// ── Login Logs ──

func (h *AuthHandler) ListLoginLogs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "30"))
	action := c.Query("action")
	username := c.Query("username")

	query := db.MySQL.Model(&model.LoginLog{})
	if action != "" {
		query = query.Where("action = ?", action)
	}
	if username != "" {
		query = query.Where("username LIKE ?", "%"+username+"%")
	}

	var total int64
	query.Count(&total)

	var logs []model.LoginLog
	query.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&logs)

	response.Success(c, gin.H{"data": logs, "total": total, "page": page, "pageSize": pageSize})
}

// ── Init admin account ──

func EnsureAdminUser() {
	var count int64
	db.MySQL.Model(&model.User{}).Count(&count)
	if count > 0 {
		return
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
	admin := model.User{
		Username:     "admin",
		PasswordHash: string(hash),
		Role:         "admin",
		IsActive:     true,
	}
	db.MySQL.Create(&admin)
	log.Println("[auth] 初始化管理员账号: admin / admin123 (请立即修改密码)")
}
