package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"
	"text/template"

	"github.com/BurntSushi/toml"
)

var (
	flagCpuProfile = ""
	flagCpu        = runtime.NumCPU()
	flagQuiet      = false
	flagDb         = ""
	flagConfig     = ""
)

func init() {
	log.SetFlags(0)
}

type command struct {
	name            string
	positionalUsage string
	shortHelp       string
	help            string
	flags           *flag.FlagSet
	addFlags        func(*command)
	run             func(*command)
	tpls            *template.Template
}

func (c *command) showUsage() {
	log.Printf("Usage: goim %s [flags] %s\n", c.name, c.positionalUsage)
	c.showFlags()
	os.Exit(1)
}

func (c *command) showHelp() {
	log.Printf("Usage: goim %s [flags] %s\n\n", c.name, c.positionalUsage)
	log.Println(strings.TrimSpace(c.help))
	log.Printf("\nThe flags are:\n\n")
	c.showFlags()
	log.Println("")
	os.Exit(1)
}

func (c *command) showFlags() {
	c.flags.VisitAll(func(fl *flag.Flag) {
		if fl.Name == "cpu-prof" { // don't show this to users
			return
		}
		var def string
		if len(fl.DefValue) > 0 {
			def = fmt.Sprintf(" (default: %s)", fl.DefValue)
		} else {
			def = " (default: \"\")"
		}
		usage := strings.Replace(fl.Usage, "\n", "\n    ", -1)
		log.Printf("-%s%s\n", fl.Name, def)
		log.Printf("    %s\n", usage)
	})
}

func (c *command) setCommonFlags() {
	c.flags.StringVar(&flagDb, "db", flagDb,
		"Overrides the database to be used. It should be a string of the "+
			"form 'driver:dsn'.\nSee the config file for more details.")
	c.flags.StringVar(&flagConfig, "config", flagConfig,
		"If set, the configuration is loaded from the file given.")
	c.flags.StringVar(&flagCpuProfile, "cpu-prof", flagCpuProfile,
		"When set, a CPU profile will be written to the file path provided.")
	c.flags.IntVar(&flagCpu, "cpu", flagCpu,
		"Sets the maximum number of CPUs that can be executing simultaneously.")
	c.flags.BoolVar(&flagQuiet, "quiet", flagQuiet,
		"When set, status messages about the progress of a command will be "+
			"omitted.\n"+
			"For example, this will hide messages that say an ID could not\n"+
			"be found for entires in the release-dates list.")
}

func (c *command) dbinfo() (driver, dsn string) {
	if len(flagDb) > 0 {
		dbInfo := strings.Split(flagDb, ":")
		driver, dsn = dbInfo[0], dbInfo[1]
	} else {
		conf, err := c.config()
		if err != nil {
			fatalf("If '-db' is not specified, then a configuration file\n"+
				"must exist in $XDG_CONFIG_HOME/goim/config.toml or be\n"+
				"specified with '-config'.\n\n"+
				"Got this error when trying to read config: %s", err)
		}
		driver, dsn = conf.Driver, conf.DataSource
	}
	return
}

func (c *command) config() (conf config, err error) {
	var fpath string
	if len(flagConfig) > 0 {
		fpath = flagConfig
	} else {
		fpath, err = xdgPaths.ConfigFile("config.toml")
	}
	_, err = toml.DecodeFile(fpath, &conf)
	if len(conf.Driver) == 0 || len(conf.DataSource) == 0 {
		err = ef("Database driver '%s' or data source '%s' cannot be empty.",
			conf.Driver, conf.DataSource)
	}
	return
}

func (c *command) tplExec(template *template.Template, data interface{}) {
	if err := template.Execute(os.Stdout, data); err != nil {
		fatalf("Could not execute template '%s': %s", template.Name(), err)
	}
}

func (c *command) tpl(name string) *template.Template {
	if c.tpls == nil {
		fpath, err := xdgPaths.ConfigFile("format.tpl")
		if err == nil {
			c.tpls, err = template.ParseFiles(fpath)
			if err != nil {
				fatalf("Problem parsing template 'format.tpl': %s", err)
			}
		}
	}
	if c.tpls != nil {
		t := c.tpls.Lookup(name)
		if t != nil {
			return t
		}
	}
	return defaultTemplate(name)
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
