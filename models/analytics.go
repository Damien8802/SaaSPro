package models

import (
	"time"
)

// AnalyticsMetric - метрика
type AnalyticsMetric struct {
	ID         string                 `json:"id" db:"id"`
	AccountID  string                 `json:"account_id" db:"account_id"`
	MetricDate time.Time              `json:"metric_date" db:"metric_date"`
	MetricType string                 `json:"metric_type" db:"metric_type"`
	Value      float64                `json:"value" db:"value"`
	Metadata   map[string]interface{} `json:"metadata" db:"metadata"`
	CreatedAt  time.Time              `json:"created_at" db:"created_at"`
}

// AnalyticsCohort - когорта
type AnalyticsCohort struct {
	ID            string    `json:"id" db:"id"`
	AccountID     string    `json:"account_id" db:"account_id"`
	CohortDate    time.Time `json:"cohort_date" db:"cohort_date"`
	CohortSize    int       `json:"cohort_size" db:"cohort_size"`
	Period        int       `json:"period" db:"period"`
	RetentionRate float64   `json:"retention_rate" db:"retention_rate"`
	Revenue       float64   `json:"revenue" db:"revenue"`
}

// ChurnPrediction - прогноз оттока
type ChurnPrediction struct {
	ID               string                 `json:"id" db:"id"`
	AccountID        string                 `json:"account_id" db:"account_id"`
	CustomerID       string                 `json:"customer_id" db:"customer_id"`
	ChurnProbability float64                `json:"churn_probability" db:"churn_probability"`
	RiskLevel        string                 `json:"risk_level" db:"risk_level"`
	Factors          map[string]interface{} `json:"factors" db:"factors"`
	PredictedDate    time.Time              `json:"predicted_date" db:"predicted_date"`
	CreatedAt        time.Time              `json:"created_at" db:"created_at"`
}

// RFMAnalysis - RFM-анализ (Recency, Frequency, Monetary)
type RFMAnalysis struct {
	CustomerID    string  `json:"customer_id"`
	CustomerName  string  `json:"customer_name"`
	Recency       int     `json:"recency"`       // дней с последней покупки
	Frequency     int     `json:"frequency"`     // количество покупок
	Monetary      float64 `json:"monetary"`      // сумма покупок
	RFMScore      string  `json:"rfm_score"`     // 111, 112, ... 555
	Segment       string  `json:"segment"`       // Champions, Loyal, etc
	SegmentColor  string  `json:"segment_color"` // цвет для UI
}