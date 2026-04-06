package services

import (
	"context"
	"fmt"
	"log"
	"math"
	"time"

	"subscription-system/database"
	"subscription-system/models"
)

// AnalyticsService - сервис для аналитики
type AnalyticsService struct{}

// NewAnalyticsService - конструктор
func NewAnalyticsService() *AnalyticsService {
	return &AnalyticsService{}
}

// StartAnalyticsScheduler - запуск планировщика аналитики
func (s *AnalyticsService) StartAnalyticsScheduler() {
	log.Println("📊 Запуск планировщика аналитики")
	
	go func() {
		// Сразу выполняем первый расчет
		s.CalculateAllMetrics()
		
		// Затем каждый день в 3 часа ночи
		ticker := time.NewTicker(24 * time.Hour)
		for range ticker.C {
			now := time.Now()
			if now.Hour() == 3 {
				s.CalculateAllMetrics()
			}
		}
	}()
}

// CalculateAllMetrics - расчет всех метрик для всех аккаунтов
func (s *AnalyticsService) CalculateAllMetrics() {
	log.Println("📊 Расчет метрик аналитики...")
	
	ctx := context.Background()
	
	// Получаем все аккаунты
	rows, err := database.Pool.Query(ctx, "SELECT id FROM accounts")
	if err != nil {
		log.Printf("❌ Ошибка получения аккаунтов: %v", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var accountID string
		if err := rows.Scan(&accountID); err != nil {
			continue
		}
		
		s.CalculateAccountMetrics(ctx, accountID)
	}
	
	log.Println("✅ Расчет метрик завершен")
}

// CalculateAccountMetrics - расчет метрик для конкретного аккаунта
// CalculateAccountMetrics - расчет метрик для конкретного аккаунта
func (s *AnalyticsService) CalculateAccountMetrics(ctx context.Context, accountID string) {
	// 1. Доход по дням
	s.calculateDailyRevenue(ctx, accountID)
	
	// 2. Новые клиенты
	s.calculateNewCustomers(ctx, accountID)
	
	// 3. Активные подписки
	s.calculateActiveSubscriptions(ctx, accountID)
	
	// 4. RFM-анализ
	s.CalculateRFMAnalysis(ctx, accountID)  // ← ИСПРАВЛЕНО
	
	// 5. Прогноз оттока
	s.predictChurn(ctx, accountID)
	
	// 6. Когортный анализ
	s.calculateCohorts(ctx, accountID)
}

// calculateDailyRevenue - доход по дням
func (s *AnalyticsService) calculateDailyRevenue(ctx context.Context, accountID string) {
	query := `
		SELECT 
			DATE(created_at) as date,
			COALESCE(SUM(amount), 0) as revenue
		FROM payments
		WHERE account_id = $1 AND status = 'completed'
		GROUP BY DATE(created_at)
		ORDER BY date DESC
		LIMIT 30
	`
	
	rows, err := database.Pool.Query(ctx, query, accountID)
	if err != nil {
		log.Printf("❌ Ошибка расчета дохода: %v", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var date time.Time
		var revenue float64
		
		if err := rows.Scan(&date, &revenue); err != nil {
			continue
		}
		
		// Сохраняем метрику
		s.saveMetric(ctx, accountID, date, "revenue", revenue, nil)
	}
}

// calculateNewCustomers - новые клиенты по дням
func (s *AnalyticsService) calculateNewCustomers(ctx context.Context, accountID string) {
	query := `
		SELECT 
			DATE(created_at) as date,
			COUNT(*) as count
		FROM customers
		WHERE account_id = $1
		GROUP BY DATE(created_at)
		ORDER BY date DESC
		LIMIT 30
	`
	
	rows, err := database.Pool.Query(ctx, query, accountID)
	if err != nil {
		log.Printf("❌ Ошибка расчета новых клиентов: %v", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var date time.Time
		var count float64
		
		if err := rows.Scan(&date, &count); err != nil {
			continue
		}
		
		s.saveMetric(ctx, accountID, date, "new_customers", count, nil)
	}
}

// calculateActiveSubscriptions - активные подписки по дням
func (s *AnalyticsService) calculateActiveSubscriptions(ctx context.Context, accountID string) {
	query := `
		SELECT 
			DATE(created_at) as date,
			COUNT(*) as count
		FROM subscriptions
		WHERE account_id = $1 AND status = 'active'
		GROUP BY DATE(created_at)
		ORDER BY date DESC
		LIMIT 30
	`
	
	rows, err := database.Pool.Query(ctx, query, accountID)
	if err != nil {
		log.Printf("❌ Ошибка расчета активных подписок: %v", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var date time.Time
		var count float64
		
		if err := rows.Scan(&date, &count); err != nil {
			continue
		}
		
		s.saveMetric(ctx, accountID, date, "active_subscriptions", count, nil)
	}
}

// calculateRFMAnalysis - RFM-анализ
func (s *AnalyticsService) CalculateRFMAnalysis(ctx context.Context, accountID string) ([]models.RFMAnalysis, error) {
	query := `
		WITH customer_stats AS (
			SELECT 
				c.id,
				c.name,
				MAX(p.created_at) as last_payment,
				COUNT(p.id) as payment_count,
				COALESCE(SUM(p.amount), 0) as total_spent
			FROM customers c
			LEFT JOIN payments p ON p.customer_id = c.id AND p.status = 'completed'
			WHERE c.account_id = $1
			GROUP BY c.id, c.name
		)
		SELECT 
			id,
			name,
			EXTRACT(DAY FROM NOW() - last_payment)::INT as recency,
			payment_count as frequency,
			total_spent as monetary
		FROM customer_stats
		WHERE last_payment IS NOT NULL
	`
	
	rows, err := database.Pool.Query(ctx, query, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []models.RFMAnalysis
	for rows.Next() {
		var rfm models.RFMAnalysis
		var recency, frequency int
		var monetary float64
		
		if err := rows.Scan(&rfm.CustomerID, &rfm.CustomerName, &recency, &frequency, &monetary); err != nil {
			continue
		}
		
		rfm.Recency = recency
		rfm.Frequency = frequency
		rfm.Monetary = monetary
		
		// Рассчитываем RFM-скор (1-5)
		rScore := s.calculateRScore(recency)
		fScore := s.calculateFScore(frequency)
		mScore := s.calculateMScore(monetary)
		
		rfm.RFMScore = fmt.Sprintf("%d%d%d", rScore, fScore, mScore)
		rfm.Segment, rfm.SegmentColor = s.getRFMSegment(rScore, fScore, mScore)
		
		results = append(results, rfm)
	}
	
	return results, nil
}

// calculateRScore - оценка по давности (1-5)
func (s *AnalyticsService) calculateRScore(days int) int {
	switch {
	case days <= 7:
		return 5
	case days <= 30:
		return 4
	case days <= 60:
		return 3
	case days <= 90:
		return 2
	default:
		return 1
	}
}

// calculateFScore - оценка по частоте (1-5)
func (s *AnalyticsService) calculateFScore(count int) int {
	switch {
	case count >= 10:
		return 5
	case count >= 5:
		return 4
	case count >= 3:
		return 3
	case count >= 1:
		return 2
	default:
		return 1
	}
}

// calculateMScore - оценка по сумме (1-5)
func (s *AnalyticsService) calculateMScore(amount float64) int {
	switch {
	case amount >= 100000:
		return 5
	case amount >= 50000:
		return 4
	case amount >= 10000:
		return 3
	case amount >= 1000:
		return 2
	default:
		return 1
	}
}

// getRFMSegment - определение сегмента по RFM-скору
func (s *AnalyticsService) getRFMSegment(r, f, m int) (string, string) {
	switch {
	case r >= 4 && f >= 4 && m >= 4:
		return "Чемпионы", "#gold"
	case r >= 3 && f >= 3 && m >= 3:
		return "Лояльные", "#4CAF50"
	case r >= 4 && f <= 2:
		return "Новые", "#2196F3"
	case r <= 2 && f >= 4 && m >= 4:
		return "Уходящие", "#FF9800"
	case r <= 2 && f <= 2:
		return "Спящие", "#9E9E9E"
	default:
		return "Средние", "#607D8B"
	}
}

// predictChurn - прогноз оттока
func (s *AnalyticsService) predictChurn(ctx context.Context, accountID string) {
	// Простая модель: если нет активности > 60 дней - высокий риск
	query := `
		SELECT 
			c.id,
			EXTRACT(DAY FROM NOW() - MAX(a.created_at))::INT as days_inactive
		FROM customers c
		LEFT JOIN activities a ON a.entity_id = c.id AND a.entity_type = 'customer'
		WHERE c.account_id = $1
		GROUP BY c.id
	`
	
	rows, err := database.Pool.Query(ctx, query, accountID)
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var customerID string
		var daysInactive int
		
		if err := rows.Scan(&customerID, &daysInactive); err != nil {
			continue
		}
		
		// Рассчитываем вероятность оттока
		probability := math.Min(float64(daysInactive)/90.0, 1.0)
		
		var riskLevel string
		switch {
		case probability > 0.7:
			riskLevel = "high"
		case probability > 0.3:
			riskLevel = "medium"
		default:
			riskLevel = "low"
		}
		
		// Сохраняем прогноз
		s.saveChurnPrediction(ctx, accountID, customerID, probability, riskLevel)
	}
}

// saveMetric - сохранение метрики
func (s *AnalyticsService) saveMetric(ctx context.Context, accountID string, date time.Time, metricType string, value float64, metadata map[string]interface{}) {
	_, err := database.Pool.Exec(ctx, `
		INSERT INTO analytics_metrics (account_id, metric_date, metric_type, value, metadata)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (account_id, metric_date, metric_type) 
		DO UPDATE SET value = EXCLUDED.value, metadata = EXCLUDED.metadata
	`, accountID, date, metricType, value, metadata)
	
	if err != nil {
		log.Printf("❌ Ошибка сохранения метрики: %v", err)
	}
}

// saveChurnPrediction - сохранение прогноза оттока
func (s *AnalyticsService) saveChurnPrediction(ctx context.Context, accountID, customerID string, probability float64, riskLevel string) {
	factors := map[string]interface{}{
		"inactivity_days": probability * 90,
	}
	
	_, err := database.Pool.Exec(ctx, `
		INSERT INTO analytics_churn_predictions (account_id, customer_id, churn_probability, risk_level, factors, predicted_date)
		VALUES ($1, $2, $3, $4, $5, NOW() + INTERVAL '30 days')
		ON CONFLICT (customer_id) 
		DO UPDATE SET churn_probability = EXCLUDED.churn_probability, 
		              risk_level = EXCLUDED.risk_level,
		              factors = EXCLUDED.factors
	`, accountID, customerID, probability, riskLevel, factors)
	
	if err != nil {
		log.Printf("❌ Ошибка сохранения прогноза: %v", err)
	}
}

// calculateCohorts - когортный анализ
func (s *AnalyticsService) calculateCohorts(ctx context.Context, accountID string) {
	// Очищаем старые данные
	database.Pool.Exec(ctx, "DELETE FROM analytics_cohorts WHERE account_id = $1", accountID)
	
	query := `
		WITH cohorts AS (
			SELECT 
				c.id,
				DATE_TRUNC('month', c.created_at) as cohort_month,
				EXTRACT('month' FROM AGE(p.created_at, c.created_at)) as period
			FROM customers c
			LEFT JOIN payments p ON p.customer_id = c.id
			WHERE c.account_id = $1
		)
		SELECT 
			cohort_month,
			period,
			COUNT(DISTINCT id) as cohort_size,
			COUNT(DISTINCT CASE WHEN period = 0 THEN id END) as retained
		FROM cohorts
		GROUP BY cohort_month, period
		ORDER BY cohort_month, period
	`
	
	rows, err := database.Pool.Query(ctx, query, accountID)
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var cohortMonth time.Time
		var period int
		var cohortSize, retained int
		
		if err := rows.Scan(&cohortMonth, &period, &cohortSize, &retained); err != nil {
			continue
		}
		
		retentionRate := float64(retained) / float64(cohortSize) * 100
		
		_, err = database.Pool.Exec(ctx, `
			INSERT INTO analytics_cohorts (account_id, cohort_date, period, cohort_size, retention_rate)
			VALUES ($1, $2, $3, $4, $5)
		`, accountID, cohortMonth, period, cohortSize, retentionRate)
		
		if err != nil {
			log.Printf("❌ Ошибка сохранения когорты: %v", err)
		}
	}
}