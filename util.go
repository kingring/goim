package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/BurntSushi/goim/imdb"
	"github.com/BurntSushi/ty/fun"

	"os"
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
		if flagQuiet {
			return
		}
		pef(format, v...)
	}
)

func createFile(fpath string) *os.File {
	f, err := os.Create(fpath)
	if err != nil {
		fatalf(err.Error())
	}
	return f
}

func openFile(fpath string) *os.File {
	f, err := os.Open(fpath)
	if err != nil {
		fatalf(err.Error())
	}
	return f
}

func openDb(driver, dsn string) *imdb.DB {
	db, err := imdb.Open(driver, dsn)
	if err != nil {
		fatalf("Could not open '%s:%s': %s", driver, dsn, err)
	}
	return db
}

func closeDb(db *imdb.DB) {
	if err := db.Close(); err != nil {
		fatalf("Could not close database: %s", err)
	}
}

func intRange(s string, min, max int) (int, int) {
	s = strings.TrimSpace(s)
	if len(s) == 0 {
		return min, max
	}
	if !strings.Contains(s, "-") {
		n, err := strconv.Atoi(s)
		if err != nil {
			fatalf("Could not parse '%s' as integer: %s", s, err)
		}
		return n, n
	}
	pieces := fun.Map(strings.TrimSpace, strings.SplitN(s, "-", 2)).([]string)

	start, end := min, max
	var err error
	if len(pieces[0]) > 0 {
		start, err = strconv.Atoi(pieces[0])
		if err != nil {
			fatalf("Could not parse '%s' as integer: %s", pieces[0], err)
		}
	}
	if len(pieces[1]) > 0 {
		end, err = strconv.Atoi(pieces[1])
		if err != nil {
			fatalf("Could not parse '%s' as integer: %s", pieces[1], err)
		}
	}
	return start, end
}
