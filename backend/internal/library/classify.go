package library

import (
	"math"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Xumeiquer/asesoria/internal/domain"
)

type Classification struct {
	TaxpayerID      *string
	Kind            string
	DocDate         string
	Year            int
	Period          string
	AmountCents     *int64
	CounterpartyNIF string
	ModelCode       string
	PeriodLabel     string
	Review          bool
	Reasons         []string
}

var (
	nifPatterns = []*regexp.Regexp{
		regexp.MustCompile(`\b\d{8}[A-Za-z]\b`),
		regexp.MustCompile(`\b[A-Za-z]\d{7}[A-Za-z0-9]\b`),
		regexp.MustCompile(`\b[XYZxyz]\d{7}[A-Za-z]\b`),
	}
	datePattern   = regexp.MustCompile(`(?i)\b(\d{1,2})[/.-](\d{1,2})[/.-](\d{2,4})\b`)
	isoPattern    = regexp.MustCompile(`\b(20\d{2})-(\d{1,2})-(\d{1,2})\b`)
	amountPattern = regexp.MustCompile(`(?i)(?:total(?:\s+factura)?|importe\s+total|base\s+imponible)[^\d\n]{0,30}([\d.]+,\d{2}|\d+(?:\.\d{3})*\.\d{2})`)
	modelPattern  = regexp.MustCompile(`\b(303|130|111|115|180|190|390|347|349|202|200|123|131|184|193|216|296|714|720|721)\b`)
	periodFirst   = regexp.MustCompile(`(?i)\b([1-4]T)\s*[-/]?\s*(20\d{2})\b`)
	periodLast    = regexp.MustCompile(`(?i)\b(20\d{2})\s*[-/]?\s*([1-4]T)\b`)
	yearPattern   = regexp.MustCompile(`\b(20\d{2})\b`)
)

var annualModels = map[string]bool{
	"180": true, "190": true, "193": true, "347": true, "349": true,
	"390": true, "714": true, "720": true, "721": true, "200": true,
}

// Classify inspects the relative path and extracted text. The folder taxonomy
// is the strongest signal; content is used for loose files or missing metadata.
func Classify(relPath, text string, taxpayers []domain.Taxpayer, active *domain.Taxpayer, modTime time.Time) Classification {
	c := Classification{Kind: "other"}
	segments := splitPath(relPath)
	var t *domain.Taxpayer
	if len(segments) > 0 {
		t = taxpayerFromFolder(segments[0], taxpayers)
	}

	var folderKind string
	var folderYear int
	var folderPeriod string
	for _, seg := range segments[1:] {
		if folderKind == "" {
			if kind := KindFromFolder(seg); kind != "" {
				folderKind = kind
			}
		}
		if folderYear == 0 {
			if year, err := strconv.Atoi(seg); err == nil && year >= 2000 && year <= 2100 {
				folderYear = year
			}
		}
		if folderPeriod == "" {
			switch strings.ToUpper(strings.TrimSpace(seg)) {
			case "1T", "2T", "3T", "4T":
				folderPeriod = strings.ToUpper(strings.TrimSpace(seg))
			case "ANUAL":
				folderPeriod = "Anual"
			case "SIN PERIODO", "SIN PERÍODO":
				folderPeriod = "Sin periodo"
			}
		}
	}

	nifs := findNIFs(text)
	if t == nil {
		t = taxpayerFromNIFs(nifs, taxpayers)
	}
	if t == nil && active != nil {
		t = active
		c.Review = true
		c.Reasons = append(c.Reasons, "sin NIF reconocible; asignado al contribuyente activo")
	}
	if t == nil {
		c.Review = true
		c.Reasons = append(c.Reasons, "no se pudo identificar el contribuyente")
	} else {
		id := t.ID
		c.TaxpayerID = &id
	}

	if folderKind != "" {
		c.Kind = folderKind
	} else {
		c.Kind = kindFromFilename(segments)
	}
	if c.Kind == "" {
		c.Kind = kindFromText(text)
	}
	if c.Kind == "" {
		c.Kind = "factura"
		c.Review = true
		c.Reasons = append(c.Reasons, "parece una factura pero no se pudo determinar si es emitida o recibida")
	}
	if c.Kind == "factura" {
		c.Review = true
		c.Reasons = append(c.Reasons, "dirección de factura sin confirmar")
	}

	date := dateFromText(text)
	if date == "" {
		date = modTime.Format("2006-01-02")
		c.Review = true
		c.Reasons = append(c.Reasons, "fecha tomada del fichero")
	}
	c.DocDate = date
	year, period := PeriodFromDate(date)
	if folderYear > 0 {
		year = folderYear
	}
	if folderPeriod != "" {
		period = folderPeriod
	}
	if model := modelPattern.FindString(text); model != "" {
		c.ModelCode = model
		annual := annualModels[model]
		if annual && folderPeriod == "" {
			period = "Anual"
		}
		if label := periodLabel(text, year, annual); label != "" {
			c.PeriodLabel = label
		}
	}
	c.Year = year
	c.Period = period

	c.AmountCents = amountFromText(text)
	c.CounterpartyNIF = counterparty(nifs, t)
	return c
}

