package srv

import (
	"fmt"
	"strings"

	"github.com/mitchellh/go-wordwrap"
	"github.com/peterbourgon/ff/v4"
	"golang.org/x/sys/unix"
)

const (
	padLeft  = "        "
	minwidth = 79
)

func versionText() string {
	var sb strings.Builder
	sb.WriteString(srvInfo.Name)
	if srvInfo.Version == "" {
		sb.WriteString(" <no version>")
	} else {
		sb.WriteString(" v")
		sb.WriteString(srvInfo.Version)
	}
	sb.WriteString(" ")
	sb.WriteString(getBuildData().String())
	return sb.String()
}

func flagsHelp(flags *ff.CoreFlags) (string, error) {
	var sb strings.Builder
	// TODO: better place for serviceinfo? Not in instance
	sb.WriteString(srvInfo.Name + "\n")
	if srvInfo.About != "" {
		sb.WriteString(helpBlock(srvInfo.About) + "\n")
	}
	sb.WriteString("FLAGS\n")
	sec := ""
	err := flags.WalkFlags(func(flag ff.Flag) error {
		flagSec := flag.GetFlags().GetName()
		if sec == "" {
			sec = flagSec
		} else {
			sb.WriteString("\n")
		}
		if sec != flagSec {
			sec = flag.GetFlags().GetName()
			sb.WriteString("\n")
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
		sb.WriteString("\n")
		return nil
	})
	if err != nil {
		return "", err
	}
	return sb.String(), nil
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
