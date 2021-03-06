package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"strings"
	"text/template"

	"github.com/BurntSushi/ty/fun"

	"github.com/BurntSushi/toml"

	"github.com/BurntSushi/goim/imdb"
	"github.com/BurntSushi/goim/imdb/search"
	"github.com/BurntSushi/goim/tpl"
)

var (
	flagCpuProfile = ""
	flagCpu        = runtime.NumCPU()
	flagQuiet      = false
	flagDb         = ""
)

var (
	sf     = fmt.Sprintf
	ef     = fmt.Errorf
	pf     = fmt.Printf
	fatalf = func(f string, v ...interface{}) { pef(f, v...); os.Exit(1) }
	pef    = func(f string, v ...interface{}) {
		fmt.Fprintf(os.Stderr, f+"\n", v...)
	}
	logf = func(format string, v ...interface{}) {
		if !flagQuiet {
			pef(format, v...)
		}
	}
	warnf = func(format string, v ...interface{}) {
		if flagWarnings {
			pef(format, v...)
		}
	}
)

type command struct {
	name            string
	positionalUsage string
	shortHelp       string
	help            string
	flags           *flag.FlagSet
	addFlags        func(*command)
	run             func(*command) bool
	tpls            *template.Template
	other           bool
}

func (c *command) showUsage() {
	pf("Usage: goim %s [flags] %s\n", c.name, c.positionalUsage)
	c.showFlags()
	os.Exit(1)
}

func (c *command) showHelp() {
	pf("Usage: goim %s [flags] %s\n\n", c.name, c.positionalUsage)
	if help := strings.TrimSpace(c.help); len(help) > 0 {
		pf("%s\n\n", help)
	}
	pf("The flags are:\n\n")
	c.showFlags()
	pf("")
	os.Exit(1)
}

func (c *command) showFlags() {
	hide := []string{"cpu-prof", "quiet", "cpu"}
	c.flags.VisitAll(func(fl *flag.Flag) {
		if fun.In(fl.Name, hide) {
			return
		}
		var def string
		if len(fl.DefValue) > 0 {
			def = fmt.Sprintf(" (default: %s)", fl.DefValue)
		} else {
			def = " (default: \"\")"
		}
		usage := strings.Replace(fl.Usage, "\n", "\n    ", -1)
		pf("-%s%s\n", fl.Name, def)
		pf("    %s\n", usage)
	})
}

func (c *command) setCommonFlags() {
	c.flags.StringVar(&flagDb, "db", flagDb,
		"Overrides the database to be used. It should be a string of the "+
			"form 'driver:dsn'.\n"+
			"It may also be a 'sqlite3' file or a 'toml' file containing "+
			"a Goim configuration.")
	c.flags.StringVar(&flagCpuProfile, "cpu-prof", flagCpuProfile,
		"When set, a CPU profile will be written to the file path provided.")
	c.flags.IntVar(&flagCpu, "cpu", flagCpu,
		"Sets the maximum number of CPUs that can be executing simultaneously.")
	c.flags.BoolVar(&flagQuiet, "quiet", flagQuiet,
		"When set, status messages about the progress of a command will be "+
			"omitted.")
}

func (c *command) dbinfo() (driver, dsn string) {
	if len(flagDb) > 0 {
		if !strings.Contains(flagDb, ":") {
			if strings.HasSuffix(flagDb, "sqlite") ||
				strings.HasSuffix(flagDb, "sqlite3") {
				driver = "sqlite3"
				dsn = flagDb
			} else if strings.HasSuffix(flagDb, "toml") {
				conf, err := c.config(flagDb)
				if err != nil {
					fatalf("Error loading '%s' as config file: %s", flagDb, err)
				}
				driver, dsn = conf.Driver, conf.DataSource
			} else {
				fatalf("Database must be of the form 'dirver:dsn'.")
			}
		} else {
			dbInfo := strings.Split(flagDb, ":")
			driver, dsn = dbInfo[0], dbInfo[1]
		}
	} else {
		conf, err := c.config("")
		if err != nil {
			fatalf("If '-db' is not specified, then a configuration file\n"+
				"must exist in $XDG_CONFIG_HOME/goim/config.toml\n\n"+
				"Got this error when trying to read config: %s", err)
		}
		driver, dsn = conf.Driver, conf.DataSource
	}
	return
}