func splitPath(rel string) []string {
	rel = filepath.ToSlash(rel)
	parts := strings.Split(rel, "/")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" && p != "." {
			out = append(out, p)
		}
	}
	return out
}

func taxpayerFromFolder(folder string, taxpayers []domain.Taxpayer) *domain.Taxpayer {
	folder = strings.TrimSpace(folder)
	for i := range taxpayers {
		t := &taxpayers[i]
		if folder == FolderName(*t) {
			return t
		}
		nif := strings.ToUpper(strings.TrimSpace(t.NIF))
		upper := strings.ToUpper(folder)
		if nif != "" && (upper == nif || strings.HasSuffix(upper, "("+nif+")") || strings.Contains(upper, " "+nif+" ")) {
			return t
		}
	}
	return nil
}

func taxpayerFromNIFs(nifs []string, taxpayers []domain.Taxpayer) *domain.Taxpayer {
	found := map[string]*domain.Taxpayer{}
	for i := range taxpayers {
		found[normalizeNIF(taxpayers[i].NIF)] = &taxpayers[i]
	}
	var match *domain.Taxpayer
	for _, nif := range nifs {
		if t := found[nif]; t != nil {
			if match != nil && match.ID != t.ID {
				return nil
			}
			match = t
		}
	}
	return match
}

func findNIFs(text string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, re := range nifPatterns {
		for _, raw := range re.FindAllString(text, -1) {
			nif := normalizeNIF(raw)
			if nif == "" || seen[nif] {
				continue
			}
			seen[nif] = true
			out = append(out, nif)
		}
	}
	return out
}

