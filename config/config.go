package config

import (
    "log"
    "os"
    "strconv"
    "strings"
    "time"
)

type Config struct {
    Port           string
    Env            string
    LogLevel       string
    StaticPath     string
    FrontendPath   string
    TemplatesPath  string
    Debug          bool
    TrustedProxies []string
    AllowedOrigins []string
    SkipAuth       bool

    // Database
    DBHost     string
    DBPort     string
    DBUser     string
    DBPassword string
    DBName     string
    DBSSLMode  string

    // JWT
    JWTSecret        string
    JWTRefreshSecret string
    JWTAccessExpiry  time.Duration
    JWTRefreshExpiry time.Duration

    // AI
    OpenRouterAPIKey string
    YandexFolderID   string
    YandexAPIKey     string
    YandexSearchKey  string
    YandexSearchUser string
    YandexSearchEmail string
    GigaChatClientID string
    GigaChatSecret   string
    GigaChatAuthKey  string

    // SMTP
    SMTPHost     string
    SMTPPort     int
    SMTPUser     string
    SMTPPassword string
    SMTPFrom     string
    SMTPFromName string
    EmailFrom    string
    SMTPTLS      bool

    // Telegram
    TelegramBotToken string
    TelegramChatID   string

    // VPN
    VPNInterface string
    VPNSubnet    string
    VPNDNS       string
}

func Load() *Config {
    cfg := &Config{
        Port:           getEnv("PORT", "8080"),
        Env:            getEnv("GIN_MODE", "debug"),
        LogLevel:       getEnv("LOG_LEVEL", "info"),
        StaticPath:     getEnv("STATIC_PATH", "./static"),
        FrontendPath:   getEnv("FRONTEND_PATH", "./frontend"),
        TemplatesPath:  getEnv("TEMPLATES_PATH", "./templates/*.html"),
        Debug:          getEnvAsBool("DEBUG", true),
        TrustedProxies: []string{},
        AllowedOrigins: getEnvAsSlice("CORS_ALLOWED_ORIGINS", []string{"*"}),
        SkipAuth:       getEnvAsBool("SKIP_AUTH", false),

        DBHost:     getEnv("DB_HOST", "localhost"),
        DBPort:     getEnv("DB_PORT", "5432"),
        DBUser:     getEnv("DB_USER", "postgres"),
        DBPassword: getEnv("DB_PASSWORD", ""),
        DBName:     getEnv("DB_NAME", "postgres"),
        DBSSLMode:  getEnv("DB_SSLMODE", "disable"),

        JWTSecret:        getEnv("JWT_ACCESS_SECRET", "default-access-secret"),
        JWTRefreshSecret: getEnv("JWT_REFRESH_SECRET", "default-refresh-secret"),
        JWTAccessExpiry:  getEnvAsDuration("JWT_ACCESS_EXPIRY", 15*time.Minute),
        JWTRefreshExpiry: getEnvAsDuration("JWT_REFRESH_EXPIRY", 30*24*time.Hour),

        // AI
        OpenRouterAPIKey: getEnv("OPENROUTER_API_KEY", ""),
        YandexFolderID:   getEnv("YANDEX_FOLDER_ID", ""),
        YandexAPIKey:     getEnv("YANDEX_API_KEY", ""),
        YandexSearchKey:  getEnv("YANDEX_SEARCH_KEY", ""),
        YandexSearchUser: getEnv("YANDEX_SEARCH_USER", ""),
        YandexSearchEmail: getEnv("YANDEX_SEARCH_EMAIL", ""),
        GigaChatClientID: getEnv("GIGACHAT_CLIENT_ID", ""),
        GigaChatSecret:   getEnv("GIGACHAT_CLIENT_SECRET", ""),
        GigaChatAuthKey:  getEnv("GIGACHAT_AUTH_KEY", ""),

        // SMTP
        SMTPHost:     getEnv("SMTP_HOST", "smtp.yandex.ru"),
        SMTPPort:     getEnvAsInt("SMTP_PORT", 587),
        SMTPUser:     getEnv("SMTP_USER", ""),
        SMTPPassword: getEnv("SMTP_PASSWORD", ""),
        SMTPFrom:     getEnv("SMTP_FROM", ""),
        SMTPFromName: getEnv("SMTP_FROM_NAME", "SaaSPro"),
        EmailFrom:    getEnv("EMAIL_FROM", ""),
        SMTPTLS:      getEnvAsBool("SMTP_TLS", true),

        // Telegram
        TelegramBotToken: getEnv("TELEGRAM_BOT_TOKEN", ""),
        TelegramChatID:   getEnv("TELEGRAM_CHAT_ID", ""),

        // VPN
        VPNInterface: getEnv("VPN_INTERFACE", "wg0"),
        VPNSubnet:    getEnv("VPN_SUBNET", "10.0.0.0/24"),
        VPNDNS:       getEnv("VPN_DNS", "8.8.8.8"),
    }

    if proxies := getEnv("TRUSTED_PROXIES", ""); proxies != "" {
        cfg.TrustedProxies = strings.Split(proxies, ",")
    }

    log.Printf("📋 Конфигурация загружена: порт=%s, режим=%s, БД=%s, SkipAuth=%v, OpenRouterKeySet=%v, TelegramSet=%v",
        cfg.Port, cfg.Env, cfg.DBName, cfg.SkipAuth, cfg.OpenRouterAPIKey != "", cfg.TelegramBotToken != "")
    return cfg
}

func getEnv(key, defaultValue string) string {
    if value := os.Getenv(key); value != "" {
        return value
    }
    return defaultValue
}

func getEnvAsBool(key string, defaultValue bool) bool {
    strVal := getEnv(key, "")
    if val, err := strconv.ParseBool(strVal); err == nil {
        return val
    }
    return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
    strVal := getEnv(key, "")
    if val, err := strconv.Atoi(strVal); err == nil {
        return val
    }
    return defaultValue
}

func getEnvAsDuration(key string, defaultValue time.Duration) time.Duration {
    strVal := getEnv(key, "")
    if val, err := time.ParseDuration(strVal); err == nil {
        return val
    }
    return defaultValue
}

func getEnvAsSlice(key string, defaultValue []string) []string {
    val := getEnv(key, "")
    if val == "" {
        return defaultValue
    }
    parts := strings.Split(val, ",")
    for i, part := range parts {
        parts[i] = strings.TrimSpace(part)
    }
    return parts
}