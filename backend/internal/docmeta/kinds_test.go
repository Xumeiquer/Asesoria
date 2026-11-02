package docmeta

import "testing"

func TestKindsHaveUniqueIDsAndFolders(t *testing.T) {
	ids := make(map[string]bool, len(Kinds))
	folders := make(map[string]bool, len(Kinds))
	for _, kind := range Kinds {
		if kind.ID == "" || kind.Label == "" || kind.Folder == "" {
			t.Fatalf("incomplete kind: %+v", kind)
		}
		if ids[kind.ID] {
			t.Errorf("duplicate kind ID %q", kind.ID)
		}
		if folders[kind.Folder] {
			t.Errorf("duplicate folder %q", kind.Folder)
		}
		ids[kind.ID] = true
		folders[kind.Folder] = true
		if got := KindFromFolder(kind.Folder); got != kind.ID {
			t.Errorf("KindFromFolder(%q) = %q, want %q", kind.Folder, got, kind.ID)
		}
	}
}

func TestKindForCatalogueExamples(t *testing.T) {
	cases := map[string]string{
		"Libro registro de facturas emitidas y recibidas": "libro_registro",
		"Libro diario": "libro_registro",
		"Facturas y justificantes de gastos deducibles":                       "factura",
		"Justificantes de las retenciones practicadas":                        "justificante",
		"Certificados de retención emitidos":                                  "certificado",
		"Retenciones soportadas o practicadas":                                "certificado",
		"Nóminas y seguros sociales":                                          "nomina",
		"Altas y bajas de trabajadores":                                       "datos_laborales",
		"Formulario TA.6 (inscripción de empresa)":                            "datos_laborales",
		"Modelos 303 del ejercicio":                                           "modelo",
		"Pagos fraccionados (130/131)":                                        "modelo",
		"Contratos de arrendamiento":                                          "contrato",
		"Contrato de constitución":                                            "escritura",
		"Escritura de apoderamiento, si actúa un representante":               "escritura",
		"Cuentas anuales y balance":                                           "cuentas_anuales",
		"Balance":                                                             "cuentas_anuales",
		"Inventario de bienes y derechos":                                     "inventario",
		"Valoración de los bienes donados":                                    "inventario",
		"Ajustes fiscales y amortizaciones":                                   "ajuste_fiscal",
		"Datos de módulos: índices, módulos y coeficientes":                   "datos_fiscales",
		"Código CNAE":                                                         "datos_fiscales",
		"Resumen anual por cliente y proveedor":                               "datos_fiscales",
		"Datos de los socios/comuneros":                                       "datos_societarios",
		"Declaración de titular real":                                         "datos_societarios",
		"DNI/NIE del titular o escritura de constitución, si se solicita NIF": "identificacion",
		"Extractos de exchanges":                                              "banco",
		"Mutua colaboradora y coberturas":                                     "datos_laborales",
	}
	for label, want := range cases {
		if got := KindFor(label); got != want {
			t.Errorf("KindFor(%q) = %q, want %q", label, got, want)
		}
	}
}
