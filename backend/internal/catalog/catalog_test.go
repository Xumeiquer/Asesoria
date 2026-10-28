package catalog

import "testing"

func TestLoadInheritsCatalogUpdatedAt(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.UpdatedAt == "" {
		t.Fatal("catalog updated_at is empty")
	}
	for _, m := range c.All() {
		if m.UpdatedAt == "" {
			t.Errorf("model %s has no updated_at", m.Code)
		}
	}
}
