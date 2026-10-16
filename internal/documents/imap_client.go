package documents

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ErrNotConnected        = errors.New("imap client not connected")
	ErrAuthFailed          = errors.New("imap authentication failed")
	ErrCommandFailed       = errors.New("imap command failed")
	ErrUnexpectedResponse  = errors.New("unexpected imap response")
)

// IMAPClient interface defines the RFC 3501 operations required for document ingestion.
type IMAPClient interface {
	Connect(ctx context.Context, addr string, useTLS bool, tlsConfig *tls.Config) error
	Login(ctx context.Context, username, password string) error
	Select(ctx context.Context, folder string) (uint32, error)
	SearchUnseen(ctx context.Context) ([]uint32, error)
	FetchMessage(ctx context.Context, seqNo uint32) ([]byte, error)
	MarkSeen(ctx context.Context, seqNo uint32) error
	Close() error
}

// Client implements IMAPClient for RFC 3501 communication.
type Client struct {
	conn       net.Conn
	reader     *bufio.Reader
	tagCounter uint64
	mu         sync.Mutex
}

// NewClient creates an unconnected IMAPClient.
func NewClient() *Client {
	return &Client{}
}

func (c *Client) nextTag() string {
	n := atomic.AddUint64(&c.tagCounter, 1)
	return fmt.Sprintf("A%04d", n)
}

// Connect establishes a TCP or TLS connection and reads the server greeting.
func (c *Client) Connect(ctx context.Context, addr string, useTLS bool, tlsConfig *tls.Config) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		_ = c.conn.Close()
	}

	dialer := &net.Dialer{
		Timeout: 15 * time.Second,
	}

	var conn net.Conn
	var err error

	if useTLS {
		if tlsConfig == nil {
			host, _, _ := net.SplitHostPort(addr)
			tlsConfig = &tls.Config{
				ServerName: host,
			}
		}
		conn, err = tls.DialWithDialer(dialer, "tcp", addr, tlsConfig)
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", addr)
	}

	if err != nil {
		return fmt.Errorf("connection to %s failed: %w", addr, err)
	}

	c.conn = conn
	c.reader = bufio.NewReader(conn)

	// Read initial greeting: e.g. "* OK IMAP4rev1 Service Ready"
	greeting, err := c.readLine(ctx)
	if err != nil {
		_ = c.conn.Close()
		c.conn = nil
		return fmt.Errorf("failed to read imap greeting: %w", err)
	}

	if strings.HasPrefix(greeting, "* BYE") {
		_ = c.conn.Close()
		c.conn = nil
		return fmt.Errorf("server rejected connection with BYE: %s", greeting)
	}

	return nil
}

func (c *Client) readLine(ctx context.Context) (string, error) {
	type readResult struct {
		line string
		err  error
	}

	resCh := make(chan readResult, 1)
	go func() {
		line, err := c.reader.ReadString('\n')
		resCh <- readResult{line: strings.TrimRight(line, "\r\n"), err: err}
	}()

	select {
	case <-ctx.Done():
		if c.conn != nil {
			_ = c.conn.SetDeadline(time.Now())
		}
		return "", ctx.Err()
	case res := <-resCh:
		if res.err != nil {
			return "", res.err
		}
		return res.line, nil
	}
}

// quoteString properly formats an IMAP quoted string, sanitizing any CRLF injection attempts.
func quoteString(s string) string {
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	return "\"" + s + "\""
}

// Login authenticates with username and password. Plaintext passwords are not logged.
func (c *Client) Login(ctx context.Context, username, password string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return ErrNotConnected
	}

	tag := c.nextTag()
	cmd := fmt.Sprintf("%s LOGIN %s %s\r\n", tag, quoteString(username), quoteString(password))
	if _, err := io.WriteString(c.conn, cmd); err != nil {
		return fmt.Errorf("failed to send LOGIN command: %w", err)
	}

	for {
		line, err := c.readLine(ctx)
		if err != nil {
			return fmt.Errorf("error reading LOGIN response: %w", err)
		}

		if strings.HasPrefix(line, tag+" ") {
			rest := strings.TrimPrefix(line, tag+" ")
			if strings.HasPrefix(rest, "OK") {
				return nil
			}
			if strings.HasPrefix(rest, "NO") {
				return fmt.Errorf("%w: %s", ErrAuthFailed, rest)
			}
			return fmt.Errorf("%w: %s", ErrCommandFailed, rest)
		}
	}
}

