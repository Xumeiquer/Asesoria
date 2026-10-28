package library

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Xumeiquer/asesoria/internal/db"
	"github.com/Xumeiquer/asesoria/internal/domain"
	"github.com/Xumeiquer/asesoria/internal/store"
)

func testTaxpayer() domain.Taxpayer {
	return domain.Taxpayer{
		ID: "t1", NIF: "B12345678", Name: "Estudio Demo SL",
		LegalForm: domain.LegalFormSL, VATRegime: "general", IRPFMethod: "directa",
	}
}

func TestFolderNameAndTree(t *testing.T) {
	root := t.TempDir()
	taxpayer := testTaxpayer()
	created, err := EnsureTaxpayerTree(root, taxpayer, []int{2025})
	if err != nil {
		t.Fatal(err)
	}
	if created == 0 {
		t.Fatal("expected folders to be created")
	}
	base := filepath.Join(root, FolderName(taxpayer))
	for _, rel := range []string{
		"Sin clasificar",
		filepath.Join("2025", "1T", "Facturas emitidas"),
		filepath.Join("2025", "4T", "Nóminas"),
		filepath.Join("2025", "Anual", "Modelos"),
	} {
		if info, err := os.Stat(filepath.Join(base, rel)); err != nil || !info.IsDir() {
			t.Fatalf("missing %s: %v", rel, err)
		}
	}
	created, err = EnsureTaxpayerTree(root, taxpayer, []int{2025})
	if err != nil {
		t.Fatal(err)
	}
	if created != 0 {
		t.Fatalf("idempotent tree created %d folders", created)
	}
}

func TestClassifyFromFolderAndContent(t *testing.T) {
	taxpayer := testTaxpayer()
	text := "Factura recibida\nProveedor: Suministros SL B87654321\n" +
		"Cliente: Estudio Demo SL B12345678\nFecha: 12/05/2025\n" +
		"Base imponible: 100,00\nTotal factura: 121,00\n"
	rel := filepath.Join(FolderName(taxpayer), "2025", "2T", "Facturas recibidas", "factura.txt")
	c := Classify(rel, text, []domain.Taxpayer{taxpayer}, nil, time.Now())
	if c.TaxpayerID == nil || *c.TaxpayerID != taxpayer.ID {
		t.Fatalf("taxpayer not identified: %+v", c)
	}
	if c.Kind != "factura_recibida" || c.DocDate != "2025-05-12" || c.Year != 2025 || c.Period != "2T" {
		t.Fatalf("unexpected classification: %+v", c)
	}
	if c.AmountCents == nil || *c.AmountCents != 12100 {
		t.Fatalf("unexpected amount: %v", c.AmountCents)
	}
	if c.CounterpartyNIF != "B87654321" {
		t.Fatalf("unexpected counterparty: %q", c.CounterpartyNIF)
	}
	if c.Review {
		t.Fatalf("unexpected review: %v", c.Reasons)
	}
}

func TestClassifyLooseFileNeedsReview(t *testing.T) {
	c := Classify("entrada/sin-datos.txt", "documento sin información fiscal", nil, nil, time.Now())
	if c.TaxpayerID != nil || !c.Review {
		t.Fatalf("expected unassigned review document: %+v", c)
	}
}

func TestExtractPlainLatin1AndDocx(t *testing.T) {
	dir := t.TempDir()
	txt := filepath.Join(dir, "nota.txt")
	if err := os.WriteFile(txt, []byte("Facturaci\xf3n y n\xf3mina"), 0o600); err != nil {
		t.Fatal(err)
	}
	text, status, err := ExtractFile(context.Background(), txt, "spa")
	if err != nil || status != OCRText {
		t.Fatalf("extract txt: %q %v", status, err)
	}
	if !strings.Contains(text, "Facturación") {
		t.Fatalf("latin1 not decoded: %q", text)
	}

	docx := filepath.Join(dir, "nota.docx")
	f, err := os.Create(docx)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, err := zw.Create("word/document.xml")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = w.Write([]byte(`<w:document><w:body><w:p><w:r><w:t>Hola mundo</w:t></w:r></w:p></w:body></w:document>`))
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	text, status, err = ExtractFile(context.Background(), docx, "spa")
	if err != nil || status != OCRText || !strings.Contains(text, "Hola mundo") {
		t.Fatalf("extract docx: %q %q %v", text, status, err)
	}
}

func TestScanAndIngest(t *testing.T) {
	ctx := context.Background()
	tmp := t.TempDir()
	conn, err := db.Open(filepath.Join(tmp, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	st := store.New(conn)
	taxpayer := testTaxpayer()
	if err := st.CreateTaxpayer(ctx, &taxpayer); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(tmp, "library")
	inbox := filepath.Join(root, "entrada")
	if err := os.MkdirAll(inbox, 0o750); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(inbox, "factura_recibida_2025.txt")
	content := "Factura recibida\nCliente: Estudio Demo SL B12345678\n" +
		"Fecha: 12/05/2025\nTotal factura: 121,00\n"
	if err := os.WriteFile(source, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	svc := New(st)
	if _, err := svc.UpdateConfig(ctx, root, "spa"); err != nil {
		t.Fatal(err)
	}
	res, err := svc.Scan(ctx, taxpayer.ID)
	if err != nil {
		t.Fatal(err)
	}
	if res.Imported != 1 || res.NeedsReview != 0 {
		t.Fatalf("unexpected scan result: %+v", res)
	}
	docs, err := st.ListDocuments(ctx, store.DocumentFilter{TaxpayerID: taxpayer.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 1 {
		t.Fatalf("expected one document, got %d", len(docs))
	}
	doc := docs[0]
	if doc.Kind != "factura_recibida" || doc.DocDate != "2025-05-12" || doc.LibraryPath == "" {
		t.Fatalf("unexpected document: %+v", doc)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(doc.LibraryPath))); err != nil {
		t.Fatalf("library copy missing: %v", err)
	}
	if !strings.Contains(doc.TextExcerpt, "Factura recibida") {
		t.Fatalf("text not extracted: %q", doc.TextExcerpt)
	}

	res, err = svc.Scan(ctx, taxpayer.ID)
	if err != nil {
		t.Fatal(err)
	}
	if res.Imported != 0 || res.Skipped == 0 {
		t.Fatalf("incremental scan imported again: %+v", res)
	}

	upload, err := svc.Ingest(ctx, UploadInput{
		Taxpayer: taxpayer,
		Kind:     "nomina",
		Filename: "nomina_junio.txt",
		MIME:     "text/plain",
		DocDate:  "2025-06-30",
		Reader:   strings.NewReader("Nómina de junio\nTotal 1.500,00\n"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if upload.Kind != "nomina" || upload.LibraryPath == "" {
		t.Fatalf("unexpected upload: %+v", upload)
	}
	_, err = svc.Ingest(ctx, UploadInput{
		Taxpayer: taxpayer,
		Kind:     "nomina",
		Filename: "nomina_junio.txt",
		DocDate:  "2025-06-30",
		Reader:   strings.NewReader("Nómina de junio\nTotal 1.500,00\n"),
	})
	if !errorsIs(err, ErrDuplicate) {
		t.Fatalf("expected duplicate, got %v", err)
	}
}

func errorsIs(err, target error) bool {
	return err == target || (err != nil && target != nil && strings.Contains(err.Error(), target.Error()))
}
