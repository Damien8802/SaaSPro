package handlers

import (
    "context"
    "crypto/rand"
    "encoding/base64"
    "fmt"
    "net/http"
    "time"
    
    "github.com/gin-gonic/gin"
    "subscription-system/database"
)

func InitVPNWithDB(pool interface{}) {}

func VPNSalesPageHandler(c *gin.Context) {
    c.HTML(http.StatusOK, "vpn-sales.html", gin.H{"title": "VPN Сервис | SaaSPro"})
}

func CreateVPNKey(c *gin.Context) {
    var req struct {
        ClientName string `json:"client_name"`
        PlanID     int    `json:"plan_id"`
    }
    if err := c.BindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    if req.ClientName == "" {
        req.ClientName = fmt.Sprintf("client_%d", time.Now().UnixNano())
    }
    if req.PlanID == 0 {
        req.PlanID = 3
    }
    
    clientID := fmt.Sprintf("vpn_%x", time.Now().UnixNano())
    privateKey := generatePrivateKey()
    publicKey := generatePublicKey()
    clientIP := fmt.Sprintf("10.0.0.%d", time.Now().UnixNano()%254+1)
    
    var planName string
    var days int
    var speed string
    var devices int
    var price float64
    
    database.Pool.QueryRow(context.Background(), `
        SELECT name, days, speed, devices, price FROM vpn_plans WHERE id = $1
    `, req.PlanID).Scan(&planName, &days, &speed, &devices, &price)
    
    expiresAt := time.Now().AddDate(0, 0, days)
    tenantID := c.GetString("tenant_id")
    if tenantID == "" {
        tenantID = "11111111-1111-1111-1111-111111111111"
    }
    
    database.Pool.Exec(context.Background(), `
        INSERT INTO vpn_keys (client_id, client_name, client_ip, private_key, public_key, 
                              plan_id, expires_at, active, tenant_id, created_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, true, $8, NOW())
    `, clientID, req.ClientName, clientIP, privateKey, publicKey, req.PlanID, expiresAt, tenantID)
    
    config := fmt.Sprintf(`[Interface]
PrivateKey = %s
Address = %s/24
DNS = 1.1.1.1, 8.8.8.8

[Peer]
PublicKey = 6FoNHb43qPnTSDCppXSb6s+krs35CJpSfz6b+5VWcQQ=
Endpoint = vpn.your-server.com:51820
AllowedIPs = 0.0.0.0/0
PersistentKeepalive = 25`, privateKey, clientIP)
    
    c.JSON(http.StatusOK, gin.H{
        "success":    true,
        "client_id":  clientID,
        "client_ip":  clientIP,
        "expires_at": expiresAt.Format("2006-01-02"),
        "expires_in": fmt.Sprintf("%d дней", days),
        "config":     config,
    })
}

func GetVPNConfig(c *gin.Context) {
    clientID := c.Param("client")
    var privateKey, clientIP string
    database.Pool.QueryRow(context.Background(), `
        SELECT private_key, client_ip FROM vpn_keys 
        WHERE client_id = $1 AND active = true AND expires_at > NOW()
    `, clientID).Scan(&privateKey, &clientIP)
    
    config := fmt.Sprintf(`[Interface]
PrivateKey = %s
Address = %s/24
DNS = 1.1.1.1, 8.8.8.8

[Peer]
PublicKey = 6FoNHb43qPnTSDCppXSb6s+krs35CJpSfz6b+5VWcQQ=
Endpoint = vpn.your-server.com:51820
AllowedIPs = 0.0.0.0/0
PersistentKeepalive = 25`, privateKey, clientIP)
    c.String(http.StatusOK, config)
}

func CheckVPNKey(c *gin.Context) {
    identifier := c.Param("client")
    var clientID, clientName string
    var active bool
    var expiresAt time.Time
    database.Pool.QueryRow(context.Background(), `
        SELECT client_id, client_name, active, expires_at FROM vpn_keys 
        WHERE client_id = $1 OR client_name = $1 LIMIT 1
    `, identifier).Scan(&clientID, &clientName, &active, &expiresAt)
    
    isActive := active && expiresAt.After(time.Now())
    daysLeft := int(time.Until(expiresAt).Hours() / 24)
    c.JSON(http.StatusOK, gin.H{
        "client_id":   clientID,
        "client_name": clientName,
        "active":      isActive,
        "expires_at":  expiresAt,
        "days_left":   daysLeft,
    })
}

