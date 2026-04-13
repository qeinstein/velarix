package docs

import (
	"testing"
)

func TestDocsDummy(t *testing.T) {
	if SwaggerInfo.Title == "" {
		t.Error("expected title to be populated by swaggo")
	}
}
