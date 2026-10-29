// Package calendar expands the declarative catalogue into concrete filings for
// a taxpayer and a year. Dates are due dates, not period dates.
package calendar

import (
	"fmt"
	"time"

	"github.com/Xumeiquer/asesoria/internal/catalog"
	"github.com/Xumeiquer/asesoria/internal/domain"
)

type Generator struct {
	Catalog *catalog.Catalog
}

func New(c *catalog.Catalog) *Generator { return &Generator{Catalog: c} }

var monthNames = []string{"", "Enero", "Febrero", "Marzo", "Abril", "Mayo", "Junio",
	"Julio", "Agosto", "Septiembre", "Octubre", "Noviembre", "Diciembre"}

// Generate returns one filing per applicable period whose due date falls in year.
func (g *Generator) Generate(t domain.Taxpayer, year int) []domain.Filing {
	out := make([]domain.Filing, 0, 40)
	for _, m := range g.Catalog.All() {
		if !m.Auto || m.Schedule == nil {
			continue
		}
		if !m.AppliesTo(t) {
			continue
		}
		for _, p := range g.periodsFor(m, year) {
			if t.ActivityStart != "" && p.end.Format("2006-01-02") < t.ActivityStart {
				continue
			}
			out = append(out, domain.Filing{
				TaxpayerID:  t.ID,
				ModelCode:   m.Code,
				ModelName:   m.Name,
				Entity:      m.Entity,
				PeriodLabel: p.label,
				PeriodStart: p.start.Format("2006-01-02"),
				PeriodEnd:   p.end.Format("2006-01-02"),
				DueDate:     p.due.Format("2006-01-02"),
				Status:      domain.StatusPending,
			})
		}
	}
	return out
}

type period struct {
	label string
	start time.Time
	end   time.Time
	due   time.Time
}

func (g *Generator) periodsFor(m catalog.Model, year int) []period {
	s := m.Schedule
	switch s.Type {
	case "monthly":
		offset := -1
		if s.PeriodOffsetMonths != nil {
			offset = *s.PeriodOffsetMonths
		}
		out := make([]period, 0, 12)
		for dueMonth := 1; dueMonth <= 12; dueMonth++ {
			periodMonth := dueMonth + offset
			periodYear := year
			for periodMonth < 1 {
				periodMonth += 12
				periodYear--
			}
			for periodMonth > 12 {
				periodMonth -= 12
				periodYear++
			}
			start := time.Date(periodYear, time.Month(periodMonth), 1, 0, 0, 0, 0, time.UTC)
			end := lastDay(periodYear, periodMonth)
			due := dueDate(year, dueMonth, s.DueDay)
			out = append(out, period{
				label: fmt.Sprintf("%s %d", monthNames[periodMonth], periodYear),
				start: start,
				end:   end,
				due:   due,
			})
		}
		return out

	case "quarterly":
		months := s.DueMonths
		if len(months) == 0 {
			months = []int{1, 4, 7, 10}
		}
		out := make([]period, 0, len(months))
		for _, dueMonth := range months {
			endMonth := dueMonth - 1
			endYear := year
			if endMonth == 0 {
				endMonth = 12
				endYear--
			}
			startMonth := endMonth - 2
			startYear := endYear
			for startMonth < 1 {
				startMonth += 12
				startYear--
			}
			quarter := (endMonth-1)/3 + 1
			out = append(out, period{
				label: fmt.Sprintf("%dT %d", quarter, endYear),
				start: time.Date(startYear, time.Month(startMonth), 1, 0, 0, 0, 0, time.UTC),
				end:   lastDay(endYear, endMonth),
				due:   dueDate(year, dueMonth, s.DueDay),
			})
		}
		return out

	case "annual":
		dueMonth := 1
		if len(s.DueMonths) > 0 {
			dueMonth = s.DueMonths[0]
		}
		offset := -1
		if s.PeriodYearOffset != nil {
			offset = *s.PeriodYearOffset
		}
		periodYear := year + offset
		return []period{{
			label: fmt.Sprintf("%d", periodYear),
			start: time.Date(periodYear, 1, 1, 0, 0, 0, 0, time.UTC),
			end:   time.Date(periodYear, 12, 31, 0, 0, 0, 0, time.UTC),
			due:   dueDate(year, dueMonth, s.DueDay),
		}}

	case "custom":
		out := make([]period, 0, len(s.Dues))
		for _, d := range s.Dues {
			periodYear := year + d.PeriodYearOffset
			startMonth, startDay := parseMD(d.PeriodStart)
			endMonth, endDay := parseMD(d.PeriodEnd)
			out = append(out, period{
				label: periodLabel(startMonth, startDay, endMonth, endDay, periodYear),
				start: time.Date(periodYear, time.Month(startMonth), startDay, 0, 0, 0, 0, time.UTC),
				end:   time.Date(periodYear, time.Month(endMonth), endDay, 0, 0, 0, 0, time.UTC),
				due:   dueDate(year, d.Month, d.Day),
			})
		}
		return out
	}
	return nil
}

func dueDate(year, month, day int) time.Time {
	var t time.Time
	if day == 0 {
		t = lastDay(year, month)
	} else {
		t = time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	}
	// AEAT/TGSS move deadlines that fall on a weekend to the next business day.
	switch t.Weekday() {
	case time.Saturday:
		t = t.AddDate(0, 0, 2)
	case time.Sunday:
		t = t.AddDate(0, 0, 1)
	}
	return t
}

func lastDay(year, month int) time.Time {
	return time.Date(year, time.Month(month)+1, 0, 0, 0, 0, 0, time.UTC)
}

func parseMD(value string) (int, int) {
	var month, day int
	_, _ = fmt.Sscanf(value, "%d-%d", &month, &day)
	return month, day
}

func periodLabel(startMonth, startDay, endMonth, endDay, year int) string {
	if startMonth == 1 && startDay == 1 && endMonth == 3 && endDay == 31 {
		return fmt.Sprintf("1T %d", year)
	}
	return fmt.Sprintf("%s-%s %d", shortMonth(startMonth), shortMonth(endMonth), year)
}

func shortMonth(month int) string {
	if month < 1 || month > 12 {
		return "?"
	}
	return monthNames[month][:3]
}
