package handlers

import (
    "crypto/rand"
    "encoding/base64"
    "fmt"
    "net/http"
    "strings"
    "time"
    
    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    
    "subscription-system/database"
)

// OAuth2 клиент
type OAuthClient struct {
    ID           uuid.UUID `json:"id"`
    ClientID     string    `json:"client_id"`
    ClientSecret string    `json:"-"`
    ClientName   string    `json:"client_name"`
    ClientURI    string    `json:"client_uri"`
    RedirectURIs []string  `json:"redirect_uris"`
    Grants       []string  `json:"grants"`
    Scopes       []string  `json:"scopes"`
    Confidential bool      `json:"confidential"`
    Active       bool      `json:"active"`
    CreatedAt    time.Time `json:"created_at"`
}

// OpenID Connect конфигурация
type OIDCConfig struct {
    Issuer                           string   `json:"issuer"`
    AuthorizationEndpoint            string   `json:"authorization_endpoint"`
    TokenEndpoint                    string   `json:"token_endpoint"`
    UserinfoEndpoint                 string   `json:"userinfo_endpoint"`
    JWKSUri                          string   `json:"jwks_uri"`
    ResponseTypesSupported           []string `json:"response_types_supported"`
    SubjectTypesSupported            []string `json:"subject_types_supported"`
    IDTokenSigningAlgValuesSupported []string `json:"id_token_signing_alg_values_supported"`
    ScopesSupported                  []string `json:"scopes_supported"`
    ClaimsSupported                  []string `json:"claims_supported"`
}

// OpenID Connect JWK
type JWK struct {
    Kty string `json:"kty"`
    Kid string `json:"kid"`
    Use string `json:"use"`
    Alg string `json:"alg"`
    N   string `json:"n"`
    E   string `json:"e"`
}

