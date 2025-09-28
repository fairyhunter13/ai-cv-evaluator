package httpserver

import (
	"context"
	"mime/multipart"
	"testing"
)

func Test_extractUploadedText_Txt_Sanitized(t *testing.T) {
	h := &multipart.FileHeader{Filename: "note.txt"}
	got, err := extractUploadedText(context.Background(), nil, h, []byte("hello\tworld\n"))
	if err != nil { t.Fatalf("unexpected err: %v", err) }
	if got == "" { t.Fatalf("want non-empty text") }
}

func Test_extractUploadedText_PDF_RequiresExtractor(t *testing.T) {
	h := &multipart.FileHeader{Filename: "doc.pdf"}
	_, err := extractUploadedText(context.Background(), nil, h, []byte("%PDF-1.7"))
	if err == nil { t.Fatalf("expected error for pdf without extractor") }
}

func Test_extractUploadedText_DOCX_RequiresExtractor(t *testing.T) {
	h := &multipart.FileHeader{Filename: "doc.docx"}
	_, err := extractUploadedText(context.Background(), nil, h, []byte("PK\x03\x04"))
	if err == nil { t.Fatalf("expected error for docx without extractor") }
}
