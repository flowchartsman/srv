package srv

import (
	"fmt"
	"os"
	"strings"

	"github.com/mitchellh/go-wordwrap"
	"github.com/peterbourgon/ff/v4"
	"golang.org/x/sys/unix"
)

const (
	padLeft  = "        "
	minwidth = 79
)

func (s *Srv) printBuildData() {
	fmt.Fprintf(os.Stdout, "%s v%s\n", s.srvInfo.Name, s.getBuildData())
	os.Exit(0)
}

func (s *Srv) printFlagsHelp(flags ff.Flags, isErr bool) {
	out := os.Stdout
	if isErr {
		out = os.Stderr
	}
	fmt.Fprintln(out, s.srvInfo.Name)
	if s.srvInfo.About != "" {
		fmt.Fprintln(out, helpBlock(s.srvInfo.About))
	}
	fmt.Fprintln(out, "FLAGS")
	var sb strings.Builder
	sec := ""
	flags.WalkFlags(func(flag ff.Flag) error {
		flagSec := flag.GetFlags().GetName()
		if sec == "" {
			sec = flagSec
		} else {
			sb.WriteString("\n")
		}
		if sec != flagSec {
			sec = flag.GetFlags().GetName()
			fmt.Fprintln(out)
		}
		sb.WriteString("    ")
		short, shortFound := flag.GetShortName()
		if shortFound {
			sb.WriteString("-")
			sb.WriteRune(short)
		}
		if long, longFound := flag.GetLongName(); longFound {
			if shortFound {
				sb.WriteString(", ")
			}
			sb.WriteString("--" + long)
		}
		if flag.GetDefault() != "" {
			sb.WriteString("=" + flag.GetDefault())
		}
		placeholder := flag.GetPlaceholder()
		if placeholder != "" {
			sb.WriteString(" ")
			if placeholder[0] == '<' && placeholder[len(placeholder)-1] == '>' {
				sb.WriteString(placeholder)
			} else {
				sb.WriteString("(" + flag.GetPlaceholder() + ")")
			}
		}
		sb.WriteString("\n")
		if flag.GetUsage() != "" {
			sb.WriteString(helpBlock(flag.GetUsage()))
		}
		out.WriteString(sb.String())
		sb.Reset()
		return nil
	})
	os.Exit(0)
}

// func isBoolFlag(flag ff.Flag) bool {
// 	if bf, ok := flag.(ff.IsBoolFlagger); ok {
// 		return bf.IsBoolFlag()
// 	}
// 	return false
// }

func termWidth() (width int) {
	ws, err := unix.IoctlGetWinsize(unix.Stdin, unix.TIOCGWINSZ)
	if err != nil {
		return -1
	}
	return int(ws.Col)
}

func helpBlock(str string) string {
	tw := termWidth()
	if tw < minwidth {
		return str
	}
	blockWidth := termWidth() - len(padLeft)*3
	if blockWidth < minwidth {
		blockWidth = minwidth
	}
	return padLeft + strings.ReplaceAll(wordwrap.WrapString(str, uint(blockWidth)), "\n", "\n"+padLeft)
}

func readableUnits(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB",
		float64(b)/float64(div), "KMGTPE"[exp])
}
