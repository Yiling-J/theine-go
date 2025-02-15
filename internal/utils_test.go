package internal

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUtils_RoundUpPowerOf2(t *testing.T) {
	tests := []struct {
		input    uint32
		expected uint32
	}{
		{0, 1},
		{1, 1},
		{2, 2},
		{3, 4},
		{7, 8},
		{8, 8},
		{9999, 16384},
	}

	for _, tc := range tests {
		t.Run(fmt.Sprintf("%+v", tc), func(t *testing.T) {
			result := RoundUpPowerOf2(tc.input)
			require.Equal(t, tc.expected, result)
		})
	}
}
