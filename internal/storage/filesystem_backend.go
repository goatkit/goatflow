package storage

import (
	"bufio"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/goatkit/goatflow/internal/platform/database"
)

// OTRS ArticleStorageFS file names.
const (
	plainFile          = "plain.txt"
	suffixContentType  = ".content_type"
	suffixContentID    = ".content_id"
	suffixContentAlt   = ".content_alternative"
	suffixDisposition  = ".disposition"
	maxFilenameBytes   = 220
	tempFilePrefix     = ".goatflow-tmp-"
	dirPerm            = 0o750
	filePerm           = 0o640
	fallbackFilename   = "file"
	reservedNameSuffix = "_"
)

var sidecarSuffixes = []string{suffixContentType, suffixContentID, suffixContentAlt, suffixDisposition}

// FilesystemStore keeps attachments and raw emails in an OTRS ArticleStorageFS
// tree: <dir>/<content_path>/<article_id>/. The content path comes from
// article_data_mime.content_path, or from the article's create_time when unset.
type FilesystemStore struct {
	dir string
	db  *sql.DB
	tx  *sql.Tx
}

// NewFilesystemStore returns an FS-backed article store rooted at dir (the
// OTRS ArticleDataDir, e.g. /opt/otrs/var/article).
func NewFilesystemStore(dir string, db *sql.DB) *FilesystemStore {
	return &FilesystemStore{dir: dir, db: db}
}

// Backend returns BackendFS.
func (s *FilesystemStore) Backend() string { return BackendFS }

// Dir returns the root directory of the tree.
func (s *FilesystemStore) Dir() string { return s.dir }

// WithTx returns a store whose article lookups run on tx (file writes are not
// transactional).
func (s *FilesystemStore) WithTx(tx *sql.Tx) ArticleStore {
	return &FilesystemStore{dir: s.dir, db: s.db, tx: tx}
}

func (s *FilesystemStore) q() querier {
	if s.tx != nil {
		return s.tx
	}
	return s.db
}

// ArticleDir returns the article's directory, or ErrNotFound when the article does not exist.
func (s *FilesystemStore) ArticleDir(ctx context.Context, articleID int64) (string, error) {
	var contentPath sql.NullString
	err := s.q().QueryRowContext(ctx, database.ConvertPlaceholders(`
		SELECT content_path FROM article_data_mime
		WHERE article_id = ? AND content_path IS NOT NULL AND content_path <> ''
		ORDER BY id LIMIT 1`), articleID).Scan(&contentPath)
	switch {
	case err == nil:
	case errors.Is(err, sql.ErrNoRows):
		var created time.Time
		err = s.q().QueryRowContext(ctx, database.ConvertPlaceholders(
			"SELECT create_time FROM article WHERE id = ?"), articleID).Scan(&created)
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrNotFound
		}
		if err != nil {
			return "", fmt.Errorf("look up article %d: %w", articleID, err)
		}
		contentPath = sql.NullString{String: ContentPath(created), Valid: true}
	default:
		return "", fmt.Errorf("look up content path of article %d: %w", articleID, err)
	}

	rel := filepath.Clean(filepath.FromSlash(contentPath.String))
	if filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("article %d has an invalid content_path %q", articleID, contentPath.String)
	}
	return filepath.Join(s.dir, rel, fmt.Sprintf("%d", articleID)), nil
}

// fsEntry is one attachment file in an article directory.
type fsEntry struct {
	Attachment
	path     string
	oldStyle bool // content type on the first line of the file (pre-sidecar OTRS)
}

// index lists the attachment files of an article directory the way OTRS
// ArticleAttachmentIndexRaw does: sorted names, skipping dot files, sidecars
// and plain.txt.
func index(dir string, articleID int64) ([]fsEntry, error) {
	files, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read article directory: %w", err)
	}

	entries := make([]fsEntry, 0, len(files))
	for _, f := range files {
		name := f.Name()
		if f.IsDir() || strings.HasPrefix(name, ".") || name == plainFile || isSidecar(name) {
			continue
		}
		info, err := f.Info()
		if err != nil {
			return nil, fmt.Errorf("stat %s: %w", name, err)
		}
		e := fsEntry{
			Attachment: Attachment{
				ArticleID:  articleID,
				FileID:     int64(len(entries) + 1),
				Filename:   name,
				Size:       info.Size(),
				CreateTime: info.ModTime(),
			},
			path: filepath.Join(dir, name),
		}
		ct, ok, err := readSidecar(e.path + suffixContentType)
		if err != nil {
			return nil, err
		}
		if ok {
			e.ContentType = ct
			if e.ContentID, _, err = readSidecar(e.path + suffixContentID); err != nil {
				return nil, err
			}
			if e.ContentAlternative, _, err = readSidecar(e.path + suffixContentAlt); err != nil {
				return nil, err
			}
			if e.Disposition, _, err = readSidecar(e.path + suffixDisposition); err != nil {
				return nil, err
			}
		} else {
			line, err := readFirstLine(e.path)
			if err != nil {
				return nil, err
			}
			e.oldStyle = true
			e.ContentType = strings.TrimRight(line, "\r\n")
			e.Size -= int64(len(line))
		}
		if e.Disposition == "" {
			e.Disposition = defaultDisposition(e.Filename, e.ContentType, e.ContentID)
		}
		entries = append(entries, e)
	}
	return entries, nil
}