func GetVPNStats(c *gin.Context) {
    var total, active int
    database.Pool.QueryRow(context.Background(), `
        SELECT COUNT(*) as total, COUNT(*) FILTER (WHERE active = true AND expires_at > NOW()) as active FROM vpn_keys
    `).Scan(&total, &active)
    c.JSON(http.StatusOK, gin.H{
        "status":         "active",
        "total_clients":  total,
        "active_clients": active,
        "servers":        []gin.H{{"name": "🇷🇺 Россия (Москва)", "ping": "5 мс"}, {"name": "🇺🇸 США (Нью-Йорк)", "ping": "120 мс"}, {"name": "🇩🇪 Германия (Франкфурт)", "ping": "45 мс"}},
    })
}

func RenewVPNKey(c *gin.Context) {
    clientID := c.Param("client")
    database.Pool.Exec(context.Background(), `
        UPDATE vpn_keys SET expires_at = expires_at + INTERVAL '30 days', active = true WHERE client_id = $1
    `, clientID)
    c.JSON(http.StatusOK, gin.H{"success": true})
}

func GetAllVPNKeys(c *gin.Context) {
    rows, _ := database.Pool.Query(context.Background(), `SELECT client_id, client_name, active, expires_at, created_at FROM vpn_keys ORDER BY created_at DESC`)
    defer rows.Close()
    var keys []gin.H
    for rows.Next() {
        var clientID, clientName string
        var active bool
        var expiresAt, createdAt time.Time
        rows.Scan(&clientID, &clientName, &active, &expiresAt, &createdAt)
        keys = append(keys, gin.H{
            "client_id":   clientID,
            "client_name": clientName,
            "active":      active,
            "expires_at":  expiresAt,
            "created_at":  createdAt,
        })
    }
    c.JSON(http.StatusOK, gin.H{"keys": keys})
}

func AdminVPNHandler(c *gin.Context) {
    c.HTML(http.StatusOK, "admin_vpn.html", gin.H{"title": "Управление VPN"})
}

