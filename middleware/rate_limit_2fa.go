package middleware

import (
    "sync"
    "time"
)

type RateLimiter2FA struct {
    attempts map[string]int
    blocked  map[string]time.Time
    mu       sync.RWMutex
}

var TwoFALimiter = &RateLimiter2FA{
    attempts: make(map[string]int),
    blocked:  make(map[string]time.Time),
}

// CheckAndIncrement - проверяет и увеличивает счётчик попыток
func (r *RateLimiter2FA) CheckAndIncrement(key string) bool {
    r.mu.Lock()
    defer r.mu.Unlock()
    
    // Проверяем, заблокирован ли пользователь
    if blockTime, exists := r.blocked[key]; exists {
        if time.Since(blockTime) < 15*time.Minute {
            return false // Всё ещё заблокирован
        }
        // Блокировка истекла, очищаем
        delete(r.blocked, key)
        delete(r.attempts, key)
        return true
    }
    
    // Получаем текущее количество попыток
    count := r.attempts[key]
    if count >= 5 {
        // Блокируем на 15 минут
        r.blocked[key] = time.Now()
        delete(r.attempts, key)
        return false
    }
    
    r.attempts[key] = count + 1
    return true
}

// Reset - сбрасывает счётчик после успешной верификации
func (r *RateLimiter2FA) Reset(key string) {
    r.mu.Lock()
    defer r.mu.Unlock()
    delete(r.attempts, key)
    delete(r.blocked, key)
}