package narou

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

var allowedHosts = map[string]struct{}{
	"syosetu.com":         {},
	"yomou.syosetu.com":   {},
	"ncode.syosetu.com":   {},
	"novel18.syosetu.com": {},
}

type Crawler struct {
	client *http.Client
}

type DownloadResult struct {
	NovelTitle             string
	OutputDir              string
	ChapterCount           int
	DownloadedChapterCount int
	SkippedChapterCount    int
}

type chapterRef struct {
	URL   string
	Title string
}

func NewCrawler() (*Crawler, error) {
	return &Crawler{
		client: &http.Client{Timeout: 25 * time.Second},
	}, nil
}

func (c *Crawler) Download(indexURL, outputRoot string) (*DownloadResult, error) {
	parsed, err := url.Parse(indexURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return nil, fmt.Errorf("URL scheme must be http or https")
	}
	if err := validateHost(parsed.Hostname()); err != nil {
		return nil, err
	}

	indexDoc, err := c.fetchDocument(parsed.String())
	if err != nil {
		return nil, fmt.Errorf("fetch index page: %w", err)
	}

	novelTitle := extractNovelTitle(indexDoc)
	if novelTitle == "" {
		return nil, fmt.Errorf("failed to parse novel title")
	}

	chapters, err := c.collectChapterRefs(parsed.String())
	if err != nil {
		return nil, fmt.Errorf("collect chapter list: %w", err)
	}
	if len(chapters) == 0 {
		return nil, fmt.Errorf("failed to parse chapter list")
	}

	outputDir := filepath.Join(outputRoot, sanitizeFilename(novelTitle))
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return nil, fmt.Errorf("create output directory: %w", err)
	}

	existingChapters, err := detectExistingChapters(outputDir)
	if err != nil {
		return nil, fmt.Errorf("scan existing chapters: %w", err)
	}

	downloadedCount := 0
	skippedCount := 0

	for i, ch := range chapters {
		chapterNumber := i + 1
		if _, exists := existingChapters[chapterNumber]; exists {
			skippedCount++
			continue
		}

		chapterDoc, err := c.fetchDocument(ch.URL)
		if err != nil {
			return nil, fmt.Errorf("fetch chapter %d (%s): %w", chapterNumber, ch.URL, err)
		}

		title := cleanText(chapterDoc.Find("h1.p-novel__title").First().Text())
		if title == "" {
			title = ch.Title
		}
		if title == "" {
			title = fmt.Sprintf("chapter-%d", chapterNumber)
		}

		content := extractChapterText(chapterDoc)
		if content == "" {
			return nil, fmt.Errorf("empty chapter content for %s", ch.URL)
		}

		fileName := fmt.Sprintf("%03d_%s.txt", chapterNumber, sanitizeFilename(title))
		filePath := filepath.Join(outputDir, fileName)
		text := strings.TrimSpace(title) + "\n\n" + content + "\n"

		if err := os.WriteFile(filePath, []byte(text), 0o644); err != nil {
			return nil, fmt.Errorf("write chapter file: %w", err)
		}
		downloadedCount++
	}

	return &DownloadResult{
		NovelTitle:             novelTitle,
		OutputDir:              outputDir,
		ChapterCount:           len(chapters),
		DownloadedChapterCount: downloadedCount,
		SkippedChapterCount:    skippedCount,
	}, nil
}

func (c *Crawler) collectChapterRefs(startURL string) ([]chapterRef, error) {
	all := make([]chapterRef, 0, 256)
	seenChapter := map[string]struct{}{}
	seenPage := map[string]struct{}{}

	current := startURL
	for {
		if _, visited := seenPage[current]; visited {
			break
		}
		seenPage[current] = struct{}{}

		doc, err := c.fetchDocument(current)
		if err != nil {
			return nil, fmt.Errorf("fetch index page %s: %w", current, err)
		}

		parsedCurrent, err := url.Parse(current)
		if err != nil {
			return nil, fmt.Errorf("parse page URL %s: %w", current, err)
		}

		for _, ch := range extractChapterRefs(doc, parsedCurrent) {
			if _, ok := seenChapter[ch.URL]; ok {
				continue
			}
			all = append(all, ch)
			seenChapter[ch.URL] = struct{}{}
		}

		next, ok := extractNextPageURL(doc, parsedCurrent)
		if !ok {
			break
		}
		current = next
	}

	sort.SliceStable(all, func(i, j int) bool {
		return chapterOrder(all[i].URL) < chapterOrder(all[j].URL)
	})

	return all, nil
}

func validateHost(host string) error {
	if _, ok := allowedHosts[host]; ok {
		return nil
	}
	if strings.HasSuffix(host, ".syosetu.com") {
		return nil
	}
	return fmt.Errorf("unsupported host: %s", host)
}

func (c *Crawler) fetchDocument(rawURL string) (*goquery.Document, error) {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "narou-crawler/1.0 (+https://syosetu.com)")
	req.Header.Set("Cookie", "over18=yes")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return goquery.NewDocumentFromReader(strings.NewReader(string(body)))
}

