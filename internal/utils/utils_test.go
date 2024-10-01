package utils_test

import (
	"testing"

	"github.com/Yiling-J/theine-go/internal/utils"
	"github.com/stretchr/testify/require"
)

func TestUtils_RoundUpPowerOf2(t *testing.T) {
	require.Equal(t, 4, utils.RoundUpPowerOf2(3))
	require.Equal(t, 256, utils.RoundUpPowerOf2(176))
}