func normalizeNIF(value string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(value) {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func counterparty(nifs []string, taxpayer *domain.Taxpayer) string {
	own := ""
	if taxpayer != nil {
		own = normalizeNIF(taxpayer.NIF)
	}
	for _, nif := range nifs {
		if nif != "" && nif != own {
			return nif
		}
	}
	return ""
}

func kindFromFilename(segments []string) string {
	if len(segments) == 0 {
		return ""
	}
	name := strings.ToLower(segments[len(segments)-1])
	has := func(parts ...string) bool {
		for _, p := range parts {
			if strings.Contains(name, p) {
				return true
			}
		}
		return false
	}
	switch {
	case has("emitida", "emitidas", "expedida"):
		return "factura_emitida"
	case has("recibida", "recibidas", "gasto", "proveedor"):
		return "factura_recibida"
	case has("nomina", "nómina", "salario"):
		return "nomina"
	case has("justificante", "carta de pago", "nrc", "acuse"):
		return "justificante"
	case has("modelo", "autoliquidacion", "autoliquidación", "declaracion", "declaración"):
		return "modelo"
	case has("contrato"):
		return "contrato"
	case has("seguro", "poliza", "póliza"):
		return "seguro"
	case has("banco", "extracto", "domiciliacion", "domiciliación"):
		return "banco"
	case has("recibo"):
		return "recibo"
	case has("factura"):
		// Direction is usually in the document body, not in the filename.
		return ""
	}
	return ""
}

func kindFromText(text string) string {
	lower := strings.ToLower(text)
	switch {
	case strings.Contains(lower, "factura emitida") || strings.Contains(lower, "factura expedida"):
		return "factura_emitida"
	case strings.Contains(lower, "factura recibida"):
		return "factura_recibida"
	case strings.Contains(lower, "nómina") || strings.Contains(lower, "nomina"):
		return "nomina"
	case strings.Contains(lower, "justificante") || strings.Contains(lower, "carta de pago"):
		return "justificante"
	case strings.Contains(lower, "contrato"):
		return "contrato"
	case strings.Contains(lower, "póliza") || strings.Contains(lower, "poliza"):
		return "seguro"
	case strings.Contains(lower, "extracto") || strings.Contains(lower, "domiciliación") || strings.Contains(lower, "domiciliacion"):
		return "banco"
	case strings.Contains(lower, "modelo") || strings.Contains(lower, "autoliquidación") || strings.Contains(lower, "autoliquidacion"):
		return "modelo"
	case strings.Contains(lower, "recibo"):
		return "recibo"
	case strings.Contains(lower, "factura"):
		return "factura"
	}
	return ""
}

func dateFromText(text string) string {
	for _, match := range isoPattern.FindAllStringSubmatch(text, -1) {
		if d := validDate(match[1], match[2], match[3]); d != "" {
			return d
		}
	}
	for _, match := range datePattern.FindAllStringSubmatch(text, -1) {
		if d := validDate(match[3], match[2], match[1]); d != "" {
			return d
		}
	}
	return ""
}

func validDate(yearRaw, monthRaw, dayRaw string) string {
	year, _ := strconv.Atoi(yearRaw)
	month, _ := strconv.Atoi(monthRaw)
	day, _ := strconv.Atoi(dayRaw)
	if year < 100 {
		if year < 70 {
			year += 2000
		} else {
			year += 1900
		}
	}
	if year < 1990 || year > 2100 || month < 1 || month > 12 || day < 1 || day > 31 {
		return ""
	}
	t := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	if t.Year() != year || int(t.Month()) != month || t.Day() != day {
		return ""
	}
	return t.Format("2006-01-02")
}

func amountFromText(text string) *int64 {
	matches := amountPattern.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return nil
	}
	// Prefer an explicit total over a base imponible when both appear.
	for _, match := range matches {
		if len(match) >= 2 && strings.Contains(strings.ToLower(match[0]), "total") {
			if cents := parseAmount(match[1]); cents != nil {
				return cents
			}
		}
	}
	for _, match := range matches {
		if len(match) >= 2 {
			if cents := parseAmount(match[1]); cents != nil {
				return cents
			}
		}
	}
	return nil
}

func parseAmount(raw string) *int64 {
	if strings.Contains(raw, ",") {
		raw = strings.ReplaceAll(raw, ".", "")
		raw = strings.ReplaceAll(raw, ",", ".")
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil || value < 0 || value > 1e9 {
		return nil
	}
	cents := int64(math.Round(value * 100))
	return &cents
}

func periodLabel(text string, fallbackYear int, annual bool) string {
	if m := periodFirst.FindStringSubmatch(text); len(m) == 3 {
		return strings.ToUpper(m[1]) + " " + m[2]
	}
	if m := periodLast.FindStringSubmatch(text); len(m) == 3 {
		return strings.ToUpper(m[2]) + " " + m[1]
	}
	if annual && fallbackYear > 0 {
		if m := yearPattern.FindString(text); m != "" {
			return "Anual " + m
		}
		return "Anual " + strconv.Itoa(fallbackYear)
	}
	return ""
}
