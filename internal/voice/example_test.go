package voice

import (
	"os"
	"testing"
)

func loadExample(t *testing.T) (*Kit, error) {
	t.Helper()
	return Load(os.DirFS("../../examples"))
}
