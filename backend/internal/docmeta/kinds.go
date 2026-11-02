// Package docmeta defines the shared taxonomy for uploaded documents. The
// same list drives the upload selector, the catalogue checklists and the
// folders created inside the document library.
package docmeta

import (
	"strings"
	"unicode"
)

const Other = "other"

// Kind is one document type. ID is stored in documents.kind; Folder is the
// human-readable directory used by the library tree.
type Kind struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Folder string `json:"folder"`
}

// Kinds is ordered as it should appear in the upload selector.
var Kinds = []Kind{
	{ID: "factura_emitida", Label: "Factura emitida", Folder: "Facturas emitidas"},
	{ID: "factura_recibida", Label: "Factura recibida", Folder: "Facturas recibidas"},
	{ID: "factura", Label: "Factura (sin determinar)", Folder: "Facturas"},
	{ID: "nomina", Label: "Nómina", Folder: "Nóminas"},
	{ID: "recibo", Label: "Recibo", Folder: "Recibos"},
	{ID: "justificante", Label: "Justificante", Folder: "Justificantes"},
	{ID: "modelo", Label: "Modelo presentado", Folder: "Modelos"},
	{ID: "contrato", Label: "Contrato", Folder: "Contratos"},
	{ID: "seguro", Label: "Seguro", Folder: "Seguros"},
	{ID: "banco", Label: "Banco", Folder: "Banco"},
	{ID: "libro_registro", Label: "Libro registro", Folder: "Libros registro"},
	{ID: "certificado", Label: "Certificado", Folder: "Certificados"},
	{ID: "escritura", Label: "Escritura / notarial", Folder: "Escrituras"},
	{ID: "cuentas_anuales", Label: "Cuentas anuales", Folder: "Cuentas anuales"},
	{ID: "inventario", Label: "Inventario y valoración", Folder: "Inventarios y valoraciones"},
	{ID: "ajuste_fiscal", Label: "Ajuste fiscal / amortización", Folder: "Ajustes fiscales"},
	{ID: "datos_fiscales", Label: "Datos fiscales", Folder: "Datos fiscales"},
	{ID: "datos_laborales", Label: "Datos laborales y Seguridad Social", Folder: "Seguridad Social"},
	{ID: "datos_societarios", Label: "Datos societarios", Folder: "Societarios"},
	{ID: "identificacion", Label: "Identificación y representación", Folder: "Identificación"},
	{ID: Other, Label: "Otro", Folder: "Otros"},
}

var (
	byID     = make(map[string]Kind, len(Kinds))
	byFolder = make(map[string]string, len(Kinds))
)

func init() {
	for _, k := range Kinds {
		byID[k.ID] = k
		byFolder[normalize(k.Folder)] = k.ID
	}
}

// All returns the supported document types.
func All() []Kind {
	out := make([]Kind, len(Kinds))
	copy(out, Kinds)
	return out
}

// Get returns the kind with the given ID, or the fallback "other" kind.
func Get(id string) Kind {
	if k, ok := byID[strings.TrimSpace(id)]; ok {
		return k
	}
	return byID[Other]
}

// Folder returns the library folder for a kind.
func Folder(id string) string {
	return Get(id).Folder
}

// KindFromFolder resolves a folder name back to its document kind.
func KindFromFolder(name string) string {
	return byFolder[normalize(name)]
}

// Requirement is a catalogue checklist item mapped to a document kind.
type Requirement struct {
	Label  string `json:"label"`
	Kind   string `json:"kind"`
	Folder string `json:"folder"`
}

// RequirementFor maps a free-text catalogue document to a kind and folder.
func RequirementFor(label string) Requirement {
	kind := KindFor(label)
	return Requirement{Label: label, Kind: kind, Folder: Folder(kind)}
}