func GetVPNKeysPage(c *gin.Context) {
    c.Header("Content-Type", "text/html; charset=utf-8")
    c.String(200, `<!DOCTYPE html>
<html lang="ru">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>SaaSPro VPN | Мои ключи</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            background: linear-gradient(135deg, #0f0c29 0%, #302b63 50%, #24243e 100%);
            min-height: 100vh;
            padding: 20px;
        }
        .container { max-width: 1200px; margin: 0 auto; }
        h1 { color: white; margin-bottom: 20px; font-size: 28px; }
        .btn-create {
            background: linear-gradient(135deg, #667eea, #764ba2);
            color: white;
            border: none;
            padding: 12px 24px;
            border-radius: 30px;
            cursor: pointer;
            font-size: 16px;
            font-weight: 600;
            margin-bottom: 20px;
        }
        .keys-grid {
            display: grid;
            grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
            gap: 20px;
        }
        .key-card {
            background: rgba(255,255,255,0.05);
            backdrop-filter: blur(10px);
            border-radius: 20px;
            padding: 20px;
            border: 1px solid rgba(255,255,255,0.1);
            transition: transform 0.3s;
        }
        .key-card:hover { transform: translateY(-5px); background: rgba(255,255,255,0.08); }
        .card-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 15px; }
        .card-header h3 { color: white; font-size: 18px; }
        .status-active { color: #10b981; font-size: 12px; padding: 4px 8px; border-radius: 20px; background: #10b98120; }
        .status-expired { color: #ef4444; font-size: 12px; padding: 4px 8px; border-radius: 20px; background: #ef444420; }
        .info-row { display: flex; justify-content: space-between; margin: 10px 0; color: rgba(255,255,255,0.7); font-size: 14px; }
        .info-value { color: white; font-weight: 500; }
        .progress { background: rgba(255,255,255,0.1); border-radius: 10px; height: 6px; margin: 15px 0; }
        .progress-bar { background: linear-gradient(90deg, #667eea, #764ba2); border-radius: 10px; height: 100%; }
        .btn-group { display: flex; gap: 10px; margin-top: 15px; flex-wrap: wrap; }
        .btn { padding: 8px 16px; border-radius: 30px; font-size: 12px; font-weight: 600; cursor: pointer; border: none; transition: all 0.3s; }
        .btn-config { background: rgba(255,255,255,0.1); color: white; }
        .btn-mobile { background: #10b981; color: white; }
        .btn-renew { background: #f59e0b; color: white; }
        .btn-revoke { background: #ef4444; color: white; }
        .btn:hover { transform: translateY(-2px); opacity: 0.9; }
        .empty-state { text-align: center; padding: 60px; color: rgba(255,255,255,0.5); }
        .modal {
            display: none;
            position: fixed;
            top: 0;
            left: 0;
            width: 100%;
            height: 100%;
            background: rgba(0,0,0,0.7);
            justify-content: center;
            align-items: center;
            z-index: 1000;
        }
        .modal-content {
            background: #1a1a2e;
            padding: 30px;
            border-radius: 24px;
            width: 90%;
            max-width: 400px;
            border: 1px solid rgba(255,255,255,0.1);
        }
        .modal-content h3 { color: white; margin-bottom: 20px; }
        .modal-content input {
            width: 100%;
            padding: 12px;
            margin-bottom: 15px;
            border-radius: 12px;
            border: 1px solid #333;
            background: #2a2a3e;
            color: white;
        }
        .modal-content select {
            width: 100%;
            padding: 12px;
            margin-bottom: 20px;
            border-radius: 12px;
            border: 1px solid #333;
            background: #2a2a3e;
            color: white;
        }
        .modal-buttons { display: flex; gap: 10px; justify-content: flex-end; }
        .modal-btn-cancel { background: #333; color: white; border: none; padding: 10px 20px; border-radius: 30px; cursor: pointer; }
        .modal-btn-confirm { background: linear-gradient(135deg, #667eea, #764ba2); color: white; border: none; padding: 10px 20px; border-radius: 30px; cursor: pointer; }
        .stats {
            display: grid;
            grid-template-columns: repeat(4, 1fr);
            gap: 15px;
            margin-bottom: 30px;
        }
        .stat-card {
            background: rgba(255,255,255,0.05);
            border-radius: 16px;
            padding: 15px;
            text-align: center;
        }
        .stat-number { font-size: 28px; font-weight: 700; color: #667eea; }
        .stat-label { font-size: 12px; color: rgba(255,255,255,0.5); margin-top: 5px; }
        @media (max-width: 768px) {
            .keys-grid { grid-template-columns: 1fr; }
            .stats { grid-template-columns: repeat(2, 1fr); }
        }
    </style>
</head>
<body>
    <div class="container">
        <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 20px;">
            <h1>🔑 Мои VPN ключи</h1>
            <button class="btn-create" onclick="openCreateModal()">+ Создать ключ</button>
        </div>
        
        <div class="stats">
            <div class="stat-card"><div class="stat-number" id="totalKeys">0</div><div class="stat-label">Всего ключей</div></div>
            <div class="stat-card"><div class="stat-number" id="activeKeys">0</div><div class="stat-label">Активных</div></div>
            <div class="stat-card"><div class="stat-number" id="expiringKeys">0</div><div class="stat-label">Истекают скоро</div></div>
            <div class="stat-card"><div class="stat-number" id="totalTraffic">0</div><div class="stat-label">GB трафика</div></div>
        </div>
        
        <div id="keysList" class="keys-grid"></div>
    </div>
    
    <div id="createModal" class="modal">
        <div class="modal-content">
            <h3>Создать VPN ключ</h3>
            <input type="text" id="clientName" placeholder="Название устройства">
            <select id="planSelect">
                <option value="1">Пробный (3 дня)</option>
                <option value="2">Старт (30 дней) - 299 ₽</option>
                <option value="3" selected>Про (90 дней) - 999 ₽</option>
                <option value="4">Премиум (365 дней) - 2999 ₽</option>
            </select>
            <div class="modal-buttons">
                <button class="modal-btn-cancel" onclick="closeModal()">Отмена</button>
                <button class="modal-btn-confirm" onclick="createKey()">Создать</button>
            </div>
        </div>
    </div>

<script>
function openCreateModal() { document.getElementById('createModal').style.display = 'flex'; }
function closeModal() { document.getElementById('createModal').style.display = 'none'; }

async function loadStats() {
    try {
        const res = await fetch('/vpn/api/stats');
        const data = await res.json();
        document.getElementById('totalKeys').innerText = data.total_keys || 0;
        document.getElementById('activeKeys').innerText = data.active_keys || 0;
        document.getElementById('expiringKeys').innerText = Math.floor((data.active_keys || 0) * 0.3);
        document.getElementById('totalTraffic').innerText = Math.floor((data.active_keys || 0) * 15.7);
    } catch(e) {}
}

async function loadKeys() {
    try {
        const res = await fetch('/vpn/api/keys');
        const data = await res.json();
        const container = document.getElementById('keysList');
        if(data.keys && data.keys.length > 0) {
            container.innerHTML = data.keys.map(k => {
                const active = k.active && new Date(k.expires_at) > new Date();
                const daysLeft = Math.ceil((new Date(k.expires_at) - new Date()) / 86400000);
                const percent = Math.min(100, Math.max(0, (daysLeft / 30) * 100));
                return '<div class="key-card">' +
                    '<div class="card-header"><h3>' + escapeHtml(k.client_name) + '</h3><span class="' + (active ? 'status-active' : 'status-expired') + '">' + (active ? 'Активен' : 'Истёк') + '</span></div>' +
                    '<div class="info-row"><span>IP адрес</span><span class="info-value">' + k.client_ip + '</span></div>' +
                    '<div class="info-row"><span>Действует до</span><span class="info-value">' + new Date(k.expires_at).toLocaleDateString() + '</span></div>' +
                    '<div class="progress"><div class="progress-bar" style="width:' + percent + '%"></div></div>' +
                    '<div class="info-row"><span>Осталось дней</span><span class="info-value">' + daysLeft + '</span></div>' +
                    '<div class="btn-group">' +
                        '<button class="btn btn-config" onclick="downloadConfig(\'' + k.client_id + '\')">📥 Конфиг</button>' +
                        '<button class="btn btn-mobile" onclick="mobileConfig(\'' + k.client_id + '\')">📱 Телефон</button>' +
                        '<button class="btn btn-renew" onclick="renewKey(\'' + k.client_id + '\')">🔄 Продлить</button>' +
                        '<button class="btn btn-revoke" onclick="revokeKey(\'' + k.client_id + '\')">🗑 Отключить</button>' +
                    '</div></div>';
            }).join('');
        } else {
            container.innerHTML = '<div class="empty-state">🔑 У вас пока нет VPN ключей</div>';
        }
    } catch(e) { console.error(e); }
}

async function createKey() {
    const name = document.getElementById('clientName').value.trim();
    if(!name) { alert('Введите название устройства'); return; }
    const plan = document.getElementById('planSelect').value;
    try {
        const res = await fetch('/vpn/api/create', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({client_name: name, plan_id: parseInt(plan)})
        });
        const data = await res.json();
        if(data.success || data.client_id) {
            closeModal();
            alert('✅ Ключ создан');
            loadStats();
            loadKeys();
        } else { alert('Ошибка'); }
    } catch(e) { alert('Ошибка'); }
}

function downloadConfig(id) { window.open('/vpn/api/keys/' + id + '/config', '_blank'); }
function mobileConfig(id) { window.open('/vpn/api/keys/' + id + '/mobile', '_blank', 'width=500,height=700'); }
async function renewKey(id) { if(confirm('Продлить?')){ await fetch('/vpn/api/renew/' + id, {method:'POST'}); alert('Продлён'); loadKeys(); } }
async function revokeKey(id) { if(confirm('Отключить?')){ await fetch('/vpn/api/keys/' + id, {method:'DELETE'}); alert('Отключён'); loadKeys(); } }

function escapeHtml(s) { if(!s) return ''; return s.replace(/[&<>]/g, function(m){ return m==='&'?'&amp;':m==='<'?'&lt;':'>'?'&gt;':m; }); }

loadStats();
loadKeys();
</script>
</body>
</html>`)
}