func isSidecar(name string) bool {
	for _, suffix := range sidecarSuffixes {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return false
}

func readSidecar(path string) (string, bool, error) {
	b, err := os.ReadFile(path) //nolint:gosec // path built from the article directory and an index entry
	if errors.Is(err, fs.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("read %s: %w", filepath.Base(path), err)
	}
	return strings.TrimRight(string(b), "\r\n"), true, nil
}

func readFirstLine(path string) (string, error) {
	f, err := os.Open(path) //nolint:gosec // path built from the article directory and an index entry
	if err != nil {
		return "", fmt.Errorf("open %s: %w", filepath.Base(path), err)
	}
	defer f.Close()
	line, err := bufio.NewReader(f).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("read %s: %w", filepath.Base(path), err)
	}
	return line, nil
}

// ListAttachments returns the article's attachments in OTRS index order. An
// unknown article has no attachments.
func (s *FilesystemStore) ListAttachments(ctx context.Context, articleID int64) ([]Attachment, error) {
	dir, err := s.ArticleDir(ctx, articleID)
	if errors.Is(err, ErrNotFound) {
		return []Attachment{}, nil
	}
	if err != nil {
		return nil, err
	}
	entries, err := index(dir, articleID)
	if err != nil {
		return nil, fmt.Errorf("list attachments of article %d: %w", articleID, err)
	}
	list := make([]Attachment, len(entries))
	for i, e := range entries {
		list[i] = e.Attachment
	}
	return list, nil
}

func (s *FilesystemStore) entry(ctx context.Context, articleID, fileID int64) (fsEntry, error) {
	dir, err := s.ArticleDir(ctx, articleID)
	if err != nil {
		return fsEntry{}, err
	}
	entries, err := index(dir, articleID)
	if err != nil {
		return fsEntry{}, fmt.Errorf("attachment %d of article %d: %w", fileID, articleID, err)
	}
	if fileID < 1 || fileID > int64(len(entries)) {
		return fsEntry{}, ErrNotFound
	}
	return entries[fileID-1], nil
}

// GetAttachment returns attachment number fileID of the article and its content.
func (s *FilesystemStore) GetAttachment(ctx context.Context, articleID, fileID int64) (Attachment, []byte, error) {
	e, err := s.entry(ctx, articleID, fileID)
	if err != nil {
		return Attachment{}, nil, err
	}
	content, err := os.ReadFile(e.path)
	if err != nil {
		return Attachment{}, nil, fmt.Errorf("read attachment %d of article %d: %w", fileID, articleID, err)
	}
	if e.oldStyle {
		if i := strings.IndexByte(string(content), '\n'); i >= 0 {
			content = content[i+1:]
		} else {
			content = []byte{}
		}
	}
	return e.Attachment, content, nil
}

// WriteAttachment writes the attachment and its sidecar files.
func (s *FilesystemStore) WriteAttachment(ctx context.Context, articleID int64, in NewAttachment) (Attachment, error) {
	in = normalizeAttachment(in)
	dir, err := s.ArticleDir(ctx, articleID)
	if err != nil {
		return Attachment{}, fmt.Errorf("write attachment of article %d: %w", articleID, err)
	}
	entries, err := index(dir, articleID)
	if err != nil {
		return Attachment{}, fmt.Errorf("write attachment of article %d: %w", articleID, err)
	}
	used := make(map[string]bool, len(entries))
	for _, e := range entries {
		used[e.Filename] = true
	}
	name := uniqueFilename(cleanFilename(in.Filename), used)

	if err := os.MkdirAll(dir, dirPerm); err != nil {
		return Attachment{}, fmt.Errorf("create article directory: %w", err)
	}
	path := filepath.Join(dir, name)
	sidecars := []struct{ suffix, value string }{
		{suffixContentType, in.ContentType},
		{suffixContentID, in.ContentID},
		{suffixContentAlt, in.ContentAlternative},
		{suffixDisposition, in.Disposition},
	}
	for _, sc := range sidecars {
		if sc.value == "" {
			continue
		}
		if err := writeFileAtomic(path+sc.suffix, []byte(sc.value)); err != nil {
			return Attachment{}, err
		}
	}
	if err := writeFileAtomic(path, in.Content); err != nil {
		return Attachment{}, err
	}

	entries, err = index(dir, articleID)
	if err != nil {
		return Attachment{}, err
	}
	for _, e := range entries {
		if e.Filename == name {
			return e.Attachment, nil
		}
	}
	return Attachment{}, fmt.Errorf("attachment %s of article %d missing after write", name, articleID)
}

