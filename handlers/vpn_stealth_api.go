package handlers

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// StealthVPNService - отдельный сервис для stealth VPN
type StealthVPNService struct {
	db      *pgxpool.Pool
	cache   map[string]string
	cacheMu sync.RWMutex
}

var stealthService *StealthVPNService

// InitStealthVPN - инициализация stealth сервиса
func InitStealthVPN(db *pgxpool.Pool) {
	stealthService = &StealthVPNService{
		db:    db,
		cache: make(map[string]string),
	}
	go stealthService.loadRoutingRules()
	log.Println("✅ Stealth VPN сервис инициализирован")
}

// loadRoutingRules - загрузка правил в кэш
func (s *StealthVPNService) loadRoutingRules() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		rows, err := s.db.Query(context.Background(),
			`SELECT domain_pattern, route_type, is_wildcard 
			 FROM vpn_smart_routing 
			 WHERE is_active = true 
			 ORDER BY priority DESC`)
		if err != nil {
			log.Printf("⚠️ Stealth VPN: ошибка загрузки правил: %v", err)
			continue
		}

		s.cacheMu.Lock()
		for k := range s.cache {
			delete(s.cache, k)
		}
		for rows.Next() {
			var domainPattern, routeType string
			var isWildcard bool
			rows.Scan(&domainPattern, &routeType, &isWildcard)

			pattern := domainPattern
			if isWildcard && !strings.HasPrefix(pattern, "*.") && !strings.HasPrefix(pattern, "*") {
				pattern = "*." + pattern
			}
			s.cache[pattern] = routeType
		}
		rows.Close()
		s.cacheMu.Unlock()

		log.Printf("✅ Stealth VPN: загружено %d правил умного роутинга", len(s.cache))
	}
}

// GetSmartRoute - определить маршрут для домена
func (s *StealthVPNService) GetSmartRoute(domain string) string {
	s.cacheMu.RLock()
	defer s.cacheMu.RUnlock()

	if route, ok := s.cache[domain]; ok {
		return route
	}

	for pattern, route := range s.cache {
		if strings.HasPrefix(pattern, "*.") {
			suffix := strings.TrimPrefix(pattern, "*.")
			if strings.HasSuffix(domain, suffix) {
				return route
			}
		}
	}

	return "vpn"
}

// GenerateVLessConfig - генерация VLESS конфигурации (без сохранения в БД - временно)
func (s *StealthVPNService) GenerateVLessConfig(userID string, domain string, port int) (map[string]interface{}, error) {
	clientUUID := uuid.New().String()
	shortID := generateShortID()
	publicKey, _ := generateRealityKeys()

	// ВРЕМЕННО ОТКЛЮЧАЕМ СОХРАНЕНИЕ В БД ИЗ-ЗА ПРОБЛЕМ С FOREIGN KEY
	// TODO: Исправить внешние ключи и вернуть сохранение
	/*
	_, err := s.db.Exec(context.Background(),
		`INSERT INTO vpn_vless_clients (user_id, client_uuid, email, short_id, public_key, private_key)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		userID, clientUUID, fmt.Sprintf("user_%s@stealth.local", userID[:8]), shortID, publicKey, privateKey)

	if err != nil {
		log.Printf("⚠️ Stealth VPN: не удалось сохранить клиента: %v", err)
	}
	*/

	log.Printf("✅ Stealth VPN: сгенерирована конфигурация для user %s", userID)

	config := map[string]interface{}{
		"protocol": "vless",
		"settings": map[string]interface{}{
			"vnext": []map[string]interface{}{
				{
					"address": domain,
					"port":    port,
					"users": []map[string]interface{}{
						{
							"id":         clientUUID,
							"flow":       "xtls-rprx-vision",
							"encryption": "none",
						},
					},
				},
			},
		},
		"streamSettings": map[string]interface{}{
			"network":  "tcp",
			"security": "reality",
			"realitySettings": map[string]interface{}{
				"serverName":  "www.microsoft.com",
				"fingerprint": "chrome",
				"publicKey":   publicKey,
				"shortId":     shortID,
			},
		},
	}

	return config, nil
}

