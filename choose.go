package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/BurntSushi/ty/fun"

	"github.com/BurntSushi/goim/imdb"
)

type choice struct {
	Index int
	imdb.SearchResult
}

// choose searches the database for the query given. If there is more than
// one result, then a list is displayed from which the user can choose.
// The corresponding search result is then returned (or nil if something went
// wrong).
//
// Note that in order to use this effectively, the flags for searching should
// be enabled. e.g., `cmdSearch.addFlags(your_command)`.
func (c *command) choose(db *imdb.DB, query string) *imdb.SearchResult {
	var entities []imdb.Entity
	if len(flagSearchEntities) > 0 {
		entities = fun.Map(imdb.EntityFromString,
			strings.Split(flagSearchEntities, ",")).([]imdb.Entity)
	}

	opts := imdb.DefaultSearch
	opts.Entities = entities
	opts.NoCase = flagSearchNoCase
	opts.Limit = flagSearchLimit
	opts.Order = []imdb.SearchOrder{{flagSearchSort, flagSearchOrder}}
	opts.Fuzzy = flagSearchFuzzy

	ystart, yend := intRange(flagSearchYear, opts.YearStart, opts.YearEnd)
	opts.YearStart, opts.YearEnd = ystart, yend

	template := c.tpl("search_result")
	results, err := opts.Search(db, query)
	if err != nil {
		fatalf("Error searching: %s", err)
	}
	if len(results) == 0 {
		return nil
	} else if len(results) == 1 {
		return &results[0]
	}
	for i, result := range results {
		c.tplExec(template, choice{i + 1, result})
	}

	var choice int
	fmt.Printf("Choice [%d-%d]: ", 1, len(results))
	if _, err := fmt.Fscanln(os.Stdin, &choice); err != nil {
		fatalf("Error reading from stdin: %s", err)
	}
	choice--
	if choice == -1 {
		return nil
	} else if choice < -1 || choice >= len(results) {
		fatalf("Invalid choice %d", choice)
	}
	return &results[choice]
}
