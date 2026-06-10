// maildump は sendmail 互換のメールダンプツール。
//
// PHP の sendmail_path から呼ばれ、標準入力で受け取ったRFC822メールを
// パースして、添付ファイルやメタ情報をファイルとして保存する。
// SMTPサーバーを経由しないため高速で、ポート公開もプロキシも不要。
//
// 保存先（既定 /var/log/maildump、環境変数 MAILDUMP_DIR で変更可）:
//
//	<dir>/<受信日 YYYY-MM-DD>/<件名>/<HHMMSS.mmm_xxxx>/
//	  meta.yaml      ... From/To/Cc/Subject/Date/Message-ID/添付一覧
//	  body.txt       ... text/plain 本文（あれば）
//	  body.html      ... text/html 本文（あれば）
//	  raw.eml        ... 受信メッセージ全体
//	  attachments/   ... 添付ファイル
//
// PHPは追加で -t -i 等の引数を渡すが、すべて無視する（保存先は環境変数で受け取る）。
package main

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

// Attachment はメタ情報に書き出す添付ファイルの概要。
type Attachment struct {
	Filename    string `yaml:"filename"`
	ContentType string `yaml:"content_type"`
	Size        int    `yaml:"size"`
}

// Meta は meta.yaml に書き出すメール全体のメタ情報。
type Meta struct {
	DateReceived string       `yaml:"date_received"`
	DateHeader   string       `yaml:"date_header,omitempty"`
	From         string       `yaml:"from,omitempty"`
	To           []string     `yaml:"to,omitempty"`
	Cc           []string     `yaml:"cc,omitempty"`
	Subject      string       `yaml:"subject"`
	MessageID    string       `yaml:"message_id,omitempty"`
	ContentType  string       `yaml:"content_type,omitempty"`
	Attachments  []Attachment `yaml:"attachments,omitempty"`
}

func main() {
	baseDir := os.Getenv("MAILDUMP_DIR")
	if baseDir == "" {
		baseDir = "/var/log/maildump"
	}
	if err := run(os.Stdin, baseDir, time.Now()); err != nil {
		fmt.Fprintf(os.Stderr, "maildump: %v\n", err)
		os.Exit(1)
	}
}

func run(input io.Reader, baseDir string, now time.Time) error {
	raw, err := io.ReadAll(input)
	if err != nil {
		return fmt.Errorf("failed to read stdin: %w", err)
	}

	msg, parseErr := mail.ReadMessage(bytes.NewReader(raw))

	meta := Meta{
		DateReceived: now.Format(time.RFC3339),
		Subject:      "(no subject)",
	}

	var textBody, htmlBody []byte
	var attachments []struct {
		name string
		data []byte
		ct   string
	}

	if parseErr == nil {
		dec := &mime.WordDecoder{}
		decode := func(s string) string {
			if d, e := dec.DecodeHeader(s); e == nil {
				return d
			}
			return s
		}

		meta.DateHeader = msg.Header.Get("Date")
		meta.From = decode(msg.Header.Get("From"))
		meta.To = splitAddresses(decode(msg.Header.Get("To")))
		meta.Cc = splitAddresses(decode(msg.Header.Get("Cc")))
		meta.MessageID = msg.Header.Get("Message-ID")
		meta.ContentType = msg.Header.Get("Content-Type")
		if s := strings.TrimSpace(decode(msg.Header.Get("Subject"))); s != "" {
			meta.Subject = s
		}

		// 本文・添付を抽出
		text, html, atts, walkErr := walkBody(msg.Header.Get("Content-Type"), msg.Header.Get("Content-Transfer-Encoding"), msg.Body)
		if walkErr == nil {
			textBody, htmlBody = text, html
			for _, a := range atts {
				attachments = append(attachments, struct {
					name string
					data []byte
					ct   string
				}{a.name, a.data, a.ct})
				meta.Attachments = append(meta.Attachments, Attachment{
					Filename:    a.name,
					ContentType: a.ct,
					Size:        len(a.data),
				})
			}
		}
	}

	// 保存先ディレクトリ: <base>/<日付>/<件名>/<時刻_ユニーク>/
	leaf := fmt.Sprintf("%s_%s", now.Format("150405.000"), randHex(2))
	subjectDir := sanitize(meta.Subject)
	if subjectDir == "" {
		subjectDir = "no-subject"
	}
	dir := filepath.Join(baseDir, now.Format("2006-01-02"), subjectDir, leaf)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create dir %s: %w", dir, err)
	}

	// raw.eml（元メッセージ全体）
	if err := writeFile(filepath.Join(dir, "raw.eml"), raw); err != nil {
		return err
	}

	// 本文
	if len(textBody) > 0 {
		if err := writeFile(filepath.Join(dir, "body.txt"), textBody); err != nil {
			return err
		}
	}
	if len(htmlBody) > 0 {
		if err := writeFile(filepath.Join(dir, "body.html"), htmlBody); err != nil {
			return err
		}
	}

	// 添付
	if len(attachments) > 0 {
		attDir := filepath.Join(dir, "attachments")
		if err := os.MkdirAll(attDir, 0o755); err != nil {
			return fmt.Errorf("failed to create attachment dir %s: %w", attDir, err)
		}
		for i, a := range attachments {
			name := sanitize(a.name)
			if name == "" {
				name = fmt.Sprintf("attachment-%d", i+1)
			}
			if err := writeFile(uniquePath(attDir, name), a.data); err != nil {
				return err
			}
		}
	}

	// meta.yaml
	y, err := yaml.Marshal(&meta)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}
	if err := writeFile(filepath.Join(dir, "meta.yaml"), y); err != nil {
		return err
	}

	return nil
}

