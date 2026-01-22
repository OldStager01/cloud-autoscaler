package simulator

import (
	"math"
	"math/rand"
	"time"
)

type Pattern interface {
	Apply(baseCPU float64) float64
	Name() string
}

var (
	PatternSteady      Pattern = &SteadyPattern{}
	PatternDaily       Pattern = &DailyPattern{}
	PatternWeekly      Pattern = &WeeklyPattern{}
	PatternRandom      Pattern = &RandomPattern{}
	PatternGradualRise Pattern = &GradualRisePattern{startTime: time.Now()}
)

func ParsePattern(name string) Pattern {
	switch name {
	case "daily":
		return PatternDaily
	case "weekly":
		return PatternWeekly
	case "random": 
		return PatternRandom
	case "gradual_rise": 
		return &GradualRisePattern{startTime: time.Now()}
	default:
		return PatternSteady
	}
}

// SteadyPattern - constant load
type SteadyPattern struct{}

func (p *SteadyPattern) Apply(baseCPU float64) float64 {
	return baseCPU
}

func (p *SteadyPattern) Name() string {
	return "steady"
}

// DailyPattern - simulates daily traffic cycle (high during business hours)
type DailyPattern struct{}

func (p *DailyPattern) Apply(baseCPU float64) float64 {
	hour := time.Now().Hour()

	// Peak hours:  9-11 AM and 2-4 PM
	// Low hours: 12-6 AM
	var modifier float64
	switch {
	case hour >= 9 && hour <= 11:
		modifier = 1.4 // 40% increase
	case hour >= 14 && hour <= 16:
		modifier = 1.3 // 30% increase
	case hour >= 17 && hour <= 20:
		modifier = 1.1 // 10% increase
	case hour >= 0 && hour <= 6:
		modifier = 0.6 // 40% decrease
	default:
		modifier = 1.0
	}

	result := baseCPU * modifier
	if result > 100 {
		result = 100
	}
	return result
}

func (p *DailyPattern) Name() string {
	return "daily"
}

// WeeklyPattern - includes weekend reduction
type WeeklyPattern struct{}

func (p *WeeklyPattern) Apply(baseCPU float64) float64 {
	now := time.Now()
	weekday := now.Weekday()
	hour := now.Hour()

	var modifier float64 = 1.0

	// Weekend reduction
	if weekday == time.Saturday || weekday == time.Sunday {
		modifier = 0.5
	} else {
		// Apply daily pattern on weekdays
		switch {
		case hour >= 9 && hour <= 11:
			modifier = 1.4
		case hour >= 14 && hour <= 16:
			modifier = 1.3
		case hour >= 0 && hour <= 6:
			modifier = 0.6
		}
	}

	result := baseCPU * modifier
	if result > 100 {
		result = 100
	}
	return result
}

func (p *WeeklyPattern) Name() string {
	return "weekly"
}

// RandomPattern - unpredictable spikes and drops
type RandomPattern struct{}

func (p *RandomPattern) Apply(baseCPU float64) float64 {
	// Random modifier between 0.5 and 1.5
	modifier := 0.5 + rand.Float64()
	result := baseCPU * modifier
	if result > 100 {
		result = 100
	}
	if result < 10 {
		result = 10
	}
	return result
}

func (p *RandomPattern) Name() string {
	return "random"
}

// GradualRisePattern - slowly increasing load
type GradualRisePattern struct {
	startTime time.Time
}

func (p *GradualRisePattern) Apply(baseCPU float64) float64 {
	elapsed := time.Since(p.startTime)
	minutes := elapsed.Minutes()

	// Increase by 2% per minute, capped at 50% increase
	increasePercent := math.Min(minutes*2, 50)
	modifier := 1.0 + (increasePercent / 100)

	result := baseCPU * modifier
	if result > 100 {
		result = 100
	}
	return result
}

func (p *GradualRisePattern) Name() string {
	return "gradual_rise"
}

// SineWavePattern - smooth oscillation
type SineWavePattern struct {
	Period    time.Duration
	Amplitude float64
}

func (p *SineWavePattern) Apply(baseCPU float64) float64 {
	if p.Period == 0 {
		p.Period = 10 * time.Minute
	}
	if p.Amplitude == 0 {
		p.Amplitude = 20
	}

	elapsed := float64(time.Now().UnixNano())
	periodNano := float64(p.Period.Nanoseconds())
	phase := (elapsed / periodNano) * 2 * math.Pi

	modifier := math.Sin(phase) * p.Amplitude
	result := baseCPU + modifier

	if result > 100 {
		result = 100
	}
	if result < 10 {
		result = 10
	}
	return result
}

func (p *SineWavePattern) Name() string {
	return "sine_wave"
}