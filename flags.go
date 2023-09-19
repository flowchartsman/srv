package srv

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"andy.dev/srv/log"
	"github.com/peterbourgon/ff/v4"
	"github.com/peterbourgon/ff/v4/ffval"
)

// ParseFlags parses all flags and returns the args left over after a successful parse.
func (s *Srv) ParseFlags() []string {
	if s.userFlags.IsParsed() {
		s.error(log.Up(1), "ignoring extra call to ParseFlags()")
		return nil
	}
	flagN := s.srvFlags
	// for i := range srvProvidedFlags {
	// 	srvProvidedFlags[i].flags.SetParent(flagN)
	// 	flagN = srvProvidedFlags[i].flags
	// }
	s.userFlags.SetParent(flagN)
	err := ff.Parse(s.userFlags, os.Args[1:],
		ff.WithEnvVars(),
		ff.WithConfigFileFlag("config"),
		ff.WithConfigFileParser(ff.PlainParser),
	)
	switch {
	case errors.Is(err, ff.ErrHelp):
		s.printFlagsHelp(s.userFlags, false)
	case err != nil:
		s.fatal(log.Up(1), "ParseFlags()", err)
	}
	flagVersion, _ := s.userFlags.GetFlag("version")
	if flagVersion.IsSet() && flagVersion.GetValue() == "true" {
		s.printBuildData()
	}
	// for i := range srvProvidedFlags {
	// 	if err := srvProvidedFlags[i].setFn(); err != nil {
	// 		sFatal(srvProvidedFlags[i].loc, "srv.ParseFlags()", err)
	// 	}
	// }
	return s.userFlags.GetArgs()
}

// FlagBool adds a boolean flag to the configuration.
//
// Returns a *bool that will be set when [ParseFlags] is called.
func (s *Srv) FlagBool(name string, defaultValue bool, usage string) *bool {
	return addUserFlag(s, name, defaultValue, usage, s.userFlags.Bool)
}

// FlagBoolVar adds a boolean flag to the configuration.
//
// Takes a *bool that will be overwritten when [ParseFlags] is called.
func (s *Srv) FlagBoolVar(ptr *bool, name string, defaultValue bool, usage string) {
	addUserFlagVar(s, name, ff.CoreFlagConfig{
		Usage: usage,
		Value: &ffval.Bool{
			Pointer: ptr,
			Default: defaultValue,
		},
	})
}

// FlagInt adds an integer flag to the configuration.
//
// Returns an *int that will be set when [ParseFlags] is called.
func (s *Srv) FlagInt(name string, defaultValue int, usage string) *int {
	return addUserFlag(s, name, defaultValue, usage, s.userFlags.Int)
}

// FlagIntVar adds an integer flag to the configuration
//
// Takes an *int that will be overwritten when [ParseFlags] is called.
func (s *Srv) FlagIntVar(ptr *int, name string, defaultValue int, usage string) {
	addUserFlagVar(s, name, ff.CoreFlagConfig{
		Usage: usage,
		Value: &ffval.Int{
			Pointer: ptr,
			Default: defaultValue,
		},
	})
}

// FlagString adds a string flag to the configuration.
//
// Returns a *string that will be set when [ParseFlags] is called.
func (s *Srv) FlagString(name string, defaultValue string, usage string) *string {
	return addUserFlag(s, name, defaultValue, usage, s.userFlags.String)
}

// FlagStringVar adds a string flag to the configuration.
//
// Takes a *string that will be overwritten when [ParseFlags] is called.
func (s *Srv) FlagStringVar(ptr *string, name string, defaultValue string, usage string) {
	addUserFlagVar(s, name, ff.CoreFlagConfig{
		Usage: usage,
		Value: &ffval.String{
			Pointer: ptr,
			Default: defaultValue,
		},
	})
}

// FlagsProvider is a function providing both a set of flags and a function
// that will initialize the value of a concrete type from the values those flags
// provide.
//
// Example:
//
//		package database
//
//		type Client struct{/*...*/}
//
//	 func NewClient(url string)(*Client, error){
//			/*...*/
//		}
//
//		func ClientFlagsProvider()(*ff.CoreFlags, func()(*Client, error){
//			flags := ff.NewFlags("database client")
//			dbURL := flags.StringLong("dburl", "", "database URL")
//			return flags, func()(*Client, error){
//				return NewClient(dbURL)
//			}
//		})
//type FlagsProvider[T any] func() (*ff.CoreFlags, func() (T, error))

