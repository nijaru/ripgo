package search

import (
	"bufio"
	"bytes"
	"os"
	"strings"

	"github.com/nijaru/ripgo/internal/config"
	"github.com/nijaru/ripgo/internal/pattern"
)

type Result struct {
	Path    string
	Matches []Match
	Before  [][]byte
	After   [][]byte
}

type Match struct {
	Line       int
	Column     int
	LineBytes  []byte
	Submatches [][2]int
}

type Searcher struct {
	cfg     *config.Config
	matcher pattern.Matcher
}

func NewSearcher(cfg *config.Config, matcher pattern.Matcher) *Searcher {
	return &Searcher{
		cfg:     cfg,
		matcher: matcher,
	}
}

func (s *Searcher) Search(path string) (Result, error) {
	result := Result{Path: path}

	file, err := os.Open(path)
	if err != nil {
		return result, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0
	matchCount := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()

		locs, ok := s.matcher.Match(line)
		if ok {
			match := Match{
				Line:       lineNum,
				Column:     locs[0] + 1,
				LineBytes:  bytes.Clone(line),
				Submatches: [][2]int{{locs[0], locs[1]}},
			}
			result.Matches = append(result.Matches, match)
			matchCount++

			if s.cfg.MaxCount > 0 && matchCount >= s.cfg.MaxCount {
				break
			}
		}
	}

	return result, scanner.Err()
}

func (s *Searcher) SearchMultiline(path string) (Result, error) {
	result := Result{Path: path}

	data, err := os.ReadFile(path)
	if err != nil {
		return result, err
	}

	if s.matcher.MatchFile(data) {
		lines := bytes.Split(data, []byte("\n"))
		for i, line := range lines {
			if _, ok := s.matcher.Match(line); ok {
				match := Match{
					Line:       i + 1,
					Column:     1,
					LineBytes:  bytes.Clone(line),
					Submatches: [][2]int{{0, len(line)}},
				}
				result.Matches = append(result.Matches, match)
			}
		}
	}

	return result, nil
}

func ExtractContext(result Result, ctxBefore, ctxAfter int) Result {
	ctx := Result{
		Path: result.Path,
	}

	for i, m := range result.Matches {
		start := i - ctxBefore
		if start < 0 {
			start = 0
		}
		end := i + ctxAfter + 1
		if end > len(result.Matches) {
			end = len(result.Matches)
		}

		for j := start; j < end; j++ {
			if j < i {
				ctx.Before = append(ctx.Before, result.Matches[j].LineBytes)
			} else if j == i {
				ctx.Matches = append(ctx.Matches, m)
			} else {
				ctx.After = append(ctx.After, result.Matches[j].LineBytes)
			}
		}
	}

	return ctx
}

func trimSuffixNL(s []byte) string {
	return strings.TrimRight(string(s), "\n")
}
