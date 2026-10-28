// Package catalog loads the declarative catalogue of tax and social security
// obligations. Keeping the catalogue as data (not code) is deliberate: AEAT and
// TGSS change forms, casillas and deadlines every year.
package catalog

import (
	_ "embed"
	"fmt"
	"sort"

	"github.com/Xumeiquer/asesoria/internal/domain"
	"gopkg.in/yaml.v3"
)

//go:embed data/models.yaml
var rawCatalog []byte

// Model describes one obligation/model in the catalogue.
type Model struct {
	Code        string    `yaml:"code" json:"code"`
	Name        string    `yaml:"name" json:"name"`
	Entity      string    `yaml:"entity" json:"entity"`
	Category    string    `yaml:"category" json:"category"`
	Periodicity string    `yaml:"periodicity" json:"periodicity"`
	UpdatedAt   string    `yaml:"updated_at,omitempty" json:"updated_at,omitempty"`
	Description string    `yaml:"description" json:"description"`
	URL         string    `yaml:"url" json:"url"`
	Auto        bool      `yaml:"auto" json:"auto"`
	Obsolete    bool      `yaml:"obsolete,omitempty" json:"obsolete,omitempty"`
	Applies     Applies   `yaml:"applies" json:"applies"`
	Schedule    *Schedule `yaml:"schedule,omitempty" json:"schedule,omitempty"`
	Documents   []string  `yaml:"documents" json:"documents"`
	Notes       string    `yaml:"notes" json:"notes"`
}

// Applies is a set of conditions. Empty fields mean "no constraint".
type Applies struct {
	LegalForms       []string `yaml:"legal_forms,omitempty" json:"legal_forms,omitempty"`
	VATRegimes       []string `yaml:"vat_regimes,omitempty" json:"vat_regimes,omitempty"`
	IRPFMethods      []string `yaml:"irpf_methods,omitempty" json:"irpf_methods,omitempty"`
	HasEmployees     *bool    `yaml:"has_employees,omitempty" json:"has_employees,omitempty"`
	HasRentals       *bool    `yaml:"has_rentals,omitempty" json:"has_rentals,omitempty"`
	EconomicActivity *bool    `yaml:"economic_activity,omitempty" json:"economic_activity,omitempty"`
}

// Schedule defines when an obligation is due and which period it covers.
type Schedule struct {
	// Type is monthly, quarterly, annual or custom.
	Type string `yaml:"type" json:"type"`
	// DueMonths is used by quarterly (and optionally monthly) schedules.
	DueMonths []int `yaml:"due_months,omitempty" json:"due_months,omitempty"`
	// DueDay is 0 for "last day of month".
	DueDay int `yaml:"due_day" json:"due_day"`
	// PeriodOffsetMonths is used by monthly schedules (default -1).
	PeriodOffsetMonths *int `yaml:"period_offset_months,omitempty" json:"period_offset_months,omitempty"`
	// PeriodYearOffset is used by annual schedules (default -1).
	PeriodYearOffset *int `yaml:"period_year_offset,omitempty" json:"period_year_offset,omitempty"`
	// Dues is used by custom schedules. Dates are MM-DD and period years are
	// relative to the due year.
	Dues []CustomDue `yaml:"dues,omitempty" json:"dues,omitempty"`
}

type CustomDue struct {
	Month            int    `yaml:"month" json:"month"`
	Day              int    `yaml:"day" json:"day"`
	PeriodStart      string `yaml:"period_start" json:"period_start"` // MM-DD
	PeriodEnd        string `yaml:"period_end" json:"period_end"`     // MM-DD
	PeriodYearOffset int    `yaml:"period_year_offset" json:"period_year_offset"`
}

type file struct {
	Version   string  `yaml:"version"`
	UpdatedAt string  `yaml:"updated_at"`
	Models    []Model `yaml:"models"`
}

// Catalog is the in-memory, immutable catalogue.
type Catalog struct {
	Version   string
	UpdatedAt string
	models    []Model
	byCode    map[string]Model
}

// Load parses the embedded catalogue.
func Load() (*Catalog, error) {
	var f file
	if err := yaml.Unmarshal(rawCatalog, &f); err != nil {
		return nil, fmt.Errorf("parse catalog: %w", err)
	}
	if len(f.Models) == 0 {
		return nil, fmt.Errorf("catalog is empty")
	}
	if f.UpdatedAt == "" {
		return nil, fmt.Errorf("catalog has no updated_at")
	}
	for i := range f.Models {
		if f.Models[i].UpdatedAt == "" {
			f.Models[i].UpdatedAt = f.UpdatedAt
		}
	}
	c := &Catalog{
		Version:   f.Version,
		UpdatedAt: f.UpdatedAt,
		models:    f.Models,
		byCode:    make(map[string]Model, len(f.Models)),
	}
	for _, m := range f.Models {
		if m.Code == "" {
			return nil, fmt.Errorf("catalog contains a model without code")
		}
		if _, dup := c.byCode[m.Code]; dup {
			return nil, fmt.Errorf("duplicate model code %q", m.Code)
		}
		c.byCode[m.Code] = m
	}
	sort.Slice(c.models, func(i, j int) bool { return c.models[i].Code < c.models[j].Code })
	return c, nil
}

// All returns every model sorted by code.
func (c *Catalog) All() []Model { return c.models }

// Get returns a model by code.
func (c *Catalog) Get(code string) (Model, bool) {
	m, ok := c.byCode[code]
	return m, ok
}

// AppliesTo reports whether the model corresponds to the taxpayer profile.
func (m Model) AppliesTo(t domain.Taxpayer) bool {
	a := m.Applies
	if len(a.LegalForms) > 0 && !contains(a.LegalForms, t.LegalForm) {
		return false
	}
	if len(a.VATRegimes) > 0 && !contains(a.VATRegimes, t.VATRegime) {
		return false
	}
	if len(a.IRPFMethods) > 0 && !contains(a.IRPFMethods, t.IRPFMethod) {
		return false
	}
	if a.HasEmployees != nil && *a.HasEmployees != t.HasEmployees {
		return false
	}
	if a.HasRentals != nil && *a.HasRentals != t.HasRentals {
		return false
	}
	if a.EconomicActivity != nil && *a.EconomicActivity != t.EconomicActivity {
		return false
	}
	return true
}

func contains(list []string, value string) bool {
	for _, v := range list {
		if v == value {
			return true
		}
	}
	return false
}