// Select chooses a mailbox folder (e.g. INBOX) and returns the number of messages.
func (c *Client) Select(ctx context.Context, folder string) (uint32, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return 0, ErrNotConnected
	}
	if strings.TrimSpace(folder) == "" {
		folder = "INBOX"
	}

	tag := c.nextTag()
	cmd := fmt.Sprintf("%s SELECT %s\r\n", tag, quoteString(folder))
	if _, err := io.WriteString(c.conn, cmd); err != nil {
		return 0, fmt.Errorf("failed to send SELECT command: %w", err)
	}

	var count uint32
	for {
		line, err := c.readLine(ctx)
		if err != nil {
			return 0, fmt.Errorf("error reading SELECT response: %w", err)
		}

		// e.g. "* 14 EXISTS"
		if strings.HasPrefix(line, "* ") && strings.HasSuffix(line, " EXISTS") {
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				if n, parseErr := strconv.ParseUint(parts[1], 10, 32); parseErr == nil {
					count = uint32(n)
				}
			}
		}

		if strings.HasPrefix(line, tag+" ") {
			rest := strings.TrimPrefix(line, tag+" ")
			if strings.HasPrefix(rest, "OK") {
				return count, nil
			}
			return 0, fmt.Errorf("%w: SELECT failed: %s", ErrCommandFailed, rest)
		}
	}
}

// SearchUnseen finds unread messages using "SEARCH UNSEEN".
func (c *Client) SearchUnseen(ctx context.Context) ([]uint32, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return nil, ErrNotConnected
	}

	tag := c.nextTag()
	cmd := fmt.Sprintf("%s SEARCH UNSEEN\r\n", tag)
	if _, err := io.WriteString(c.conn, cmd); err != nil {
		return nil, fmt.Errorf("failed to send SEARCH command: %w", err)
	}

	var seqNos []uint32
	for {
		line, err := c.readLine(ctx)
		if err != nil {
			return nil, fmt.Errorf("error reading SEARCH response: %w", err)
		}

		// E.g. "* SEARCH 1 2 5"
		if strings.HasPrefix(line, "* SEARCH") {
			fields := strings.Fields(strings.TrimPrefix(line, "* SEARCH"))
			for _, f := range fields {
				if n, parseErr := strconv.ParseUint(f, 10, 32); parseErr == nil {
					seqNos = append(seqNos, uint32(n))
				}
			}
		}

		if strings.HasPrefix(line, tag+" ") {
			rest := strings.TrimPrefix(line, tag+" ")
			if strings.HasPrefix(rest, "OK") {
				return seqNos, nil
			}
			return nil, fmt.Errorf("%w: SEARCH failed: %s", ErrCommandFailed, rest)
		}
	}
}

// FetchMessage retrieves the raw RFC 822 email message by sequence number.
func (c *Client) FetchMessage(ctx context.Context, seqNo uint32) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return nil, ErrNotConnected
	}

	tag := c.nextTag()
	cmd := fmt.Sprintf("%s FETCH %d (BODY.PEEK[])\r\n", tag, seqNo)
	if _, err := io.WriteString(c.conn, cmd); err != nil {
		return nil, fmt.Errorf("failed to send FETCH command: %w", err)
	}

	var msgContent []byte
	var literalFound bool

	for {
		line, err := c.readLine(ctx)
		if err != nil {
			return nil, fmt.Errorf("error reading FETCH response line: %w", err)
		}

		// Check for literal indicator e.g. "* 1 FETCH (BODY[] {1234}"
		if !literalFound && strings.Contains(line, "{") && strings.HasSuffix(line, "}") {
			start := strings.LastIndex(line, "{")
			end := strings.LastIndex(line, "}")
			if start >= 0 && end > start {
				sizeStr := line[start+1 : end]
				if size, err := strconv.Atoi(sizeStr); err == nil && size >= 0 {
					buf := make([]byte, size)
					if _, err := io.ReadFull(c.reader, buf); err != nil {
						return nil, fmt.Errorf("failed to read literal bytes: %w", err)
					}
					msgContent = buf
					literalFound = true
					continue
				}
			}
		}

		if strings.HasPrefix(line, tag+" ") {
			rest := strings.TrimPrefix(line, tag+" ")
			if strings.HasPrefix(rest, "OK") {
				return msgContent, nil
			}
			return nil, fmt.Errorf("%w: FETCH failed: %s", ErrCommandFailed, rest)
		}
	}
}