// generateShortID - генерация короткого ID
func generateShortID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// generateRealityKeys - генерация ключей
func generateRealityKeys() (publicKey, privateKey string) {
	priv := make([]byte, 32)
	pub := make([]byte, 32)
	rand.Read(priv)
	rand.Read(pub)

	privateKey = base64.StdEncoding.EncodeToString(priv)
	publicKey = base64.StdEncoding.EncodeToString(pub)

	return publicKey, privateKey
}

// ========== API HANDLERS ==========

// StealthVPNPageHandler - страница stealth VPN
func StealthVPNPageHandler(c *gin.Context) {
	c.HTML(http.StatusOK, "stealth-vpn.html", gin.H{
		"title":   "Stealth VPN - Невидимый VPN сервис",
		"message": "Ваш трафик неотличим от обычного HTTPS",
	})
}

// GetVLessConfigHandler - получить VLESS конфиг
func GetVLessConfigHandler(c *gin.Context) {
	if stealthService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Stealth VPN service not initialized"})
		return
	}

	// Получаем user_id из контекста
	userID, exists := c.Get("user_id")
	if !exists {
		if uid, ok := c.Get("userID"); ok {
			userID = uid
			exists = true
		}
	}

	if !exists {
		log.Printf("❌ GetVLessConfigHandler: user_id not found in context")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found - please login first"})
		return
	}

	log.Printf("📝 GetVLessConfigHandler: userID=%s", userID)

	domain := c.DefaultQuery("domain", "vpn.stealth.local")
	port := 443
	if p := c.Query("port"); p != "" {
		fmt.Sscanf(p, "%d", &port)
	}

	config, err := stealthService.GenerateVLessConfig(userID.(string), domain, port)
	if err != nil {
		log.Printf("❌ GetVLessConfigHandler: generate error - %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	configJSON, _ := json.MarshalIndent(config, "", "  ")

	c.JSON(http.StatusOK, gin.H{
		"success":        true,
		"protocol":       "vless+reality",
		"config":         config,
		"config_string":  string(configJSON),
		"instructions":   "Скопируйте config_string и импортируйте в v2rayN / Nekoray / Sing-box",
	})
}
// GetSmartRulesHandler - получить правила умного роутинга
func GetSmartRulesHandler(c *gin.Context) {
	if stealthService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Service not initialized"})
		return
	}

	// Получаем user_id из контекста
	userID, exists := c.Get("user_id")
	if !exists {
		if uid, ok := c.Get("userID"); ok {
			userID = uid
			exists = true
		}
	}
	
	if !exists {
		log.Printf("❌ GetSmartRulesHandler: user_id not found in context")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found - please login first"})
		return
	}

	log.Printf("📝 GetSmartRulesHandler: userID=%s", userID)

	rows, err := stealthService.db.Query(context.Background(),
		`SELECT id, domain_pattern, route_type, is_wildcard, priority 
		 FROM vpn_smart_routing 
		 WHERE user_id = $1 AND is_active = true 
		 ORDER BY priority DESC`,
		userID)

	if err != nil {
		log.Printf("❌ GetSmartRulesHandler: query error - %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var rules []gin.H
	for rows.Next() {
		var id int
		var domainPattern, routeType string
		var isWildcard bool
		var priority int
		err := rows.Scan(&id, &domainPattern, &routeType, &isWildcard, &priority)
		if err != nil {
			log.Printf("❌ GetSmartRulesHandler: scan error - %v", err)
			continue
		}

		rules = append(rules, gin.H{
			"id":             id,
			"domain_pattern": domainPattern,
			"route_type":     routeType,
			"is_wildcard":    isWildcard,
			"priority":       priority,
		})
	}

	log.Printf("✅ GetSmartRulesHandler: found %d rules", len(rules))
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"rules":   rules,
	})
}// AddSmartRuleHandler - добавить правило
func AddSmartRuleHandler(c *gin.Context) {
	if stealthService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Stealth VPN service not initialized"})
		return
	}

	// Получаем user_id из контекста
	userID, exists := c.Get("user_id")
	if !exists {
		if uid, ok := c.Get("userID"); ok {
			userID = uid
			exists = true
		}
	}
	
	if !exists {
		log.Printf("❌ AddSmartRuleHandler: user_id not found in context")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found - please login first"})
		return
	}

	var req struct {
		DomainPattern string `json:"domain_pattern"`
		RouteType     string `json:"route_type"`
		IsWildcard    bool   `json:"is_wildcard"`
	}

	if err := c.BindJSON(&req); err != nil {
		log.Printf("❌ AddSmartRuleHandler: bind error - %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.DomainPattern == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "domain_pattern is required"})
		return
	}

	if req.RouteType == "" {
		req.RouteType = "vpn"
	}

	log.Printf("📝 Добавление правила: user=%s, domain=%s, type=%s, wildcard=%v", 
		userID, req.DomainPattern, req.RouteType, req.IsWildcard)

	// Проверяем, существует ли уже такое правило
	var existsRule bool
	err := stealthService.db.QueryRow(context.Background(),
		`SELECT EXISTS(SELECT 1 FROM vpn_smart_routing 
		 WHERE user_id = $1 AND domain_pattern = $2)`,
		userID, req.DomainPattern).Scan(&existsRule)
	if err != nil {
		log.Printf("❌ AddSmartRuleHandler: check error - %v", err)
	}

	if existsRule {
		// Обновляем существующее правило
		_, err = stealthService.db.Exec(context.Background(),
			`UPDATE vpn_smart_routing 
			 SET route_type = $1, is_wildcard = $2, is_active = true 
			 WHERE user_id = $3 AND domain_pattern = $4`,
			req.RouteType, req.IsWildcard, userID, req.DomainPattern)
	} else {
		// Вставляем новое правило
		_, err = stealthService.db.Exec(context.Background(),
			`INSERT INTO vpn_smart_routing (user_id, domain_pattern, is_wildcard, route_type, priority, is_active) 
			 VALUES ($1, $2, $3, $4, 100, true)`,
			userID, req.DomainPattern, req.IsWildcard, req.RouteType)
	}

	if err != nil {
		log.Printf("❌ AddSmartRuleHandler: insert/update error - %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Обновляем кэш
	go stealthService.loadRoutingRules()

	log.Printf("✅ AddSmartRuleHandler: правило добавлено/обновлено")
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Rule added successfully",
	})
}// DeleteSmartRuleHandler - удалить правило
func DeleteSmartRuleHandler(c *gin.Context) {
	if stealthService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Service not initialized"})
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		if uid, ok := c.Get("userID"); ok {
			userID = uid
			exists = true
		}
	}

	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
		return
	}

	ruleID := c.Param("id")

	var ownerID string
	err := stealthService.db.QueryRow(context.Background(),
		"SELECT user_id FROM vpn_smart_routing WHERE id = $1", ruleID).Scan(&ownerID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Rule not found"})
		return
	}

	if ownerID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	_, err = stealthService.db.Exec(context.Background(),
		"DELETE FROM vpn_smart_routing WHERE id = $1", ruleID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	go stealthService.loadRoutingRules()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Rule deleted successfully",
	})
}

// GetStealthPlansHandler - получить stealth тарифы
func GetStealthPlansHandler(c *gin.Context) {
	if stealthService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Service not initialized"})
		return
	}

	rows, err := stealthService.db.Query(context.Background(),
		`SELECT id, name, price, days, speed, devices, supports_stealth, stealth_protocols
		 FROM vpn_plans 
		 WHERE supports_stealth = true 
		 ORDER BY price ASC`)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var plans []gin.H
	for rows.Next() {
		var id int
		var name, speed string
		var price float64
		var days, devices int
		var supportsStealth bool
		var stealthProtocols interface{}

		rows.Scan(&id, &name, &price, &days, &speed, &devices, &supportsStealth, &stealthProtocols)

		plans = append(plans, gin.H{
			"id":                id,
			"name":              name,
			"price":             price,
			"days":              days,
			"speed":             speed,
			"devices":           devices,
			"supports_stealth":  supportsStealth,
			"stealth_protocols": stealthProtocols,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"plans":   plans,
	})
}