type extractedAttachment struct {
	name string
	data []byte
	ct   string
}

// walkBody は Content-Type に応じて本文(text/html)と添付を再帰的に抽出する。
func walkBody(contentType, cte string, body io.Reader) (text, html []byte, atts []extractedAttachment, err error) {
	mediaType, params, perr := mime.ParseMediaType(contentType)
	if perr != nil {
		// Content-Typeが無い/壊れている場合は全体をプレーンテキスト扱い
		data, _ := io.ReadAll(body)
		return decodeCTE(cte, data), nil, nil, nil
	}

	if strings.HasPrefix(mediaType, "multipart/") {
		boundary := params["boundary"]
		if boundary == "" {
			return nil, nil, nil, fmt.Errorf("multipart without boundary")
		}
		mr := multipart.NewReader(body, boundary)
		for {
			part, perr := mr.NextPart()
			if perr == io.EOF {
				break
			}
			if perr != nil {
				return text, html, atts, nil // 途中で壊れても拾えた分は返す
			}
			pct := part.Header.Get("Content-Type")
			pcte := part.Header.Get("Content-Transfer-Encoding")
			filename := part.FileName()
			disposition := part.Header.Get("Content-Disposition")

			pmedia, _, _ := mime.ParseMediaType(pct)

			// ネストした multipart は再帰
			if strings.HasPrefix(pmedia, "multipart/") {
				t, h, a, _ := walkBody(pct, pcte, part)
				if len(t) > 0 && len(text) == 0 {
					text = t
				}
				if len(h) > 0 && len(html) == 0 {
					html = h
				}
				atts = append(atts, a...)
				part.Close()
				continue
			}

			data, _ := io.ReadAll(part)
			data = decodeCTE(pcte, data)

			isAttachment := filename != "" || strings.HasPrefix(strings.ToLower(disposition), "attachment")
			switch {
			case isAttachment:
				atts = append(atts, extractedAttachment{name: filename, data: data, ct: pmedia})
			case pmedia == "text/plain" && len(text) == 0:
				text = data
			case pmedia == "text/html" && len(html) == 0:
				html = data
			default:
				// その他のインラインパートは添付扱いで取りこぼさない
				if filename == "" {
					filename = "part"
				}
				atts = append(atts, extractedAttachment{name: filename, data: data, ct: pmedia})
			}
			part.Close()
		}
		return text, html, atts, nil
	}

	// 単一パート
	data, _ := io.ReadAll(body)
	data = decodeCTE(cte, data)
	if mediaType == "text/html" {
		return nil, data, nil, nil
	}
	return data, nil, nil, nil
}

// decodeCTE は Content-Transfer-Encoding に従ってデコードする。
func decodeCTE(cte string, data []byte) []byte {
	switch strings.ToLower(strings.TrimSpace(cte)) {
	case "base64":
		// base64本文は改行で折り返されているため除去してからデコード
		clean := bytes.Map(func(r rune) rune {
			if r == '\r' || r == '\n' || r == ' ' || r == '\t' {
				return -1
			}
			return r
		}, data)
		if decoded, err := base64.StdEncoding.DecodeString(string(clean)); err == nil {
			return decoded
		}
		return data
	case "quoted-printable":
		if decoded, err := io.ReadAll(quotedprintable.NewReader(bytes.NewReader(data))); err == nil {
			return decoded
		}
		return data
	default:
		return data
	}
}

var invalidNameChars = regexp.MustCompile(`[^\p{L}\p{N}\-_.()（）　 ]+`)

// sanitize はファイル/ディレクトリ名に使えるよう文字列を整える。
func sanitize(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "/", "-")
	s = strings.ReplaceAll(s, "\\", "-")
	s = invalidNameChars.ReplaceAllString(s, "_")
	s = strings.Trim(s, " 　._-")
	if len(s) > 120 {
		s = truncateUTF8(s, 120)
	}
	return s
}

func truncateUTF8(s string, maxBytes int) string {
	if maxBytes <= 0 {
		return ""
	}
	if len(s) <= maxBytes {
		return s
	}

	end := 0
	for len(s[end:]) > 0 {
		_, size := utf8.DecodeRuneInString(s[end:])
		if end+size > maxBytes {
			break
		}
		end += size
	}
	return s[:end]
}

// splitAddresses はアドレスヘッダをパースしてリスト化する。失敗時は素のまま1件返す。
func splitAddresses(header string) []string {
	header = strings.TrimSpace(header)
	if header == "" {
		return nil
	}
	if addrs, err := mail.ParseAddressList(header); err == nil {
		out := make([]string, 0, len(addrs))
		for _, a := range addrs {
			if a.Name != "" {
				out = append(out, fmt.Sprintf("%s <%s>", a.Name, a.Address))
			} else {
				out = append(out, a.Address)
			}
		}
		return out
	}
	return []string{header}
}

// uniquePath は同名ファイルがあれば連番を付けて衝突を避ける。
func uniquePath(dir, name string) string {
	p := filepath.Join(dir, name)
	if _, err := os.Stat(p); os.IsNotExist(err) {
		return p
	}
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	for i := 1; ; i++ {
		cand := filepath.Join(dir, fmt.Sprintf("%s-%d%s", base, i, ext))
		if _, err := os.Stat(cand); os.IsNotExist(err) {
			return cand
		}
	}
}

func writeFile(path string, data []byte) error {
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("failed to write %s: %w", path, err)
	}
	return nil
}

func randHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "0000"
	}
	return fmt.Sprintf("%x", b)
}
