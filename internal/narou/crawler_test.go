package narou

import (
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

func TestExtractChapterRefsFromSample(t *testing.T) {
	doc := mustLoadSampleDoc(t, filepath.Join("..", "..", "sample", "ncode-index.html"), "https://ncode.syosetu.com/n7678ma/")
	base, err := doc.Url.Parse("https://ncode.syosetu.com/n7678ma/")
	if err != nil {
		t.Fatalf("parse base URL: %v", err)
	}

	chapters := extractChapterRefs(doc, base)
	if len(chapters) != 9 {
		t.Fatalf("expected 9 chapters, got %d", len(chapters))
	}
	if chapters[0].Title == "" {
		t.Fatalf("first chapter title should not be empty")
	}
	if !strings.Contains(chapters[0].URL, "/1/") {
		t.Fatalf("unexpected first chapter URL: %s", chapters[0].URL)
	}
}

func TestExtractChapterTextFromSample(t *testing.T) {
	doc := mustLoadSampleDoc(t, filepath.Join("..", "..", "sample", "ncode-chapter.html"), "https://ncode.syosetu.com/n7678ma/1/")
	text := extractChapterText(doc)
	if text == "" {
		t.Fatalf("expected non-empty chapter text")
	}
	if !strings.Contains(text, "何としても探し出せ") {
		t.Fatalf("expected extracted chapter text to contain sample phrase")
	}
}

func TestExtractNovelTitleFromNovel18Sample(t *testing.T) {
	doc := mustLoadSampleDoc(t, filepath.Join("..", "..", "sample", "novel18-index.html"), "https://novel18.syosetu.com/n7524gn/")
	title := extractNovelTitle(doc)
	if title != "寝取り魔法使いの冒険" {
		t.Fatalf("unexpected title: %s", title)
	}
}

func TestExtractNextPageURLFromNovel18Sample(t *testing.T) {
	doc := mustLoadSampleDoc(t, filepath.Join("..", "..", "sample", "novel18-index.html"), "https://novel18.syosetu.com/n7524gn/")
	next, ok := extractNextPageURL(doc, doc.Url)
	if !ok {
		t.Fatalf("expected next page URL")
	}
	if next != "https://novel18.syosetu.com/n7524gn/?p=2" {
		t.Fatalf("unexpected next page URL: %s", next)
	}
}

func TestDetectExistingChapters(t *testing.T) {
	dir := t.TempDir()
	files := []string{
		"001_First.txt",
		"002_Second.txt",
		"010_Tenth.txt",
		"notes.txt",
	}

	for _, name := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatalf("write temp file %s: %v", name, err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "subdir"), 0o755); err != nil {
		t.Fatalf("create temp dir: %v", err)
	}

	got, err := detectExistingChapters(dir)
	if err != nil {
		t.Fatalf("detect existing chapters: %v", err)
	}

	want := map[int]struct{}{1: {}, 2: {}, 10: {}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected existing chapters: got %#v want %#v", got, want)
	}
}

func mustLoadSampleDoc(t *testing.T, path string, baseURL string) *goquery.Document {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read sample file %s: %v", path, err)
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(b)))
	if err != nil {
		t.Fatalf("parse sample file %s: %v", path, err)
	}
	u, err := url.Parse(baseURL)
	if err != nil {
		t.Fatalf("parse URL: %v", err)
	}
	doc.Url = u
	return doc
}