func GetVPNKeysAPI(c *gin.Context) {
    tenantID := c.GetString("tenant_id")
    if tenantID == "" {
        tenantID = "11111111-1111-1111-1111-111111111111"
    }
    rows, err := database.Pool.Query(context.Background(), `
        SELECT client_id, client_name, client_ip, active, expires_at, created_at FROM vpn_keys WHERE tenant_id = $1 ORDER BY created_at DESC
    `, tenantID)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    defer rows.Close()
    var keys []gin.H
    for rows.Next() {
        var clientID, clientName, clientIP string
        var active bool
        var expiresAt, createdAt time.Time
        rows.Scan(&clientID, &clientName, &clientIP, &active, &expiresAt, &createdAt)
        keys = append(keys, gin.H{
            "client_id":   clientID,
            "client_name": clientName,
            "client_ip":   clientIP,
            "active":      active,
            "expires_at":  expiresAt,
            "created_at":  createdAt,
        })
    }
    c.JSON(http.StatusOK, gin.H{"keys": keys})
}

func GetVPNStatsAPI(c *gin.Context) {
    tenantID := c.GetString("tenant_id")
    var total, active int
    database.Pool.QueryRow(context.Background(), `
        SELECT COUNT(*) as total, COUNT(*) FILTER (WHERE active = true AND expires_at > NOW()) as active
        FROM vpn_keys WHERE tenant_id = $1
    `, tenantID).Scan(&total, &active)
    c.JSON(http.StatusOK, gin.H{"total_keys": total, "active_keys": active})
}

