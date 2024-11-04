package main

import (
	"math"
)

func RSI(ohlc []float64, period int) []float64 {
	if len(ohlc) < period {
		return nil
	}

	rsi := make([]float64, len(ohlc))
	gains := make([]float64, len(ohlc))
	losses := make([]float64, len(ohlc))

	// 计算每日的涨跌
	for i := 1; i < len(ohlc); i++ {
		change := ohlc[i] - ohlc[i-1]
		gains[i] = math.Max(0, change)
		losses[i] = math.Max(0, -change)
	}

	// 计算初始平均涨幅和跌幅
	var avgGain, avgLoss float64
	for i := 1; i <= period; i++ {
		avgGain += gains[i]
		avgLoss += losses[i]
	}
	avgGain /= float64(period)
	avgLoss /= float64(period)

	// 从第 period+1 个数据点开始计算 RSI
	for i := period; i < len(ohlc); i++ {
		if i > period {
			avgGain = (avgGain*(float64(period)-1) + gains[i]) / float64(period)
			avgLoss = (avgLoss*(float64(period)-1) + losses[i]) / float64(period)
		}

		// 计算 RSI
		if avgLoss == 0 {
			rsi[i] = 100 // 避免除以 0
		} else {
			rs := avgGain / avgLoss
			rsi[i] = 100 - (100 / (1 + rs))
		}
	}

	return rsi
}
