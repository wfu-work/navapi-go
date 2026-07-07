package services

import "math"

const amountMicrosPerUnit int64 = 1_000_000
const amountMicrosPerCent int64 = 10_000

func AmountCentsToMicros(cents int64) int64 {
	if cents <= 0 {
		return 0
	}
	return cents * amountMicrosPerCent
}

func WholeAmountToMicros(amount int64) int64 {
	if amount <= 0 {
		return 0
	}
	return amount * amountMicrosPerUnit
}

func CostToAmountMicros(cost float64) int64 {
	if cost <= 0 {
		return 0
	}
	return int64(math.Ceil(cost * float64(amountMicrosPerUnit)))
}

func AmountMicrosToCost(micros int64) float64 {
	if micros <= 0 {
		return 0
	}
	return float64(micros) / float64(amountMicrosPerUnit)
}
