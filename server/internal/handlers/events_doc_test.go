package handlers_test

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// docExample is a JSON example of api/events.md, tagged client or server.
type docExample struct {
	side string
	raw  string
}

var exampleBlock = regexp.MustCompile("(?s)```json (client|server)\n(.*?)```")

func eventExamples(t *testing.T) []docExample {
	t.Helper()
	b, err := os.ReadFile("../../../api/events.md")
	if err != nil {
		t.Fatal(err)
	}
	var out []docExample
	for _, m := range exampleBlock.FindAllStringSubmatch(string(b), -1) {
		out = append(out, docExample{side: m[1], raw: m[2]})
	}
	if len(out) < 40 {
		t.Fatalf("only %d examples in events.md", len(out))
	}
	return out
}

// TestEventsDoc keeps api/events.md honest (F010-R44): every example is
// valid JSON with a type; the server understands every client message
// (anything but bad_message or unknown_type), and every server message is
// one the server code sends.
func TestEventsDoc(t *testing.T) {
	e := newPlayEnv(t)
	srv := httptest.NewServer(e.h)
	t.Cleanup(srv.Close)
	c := e.socket(srv, e.admin)

	var source strings.Builder
	for _, dir := range []string{"../party", "../realtime"} {
		files, _ := filepath.Glob(filepath.Join(dir, "*.go"))
		for _, f := range files {
			if strings.HasSuffix(f, "_test.go") {
				continue
			}
			b, _ := os.ReadFile(f)
			source.Write(b)
		}
	}
	for i, ex := range eventExamples(t) {
		var msg struct {
			Type string          `json:"type"`
			Data json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal([]byte(ex.raw), &msg); err != nil || msg.Type == "" {
			t.Errorf("example %d isn't a message: %v\n%s", i, err, ex.raw)
			continue
		}
		if ex.side == "server" {
			if !strings.Contains(source.String(), `"`+msg.Type+`"`) {
				t.Errorf("server message %s isn't sent anywhere", msg.Type)
			}
			continue
		}
		var data any
		json.Unmarshal(msg.Data, &data)
		if _, code := c.send(msg.Type, data); code == "bad_message" || code == "unknown_type" {
			t.Errorf("client message %s: %s", msg.Type, code)
		}
	}
}
