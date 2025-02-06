package parameters

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_ForceCompilerError(t *testing.T) {
	// Try to force a panic
	result := ValidateSingleParameterSchema(nil, nil, "", "", "", "", "", nil)

	// Ideally this would result in an error response, current behavior swallows the error
	require.Empty(t, result)
}