// MarkSeen sets the \Seen flag on the specified message sequence number.
func (c *Client) MarkSeen(ctx context.Context, seqNo uint32) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return ErrNotConnected
	}

	tag := c.nextTag()
	cmd := fmt.Sprintf("%s STORE %d +FLAGS (\\Seen)\r\n", tag, seqNo)
	if _, err := io.WriteString(c.conn, cmd); err != nil {
		return fmt.Errorf("failed to send STORE command: %w", err)
	}

	for {
		line, err := c.readLine(ctx)
		if err != nil {
			return fmt.Errorf("error reading STORE response: %w", err)
		}

		if strings.HasPrefix(line, tag+" ") {
			rest := strings.TrimPrefix(line, tag+" ")
			if strings.HasPrefix(rest, "OK") {
				return nil
			}
			return fmt.Errorf("%w: STORE failed: %s", ErrCommandFailed, rest)
		}
	}
}

// Close gracefully issues LOGOUT and closes the connection.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return nil
	}

	tag := c.nextTag()
	_, _ = io.WriteString(c.conn, tag+" LOGOUT\r\n")

	err := c.conn.Close()
	c.conn = nil
	c.reader = nil
	return err
}

// ExtractedDocument represents a document attachment extracted from an email.
type ExtractedDocument struct {
	Filename    string
	ContentType string
	Data        []byte
}

// SupportedDocumentExtensions defines valid file extensions for OCR and indexing.
var SupportedDocumentExtensions = []string{
	".pdf",
	".png",
	".jpg",
	".jpeg",
	".tiff",
	".tif",
}

// IsSupportedDocumentExtension checks whether a filename has an accepted document extension.
func IsSupportedDocumentExtension(filename string) bool {
	ext := strings.ToLower(filepath.Ext(strings.TrimSpace(filename)))
	for _, supported := range SupportedDocumentExtensions {
		if ext == supported {
			return true
		}
	}
	return false
}

// DetectDocumentMagic checks the header bytes of a document and returns its validated MIME type.
func DetectDocumentMagic(data []byte) (string, bool) {
	if len(data) < 4 {
		return "", false
	}

	// 1. PDF: starts with "%PDF"
	if bytes.HasPrefix(data, []byte("%PDF")) {
		return "application/pdf", true
	}

	// 2. PNG: 0x89 0x50 0x4E 0x47 0x0D 0x0A 0x1A 0x0A
	if len(data) >= 8 && bytes.Equal(data[:8], []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}) {
		return "image/png", true
	}

	// 3. JPEG: 0xFF 0xD8 0xFF
	if len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
		return "image/jpeg", true
	}

	// 4. TIFF: Little-endian (II*\0) or Big-endian (MM\0*)
	if len(data) >= 4 {
		if (data[0] == 0x49 && data[1] == 0x49 && data[2] == 0x2A && data[3] == 0x00) ||
			(data[0] == 0x4D && data[1] == 0x4D && data[2] == 0x00 && data[3] == 0x2A) {
			return "image/tiff", true
		}
	}

	return "", false
}

