package documents

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

var (
	ErrUnsupportedFormat = errors.New("unsupported document format; only .pdf, .png, .jpg, .jpeg, .tiff, .tif are accepted")
	ErrEmptyDocument     = errors.New("document file is empty")
	ErrMissingFilePart   = errors.New("multipart field 'file' is required")
)

// IngestedDocument contains metadata for a document written to the consume directory.
type IngestedDocument struct {
	ID          string `json:"id"`
	Filename    string `json:"filename"`
	Format      string `json:"format"`
	SizeBytes   int64  `json:"size_bytes"`
	ConsumePath string `json:"consume_path"`
	IngestedAt  string `json:"ingested_at"`

	// Backward-compatibility aliases
	Target      string `json:"target,omitempty"`
	Path        string `json:"path,omitempty"`
	Size        int64  `json:"size,omitempty"`
	ContentType string `json:"content_type,omitempty"`
	Source      string `json:"source,omitempty"`
}

// IngestEngine coordinates document intake and validation.
type IngestEngine struct {
	consumeDir  string
	broadcaster func(event string, data any)
}

// NewIngestEngine initializes IngestEngine.
func NewIngestEngine(consumeDir string, broadcaster func(event string, data any)) (*IngestEngine, error) {
	if consumeDir == "" {
		consumeDir = "/data/documents/consume"
	}
	if err := os.MkdirAll(consumeDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create documents consume directory %q: %w", consumeDir, err)
	}
	return &IngestEngine{
		consumeDir:  consumeDir,
		broadcaster: broadcaster,
	}, nil
}

// SaveDocument validates the document signature and extension, and writes it to consumeDir/<uuid>_<filename> with 0644 permissions.
func (e *IngestEngine) SaveDocument(ctx context.Context, filename string, r io.Reader, source string) (*IngestedDocument, error) {
	cleanName := filepath.Base(filepath.Clean(filename))
	if cleanName == "." || cleanName == "/" || cleanName == "" {
		cleanName = "document.pdf"
	}

	if !IsSupportedDocumentExtension(cleanName) {
		return nil, fmt.Errorf("%w: invalid extension for %s", ErrUnsupportedFormat, cleanName)
	}

	// Read initial header bytes for magic validation
	headerBuf := make([]byte, 512)
	n, readErr := io.ReadFull(r, headerBuf)
	if readErr != nil && readErr != io.EOF && readErr != io.ErrUnexpectedEOF {
		return nil, fmt.Errorf("failed reading document header: %w", readErr)
	}
	if n == 0 {
		return nil, ErrEmptyDocument
	}
	headerBytes := headerBuf[:n]

	contentType, ok := DetectDocumentMagic(headerBytes)
	if !ok {
		return nil, fmt.Errorf("%w: invalid magic bytes for %s", ErrUnsupportedFormat, cleanName)
	}

	docID := generateDocUUID()
	targetName := fmt.Sprintf("%s_%s", docID, cleanName)
	targetPath := filepath.Join(e.consumeDir, targetName)

	stagingPath := filepath.Join(e.consumeDir, fmt.Sprintf(".uploading-%s-%s", docID, cleanName))
	f, err := os.OpenFile(stagingPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed creating staging file: %w", err)
	}

	cleanup := true
	defer func() {
		if cleanup {
			_ = f.Close()
			_ = os.Remove(stagingPath)
		}
	}()

	// Write header bytes first
	if _, err := f.Write(headerBytes); err != nil {
		return nil, fmt.Errorf("failed writing header to staging file: %w", err)
	}

	// Stream rest of reader
	written, copyErr := io.Copy(f, r)
	if copyErr != nil {
		return nil, fmt.Errorf("failed streaming document: %w", copyErr)
	}
	totalSize := int64(n) + written

	if syncErr := f.Sync(); syncErr != nil {
		return nil, fmt.Errorf("failed to sync document to disk: %w", syncErr)
	}
	if closeErr := f.Close(); closeErr != nil {
		return nil, fmt.Errorf("failed closing staging file: %w", closeErr)
	}

	// Atomically rename into final target
	if err := os.Rename(stagingPath, targetPath); err != nil {
		return nil, fmt.Errorf("failed promoting document to consume directory: %w", err)
	}
	cleanup = false

	_ = os.Chmod(targetPath, 0644)

	if source == "" {
		source = "upload"
	}

	nowStr := time.Now().UTC().Format(time.RFC3339)
	item := &IngestedDocument{
		ID:          docID,
		Filename:    cleanName,
		Format:      contentType,
		SizeBytes:   totalSize,
		ConsumePath: targetPath,
		IngestedAt:  nowStr,
		Target:      targetName,
		Path:        targetPath,
		Size:        totalSize,
		ContentType: contentType,
		Source:      source,
	}

	// Emit SSE event matching DMSDocumentIngestedEvent schema and backwards-compat fields
	if e.broadcaster != nil {
		e.broadcaster(EventDocumentIngested, map[string]any{
			"id":           item.ID,
			"filename":     item.Filename,
			"format":       item.Format,
			"size_bytes":   item.SizeBytes,
			"source":       item.Source,
			"ingested_at":  item.IngestedAt,
			"target":       item.Target,
			"path":         item.Path,
			"size":         item.Size,
			"content_type": item.ContentType,
		})
	}

	return item, nil
}

