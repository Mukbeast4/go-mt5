package analysis

func SMA(data []float64, period int) []float64 {
	if period <= 0 || len(data) < period {
		return nil
	}

	result := make([]float64, len(data))
	for i := range result {
		if i < period-1 {
			continue
		}
		sum := 0.0
		for j := i - period + 1; j <= i; j++ {
			sum += data[j]
		}
		result[i] = sum / float64(period)
	}
	return result
}

func EMA(data []float64, period int) []float64 {
	if period <= 0 || len(data) < period {
		return nil
	}

	result := make([]float64, len(data))
	multiplier := 2.0 / float64(period+1)

	sum := 0.0
	for i := range period {
		sum += data[i]
	}
	result[period-1] = sum / float64(period)

	for i := period; i < len(data); i++ {
		result[i] = (data[i]-result[i-1])*multiplier + result[i-1]
	}

	return result
}