// KindFor maps the free-text document lists in the catalogue to the shared
// taxonomy. The rules are intentionally explicit and ordered: a document such
// as "Libro registro de facturas emitidas y recibidas" belongs in the books
// folder, not in the invoices folder.
func KindFor(label string) string {
	s := normalize(label)
	switch {
	case s == "":
		return Other

	// Corporate resolutions that happen to mention annual accounts.
	case strings.Contains(s, "certificacion de los acuerdos"):
		return "datos_societarios"

	// Identity documents. Checked before "escritura" because some entries
	// combine a DNI/NIE with an incorporation deed.
	case strings.Contains(s, "dni"), strings.Contains(s, "nie"),
		strings.Contains(s, "nif de la empresa"), strings.Contains(s, "representante legal"):
		return "identificacion"

	// Personal/family data for IRPF, not an insurance policy.
	case strings.Contains(s, "datos de seguros, vivienda y familia"):
		return "datos_fiscales"

	// An incorporation contract is a notarial document, not a rental or
	// professional services contract.
	case strings.Contains(s, "contrato de constitucion"):
		return "escritura"

	// This entry can also be filed as a certificate, but it belongs with the
	// succession documents.
	case strings.Contains(s, "testamento"):
		return "escritura"

	case strings.HasPrefix(s, "libro "):
		return "libro_registro"

	case strings.Contains(s, "nomina"):
		return "nomina"

	case strings.Contains(s, "factura"), strings.Contains(s, "importacion"),
		strings.Contains(s, "exportacion"):
		return "factura"

	case strings.Contains(s, "justificante"), strings.Contains(s, "carta de pago"):
		return "justificante"

	case strings.Contains(s, "certificado"), strings.Contains(s, "certificacion"),
		strings.Contains(s, "retenciones"):
		return "certificado"

	case strings.Contains(s, "trabajador"), strings.Contains(s, "perceptores"),
		strings.Contains(s, "altas y bajas"), strings.Contains(s, "cotizacion"),
		strings.Contains(s, "seguridad social"), strings.Contains(s, "parte de alta"),
		strings.Contains(s, "parte de baja"), strings.Contains(s, "partes de confirmacion"),
		strings.Contains(s, "incapacidad temporal"), strings.Contains(s, "mutua"),
		strings.Contains(s, "ta.6"), strings.Contains(s, "ta.7"),
		strings.Contains(s, "historial de cotizacion"), strings.Contains(s, "incidencias del mes"),
		strings.Contains(s, "causa y fecha de baja"):
		return "datos_laborales"

	case strings.Contains(s, "modelo"), strings.Contains(s, "autoliquidacion"),
		strings.Contains(s, "pagos fraccionados"):
		return "modelo"

	case strings.Contains(s, "contrato"), strings.Contains(s, "arrendamiento"),
		strings.Contains(s, "arrendadores"), strings.Contains(s, "no residentes"):
		return "contrato"

	case strings.Contains(s, "cuenta bancaria"), strings.Contains(s, "extracto"),
		strings.Contains(s, "extractos"), strings.Contains(s, "exchanges"):
		return "banco"

	case strings.Contains(s, "escritura"), strings.Contains(s, "notarial"),
		strings.Contains(s, "titulos de propiedad"), strings.Contains(s, "testamento"),
		strings.Contains(s, "declaracion de herederos"), strings.Contains(s, "ultimas voluntades"),
		strings.Contains(s, "donacion"):
		return "escritura"

	case strings.Contains(s, "cuentas anuales"), strings.Contains(s, "balance"),
		strings.Contains(s, "perdidas y ganancias"), strings.Contains(s, "memoria"),
		strings.Contains(s, "patrimonio neto"), strings.Contains(s, "flujos de efectivo"),
		strings.Contains(s, "informe de gestion"), strings.Contains(s, "informe del auditor"),
		strings.Contains(s, "propuesta de aplicacion del resultado"),
		strings.Contains(s, "rentas y gastos de la entidad"),
		strings.Contains(s, "resultado contable"):
		return "cuentas_anuales"

	// Valuation methods for related-party operations are fiscal data, while
	// asset valuations belong in the inventory folder.
	case strings.Contains(s, "metodos de valoracion"):
		return "datos_fiscales"

	case strings.Contains(s, "inventario"), strings.Contains(s, "valoracion"),
		strings.Contains(s, "valoraciones"), strings.Contains(s, "deudas y cargas"):
		return "inventario"

	case strings.Contains(s, "ajuste"), strings.Contains(s, "amortizacion"),
		strings.Contains(s, "deducciones"), strings.Contains(s, "bonificaciones"),
		strings.Contains(s, "estimacion de rendimientos"):
		return "ajuste_fiscal"

	case strings.Contains(s, "socios"), strings.Contains(s, "comuneros"),
		strings.Contains(s, "titular real"), strings.Contains(s, "acuerdos de aprobacion"):
		return "datos_societarios"

	case strings.Contains(s, "cnae"), strings.Contains(s, "iae"),
		strings.Contains(s, "actividad"), strings.Contains(s, "modulos"),
		strings.Contains(s, "cliente"), strings.Contains(s, "proveedor"),
		strings.Contains(s, "nif-iva"), strings.Contains(s, "operaciones vinculadas"),
		strings.Contains(s, "documentacion del grupo"), strings.Contains(s, "direcciones y saldos"),
		strings.Contains(s, "local"), strings.Contains(s, "domicilio"),
		strings.Contains(s, "fecha de inicio"), strings.Contains(s, "datos fiscales"):
		return "datos_fiscales"

	case strings.Contains(s, "seguro"):
		return "seguro"

	case strings.Contains(s, "recibo"):
		return "recibo"
	}
	return Other
}

var accentReplacer = strings.NewReplacer(
	"á", "a", "à", "a", "ä", "a", "â", "a", "ã", "a",
	"é", "e", "è", "e", "ë", "e", "ê", "e",
	"í", "i", "ì", "i", "ï", "i", "î", "i",
	"ó", "o", "ò", "o", "ö", "o", "ô", "o", "õ", "o",
	"ú", "u", "ù", "u", "ü", "u", "û", "u",
	"ñ", "n", "ç", "c",
)

func normalize(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = accentReplacer.Replace(value)
	value = strings.Map(func(r rune) rune {
		if unicode.Is(unicode.Mn, r) {
			return -1
		}
		return r
	}, value)
	return strings.Join(strings.Fields(value), " ")
}
