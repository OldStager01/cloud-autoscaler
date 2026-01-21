package models

import "time"

// Prediction represents a predicted metric value
type Prediction struct {
	ID           int       `json:"id,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	ClusterID    string    `json:"cluster_id"`
	ForecastTime time.Time `json:"forecast_time"`
	PredictedCPU float64   `json:"predicted_cpu"`
	ActualCPU    *float64  `json:"actual_cpu,omitempty"`
	Confidence   float64   `json:"confidence"`
	ModelVersion string    `json:"model_version,omitempty"`
}

func NewPrediction(clusterID string, forecastTime time.Time, predictedCPU, confidence float64) *Prediction {
	return &Prediction{
		CreatedAt:    time.Now(),
		ClusterID:    clusterID,
		ForecastTime: forecastTime,
		PredictedCPU: predictedCPU,
		Confidence:   confidence,
	}
}

func (p *Prediction) IsHighConfidence(threshold float64) bool {
	return p.Confidence >= threshold
}

// PatternRecord represents aggregated historical pattern
type PatternRecord struct {
	ClusterID   string    `json:"cluster_id"`
	DayOfWeek   int       `json:"day_of_week"`
	HourOfDay   int       `json:"hour_of_day"`
	AvgCPU      float64   `json:"avg_cpu"`
	AvgMemory   float64   `json:"avg_memory"`
	AvgLoad     float64   `json:"avg_load"`
	SampleCount int       `json:"sample_count"`
	LastUpdated time.Time `json:"last_updated"`
}