// ExtractAttachments inspects raw RFC 822 email message bytes and extracts valid PDF/image documents.
func ExtractAttachments(rawMsg []byte) ([]ExtractedDocument, error) {
	if len(rawMsg) == 0 {
		return nil, nil
	}

	msg, err := mail.ReadMessage(bytes.NewReader(rawMsg))
	if err != nil {
		return nil, fmt.Errorf("failed to parse email message headers: %w", err)
	}

	var results []ExtractedDocument
	contentTypeHeader := msg.Header.Get("Content-Type")
	if contentTypeHeader == "" {
		contentTypeHeader = "text/plain"
	}

	mediaType, params, err := mime.ParseMediaType(contentTypeHeader)
	if err != nil {
		mediaType = "text/plain"
	}

	if strings.HasPrefix(mediaType, "multipart/") {
		boundary := params["boundary"]
		if boundary != "" {
			err = parseMultipartParts(msg.Body, boundary, &results)
			if err != nil {
				return nil, err
			}
		}
	} else {
		// Single part message: check if it is directly a PDF or image
		bodyBytes, err := io.ReadAll(msg.Body)
		if err == nil && len(bodyBytes) > 0 {
			enc := msg.Header.Get("Content-Transfer-Encoding")
			decoded, err := decodeTransferEncoding(bodyBytes, enc)
			if err == nil && len(decoded) > 0 {
				detectedType, ok := DetectDocumentMagic(decoded)
				if ok {
					ext := ".pdf"
					switch detectedType {
					case "image/jpeg":
						ext = ".jpg"
					case "image/png":
						ext = ".png"
					case "image/tiff":
						ext = ".tiff"
					}
					results = append(results, ExtractedDocument{
						Filename:    "document" + ext,
						ContentType: detectedType,
						Data:        decoded,
					})
				}
			}
		}
	}

	return results, nil
}

func parseMultipartParts(r io.Reader, boundary string, results *[]ExtractedDocument) error {
	mr := multipart.NewReader(r, boundary)
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil // End or malformed part, recover cleanly
		}

		partHeader := part.Header
		partContentType := partHeader.Get("Content-Type")
		subMediaType, subParams, _ := mime.ParseMediaType(partContentType)

		if strings.HasPrefix(subMediaType, "multipart/") {
			subBoundary := subParams["boundary"]
			if subBoundary != "" {
				_ = parseMultipartParts(part, subBoundary, results)
			}
			_ = part.Close()
			continue
		}

		filename := part.FileName()
		if filename == "" {
			disp := partHeader.Get("Content-Disposition")
			_, dispParams, _ := mime.ParseMediaType(disp)
			filename = dispParams["filename"]
		}
		if filename == "" {
			filename = subParams["name"]
		}

		// Decode RFC 2047 encoded filenames (e.g. =?UTF-8?B?...?= or =?UTF-8?Q?...?=)
		if strings.HasPrefix(filename, "=?") {
			dec := new(mime.WordDecoder)
			if decodedName, decErr := dec.DecodeHeader(filename); decErr == nil {
				filename = decodedName
			}
		}

		partBytes, readErr := io.ReadAll(part)
		_ = part.Close()
		if readErr != nil || len(partBytes) == 0 {
			continue
		}

		// Decode transfer encoding
		enc := partHeader.Get("Content-Transfer-Encoding")
		decoded, decErr := decodeTransferEncoding(partBytes, enc)
		if decErr != nil {
			decoded = partBytes
		}

		// Validate magic bytes
		detectedType, ok := DetectDocumentMagic(decoded)
		if !ok {
			continue
		}

		// If filename is supported or detected as PDF/image, retain
		if filename == "" {
			if detectedType == "application/pdf" {
				filename = "attachment.pdf"
			} else {
				filename = "attachment.jpg"
			}
		} else if !IsSupportedDocumentExtension(filename) {
			continue
		}

		*results = append(*results, ExtractedDocument{
			Filename:    filename,
			ContentType: detectedType,
			Data:        decoded,
		})
	}

	return nil
}

func decodeTransferEncoding(data []byte, encoding string) ([]byte, error) {
	enc := strings.ToLower(strings.TrimSpace(encoding))
	switch enc {
	case "base64":
		// Clean any trailing or internal whitespace/newlines
		clean := strings.Map(func(r rune) rune {
			if r == '\r' || r == '\n' || r == ' ' || r == '\t' {
				return -1
			}
			return r
		}, string(data))

		return base64.StdEncoding.DecodeString(clean)
	case "quoted-printable":
		r := quotedprintable.NewReader(bytes.NewReader(data))
		return io.ReadAll(r)
	default:
		return data, nil
	}
}