func CreateVPNKeyAPI(c *gin.Context) {
    var req struct {
        ClientName string `json:"client_name"`
        PlanID     int    `json:"plan_id"`
    }
    c.BindJSON(&req)
    c.Request.URL.Path = "/api/vpn/create"
    CreateVPNKey(c)
}

func RevokeVPNKeyAPI(c *gin.Context) {
    clientID := c.Param("id")
    database.Pool.Exec(context.Background(), `UPDATE vpn_keys SET active = false WHERE client_id = $1`, clientID)
    c.JSON(http.StatusOK, gin.H{"success": true})
}

func DownloadVPNConfig(c *gin.Context) {
    clientID := c.Param("id")
    var privateKey, clientIP string
    database.Pool.QueryRow(context.Background(), `
        SELECT private_key, client_ip FROM vpn_keys WHERE client_id = $1 AND active = true AND expires_at > NOW()
    `, clientID).Scan(&privateKey, &clientIP)
    
    config := fmt.Sprintf(`[Interface]
PrivateKey = %s
Address = %s/24
DNS = 1.1.1.1, 8.8.8.8

[Peer]
PublicKey = 6FoNHb43qPnTSDCppXSb6s+krs35CJpSfz6b+5VWcQQ=
Endpoint = vpn.your-server.com:51820
AllowedIPs = 0.0.0.0/0
PersistentKeepalive = 25`, privateKey, clientIP)
    
    c.Header("Content-Type", "application/x-wireguard-config")
    c.Header("Content-Disposition", "attachment; filename=vpn.conf")
    c.String(http.StatusOK, config)
}

func CreateVPNTables(c *gin.Context) {
    queries := []string{
        `CREATE TABLE IF NOT EXISTS vpn_keys (
            id SERIAL PRIMARY KEY,
            client_id VARCHAR(100) UNIQUE NOT NULL,
            client_name VARCHAR(100) NOT NULL,
            client_ip VARCHAR(15) NOT NULL,
            private_key TEXT NOT NULL,
            public_key TEXT NOT NULL,
            plan_id INTEGER REFERENCES vpn_plans(id),
            expires_at TIMESTAMP NOT NULL,
            active BOOLEAN DEFAULT TRUE,
            tenant_id UUID DEFAULT '11111111-1111-1111-1111-111111111111',
            created_at TIMESTAMP DEFAULT NOW()
        );`,
        `CREATE INDEX IF NOT EXISTS idx_vpn_keys_client_id ON vpn_keys(client_id);`,
    }
    for _, q := range queries {
        database.Pool.Exec(c.Request.Context(), q)
    }
    c.JSON(http.StatusOK, gin.H{"success": true})
}

func generatePrivateKey() string {
    b := make([]byte, 32)
    rand.Read(b)
    return base64.StdEncoding.EncodeToString(b)
}

func generatePublicKey() string {
    b := make([]byte, 32)
    rand.Read(b)
    return base64.StdEncoding.EncodeToString(b)
}
// DownloadMobileConfig - скачать конфиг для телефона (QR код + инструкция)
func DownloadMobileConfig(c *gin.Context) {
    clientID := c.Param("id")
    
    // Получаем user_id из контекста
    userID, exists := c.Get("user_id")
    if !exists {
        c.HTML(http.StatusOK, "mobile_config", gin.H{
            "error": "Ошибка авторизации",
        })
        return
    }
    
    var clientName, privateKey, clientIP string
    err := database.Pool.QueryRow(context.Background(), `
        SELECT client_name, private_key, client_ip FROM vpn_keys 
        WHERE client_id = $1 AND user_id = $2 AND active = true AND expires_at > NOW()
    `, clientID, userID).Scan(&clientName, &privateKey, &clientIP)
    
    if err != nil {
        c.HTML(http.StatusOK, "mobile_config", gin.H{
            "error": "Ключ не найден или истёк",
        })
        return
    }
    
    // Формируем конфиг для WireGuard
    config := fmt.Sprintf(`[Interface]
PrivateKey = %s
Address = %s/24
DNS = 1.1.1.1, 8.8.8.8

[Peer]
PublicKey = 6FoNHb43qPnTSDCppXSb6s+krs35CJpSfz6b+5VWcQQ=
Endpoint = vpn.your-server.com:51820
AllowedIPs = 0.0.0.0/0
PersistentKeepalive = 25`, privateKey, clientIP)
    
    c.HTML(http.StatusOK, "mobile_config", gin.H{
        "client_id":   clientID,
        "client_name": clientName,
        "client_ip":   clientIP,
        "config":      config,
    })
}