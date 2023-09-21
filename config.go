package srv

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v4"
	"github.com/peterbourgon/ff/v4/ffval"
)

var (
	errHelp    = errors.New("--help")
	errVersion = errors.New("--version")
)

type srvConfig struct {
	logFormat string
	logLevel  string
	pushURL   string
	flags     *ff.CoreFlags
}

func initConfig() (*srvConfig, error) {
	config := &srvConfig{}
	commonFlags := ff.NewFlags("srv config")
	commonFlags.AddFlag(ff.CoreFlagConfig{
		LongName:    "log-level",
		Placeholder: "debug|info|warn|error",
		Usage:       "logging level",
		Value: &ffval.String{
			ParseFunc: func(s string) (string, error) {
				s = strings.ToLower(s)
				switch s {
				case "debug", "info", "warn", "error":
					// fine
				default:
					return "", fmt.Errorf("invalid log level")
				}
				return s, nil
			},
			Pointer: &config.logLevel,
			Default: "info",
		},
	})
	commonFlags.AddFlag(ff.CoreFlagConfig{
		LongName:    "log-format",
		Placeholder: "text|json|human|auto",
		Usage:       `logging format - "auto" will pick 'human' if attached to tty, 'json' otherwise`,
		Value: &ffval.String{
			ParseFunc: func(s string) (string, error) {
				s = strings.ToLower(s)
				switch s {
				case "text", "json", "human", "auto":
					// fine
				default:
					return "", fmt.Errorf("invalid log format")
				}
				return s, nil
			},
			Pointer: &config.logFormat,
			Default: "auto",
		},
	})
	commonFlags.AddFlag(ff.CoreFlagConfig{
		LongName:    "push-url",
		Placeholder: "http[s]://<Pushgateway host>",
		Usage:       `URL to a Pushgateway host. Pushes all metrics to this host at shutdown using the service name as the job.`,
		Value: &ffval.String{
			Pointer: &config.pushURL,
		},
	})
	// TODO: --printconfig = <file|k8s|cmdline|env>
	// TODO: --version vs. --version=full
	commonFlags.AddFlag(ff.CoreFlagConfig{
		ShortName:     0,
		LongName:      "version",
		NoPlaceholder: true,
		Usage:         "print version info",
		Value: &ffval.Bool{
			Default: false,
		},
		NoDefault: true,
	})
	commonFlags.AddFlag(ff.CoreFlagConfig{
		ShortName:     0,
		LongName:      "config",
		Placeholder:   "<path to file>",
		NoPlaceholder: false,
		Usage:         "load configuration from a file instead",
		Value: &ffval.String{
			Default: "",
		},
		NoDefault: true,
	})
	// commonFlags.Bool(0, "version", false, "print version info")
	// commonFlags.String(0, "config", "", "config file")
	err := ff.Parse(commonFlags, os.Args[1:],
		ff.WithEnvVars(),
		ff.WithEnvVarPrefix("SRV"),
		ff.WithConfigFileFlag("config"),
		ff.WithConfigFileParser(ff.PlainParser), // todo: ignore version
	)
	switch {
	case err == nil:
	case errors.Is(err, ff.ErrHelp):
		// deferring `--help` so that user flags can appear before
		// Srv.ParseFlags()
	case errors.Is(err, ff.ErrUnknownFlag):
		// deferring unknown flags until the user flags have a chance to
		// represent their own flags later
	case errors.Is(err, os.ErrNotExist):
		if configFlag, hasConfigFlag := commonFlags.GetFlag("config"); hasConfigFlag {
			if configFlag.IsSet() {
				return nil, fmt.Errorf("couldn't open config file: %v", err)
			}
		}
		return nil, err
	default:
		return nil, err
	}
	config.flags = commonFlags
	return config, nil
}
