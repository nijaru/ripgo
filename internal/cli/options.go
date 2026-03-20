package cli

type Options struct {
	Pattern          string   `arg:"" name:"pattern" help:"Pattern to search for"`
	Paths            []string `arg:"" name:"paths" optional:"" help:"Paths to search (default: .)"`
	FixedStrings     bool     `short:"F" help:"Interpret pattern as fixed strings"`
	IgnoreCase       bool     `short:"i" help:"Case insensitive search"`
	SmartCase        bool     `short:"I" help:"Case insensitive unless pattern contains uppercase"`
	Regex            bool     `short:"e" help:"Interpret pattern as regex (default)"`
	Pcre2            bool     `short:"P" help:"Interpret pattern as PCRE2 regex"`
	Hidden           bool     `short:"a" help:"Search hidden files"`
	NoIgnore         bool     `name:"no-ignore" help:"Don't respect .gitignore files"`
	FollowSymlinks   bool     `short:"L" help:"Follow symbolic links"`
	MaxCount         int      `short:"m" help:"Maximum matches per file"`
	ContextBefore    int      `short:"B" help:"Lines of context before match"`
	ContextAfter     int      `short:"A" help:"Lines of context after match"`
	Context          int      `short:"C" help:"Lines of context before and after match"`
	LineNumber       bool     `short:"n" help:"Show line numbers"`
	Column           bool     `short:"b" help:"Show byte offset of match"`
	Count            bool     `short:"c" help:"Show count of matches"`
	FilesWithMatches bool     `short:"l" help:"Show only filenames with matches"`
	Quiet            bool     `short:"q" help:"Exit with 0 if match, 1 if no match"`
	MaxFileSize      string   `name:"max-filesize" help:"Ignore files larger than size"`
	GlobInclude      []string `name:"glob" help:"Include files matching glob"`
	GlobExclude      []string `name:"glob-not" help:"Exclude files matching glob"`
	Type             []string `short:"t" help:"Search only files of this type"`
	TypeNot          []string `short:"T" help:"Do not search files of this type"`
	TypeList         bool     `name:"type-list" help:"Show all supported file types"`
	NoBinary         bool     `name:"no-binary" help:"Don't search binary files"`
	OnlyBinary       bool     `name:"only-binary" help:"Search only binary files"`
	Json             bool     `name:"json" help:"JSON output"`
	Multiline        bool     `short:"U" help:"Multiline search"`
	Heading          bool     `name:"heading" default:"false" negatable:"" help:"Show filename once per group of matches"`
	Color            string   `name:"color" default:"auto" enum:"always,never,auto" help:"Control color output (always, never, auto)"`
	Sort             string   `name:"sort" help:"Sort results (path, modified, accessed, created, none) (default: none)"`
	Threads          int      `short:"j" help:"Number of worker threads (default: auto)"`
	Version          bool     `short:"v" help:"Show version"`
}

var Version = "0.1.0"
