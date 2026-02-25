package main

import "testing"

func TestValidYieldRate(t *testing.T) {
	if !validYieldRate(0.8) { t.Fatalf("0.8 should be valid") }
	if validYieldRate(0) { t.Fatalf("0 should be invalid") }
	if validYieldRate(1.1) { t.Fatalf("1.1 should be invalid") }
}

func TestValidRatioPct(t *testing.T) {
	if !validRatioPct(100) { t.Fatalf("100 should be valid") }
	if validRatioPct(0) { t.Fatalf("0 should be invalid") }
	if validRatioPct(101) { t.Fatalf("101 should be invalid") }
}

func TestRatioSumExceed(t *testing.T) {
	if !ratioSumExceed(90, 10, 25) { t.Fatalf("should exceed") }
	if ratioSumExceed(90, 10, 20) { t.Fatalf("should not exceed") }
}
