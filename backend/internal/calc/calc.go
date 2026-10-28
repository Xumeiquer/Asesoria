// Package calc implements the calculations the ERP can do by itself. It is
// intentionally conservative: every casilla mapping is versioned and any model
// without a verified calculator is left for manual completion.
package calc

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Xumeiquer/asesoria/internal/domain"
)

type Line struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Value string `json:"value"`
	Kind  string `json:"kind"` // base | tipo | cuota | resultado
}

type Result struct {
	Model      string            `json:"model"`
	Version    string            `json:"version"`
	Lines      []Line            `json:"lines"`
	Values     map[string]string `json:"values"`
	Warnings   []string          `json:"warnings"`
	Calculated string            `json:"calculated_at"`
}

func (r *Result) toMap() map[string]any {
	return map[string]any{
		"model":         r.Model,
		"version":       r.Version,
		"lines":         r.Lines,
		"values":        r.Values,
		"warnings":      r.Warnings,
		"calculated_at": r.Calculated,
	}
}

// Calculate dispatches to the calculator for a model, if one exists.
func Calculate(modelCode string, invoices []domain.Invoice, previousPaymentCents int64) (map[string]any, bool) {
	var res *Result
	switch modelCode {
	case "303":
		res = calc303(invoices)
	case "130":
		res = calc130(invoices, previousPaymentCents)
	default:
		return nil, false
	}
	res.Calculated = time.Now().UTC().Format(time.RFC3339)
	return res.toMap(), true
}

func newResult(model, version string) *Result {
	return &Result{Model: model, Version: version, Values: map[string]string{}, Lines: []Line{}}
}

func (r *Result) add(id, label string, cents int64, kind string) {
	value := formatCents(cents)
	r.Lines = append(r.Lines, Line{ID: id, Label: label, Value: value, Kind: kind})
	r.Values[id] = value
}

func (r *Result) set(id, label, value, kind string) {
	r.Lines = append(r.Lines, Line{ID: id, Label: label, Value: value, Kind: kind})
	r.Values[id] = value
}

func (r *Result) warn(format string, args ...any) {
	r.Warnings = append(r.Warnings, fmt.Sprintf(format, args...))
}

// calc303 implements the general VAT regime of the 303 form. Casillas 01-28 are
// the stable core of the form; special regimes are out of scope for now.
func calc303(invoices []domain.Invoice) *Result {
	r := newResult("303", "2026.1")

	var issued21, issued21VAT, issued10, issued10VAT, issued4, issued4VAT, issuedOther, issuedOtherVAT int64
	var invBase, invVAT, importsBase, importsVAT, interiorBase, interiorVAT int64
	var issuedCount, receivedCount int

	for _, inv := range invoices {
		switch inv.Direction {
		case domain.DirectionIssued:
			issuedCount++
			switch inv.VATRate {
			case 2100:
				issued21 += inv.BaseCents
				issued21VAT += inv.VATCents
			case 1000:
				issued10 += inv.BaseCents
				issued10VAT += inv.VATCents
			case 400:
				issued4 += inv.BaseCents
				issued4VAT += inv.VATCents
			default:
				issuedOther += inv.BaseCents
				issuedOtherVAT += inv.VATCents
			}
		case domain.DirectionReceived:
			receivedCount++
			switch inv.Category {
			case "inversion":
				invBase += inv.BaseCents
				invVAT += inv.VATCents
			case "importacion":
				importsBase += inv.BaseCents
				importsVAT += inv.VATCents
			default:
				interiorBase += inv.BaseCents
				interiorVAT += inv.VATCents
			}
		}
	}

	r.add("01", "Base imponible al 21%", issued21, "base")
	r.set("02", "Tipo impositivo", "21", "tipo")
	r.add("03", "Cuota devengada al 21%", issued21VAT, "cuota")
	r.add("04", "Base imponible al 10%", issued10, "base")
	r.set("05", "Tipo impositivo", "10", "tipo")
	r.add("06", "Cuota devengada al 10%", issued10VAT, "cuota")
	r.add("07", "Base imponible al 4%", issued4, "base")
	r.set("08", "Tipo impositivo", "4", "tipo")
	r.add("09", "Cuota devengada al 4%", issued4VAT, "cuota")
	r.add("10", "Base imponible a otros tipos", issuedOther, "base")
	r.add("12", "Cuota devengada a otros tipos", issuedOtherVAT, "cuota")
	r.add("16", "Total cuota devengada", issued21VAT+issued10VAT+issued4VAT+issuedOtherVAT, "resultado")

	r.add("17", "Base deducible en bienes de inversión", invBase, "base")
	r.add("18", "Cuota deducible en bienes de inversión", invVAT, "cuota")
	r.add("21", "Base deducible en importaciones", importsBase, "base")
	r.add("22", "Cuota deducible en importaciones", importsVAT, "cuota")
	r.add("23", "Base deducible en operaciones interiores", interiorBase, "base")
	r.add("24", "Cuota deducible en operaciones interiores", interiorVAT, "cuota")
	r.add("27", "Total cuota deducible", invVAT+importsVAT+interiorVAT, "resultado")

	devengado := issued21VAT + issued10VAT + issued4VAT + issuedOtherVAT
	deducible := invVAT + importsVAT + interiorVAT
	r.add("28", "Resultado del régimen general", devengado-deducible, "resultado")

	if issuedCount == 0 {
		r.warn("No hay facturas emitidas en el periodo: revisa que el periodo sea correcto antes de presentar.")
	}
	if receivedCount == 0 {
		r.warn("No hay facturas recibidas en el periodo: si tienes gastos deducibles, añádelos.")
	}
	if devengado-deducible < 0 {
		r.warn("El resultado es negativo: se compensará en liquidaciones siguientes según corresponda.")
	}
	r.warn("Casillas 01-28 pendientes de validar contra el formulario oficial del ejercicio 2026. Los regímenes especiales no están cubiertos.")
	return r
}

