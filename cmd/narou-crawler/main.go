package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/STRockefeller/narou-crawler/internal/narou"
)

func main() {
	var outputRoot string
	flag.StringVar(&outputRoot, "o", ".", "output directory")
	flag.Parse()

	if flag.NArg() != 1 {
		fmt.Fprintf(os.Stderr, "Usage: %s [-o output_dir] <index_url>\n", filepath.Base(os.Args[0]))
		os.Exit(2)
	}

	indexURL := flag.Arg(0)

	crawler, err := narou.NewCrawler()
	if err != nil {
		log.Fatalf("failed to initialize crawler: %v", err)
	}

	result, err := crawler.Download(indexURL, outputRoot)
	if err != nil {
		log.Fatalf("download failed: %v", err)
	}

	fmt.Printf("Saved novel: %s\n", result.NovelTitle)
	fmt.Printf("Output dir: %s\n", result.OutputDir)
	fmt.Printf("Total chapters: %d\n", result.ChapterCount)
	fmt.Printf("Downloaded: %d\n", result.DownloadedChapterCount)
	fmt.Printf("Skipped existing: %d\n", result.SkippedChapterCount)
}