func extractNovelTitle(doc *goquery.Document) string {
	selectors := []string{
		"h1.p-novel__title",
		"h1.novel_title",
		"#novel_color .novel_title",
	}

	for _, selector := range selectors {
		title := cleanText(doc.Find(selector).First().Text())
		if title != "" {
			return title
		}
	}

	title := cleanText(doc.Find("meta[property='og:title']").AttrOr("content", ""))
	if title != "" {
		return title
	}

	title = cleanText(doc.Find("title").First().Text())
	if title != "" {
		if i := strings.Index(title, " - "); i > 0 {
			title = strings.TrimSpace(title[:i])
		}
		return title
	}

	return ""
}

func extractChapterRefs(doc *goquery.Document, baseURL *url.URL) []chapterRef {
	chapters := make([]chapterRef, 0, 64)
	seen := map[string]struct{}{}

	doc.Find("a.p-eplist__subtitle").Each(func(_ int, s *goquery.Selection) {
		href, ok := s.Attr("href")
		if !ok || strings.TrimSpace(href) == "" {
			return
		}

		resolved, err := baseURL.Parse(href)
		if err != nil {
			return
		}
		u := resolved.String()
		if _, exists := seen[u]; exists {
			return
		}

		title := cleanText(s.Text())
		chapters = append(chapters, chapterRef{URL: u, Title: title})
		seen[u] = struct{}{}
	})

	return chapters
}

func extractNextPageURL(doc *goquery.Document, baseURL *url.URL) (string, bool) {
	selectors := []string{
		"a.c-pager__item--next",
		"a.novelview_pager-next",
	}

	for _, selector := range selectors {
		next := doc.Find(selector).First()
		if next.Length() == 0 {
			continue
		}
		href, ok := next.Attr("href")
		if !ok || strings.TrimSpace(href) == "" {
			continue
		}
		resolved, err := baseURL.Parse(href)
		if err != nil {
			continue
		}
		return resolved.String(), true
	}

	return "", false
}

var chapterNumRegex = regexp.MustCompile(`/([0-9]+)/?$`)
var savedChapterFileRegex = regexp.MustCompile(`^(\d+)_.*\.txt$`)

func chapterOrder(chapterURL string) int {
	m := chapterNumRegex.FindStringSubmatch(chapterURL)
	if len(m) < 2 {
		return 1 << 30
	}

	var n int
	for _, ch := range m[1] {
		n = n*10 + int(ch-'0')
	}
	return n
}

func detectExistingChapters(outputDir string) (map[int]struct{}, error) {
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		return nil, err
	}

	existing := make(map[int]struct{}, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		matches := savedChapterFileRegex.FindStringSubmatch(entry.Name())
		if len(matches) < 2 {
			continue
		}

		var number int
		for _, ch := range matches[1] {
			number = number*10 + int(ch-'0')
		}
		existing[number] = struct{}{}
	}

	return existing, nil
}

func extractChapterText(doc *goquery.Document) string {
	body := doc.Find("div.p-novel__body")
	if body.Length() == 0 {
		return ""
	}

	blocks := make([]string, 0, 3)
	body.Find("div.p-novel__text").Each(func(_ int, s *goquery.Selection) {
		text := extractTextBlock(s)
		text = strings.TrimSpace(text)
		if text != "" {
			blocks = append(blocks, text)
		}
	})

	return strings.Join(blocks, "\n\n")
}

func extractTextBlock(s *goquery.Selection) string {
	if s.Find("p").Length() == 0 {
		return selectionTextWithBreaks(s)
	}

	lines := make([]string, 0, 128)
	s.Find("p").Each(func(_ int, p *goquery.Selection) {
		t := strings.ReplaceAll(p.Text(), "\u00a0", " ")
		t = strings.ReplaceAll(t, "\r\n", "\n")
		t = strings.ReplaceAll(t, "\r", "\n")
		t = strings.TrimSpace(t)
		lines = append(lines, t)
	})

	end := len(lines)
	for end > 0 && lines[end-1] == "" {
		end--
	}
	return strings.Join(lines[:end], "\n")
}

func selectionTextWithBreaks(s *goquery.Selection) string {
	clone := s.Clone()
	clone.Find("br").Each(func(_ int, br *goquery.Selection) {
		br.ReplaceWithHtml("\n")
	})

	text := cleanText(clone.Text())
	text = strings.ReplaceAll(text, "\u00a0", " ")
	lines := strings.Split(text, "\n")
	normalized := make([]string, 0, len(lines))
	for _, line := range lines {
		normalized = append(normalized, strings.TrimRight(line, " \t"))
	}
	return strings.Join(normalized, "\n")
}

func cleanText(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	s = strings.TrimSpace(s)
	return s
}

var invalidFilenameChars = regexp.MustCompile(`[<>:"/\\|?*\x00-\x1F]`)
var repeatedSpace = regexp.MustCompile(`\s+`)

func sanitizeFilename(name string) string {
	cleaned := invalidFilenameChars.ReplaceAllString(name, "_")
	cleaned = repeatedSpace.ReplaceAllString(cleaned, " ")
	cleaned = strings.TrimSpace(cleaned)
	cleaned = strings.Trim(cleaned, ".")
	if cleaned == "" {
		return "untitled"
	}
	return cleaned
}