// calc130 implements the 20% IRPF instalment for estimación directa.
func calc130(invoices []domain.Invoice, previousPayments int64) *Result {
	r := newResult("130", "2026.1")
	var income, expenses, retentions int64
	for _, inv := range invoices {
		switch inv.Direction {
		case domain.DirectionIssued:
			income += inv.BaseCents
			retentions += inv.RetentionCents
		case domain.DirectionReceived:
			expenses += inv.BaseCents
		}
	}
	net := income - expenses
	instalment := int64(0)
	if net > 0 {
		instalment = (net*20 + 50) / 100
	}
	payable := instalment - retentions - previousPayments
	if payable < 0 {
		payable = 0
	}

	r.add("01", "Ingresos íntegros acumulados", income, "base")
	r.add("02", "Gastos deducibles acumulados", expenses, "base")
	r.add("03", "Rendimiento neto acumulado", net, "resultado")
	r.add("04", "20% del rendimiento neto acumulado", instalment, "cuota")
	r.add("05", "Retenciones e ingresos a cuenta", retentions, "cuota")
	r.add("06", "Pagos fraccionados anteriores", previousPayments, "cuota")
	r.add("07", "Resultado a ingresar", payable, "resultado")

	if net < 0 {
		r.warn("El rendimiento neto es negativo: no procede ingreso, pero el resultado se arrastra.")
	}
	if retentions == 0 {
		r.warn("No se han detectado retenciones en facturas emitidas. Si tus clientes retienen IRPF, regístralas.")
	}
	r.warn("Cálculo acumulado desde el 1 de enero. Se usan todas las facturas recibidas como gasto: revisa amortizaciones y gastos no deducibles.")
	r.warn("Casillas pendientes de validar contra el formulario oficial del ejercicio 2026.")
	return r
}

func formatCents(cents int64) string {
	sign := ""
	if cents < 0 {
		sign = "-"
		cents = -cents
	}
	return sign + strconv.FormatInt(cents/100, 10) + "." + fmt.Sprintf("%02d", cents%100)
}

// FormatEuros is exported for handlers that need to present amounts.
func FormatEuros(cents int64) string { return formatCents(cents) }

// ParseEuros parses a decimal string into cents, rejecting malformed input.
func ParseEuros(value string) (int64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}
	value = strings.ReplaceAll(value, ",", ".")
	negative := strings.HasPrefix(value, "-")
	value = strings.TrimPrefix(value, "-")
	parts := strings.SplitN(value, ".", 2)
	units, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, err
	}
	var cents int64
	if len(parts) == 2 {
		fraction := parts[1]
		if len(fraction) > 2 {
			fraction = fraction[:2]
		}
		for len(fraction) < 2 {
			fraction += "0"
		}
		cents, err = strconv.ParseInt(fraction, 10, 64)
		if err != nil {
			return 0, err
		}
	}
	total := units*100 + cents
	if negative {
		total = -total
	}
	return total, nil
}
