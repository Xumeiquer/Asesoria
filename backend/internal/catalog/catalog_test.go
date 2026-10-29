package catalog

import (
	"strings"
	"testing"
)

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

func TestModelsHaveDescriptionAndOfficialURL(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	for _, m := range c.All() {
		if strings.TrimSpace(m.Description) == "" {
			t.Errorf("model %s has no description", m.Code)
		}
		if !strings.HasPrefix(m.URL, "https://") {
			t.Errorf("model %s has a non-HTTPS URL: %q", m.Code, m.URL)
		}
		if strings.TrimSpace(m.Periodicity) == "" {
			t.Errorf("model %s has no periodicity", m.Code)
		}
	}
}

func TestDocumentationChecklists(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	for _, m := range c.All() {
		if len(m.Documents) == 0 {
			if !m.Obsolete && m.Code != "RETA-COT" {
				t.Errorf("model %s has no documents checklist", m.Code)
			}
			continue
		}
		seen := make(map[string]bool, len(m.Documents))
		for _, raw := range m.Documents {
			doc := strings.TrimSpace(raw)
			if doc == "" {
				t.Errorf("model %s has an empty document entry", m.Code)
				continue
			}
			if seen[doc] {
				t.Errorf("model %s repeats document %q", m.Code, doc)
			}
			seen[doc] = true
			if m.Code != "037" && strings.Contains(doc, "037") {
				t.Errorf("model %s references the obsolete model 037 in its checklist: %q", m.Code, doc)
			}
		}
	}
}
