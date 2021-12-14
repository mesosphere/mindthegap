package calculator_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mesosphere/golang-repository-template/pkg/calculator"
	"github.com/mesosphere/golang-repository-template/pkg/testutils"
)

func TestAdd(t *testing.T) {
	t.Parallel()
	tests := []struct {
		i, j, expected int
	}{{
		1, 1, 2,
	}, {
		1, 2, 3,
	}, {
		100, 200, 300,
	}}
	for _, tt := range tests {
		tt := tt // Capture range variable.
		t.Run(fmt.Sprintf("%d+%d=%d", tt.i, tt.j, tt.expected), func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, calculator.Add(tt.i, tt.j))
		})
	}
}

func TestAddIntegration(t *testing.T) {
	testutils.SkipIfShort(t, "skipping integration tests")
	t.Parallel()
	assert.Equal(t, 3, calculator.Add(1, 2))
}