// config loads the configuration from the file path given. If fpath has length
// 0, then it will try to load the config from $XDG_CONFIG_HOME.
func (c *command) config(fpath string) (conf config, err error) {
	if len(fpath) == 0 {
		fpath, err = xdgPaths.ConfigFile("config.toml")
	}
	_, err = toml.DecodeFile(fpath, &conf)
	if len(conf.Driver) == 0 || len(conf.DataSource) == 0 {
		err = ef("Database driver '%s' or data source '%s' cannot be empty.",
			conf.Driver, conf.DataSource)
	}
	return
}

func (c *command) oneEntity(db *imdb.DB) (imdb.Entity, bool) {
	r, ok := c.oneResult(db)
	if !ok {
		return nil, false
	}
	ent, err := r.GetEntity(db)
	if err != nil {
		pef("%s\n", err)
		return nil, false
	}
	return ent, true
}

func (c *command) oneResult(db *imdb.DB) (*search.Result, bool) {
	rs, ok := c.results(db, true)
	if !ok || len(rs) == 0 {
		return nil, false
	}
	return &rs[0], true
}

func (c *command) results(db *imdb.DB, one bool) ([]search.Result, bool) {
	searcher, err := search.Query(db, strings.Join(c.flags.Args(), " "))
	if err != nil {
		pef("%s", err)
		return nil, false
	}
	searcher.Chooser(c.chooser)

	results, err := searcher.Results()
	if err != nil {
		pef("%s", err)
		return nil, false
	}
	if len(results) == 0 {
		pef("No results found.")
		return nil, false
	}
	if one {
		r, err := searcher.Pick(results)
		if err != nil {
			pef("%s", err)
			return nil, false
		}
		if r == nil {
			pef("No results to pick from.")
			return nil, false
		}
		return []search.Result{*r}, true
	}
	return results, true
}

func (c *command) chooser(
	results []search.Result,
	what string,
) (*search.Result, error) {
	pf("%s is ambiguous. Please choose one:\n", what)
	template := c.tpl("search_result")
	for i, result := range results {
		c.tplExec(template, tpl.Args{E: result, A: tpl.Attrs{"Index": i + 1}})
	}

	var choice int
	pf("Choice [%d-%d]: ", 1, len(results))
	if _, err := fmt.Fscanln(os.Stdin, &choice); err != nil {
		return nil, ef("Error reading from stdin: %s", err)
	}
	choice--
	if choice == -1 {
		return nil, nil
	} else if choice < -1 || choice >= len(results) {
		return nil, ef("Invalid choice %d", choice)
	}
	return &results[choice], nil
}

func areYouSure(yesno string) bool {
	var answer string
	pf("%s [y/n]: ", yesno)
	if _, err := fmt.Fscanln(os.Stdin, &answer); err != nil {
		pef("Error reading from stdin: %s", err)
		return false
	}
	answer = strings.ToLower(answer)
	if len(answer) >= 1 && answer[0] == 'y' {
		return true
	}
	return false
}

func (c *command) tplExec(t *template.Template, data interface{}) {
	if err := tpl.ExecText(t, os.Stdout, data); err != nil {
		fatalf(err.Error())
	}
}

func (c *command) tpl(name string) *template.Template {
	if c.tpls == nil {
		fpath, err := xdgPaths.ConfigFile("command.tpl")
		if err != nil {
			fpath = ""
		}
		c.tpls, err = tpl.ParseText(fpath)
		if err != nil {
			fatalf(err.Error())
		}
	}
	t := c.tpls.Lookup(name)
	if t == nil {
		fatalf("Could not find template with name '%s'.", name)
	}
	return t
}

func (c *command) assertNArg(n int) {
	if c.flags.NArg() != n {
		c.showUsage()
	}
}

func (c *command) assertLeastNArg(n int) {
	if c.flags.NArg() < n {
		c.showUsage()
	}
}

func createFile(fpath string) *os.File {
	f, err := os.Create(fpath)
	if err != nil {
		fatalf(err.Error())
	}
	return f
}

func openDb(driver, dsn string) *imdb.DB {
	db, err := imdb.Open(driver, dsn)
	if err != nil {
		fatalf("Could not open %s database: %s", driver, err)
	}
	return db
}

func closeDb(db *imdb.DB) {
	if err := db.Close(); err != nil {
		fatalf("Could not close database: %s", err)
	}
}
