package logview

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveSomeWithSubjectAndTags(t *testing.T) {
	root := t.TempDir()
	m := testModel(map[string]bool{})
	m.o.Root, m.o.Target = root, "dev"
	m.feed([]byte(sample))
	m.selectEntry(m.entries[1], false)
	m.selectEntry(m.entries[2], true)
	path, err := m.saveSome(saveAnswer{Subject: "ورود خراب است", Tags: "auth, bug", What: saveSelected, Format: formatText})
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(root, path))
	text := string(data)
	if !strings.Contains(text, "# subject: ورود خراب است") || !strings.Contains(text, "# tags: auth, bug") ||
		!strings.Contains(text, "Bazaar: connected") || strings.Contains(text, "/v1/orders") {
		t.Errorf("saved:\n%s", text)
	}
	sessions := Sessions(root, m.look.Dir)
	if len(sessions) != 1 || sessions[0].Subject != "ورود خراب است" || strings.Join(sessions[0].Tags, ",") != "auth,bug" ||
		!sessions[0].Starred || sessions[0].Raw == "" {
		t.Errorf("sessions = %+v", sessions)
	}
	for _, f := range []string{formatMD, formatJSON} {
		path, err := m.saveSome(saveAnswer{Subject: "x", What: saveAll, Format: f})
		if err != nil || !strings.HasSuffix(path, "."+f) {
			t.Errorf("%s: %s %v", f, path, err)
		}
	}
}
