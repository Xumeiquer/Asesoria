// Package domain contains the core entities of the application.
package domain

const (
	LegalFormAutonomo = "autonomo"
	LegalFormSL       = "sl"

	StatusPending       = "pending"
	StatusInProgress    = "in_progress"
	StatusReady         = "ready"
	StatusPresented     = "presented"
	StatusLate          = "late"
	StatusNotApplicable = "not_applicable"

	DirectionIssued   = "issued"
	DirectionReceived = "received"
)

// Taxpayer is the person or company the application works for.
type Taxpayer struct {
	ID               string   `json:"id"`
	NIF              string   `json:"nif"`
	Name             string   `json:"name"`
	LegalForm        string   `json:"legal_form"`
	Email            string   `json:"email"`
	Phone            string   `json:"phone"`
	VATRegime        string   `json:"vat_regime"`
	IRPFMethod       string   `json:"irpf_method"`
	HasEmployees     bool     `json:"has_employees"`
	HasRentals       bool     `json:"has_rentals"`
	EconomicActivity bool     `json:"economic_activity"`
	CNAEs            []string `json:"cnaes"`
	FiscalYearEnd    string   `json:"fiscal_year_end"`
	IBAN             string   `json:"iban"`
	ActivityStart    string   `json:"activity_start"`

	// RETA profile. Amounts are monthly bases and annual net-income forecasts
	// expressed in cents. RetaBaseFrom is the first day the chosen base applies.
	RetaBaseCents      int64  `json:"reta_base_cents"`
	RetaBaseFrom       string `json:"reta_base_from"`
	RetaNetIncomeCents int64  `json:"reta_net_income_cents"`
	RetaNetIncomeYear  int    `json:"reta_net_income_year"`

	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// RetaBaseChange is an effective-dated snapshot of the RETA contribution
// profile. It lets the application explain which base applied at each moment.
type RetaBaseChange struct {
	ID             string `json:"id"`
	TaxpayerID     string `json:"taxpayer_id"`
	BaseCents      int64  `json:"base_cents"`
	BaseFrom       string `json:"base_from"`
	NetIncomeCents int64  `json:"net_income_cents"`
	NetIncomeYear  int    `json:"net_income_year"`
	CreatedAt      string `json:"created_at"`
}

// Filing is one concrete obligation instance for a taxpayer and period.
type Filing struct {
	ID                string         `json:"id"`
	TaxpayerID        string         `json:"taxpayer_id"`
	ModelCode         string         `json:"model_code"`
	ModelName         string         `json:"model_name"`
	Entity            string         `json:"entity"`
	PeriodLabel       string         `json:"period_label"`
	PeriodStart       string         `json:"period_start"`
	PeriodEnd         string         `json:"period_end"`
	DueDate           string         `json:"due_date"`
	Status            string         `json:"status"`
	Data              map[string]any `json:"data"`
	Computed          map[string]any `json:"computed"`
	Notes             string         `json:"notes"`
	PresentedAt       *string        `json:"presented_at"`
	ReceiptDocumentID *string        `json:"receipt_document_id"`
	CreatedAt         string         `json:"created_at"`
	UpdatedAt         string         `json:"updated_at"`
}

// Document is an uploaded or imported file. Legacy uploads are stored
// content-addressed; library imports live under the configured library root.
type Document struct {
	ID              string         `json:"id"`
	TaxpayerID      *string        `json:"taxpayer_id"`
	FilingID        *string        `json:"filing_id"`
	Kind            string         `json:"kind"`
	Filename        string         `json:"filename"`
	MIME            string         `json:"mime"`
	Size            int64          `json:"size"`
	SHA256          string         `json:"sha256"`
	StoragePath     string         `json:"-"`
	StorageRoot     string         `json:"-"`
	LibraryPath     string         `json:"library_path,omitempty"`
	SourcePath      string         `json:"source_path,omitempty"`
	DocDate         string         `json:"doc_date"`
	AmountCents     *int64         `json:"amount_cents"`
	CounterpartyNIF string         `json:"counterparty_nif"`
	TextContent     string         `json:"-"`
	TextExcerpt     string         `json:"text_excerpt,omitempty"`
	OCRStatus       string         `json:"ocr_status"`
	ReviewStatus    string         `json:"review_status"`
	Metadata        map[string]any `json:"metadata"`
	CreatedAt       string         `json:"created_at"`
}

// Invoice is an issued or received invoice. Amounts are stored in cents.
type Invoice struct {
	ID               string  `json:"id"`
	TaxpayerID       string  `json:"taxpayer_id"`
	Direction        string  `json:"direction"`
	Number           string  `json:"number"`
	InvoiceDate      string  `json:"invoice_date"`
	CounterpartyNIF  string  `json:"counterparty_nif"`
	CounterpartyName string  `json:"counterparty_name"`
	Concept          string  `json:"concept"`
	BaseCents        int64   `json:"base_cents"`
	VATRate          int     `json:"vat_rate"`
	VATCents         int64   `json:"vat_cents"`
	RetentionRate    int     `json:"retention_rate"`
	RetentionCents   int64   `json:"retention_cents"`
	TotalCents       int64   `json:"total_cents"`
	Category         string  `json:"category"`
	DocumentID       *string `json:"document_id"`
	CreatedAt        string  `json:"created_at"`
}

// PairingCode is a short-lived code shown by the ERP to pair a browser extension.
type PairingCode struct {
	Code      string  `json:"code"`
	CreatedAt string  `json:"created_at"`
	ExpiresAt string  `json:"expires_at"`
	UsedAt    *string `json:"used_at"`
}

// APIToken is a hashed long-lived token used by paired extensions.
type APIToken struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	CreatedAt  string  `json:"created_at"`
	LastUsedAt *string `json:"last_used_at"`
}

// Dashboard is the aggregate returned by the home endpoint.
type Dashboard struct {
	Taxpayers               int            `json:"taxpayers"`
	DueSoon                 int            `json:"due_soon"`
	Overdue                 int            `json:"overdue"`
	Presented               int            `json:"presented"`
	PresentedWithReceipt    int            `json:"presented_with_receipt"`
	PresentedWithoutReceipt int            `json:"presented_without_receipt"`
	Pending                 int            `json:"pending"`
	Documents               int            `json:"documents"`
	Invoices                int            `json:"invoices"`
	Upcoming                []Filing       `json:"upcoming"`
	ByEntity                map[string]int `json:"by_entity"`
}