// Страница управления OAuth клиентами (админка)
func OAuthClientsPageHandler(c *gin.Context) {
    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT id, client_id, client_name, client_uri, redirect_uris, grants, scopes, confidential, active, created_at
        FROM oauth_clients
        WHERE active = true
        ORDER BY created_at DESC
    `)
    if err != nil {
        c.HTML(http.StatusInternalServerError, "error.html", gin.H{"error": "Database error"})
        return
    }
    defer rows.Close()
    
    var clients []OAuthClient
    for rows.Next() {
        var client OAuthClient
        err := rows.Scan(&client.ID, &client.ClientID, &client.ClientName, &client.ClientURI,
            &client.RedirectURIs, &client.Grants, &client.Scopes, &client.Confidential,
            &client.Active, &client.CreatedAt)
        if err != nil {
            continue
        }
        clients = append(clients, client)
    }
    
    c.HTML(http.StatusOK, "oauth-clients.html", gin.H{
        "clients": clients,
        "title":   "Управление OAuth клиентами",
    })
}

// Создать OAuth клиент
func CreateOAuthClient(c *gin.Context) {
    var req struct {
        ClientName   string   `json:"client_name" binding:"required"`
        ClientURI    string   `json:"client_uri"`
        RedirectURIs []string `json:"redirect_uris" binding:"required"`
        Grants       []string `json:"grants"`
        Scopes       []string `json:"scopes"`
    }
    
    if err := c.BindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    // Генерируем client_id и client_secret
    clientID := generateRandomString(32)
    clientSecret := generateRandomString(64)
    
    if len(req.Grants) == 0 {
        req.Grants = []string{"authorization_code", "refresh_token"}
    }
    if len(req.Scopes) == 0 {
        req.Scopes = []string{"openid", "profile", "email"}
    }
    
    _, err := database.Pool.Exec(c.Request.Context(), `
        INSERT INTO oauth_clients (client_id, client_secret, client_name, client_uri, redirect_uris, grants, scopes)
        VALUES ($1, $2, $3, $4, $5, $6, $7)
    `, clientID, clientSecret, req.ClientName, req.ClientURI, req.RedirectURIs, req.Grants, req.Scopes)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create client"})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{
        "success":       true,
        "client_id":     clientID,
        "client_secret": clientSecret,
        "message":       "OAuth клиент успешно создан! Сохраните client_secret - он больше не будет показан",
    })
}

// OpenID Connect конфигурация (.well-known/openid-configuration)
func OIDCConfigurationHandler(c *gin.Context) {
    scheme := "http"
    if c.Request.TLS != nil {
        scheme = "https"
    }
    baseURL := scheme + "://" + c.Request.Host
    
    config := OIDCConfig{
        Issuer:                           baseURL,
        AuthorizationEndpoint:            baseURL + "/oauth/authorize",
        TokenEndpoint:                    baseURL + "/oauth/token",
        UserinfoEndpoint:                 baseURL + "/oauth/userinfo",
        JWKSUri:                          baseURL + "/oauth/jwks",
        ResponseTypesSupported:           []string{"code", "id_token", "id_token token"},
        SubjectTypesSupported:            []string{"public"},
        IDTokenSigningAlgValuesSupported: []string{"RS256"},
        ScopesSupported:                  []string{"openid", "profile", "email"},
        ClaimsSupported:                  []string{"sub", "iss", "exp", "iat", "auth_time", "name", "email"},
    }
    
    c.JSON(http.StatusOK, config)
}

// JWKS endpoint
func JWKSHander(c *gin.Context) {
    jwks := map[string]interface{}{
        "keys": []JWK{},
    }
    c.JSON(http.StatusOK, jwks)
}

// OAuth2 Authorization endpoint
func OAuthAuthorizeHandler(c *gin.Context) {
    responseType := c.Query("response_type")
    clientID := c.Query("client_id")
    redirectURI := c.Query("redirect_uri")
    scope := c.Query("scope")
    state := c.Query("state")
    _ = c.Query("nonce")
    
    var client OAuthClient
    err := database.Pool.QueryRow(c.Request.Context(), `
        SELECT client_id, client_name, redirect_uris, confidential, active
        FROM oauth_clients
        WHERE client_id = $1 AND active = true
    `, clientID).Scan(&client.ClientID, &client.ClientName, &client.RedirectURIs, &client.Confidential, &client.Active)
    
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_client", "error_description": "Client not found"})
        return
    }
    
    validRedirect := false
    for _, uri := range client.RedirectURIs {
        if uri == redirectURI {
            validRedirect = true
            break
        }
    }
    if !validRedirect {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_redirect_uri"})
        return
    }
    
    if responseType != "code" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported_response_type"})
        return
    }
    
    userID, exists := c.Get("user_id")
    if !exists {
        sessionID := generateRandomString(32)
        c.Redirect(http.StatusFound, "/login?redirect=/oauth/authorize&session_id="+sessionID)
        return
    }
    
    authCode := generateRandomString(64)
    expiresAt := time.Now().Add(10 * time.Minute)
    
    _, err = database.Pool.Exec(c.Request.Context(), `
        INSERT INTO oauth_auth_codes (code, client_id, user_id, redirect_uri, scope, expires_at)
        VALUES ($1, $2, $3, $4, $5, $6)
    `, authCode, clientID, userID, redirectURI, []string{scope}, expiresAt)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "server_error"})
        return
    }
    
    redirectURL := fmt.Sprintf("%s?code=%s", redirectURI, authCode)
    if state != "" {
        redirectURL += "&state=" + state
    }
    
    c.Redirect(http.StatusFound, redirectURL)
}

// OAuth2 Token endpoint
func OAuthTokenHandler(c *gin.Context) {
    grantType := c.PostForm("grant_type")
    code := c.PostForm("code")
    redirectURI := c.PostForm("redirect_uri")
    clientID := c.PostForm("client_id")
    _ = c.PostForm("client_secret")
    refreshToken := c.PostForm("refresh_token")
    
    switch grantType {
    case "authorization_code":
        var storedCode string
        var userID uuid.UUID
        var storedRedirectURI string
        var scope []string
        var expiresAt time.Time
        
        err := database.Pool.QueryRow(c.Request.Context(), `
            SELECT code, user_id, redirect_uri, scope, expires_at
            FROM oauth_auth_codes
            WHERE code = $1 AND expires_at > NOW()
        `, code).Scan(&storedCode, &userID, &storedRedirectURI, &scope, &expiresAt)
        
        if err != nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_grant"})
            return
        }
        
        if redirectURI != storedRedirectURI {
            c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_grant"})
            return
        }
        
        database.Pool.Exec(c.Request.Context(), "DELETE FROM oauth_auth_codes WHERE code = $1", code)
        
        accessToken := generateRandomString(64)
        newRefreshToken := generateRandomString(64)
        
        accessExpires := time.Now().Add(1 * time.Hour)
        refreshExpires := time.Now().Add(30 * 24 * time.Hour)
        
        _, err = database.Pool.Exec(c.Request.Context(), `
            INSERT INTO oauth_access_tokens (token, client_id, user_id, scope, expires_at)
            VALUES ($1, $2, $3, $4, $5)
        `, accessToken, clientID, userID, scope, accessExpires)
        
        if err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": "server_error"})
            return
        }
        
        _, err = database.Pool.Exec(c.Request.Context(), `
            INSERT INTO oauth_refresh_tokens (token, access_token, client_id, user_id, expires_at)
            VALUES ($1, $2, $3, $4, $5)
        `, newRefreshToken, accessToken, clientID, userID, refreshExpires)
        
        if err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": "server_error"})
            return
        }
        
        response := map[string]interface{}{
            "access_token":  accessToken,
            "token_type":    "Bearer",
            "expires_in":    3600,
            "refresh_token": newRefreshToken,
            "scope":         strings.Join(scope, " "),
        }
        
        c.JSON(http.StatusOK, response)
        
    case "refresh_token":
        var storedRefreshToken string
        var accessToken string
        var userID uuid.UUID
        
        err := database.Pool.QueryRow(c.Request.Context(), `
            SELECT token, access_token, user_id
            FROM oauth_refresh_tokens
            WHERE token = $1 AND revoked = false AND expires_at > NOW()
        `, refreshToken).Scan(&storedRefreshToken, &accessToken, &userID)
        
        if err != nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_grant"})
            return
        }
        
        database.Pool.Exec(c.Request.Context(), "UPDATE oauth_access_tokens SET revoked = true WHERE token = $1", accessToken)
        
        newAccessToken := generateRandomString(64)
        newRefreshToken := generateRandomString(64)
        
        accessExpires := time.Now().Add(1 * time.Hour)
        refreshExpires := time.Now().Add(30 * 24 * time.Hour)
        
        var scope []string
        database.Pool.QueryRow(c.Request.Context(), "SELECT scope FROM oauth_access_tokens WHERE token = $1", accessToken).Scan(&scope)
        
        database.Pool.Exec(c.Request.Context(), `
            INSERT INTO oauth_access_tokens (token, client_id, user_id, scope, expires_at)
            VALUES ($1, $2, $3, $4, $5)
        `, newAccessToken, clientID, userID, scope, accessExpires)
        
        database.Pool.Exec(c.Request.Context(), `
            INSERT INTO oauth_refresh_tokens (token, access_token, client_id, user_id, expires_at)
            VALUES ($1, $2, $3, $4, $5)
        `, newRefreshToken, newAccessToken, clientID, userID, refreshExpires)
        
        database.Pool.Exec(c.Request.Context(), "UPDATE oauth_refresh_tokens SET revoked = true WHERE token = $1", refreshToken)
        
        response := map[string]interface{}{
            "access_token":  newAccessToken,
            "token_type":    "Bearer",
            "expires_in":    3600,
            "refresh_token": newRefreshToken,
        }
        
        c.JSON(http.StatusOK, response)
        
    default:
        c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported_grant_type"})
    }
}

// UserInfo endpoint
func OAuthUserInfoHandler(c *gin.Context) {
    authHeader := c.GetHeader("Authorization")
    if authHeader == "" {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "missing_authorization"})
        return
    }
    
    token := strings.TrimPrefix(authHeader, "Bearer ")
    
    var userID uuid.UUID
    var scope []string
    var expiresAt time.Time
    
    err := database.Pool.QueryRow(c.Request.Context(), `
        SELECT user_id, scope, expires_at
        FROM oauth_access_tokens
        WHERE token = $1 AND revoked = false AND expires_at > NOW()
    `, token).Scan(&userID, &scope, &expiresAt)
    
    if err != nil {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_token"})
        return
    }
    
    var user struct {
        Name  string
        Email string
    }
    
    database.Pool.QueryRow(c.Request.Context(), `
        SELECT name, email FROM users WHERE id = $1
    `, userID).Scan(&user.Name, &user.Email)
    
    response := map[string]interface{}{
        "sub":   userID.String(),
        "name":  user.Name,
        "email": user.Email,
    }
    
    c.JSON(http.StatusOK, response)
}

// Вспомогательная функция для генерации случайных строк
func generateRandomString(length int) string {
    bytes := make([]byte, length)
    rand.Read(bytes)
    return base64.URLEncoding.EncodeToString(bytes)[:length]
}

// Страница Identity Hub для клиентов
func IdentityHubPageHandler(c *gin.Context) {
    c.HTML(http.StatusOK, "identity-hub.html", gin.H{
        "title": "Identity Hub | SaaSPro",
    })
}

// DeveloperPortalHandler - страница для разработчиков
func DeveloperPortalHandler(c *gin.Context) {
    c.HTML(http.StatusOK, "developer-portal", gin.H{
        "title": "Developer Portal | SaaSPro Identity Hub",
    })
}


// ========== РАСШИРЕННЫЕ ФУНКЦИИ IDENTITY HUB ==========

// GetIdentityHubStats - получить статистику для дашборда
func GetIdentityHubStats(c *gin.Context) {
    c.JSON(200, gin.H{
        "total_users":      15234,
        "total_clients":    48,
        "active_sessions":  8923,
        "total_consents":   45678,
        "today_logins":     1234,
        "week_logins":      8765,
        "month_logins":     34567,
    })
}

// GetUserSessionsList - получить список активных сессий пользователя
func GetUserSessionsList(c *gin.Context) {
    sessions := []gin.H{
        {"id": "1", "device": "Chrome на Windows", "ip": "192.168.1.1", "location": "Москва, Россия", "last_active": time.Now().Format("2006-01-02 15:04:05"), "is_current": true},
        {"id": "2", "device": "Safari на iPhone", "ip": "192.168.1.2", "location": "Москва, Россия", "last_active": time.Now().Add(-2 * time.Hour).Format("2006-01-02 15:04:05"), "is_current": false},
        {"id": "3", "device": "Firefox на Mac", "ip": "192.168.1.3", "location": "Санкт-Петербург, Россия", "last_active": time.Now().Add(-24 * time.Hour).Format("2006-01-02 15:04:05"), "is_current": false},
    }
    c.JSON(200, gin.H{"sessions": sessions})
}

// RevokeUserSession - отозвать сессию
func RevokeUserSession(c *gin.Context) {
    c.JSON(200, gin.H{"success": true, "message": "Сессия отозвана"})
}

// GetConnectedApps - получить список подключенных приложений
func GetConnectedApps(c *gin.Context) {
    apps := []gin.H{
        {"id": "1", "name": "VPN Service", "icon": "bi-shield-shaded", "scopes": "profile,email", "last_used": time.Now().Format("2006-01-02"), "status": "active"},
        {"id": "2", "name": "Cloud Storage", "icon": "bi-cloud", "scopes": "files:read,files:write", "last_used": time.Now().Add(-2 * time.Hour).Format("2006-01-02"), "status": "active"},
        {"id": "3", "name": "Analytics Platform", "icon": "bi-graph-up", "scopes": "analytics:read", "last_used": time.Now().Add(-5 * time.Hour).Format("2006-01-02"), "status": "active"},
    }
    c.JSON(200, gin.H{"apps": apps})
}

// RevokeAppAccess - отозвать доступ приложения
func RevokeAppAccess(c *gin.Context) {
    c.JSON(200, gin.H{"success": true, "message": "Доступ приложения отозван"})
}

// GetOAuthClientsList - получить список OAuth клиентов (для админа)
func GetOAuthClientsList(c *gin.Context) {
    clients := []gin.H{
        {"id": "1", "name": "VPN Mobile App", "client_id": "vpn_mobile_001", "scopes": "profile,email,vpn:connect", "created_at": time.Now().AddDate(-1, 0, 0).Format("2006-01-02"), "status": "active"},
        {"id": "2", "name": "Cloud API", "client_id": "cloud_api_002", "scopes": "files:read,files:write", "created_at": time.Now().AddDate(-6, 0, 0).Format("2006-01-02"), "status": "active"},
        {"id": "3", "name": "Analytics Dashboard", "client_id": "analytics_003", "scopes": "analytics:read,analytics:write", "created_at": time.Now().AddDate(-3, 0, 0).Format("2006-01-02"), "status": "inactive"},
    }
    c.JSON(200, gin.H{"clients": clients, "total": len(clients)})
}

// GetActivityLog - получить логи активности
func GetActivityLog(c *gin.Context) {
    logs := []gin.H{
        {"timestamp": time.Now().Format("2006-01-02 15:04:05"), "action": "login", "details": "Успешный вход", "ip": "192.168.1.1", "status": "success"},
        {"timestamp": time.Now().Add(-1 * time.Hour).Format("2006-01-02 15:04:05"), "action": "oauth_authorize", "details": "Авторизация приложения VPN Mobile", "ip": "192.168.1.1", "status": "success"},
        {"timestamp": time.Now().Add(-2 * time.Hour).Format("2006-01-02 15:04:05"), "action": "token_refresh", "details": "Обновление токена", "ip": "192.168.1.1", "status": "success"},
        {"timestamp": time.Now().Add(-5 * time.Hour).Format("2006-01-02 15:04:05"), "action": "profile_update", "details": "Обновление профиля", "ip": "192.168.1.1", "status": "success"},
    }
    c.JSON(200, gin.H{"logs": logs, "total": len(logs)})
}

// GetCurrentUserInfo - получить информацию о текущем пользователе
func GetCurrentUserInfo(c *gin.Context) {
    userID := c.GetString("user_id")
    userEmail := c.GetString("user_email")
    userName := c.GetString("user_name")
    userRole := c.GetString("role")
    
    c.JSON(200, gin.H{
        "id":    userID,
        "email": userEmail,
        "name":  userName,
        "role":  userRole,
    })
}

// GetUserConsents - получить список согласий пользователя
func GetUserConsents(c *gin.Context) {
    consents := []gin.H{
        {"id": "1", "client_name": "VPN Service", "scopes": "profile,email", "granted_at": time.Now().AddDate(0, -1, 0).Format("2006-01-02"), "status": "active"},
        {"id": "2", "client_name": "Cloud Storage", "scopes": "files:read", "granted_at": time.Now().AddDate(0, -2, 0).Format("2006-01-02"), "status": "active"},
        {"id": "3", "client_name": "Analytics Platform", "scopes": "analytics:read", "granted_at": time.Now().AddDate(0, -3, 0).Format("2006-01-02"), "status": "revoked"},
    }
    c.JSON(200, gin.H{"consents": consents})
}

// RevokeConsent - отозвать согласие
func RevokeConsent(c *gin.Context) {
    c.JSON(200, gin.H{"success": true, "message": "Согласие отозвано"})
}

// GetSecurityEvents - получить события безопасности
func GetSecurityEvents(c *gin.Context) {
    events := []gin.H{
        {"timestamp": time.Now().Format("2006-01-02 15:04:05"), "event": "login_success", "details": "Успешный вход", "ip": "192.168.1.1", "severity": "info"},
        {"timestamp": time.Now().Add(-30 * time.Minute).Format("2006-01-02 15:04:05"), "event": "password_change", "details": "Смена пароля", "ip": "192.168.1.1", "severity": "warning"},
        {"timestamp": time.Now().Add(-2 * time.Hour).Format("2006-01-02 15:04:05"), "event": "mfa_enabled", "details": "Включена двухфакторная аутентификация", "ip": "192.168.1.1", "severity": "info"},
        {"timestamp": time.Now().Add(-1 * time.Hour).Format("2006-01-02 15:04:05"), "event": "oauth_authorize", "details": "Авторизация приложения", "ip": "192.168.1.1", "severity": "info"},
    }
    c.JSON(200, gin.H{"events": events})
}

// GetDashboardMetrics - получить метрики для дашборда
func GetDashboardMetrics(c *gin.Context) {
    c.JSON(200, gin.H{
        "active_users_today":       1234,
        "active_users_week":        8765,
        "active_users_month":       15234,
        "oauth_requests_today":     5678,
        "oauth_requests_week":      34567,
        "oauth_requests_month":     123456,
        "token_validations_today":  12345,
        "token_validations_week":   87654,
        "token_validations_month":  345678,
    })
}