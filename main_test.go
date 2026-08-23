package main

import "testing"

func TestGenFromTxtRecords(t *testing.T) {
	tests := []struct {
		name       string
		txtRecords []string
		want       int
	}{
		{name: "no records", txtRecords: nil, want: 1},
		{name: "empty records", txtRecords: []string{}, want: 1},
		{name: "gen2", txtRecords: []string{"gen=2"}, want: 2},
		{name: "gen3", txtRecords: []string{"gen=3"}, want: 3},
		{name: "gen among other records", txtRecords: []string{"id=shellyplus1-abc", "gen=2", "app=Plus1"}, want: 2},
		{name: "invalid gen falls back to 1", txtRecords: []string{"gen=abc"}, want: 1},
		{name: "zero gen falls back to 1", txtRecords: []string{"gen=0"}, want: 1},
		{name: "negative gen falls back to 1", txtRecords: []string{"gen=-1"}, want: 1},
		{name: "first valid record wins", txtRecords: []string{"gen=2", "gen=3"}, want: 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := genFromTxtRecords(tt.txtRecords); got != tt.want {
				t.Errorf("genFromTxtRecords(%v) = %d, want %d", tt.txtRecords, got, tt.want)
			}
		})
	}
}

func TestShouldUpdate(t *testing.T) {
	tests := []struct {
		name      string
		gen       int
		targetGen int
		want      bool
	}{
		{name: "no filter updates all generations", gen: 1, targetGen: 0, want: true},
		{name: "no filter updates gen4 too", gen: 4, targetGen: 0, want: true},
		{name: "gen1 filter selects gen1", gen: 1, targetGen: 1, want: true},
		{name: "gen1 filter skips gen2", gen: 2, targetGen: 1, want: false},
		{name: "gen2 filter selects gen2 only", gen: 2, targetGen: 2, want: true},
		{name: "gen2 filter skips gen1", gen: 1, targetGen: 2, want: false},
		{name: "gen2 filter skips gen3", gen: 3, targetGen: 2, want: false},
		{name: "gen3 filter selects gen3 only", gen: 3, targetGen: 3, want: true},
		{name: "gen3 filter skips gen2", gen: 2, targetGen: 3, want: false},
		{name: "gen4 filter selects gen4 only", gen: 4, targetGen: 4, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldUpdate(tt.gen, tt.targetGen); got != tt.want {
				t.Errorf("shouldUpdate(%d, %d) = %v, want %v", tt.gen, tt.targetGen, got, tt.want)
			}
		})
	}
}
