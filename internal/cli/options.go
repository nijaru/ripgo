package cli

// CLI is the explicit command tree shown by top-level help. The executable
// handles the legacy content-search syntax separately before parsing this tree.
type CLI struct {
	Search Options     `cmd:"search" help:"Search file contents"`
	Find   FindOptions `cmd:"find" help:"Find paths by name"`
}

type Options struct {
	Pattern          string   `arg:"" name:"pattern" optional:"" help:"Pattern to search for"`
	Paths            []string `arg:"" name:"paths" optional:"" help:"Paths to search (default: .)"`
	FixedStrings     bool     `short:"F" help:"Interpret pattern as fixed strings"`
	IgnoreCase       bool     `short:"i" help:"Case insensitive search"`
	SmartCase        bool     `short:"I" help:"Case insensitive unless pattern contains uppercase"`
	Regex            bool     `short:"e" help:"Interpret pattern as regex (default)"`
	Pcre2            bool     `short:"P" help:"Interpret pattern as PCRE2 regex"`
	WordRegexp       bool     `short:"w" help:"Match whole words only"`
	Hidden           bool     `short:"a" help:"Search hidden files"`
	NoIgnore         bool     `name:"no-ignore" help:"Don't respect .gitignore files"`
	Unrestricted     int      `short:"u" help:"Reduce ignore levels (-u, -uu, -uuu)"`
	FollowSymlinks   bool     `short:"L" help:"Follow symbolic links"`
	MaxCount         int      `short:"m" help:"Maximum matches per file"`
	ContextBefore    int      `short:"B" help:"Lines of context before match"`
	ContextAfter     int      `short:"A" help:"Lines of context after match"`
	Context          int      `short:"C" help:"Lines of context before and after match"`
	LineNumber       bool     `short:"n" help:"Show line numbers"`
	Column           bool     `short:"b" help:"Show byte offset of match"`
	Count            bool     `short:"c" help:"Show count of matches"`
	FilesWithMatches bool     `short:"l" help:"Show only filenames with matches"`
	OnlyMatching     bool     `short:"o" help:"Show only matching parts of a line"`
	Quiet            bool     `short:"q" help:"Exit with 0 if match, 1 if no match"`
	MaxColumns       int      `short:"M" help:"Limit the length of output lines"`
	MaxFileSize      string   `name:"max-filesize" help:"Ignore files larger than size"`
	GlobInclude      []string `name:"glob" help:"Include files matching glob"`
	GlobExclude      []string `name:"glob-not" help:"Exclude files matching glob"`
	Type             []string `short:"t" help:"Search only files of this type"`
	TypeNot          []string `short:"T" help:"Do not search files of this type"`
	TypeList         bool     `name:"type-list" help:"Show all supported file types"`
	NoBinary         bool     `name:"no-binary" help:"Don't search binary files"`
	OnlyBinary       bool     `name:"only-binary" help:"Search only binary files"`
	Replace          string   `short:"r" help:"Replacement string"`
	Json             bool     `name:"json" help:"JSON output"`

	Multiline bool   `short:"U" help:"Multiline search"`
	Heading   bool   `name:"heading" default:"false" negatable:"" help:"Show filename once per group of matches"`
	Color     string `name:"color" default:"auto" enum:"always,never,auto" help:"Control color output (always, never, auto)"`
	Sort      string `name:"sort" help:"Sort results (path, modified, accessed, created, none) (default: none)"`
	Threads   int    `short:"j" help:"Number of worker threads (default: auto)"`
	Version   bool   `short:"v" help:"Show version"`
}

// FindOptions contains flags for the explicit path-finding command.
type FindOptions struct {
	Pattern        string   `arg:"" name:"pattern" optional:"" help:"Pattern to match against paths"`
	Paths          []string `arg:"" name:"paths" optional:"" help:"Paths to search (default: .)"`
	Glob           bool     `short:"g" help:"Interpret pattern as a glob"`
	FixedStrings   bool     `short:"F" help:"Interpret pattern as a fixed string"`
	IgnoreCase     bool     `short:"i" help:"Case insensitive matching"`
	FullPath       bool     `name:"full-path" help:"Match the full path instead of the basename"`
	Type           []string `short:"t" help:"Find only files (f), directories (d), or symlinks (l)"`
	Extension      []string `short:"e" name:"extension" help:"Filter by extension"`
	Hidden         bool     `short:"a" help:"Include hidden paths"`
	NoIgnore       bool     `name:"no-ignore" help:"Don't respect ignore files"`
	FollowSymlinks bool     `short:"L" help:"Follow symbolic links"`
	MinDepth       int      `name:"min-depth" help:"Minimum root-relative depth"`
	MaxDepth       *int     `name:"max-depth" help:"Maximum root-relative depth"`
	MinSize        string   `name:"min-size" help:"Minimum size (bytes, k, m, or g)"`
	MaxSize        string   `name:"max-size" help:"Maximum size (bytes, k, m, or g)"`
	Absolute       bool     `name:"absolute" help:"Print absolute paths"`
	Print0         bool     `name:"print0" help:"Terminate paths with NUL"`
	Color          string   `name:"color" default:"auto" enum:"always,never,auto" help:"Control color output"`
	Sort           string   `name:"sort" default:"none" enum:"none,path" help:"Sort output paths"`
	Threads        int      `short:"j" help:"Number of worker threads (default: auto)"`
	Exec           string   `name:"exec" help:"Run a shell-free command for each match (use {})"`
	ExecBatch      string   `name:"exec-batch" help:"Run a shell-free command per bounded batch (use standalone {})"`
	ExecBatchSize  int      `name:"exec-batch-size" default:"100" help:"Maximum paths per --exec-batch invocation"`
	Delete         bool     `name:"delete" help:"Delete matched files or symlinks (requires --type f or --type l)"`
	DryRun         bool     `name:"dry-run" help:"Preview --delete without changing files"`
	Version        bool     `short:"v" help:"Show version"`
}

var Version = "0.1.0"