// DeleteAttachment removes attachment number fileID and its sidecars. Later
// attachments of the article move down one index.
func (s *FilesystemStore) DeleteAttachment(ctx context.Context, articleID, fileID int64) error {
	e, err := s.entry(ctx, articleID, fileID)
	if err != nil {
		return err
	}
	for _, p := range append([]string{e.path}, sidecarPaths(e.path)...) {
		if err := os.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("delete attachment %d of article %d: %w", fileID, articleID, err)
		}
	}
	return nil
}

func sidecarPaths(path string) []string {
	paths := make([]string, len(sidecarSuffixes))
	for i, suffix := range sidecarSuffixes {
		paths[i] = path + suffix
	}
	return paths
}

// WritePlain writes plain.txt.
func (s *FilesystemStore) WritePlain(ctx context.Context, articleID int64, raw []byte, _ int) error {
	dir, err := s.ArticleDir(ctx, articleID)
	if err != nil {
		return fmt.Errorf("write plain email of article %d: %w", articleID, err)
	}
	if err := os.MkdirAll(dir, dirPerm); err != nil {
		return fmt.Errorf("create article directory: %w", err)
	}
	return writeFileAtomic(filepath.Join(dir, plainFile), raw)
}

// ReadPlain reads plain.txt.
func (s *FilesystemStore) ReadPlain(ctx context.Context, articleID int64) ([]byte, error) {
	dir, err := s.ArticleDir(ctx, articleID)
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(filepath.Join(dir, plainFile)) //nolint:gosec // path built from the article directory
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("read plain email of article %d: %w", articleID, err)
	}
	return raw, nil
}

// DeleteArticle removes the article directory.
func (s *FilesystemStore) DeleteArticle(ctx context.Context, articleID int64) error {
	dir, err := s.ArticleDir(ctx, articleID)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("delete content of article %d: %w", articleID, err)
	}
	return nil
}

// Stats counts the content files (attachments and plain.txt, not sidecars)
// in the tree and their bytes.
func (s *FilesystemStore) Stats() (files, size int64, err error) {
	err = filepath.WalkDir(s.dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) && path == s.dir {
				return fs.SkipAll
			}
			return err
		}
		name := d.Name()
		if d.IsDir() || strings.HasPrefix(name, ".") || isSidecar(name) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		files++
		size += info.Size()
		return nil
	})
	if err != nil {
		return 0, 0, fmt.Errorf("scan %s: %w", s.dir, err)
	}
	return files, size, nil
}

// writeFileAtomic writes data to a temp file in the target directory and renames it into place.
func writeFileAtomic(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), tempFilePrefix+"*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()
	_, werr := tmp.Write(data)
	cerr := tmp.Close()
	if werr == nil {
		werr = cerr
	}
	if werr == nil {
		werr = os.Chmod(tmpName, filePerm)
	}
	if werr == nil {
		werr = os.Rename(tmpName, path)
	}
	if werr != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("write %s: %w", filepath.Base(path), werr)
	}
	return nil
}

// cleanFilename makes an attachment filename safe as a single file name in an
// article directory: no path separators or control characters, no leading dot,
// no clash with plain.txt or sidecar names, at most maxFilenameBytes bytes.
func cleanFilename(name string) string {
	var b strings.Builder
	for _, r := range name {
		if r == '/' || r == '\\' || r < 0x20 || r == 0x7f || r == utf8.RuneError {
			b.WriteByte('_')
			continue
		}
		b.WriteRune(r)
	}
	name = strings.TrimSpace(b.String())
	if strings.HasPrefix(name, ".") {
		name = "_" + strings.TrimLeft(name, ".")
	}
	if name == "" || name == "_" {
		name = fallbackFilename
	}
	if len(name) > maxFilenameBytes {
		ext := filepath.Ext(name)
		if len(ext) > 20 {
			ext = ""
		}
		base := name[:len(name)-len(ext)]
		limit := maxFilenameBytes - len(ext)
		for limit > 0 && !utf8.RuneStart(base[limit]) {
			limit--
		}
		name = base[:limit] + ext
	}
	if name == plainFile || isSidecar(name) {
		name += reservedNameSuffix
	}
	return name
}
