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

func ATR(ohlc [][]float64, period int) []float64 {
	if len(ohlc) < period {
		return nil
	}

	tr := make([]float64, len(ohlc))
	atr := make([]float64, len(ohlc))

	// 计算 RT
	for i := 1; i < len(ohlc); i++ {
		high := ohlc[i][2]
		low := ohlc[i][3]
		prevClose := ohlc[i-1][4]

		tr1 := high - low
		tr2 := math.Abs(high - prevClose)
		tr3 := math.Abs(low - prevClose)
		tr[i] = math.Max(tr1, math.Max(tr2, tr3))
	}

	// 初始化 ATR 为前 period 天的 TR 平均值
	var sumTR float64
	for i := 1; i <= period; i++ {
		sumTR += tr[i]
	}
	atr[period] = sumTR / float64(period)

	// 计算之后的 ATR 值
	for i := period + 1; i < len(ohlc); i++ {
		atr[i] = (atr[i-1]*(float64(period)-1) + tr[i]) / float64(period)
	}

	return atr
}

func EMA(ohlc []float64, period int) []float64 {
	// 检查是否有足够的数据
	if len(ohlc) < period {
		return nil
	}

	alpha := 2.0 / float64(period+1)
	ema := make([]float64, len(ohlc))

	// 计算前 period 个收盘价的平均值作为起始 EMA 值
	var sum float64
	for i := 0; i < period; i++ {
		sum += ohlc[i]
	}
	ema[period-1] = sum / float64(period)

	// 从第 period 个数据点开始计算 EMA
	for i := period; i < len(ohlc); i++ {
		closePrice := ohlc[i]
		ema[i] = closePrice*alpha + ema[i-1]*(1-alpha)
	}

	return ema
}

func SMA(ohlc []float64, period int) []float64 {
	if len(ohlc) < period {
		return nil // 如果数据不足以计算初始的 SMA，返回空数组
	}

	sma := make([]float64, len(ohlc))
	var sum float64

	// 计算初始的 SMA
	for i := 0; i < period; i++ {
		sum += ohlc[i] // 计算前 period 个收盘价之和
	}
	sma[period-1] = sum / float64(period) // 第一个 SMA 值

	// 计算后续的 SMA
	for i := period; i < len(ohlc); i++ {
		sum += ohlc[i] - ohlc[i-period] // 添加当前收盘价并减去超出范围的收盘价
		sma[i] = sum / float64(period)  // 更新 SMA 值
	}

	return sma
}

func RMA(ohlc []float64, period int) []float64 {
	// 检查是否有足够的数据
	if len(ohlc) < period {
		return nil
	}
	ema := make([]float64, len(ohlc))
	alpha := 1.0 / float64(period)
	ema[0] = ohlc[0] // First value is the same as the first data point

	for i := 1; i < len(ohlc); i++ {
		ema[i] = alpha*ohlc[i] + (1-alpha)*ema[i-1]
	}
	return ema
}

func CRSI(ohlc []float64, period int) []float64 {
	// 检查是否有足够的数据
	if len(ohlc) < period {
		return nil
	}
	up := make([]float64, len(ohlc))
	down := make([]float64, len(ohlc))

	// Calculate up and down values
	for i := 1; i < len(ohlc); i++ {
		change := ohlc[i] - ohlc[i-1]
		up[i] = math.Max(change, 0)
		down[i] = -math.Min(change, 0)
	}

	// Apply RMA to get smoothed up and down
	upRMA := RMA(up, period)
	downRMA := RMA(down, period)

	// Calculate RSI
	rsi := make([]float64, len(ohlc))
	for i := 0; i < len(ohlc); i++ {
		if downRMA[i] == 0 {
			rsi[i] = 100
		} else if upRMA[i] == 0 {
			rsi[i] = 0
		} else {
			rsi[i] = 100 - (100 / (1 + upRMA[i]/downRMA[i]))
		}
	}

	crsi := make([]float64, len(rsi))
	for i := 0; i < len(rsi); i++ {
		if i == 0 {
			crsi[i] = rsi[i] // Set the first value to RSI[0]
		} else if i >= 4 {
			crsi[i] = 0.12*(2*rsi[i]-rsi[i-4]) + 0.88*crsi[i-1]
		} else {
			crsi[i] = rsi[i] // For the first few points, use RSI directly
		}
	}
	return crsi
}