// FlagsValue takes a pointer to a value of type T, provided by the given
// FlagsProvider. The FlagsProvider will then add the set of flags necessary to
// instantiate the value to your configuration, and initialize the value when
// [ParseFlags] is called.
//
// Example:
//
//	var dbClient *database.Client
//	FlagsValue(&dbClient, database.ClientFlagsProvider)
//	srv.ParseFlags()
//
//	//dbClient is now initialized.
//	dbClient.Connect(dbURL)
// func FlagsValue[T any](ptr *T, provider FlagsProvider[T]) {
// 	flags, setter := provider()
// 	srvProvidedFlags = append(srvProvidedFlags, providerCtx{
// 		flags: flags,
// 		setFn: func() error {
// 			val, err := setter()
// 			if err != nil {
// 				return err
// 			}
// 			*ptr = val
// 			return nil
// 		},
// 	})
// }

// type providerCtx struct {
// 	flags *ff.CoreFlags
// 	loc   log.CodeLocation
// 	setFn func() error
// }

// TODO: Better Sync here w/flags
func addUserFlag[T any](s *Srv, flagName string, defaultValue T, usage string, setter func(rune, string, T, string) *T) *T {
	caller := log.Up(2)
	if s.started {
		s.fatal(caller, "flags can't be added after Start()")
	}
	if s.userFlags.IsParsed() {
		s.fatal(caller, "flags can't be added after ParseFlags()")
	}
	s.hasUserFlags = true
	short, long, err := parseFlagName(flagName)
	if err != nil {
		s.fatal(caller, "invalid flag name", err)
	}
	defer func(caller log.CodeLocation) {
		if v := recover(); v != nil {
			s.fatal(caller, "invalid flag", fmt.Errorf("%v", v))
		}
	}(caller)
	ptr := setter(short, long, defaultValue, usage)
	return ptr
}

// TODO: this one can actually be a method again
// TODO: Better Sync here w/flags
func addUserFlagVar(s *Srv, flagName string, flag ff.CoreFlagConfig) {
	caller := log.Up(2)
	if s.started {
		s.fatal(caller, "flags can't be added after Start()")
	}
	if s.userFlags.IsParsed() {
		s.fatal(caller, "flags can't be added after ParseFlags()")
	}
	s.hasUserFlags = true
	short, long, err := parseFlagName(flagName)
	if err != nil {
		s.fatal(caller, "invalid flag name", err)
	}
	flag.ShortName = short
	flag.LongName = long
	_, err = s.userFlags.AddFlag(flag)
	if err != nil {
		s.fatal(caller, "invalid flag", err)
	}
}

var (
	shortValid = regexp.MustCompile(`^[A-Za-z0-9]$`)
	longValid  = regexp.MustCompile(`^[a-z][a-z0-9-]+$`)
)

func parseFlagName(namespec string) (short rune, long string, err error) {
	var (
		hasShort bool
		hasLong  bool
		shortstr string
	)
	if namespec == "" {
		return flagErr("no name provided")
	}
	parts := strings.Split(namespec, `|`)
	switch len(parts) {
	case 1:
		if len(parts[0]) < 2 {
			hasShort = true
			shortstr = parts[0]
			break
		}
		hasLong = true
		long = parts[0]
	case 2:
		hasLong = true
		hasShort = true
		shortstr = parts[0]
		long = parts[1]
	default:
		return flagErr("must be [<short>|]<long>")
	}
	if hasShort {
		switch len(shortstr) {
		case 1:
			if !shortValid.MatchString(shortstr) {
				return flagErr("short flag char must be [a-zA-Z0-9] (got:%q)", shortstr)
			}
			short = rune(shortstr[0])
		default:
			return flagErr("short flag must be a single character (got: %q)", shortstr)
		}
	}
	if hasLong {
		switch len(long) {
		case 0:
			return flagErr("long flag name is empty")
		case 1:
			return flagErr("long flag name must be > 1 character (got: %q)", long)
		default:
			if !longValid.MatchString(long) {
				return flagErr("long flag name must be [a-z][a-z0-9-]+ (got: %q)", long)
			}
		}
	}
	return short, long, nil
}

func flagErr(format string, a ...any) (rune, string, error) {
	return flagErr(format, a...)
}
