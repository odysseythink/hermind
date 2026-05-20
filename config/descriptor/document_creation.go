package descriptor

func init() {
	Register(Section{
		Key:     "document_creation",
		Label:   "Document Creation",
		Summary: "Configure document creation tools for generating text files, Word documents, PowerPoint presentations, PDFs, and Excel spreadsheets.",
		GroupID: "skills",
		Shape:   ShapeMap,
		Fields: []FieldSpec{
			{Name: "enabled", Label: "Enabled", Kind: FieldBool, Help: "Enable document creation tools.", Default: true},
			{Name: "create_text_file", Label: "Text files", Kind: FieldBool, Help: "Create text files (.txt, .md, .json, .csv, etc.)", Default: true},
			{Name: "create_word_document", Label: "Word documents", Kind: FieldBool, Help: "Create Microsoft Word documents (.docx)", Default: true},
			{Name: "create_pptx_presentation", Label: "PowerPoint", Kind: FieldBool, Help: "Create PowerPoint presentations (.pptx)", Default: true},
			{Name: "create_pdf_document", Label: "PDF documents", Kind: FieldBool, Help: "Create PDF documents", Default: true},
			{Name: "create_excel_spreadsheet", Label: "Excel spreadsheets", Kind: FieldBool, Help: "Create Excel spreadsheets (.xlsx)", Default: true},
		},
	})
}
