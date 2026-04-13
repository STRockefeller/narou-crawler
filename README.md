# narou-crawler

A command-line tool for downloading Narou-family web novels as plain text files.

Supported sites:

- https://syosetu.com
- https://yomou.syosetu.com
- https://ncode.syosetu.com
- https://novel18.syosetu.com

## Installation

Install with the full module path:

```bash
go install github.com/STRockefeller/narou-crawler/cmd/narou-crawler@latest
```

You can also install from the repository root during local development:

```bash
go install github.com/STRockefeller/narou-crawler/cmd/narou-crawler
```

After installation:

```bash
narou-crawler https://ncode.syosetu.com/n7678ma/
```

## Usage

```bash
narou-crawler [-o output_dir] <index_url>
```

Example:

```bash
narou-crawler -o ./downloads https://ncode.syosetu.com/n7678ma/
```

## Behavior

- Creates a folder named after the novel title.
- Downloads each chapter as a separate `.txt` file.
- Saves files as `NNN_chapter-title.txt`, for example `001_Prologue.txt`.
- If the output folder already exists, scans existing chapter files and downloads only missing chapters.
