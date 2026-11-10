// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package gcc

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRateControllerRun(t *testing.T) {
	cases := []struct {
		name           string
		initialBitrate int
		usage          []usage
		expected       []DelayStats
	}{
		{
			name:           "empty",
			initialBitrate: 100_000,
			usage:          []usage{},
			expected:       []DelayStats{},
		},
		{
			name:           "increasesMultiplicativelyBy8000",
			initialBitrate: 100_000,
			usage:          []usage{usageNormal, usageNormal},
			expected: []DelayStats{{
				Usage:         usageNormal,
				State:         stateIncrease,
				TargetBitrate: 108_000,
				Estimate:      0,
				Threshold:     0,
			}},
		},
	}

	t0 := time.Time{}
	mockNoFn := func() time.Time {
		t0 = t0.Add(100 * time.Millisecond)

		return t0
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := make(chan DelayStats)
			dc := newRateController(mockNoFn, 100_000, 1_000, 50_000_000, func(ds DelayStats) {
				out <- ds
			})
			in := make(chan DelayStats)
			dc.onReceivedRate(100_000)
			dc.updateRTT(300 * time.Millisecond)
			go func() {
				defer close(out)
				for _, state := range tc.usage {
					dc.onDelayStats(DelayStats{
						Measurement:   0,
						Estimate:      0,
						Threshold:     0,
						Usage:         state,
						State:         0,
						TargetBitrate: 0,
					})
				}
				close(in)
			}()
			received := []DelayStats{}
			for ds := range out {
				received = append(received, ds)
			}
			if len(tc.expected) > 0 {
				assert.Equal(t, tc.expected[0], received[0])
			}
		})
	}
}

// A quiet desktop can send below the available bandwidth. Once the controller
// chooses stateIncrease, its receive-rate cap must not lower the target.
func TestRateControllerIncreasePreservesTargetForLowReceivedRate(t *testing.T) {
	var output []DelayStats
	controller := newRateController(time.Now, 100_000, 1_000, 50_000_000, func(stats DelayStats) {
		output = append(output, stats)
	})
	controller.onReceivedRate(50_000)
	controller.updateRTT(300 * time.Millisecond)
	controller.latestDecreaseRate = &exponentialMovingAverage{
		average:      50_000,
		stdDeviation: 10_000,
	}
	controller.onDelayStats(DelayStats{Usage: usageNormal})

	controller.onDelayStats(DelayStats{Usage: usageNormal})

	require.Len(t, output, 1)
	require.Equal(t, stateIncrease, output[0].State)
	require.Equal(t, 100_000, output[0].TargetBitrate)
}