// IngestMultipart parses a multipart request and writes the uploaded document.
func (e *IngestEngine) IngestMultipart(ctx context.Context, mr *multipart.Reader, source string) (*IngestedDocument, error) {
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading multipart payload failed: %w", err)
		}

		formName := part.FormName()
		if formName == "file" || formName == "document" || part.FileName() != "" {
			filename := part.FileName()
			if filename == "" {
				filename = "document.pdf"
			}
			res, saveErr := e.SaveDocument(ctx, filename, part, source)
			_ = part.Close()
			return res, saveErr
		}
		_ = part.Close()
	}

	return nil, ErrMissingFilePart
}

// IngestHTTPHandler provides a standard http.HandlerFunc for POST /api/v1/documents/upload.
func (e *IngestEngine) IngestHTTPHandler(writeProblem func(w http.ResponseWriter, status int, title, detail, instance string), writeJSON func(w http.ResponseWriter, status int, v any)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		mr, err := r.MultipartReader()
		if err != nil {
			writeProblem(w, http.StatusBadRequest, "Invalid Request", "Request must be multipart/form-data: "+err.Error(), r.URL.Path)
			return
		}

		doc, err := e.IngestMultipart(r.Context(), mr, "upload")
		if err != nil {
			if errors.Is(err, ErrUnsupportedFormat) {
				writeProblem(w, http.StatusBadRequest, "Unsupported Format", err.Error(), r.URL.Path)
				return
			}
			if errors.Is(err, ErrEmptyDocument) {
				writeProblem(w, http.StatusBadRequest, "Empty Document", "Uploaded document payload is empty", r.URL.Path)
				return
			}
			if errors.Is(err, ErrMissingFilePart) {
				writeProblem(w, http.StatusBadRequest, "Missing File", "Multipart field 'file' is required", r.URL.Path)
				return
			}
			writeProblem(w, http.StatusBadRequest, "Upload Failed", err.Error(), r.URL.Path)
			return
		}

		writeJSON(w, http.StatusCreated, doc)
	}
}

// MakeSamplePDF creates a valid minimal PDF buffer for testing.
func MakeSamplePDF(text string) []byte {
	var buf bytes.Buffer
	buf.WriteString("%PDF-1.4\n")
	buf.WriteString("1 0 obj << /Type /Catalog /Pages 2 0 R >> endobj\n")
	buf.WriteString("2 0 obj << /Type /Pages /Kids [3 0 R] /Count 1 >> endobj\n")
	buf.WriteString(fmt.Sprintf("3 0 obj << /Type /Page /Parent 2 0 R /Contents (%s) >> endobj\n", text))
	buf.WriteString("xref\n0 4\n0000000000 65535 f \n0000000010 00000 n \n0000000060 00000 n \n0000000117 00000 n \n")
	buf.WriteString("trailer << /Size 4 /Root 1 0 R >>\nstartxref\n200\n%%EOF\n")
	return buf.Bytes()
}
