package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

func TestRunSavesPlainTextMail(t *testing.T) {
	baseDir := t.TempDir()
	now := time.Date(2026, 6, 9, 12, 34, 56, 789000000, time.FixedZone("JST", 9*60*60))
	message := strings.Join([]string{
		"From: Sender <sender@example.test>",
		"To: Receiver <receiver@example.test>",
		"Subject: Test mail",
		"Content-Type: text/plain; charset=UTF-8",
		"",
		"hello",
	}, "\r\n")

	if err := run(strings.NewReader(message), baseDir, now); err != nil {
		t.Fatalf("run() error = %v", err)
	}

	dir := singleMailDir(t, baseDir)
	assertFileContent(t, filepath.Join(dir, "raw.eml"), message)
	assertFileContent(t, filepath.Join(dir, "body.txt"), "hello")

	data, err := os.ReadFile(filepath.Join(dir, "meta.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var meta Meta
	if err := yaml.Unmarshal(data, &meta); err != nil {
		t.Fatal(err)
	}
	if meta.Subject != "Test mail" {
		t.Fatalf("subject = %q, want %q", meta.Subject, "Test mail")
	}
	if meta.DateReceived != now.Format(time.RFC3339) {
		t.Fatalf("date_received = %q, want %q", meta.DateReceived, now.Format(time.RFC3339))
	}
}

func TestRunSavesMultipartBodiesAndAttachment(t *testing.T) {
	baseDir := t.TempDir()
	message := strings.Join([]string{
		"From: sender@example.test",
		"To: receiver@example.test",
		"Subject: Multipart",
		"Content-Type: multipart/mixed; boundary=outer",
		"",
		"--outer",
		"Content-Type: multipart/alternative; boundary=inner",
		"",
		"--inner",
		"Content-Type: text/plain; charset=UTF-8",
		"",
		"plain body",
		"--inner",
		"Content-Type: text/html; charset=UTF-8",
		"",
		"<p>html body</p>",
		"--inner--",
		"--outer",
		"Content-Type: application/octet-stream",
		"Content-Disposition: attachment; filename=\"sample.txt\"",
		"Content-Transfer-Encoding: base64",
		"",
		"YXR0YWNobWVudA==",
		"--outer--",
	}, "\r\n")

	if err := run(strings.NewReader(message), baseDir, time.Now()); err != nil {
		t.Fatalf("run() error = %v", err)
	}

	dir := singleMailDir(t, baseDir)
	assertFileContent(t, filepath.Join(dir, "body.txt"), "plain body")
	assertFileContent(t, filepath.Join(dir, "body.html"), "<p>html body</p>")
	assertFileContent(t, filepath.Join(dir, "attachments", "sample.txt"), "attachment")
}

func TestRunReturnsWriteError(t *testing.T) {
	baseDir := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(baseDir, []byte("file"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := run(strings.NewReader("Subject: fail\r\n\r\nbody"), baseDir, time.Now())
	if err == nil {
		t.Fatal("run() error = nil, want directory creation error")
	}
}

func TestSanitizeTruncatesAtUTF8Boundary(t *testing.T) {
	got := sanitize(strings.Repeat("あ", 100))
	if !utf8.ValidString(got) {
		t.Fatalf("sanitize() returned invalid UTF-8: %q", got)
	}
	if len(got) > 120 {
		t.Fatalf("sanitize() byte length = %d, want <= 120", len(got))
	}
}

func singleMailDir(t *testing.T, baseDir string) string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(baseDir, "*", "*", "*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 {
		t.Fatalf("mail directories = %v, want exactly one", matches)
	}
	return matches[0]
}

func assertFileContent(t *testing.T, path, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(data)); got != strings.TrimSpace(want) {
		t.Fatalf("%s = %q, want %q", path, got, strings.TrimSpace(want))
	}
}
