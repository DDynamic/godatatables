// Copyright (c) 2019 Dylan Seidt

// Package godatatables contains the main DataTables function
package godatatables

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"facette.io/natsort"
	"github.com/jmoiron/sqlx"
)

// DataTables is the primary rendering function. t is the table name, columns is a comma separated list of database columns.
// Columns are determined by a comma-space. If using functions, do not put a space between parameters.
func DataTables(mysqlDb *sql.DB, t string, columns string, naturalSort bool, additionalWhere string, w http.ResponseWriter, r *http.Request) {
	db := sqlx.NewDb(mysqlDb, "mysql")

	if additionalWhere != "" {
		additionalWhere = " WHERE " + additionalWhere
	}

	// Total records in database
	rows, err := db.Query("SELECT COUNT(*) FROM " + t + additionalWhere)

	if err != nil {
		log.Fatal(err)
	}

	defer rows.Close()

	var total int

	for rows.Next() {
		if err := rows.Scan(&total); err != nil {
			log.Fatal(err)
		}
	}

	// Total records filtered
	filteredQuery := "FROM " + t

	rows, err = db.Query("SELECT COUNT(*) " + filteredQuery + additionalWhere)

	if err != nil {
		log.Fatal(err)
	}

	defer rows.Close()

	var filtered int

	for rows.Next() {
		if err := rows.Scan(&filtered); err != nil {
			log.Fatal(err)
		}
	}

	// Search
	search := ""
	checkColumns := strings.Split(columns, ", ")
	var splitColumns []string

	for _, column := range checkColumns {
		if !strings.Contains(column, "(") {
			splitColumns = append(splitColumns, column)
		}
	}

	for i, column := range splitColumns {
		search += string(column) + " LIKE CONCAT(:search, '%')"

		if i != len(splitColumns)-1 {
			search += " OR "
		}
	}

	orderColumn, _ := strconv.Atoi(r.FormValue("order[0][column]"))

	order := ""
	limit := ""

	// Order and Limit
	if !naturalSort {
		order += "ORDER BY " + checkColumns[orderColumn] + " " + r.FormValue("order[0][dir]")
		limit += "LIMIT :length OFFSET :start"
	}

	if additionalWhere != "" {
		filteredQuery += additionalWhere + " AND "
	} else {
		filteredQuery += " WHERE"
	}

	start, _ := strconv.Atoi(r.FormValue("start"))
	length, _ := strconv.Atoi(r.FormValue("length"))

	// Get all records with pagnation
	frows, err := db.NamedQuery("SELECT "+columns+" "+filteredQuery+" "+search+" "+order+" "+limit, map[string]interface{}{
		"search": r.FormValue("search[value]"),
		"length": length,
		"start":  start,
	})

	if err != nil {
		fmt.Println(err)
	}

	cols, err := frows.Columns()

	if err != nil {
		fmt.Println(err)
	}

	var result [][]interface{}

	for frows.Next() {
		columns := make([]interface{}, len(cols))
		columnPointers := make([]interface{}, len(cols))

		for i := range columns {
			columnPointers[i] = &columns[i]
		}

		if err := frows.Scan(columnPointers...); err == nil {
			var m []interface{}

			for i := range cols {
				val := columnPointers[i].(*interface{})
				value := *val

				switch value.(type) {
				case []uint8:
					m = append(m, string(value.([]uint8)))
				case int64:
					m = append(m, value.(int64))
				case float64:
					m = append(m, value.(float64))
				case time.Time:
					m = append(m, value.(time.Time))
				}
			}

			result = append(result, m)
		}
	}

	final := make([][]interface{}, 0)

	if naturalSort {
		var keys []string

		for _, r := range result {
			keys = append(keys, fmt.Sprintf("%v", r[orderColumn]))
		}

		natsort.Sort(keys)

		for _, key := range keys {
			for i, r := range result {
				if key == fmt.Sprintf("%v", r[orderColumn]) {
					final = append(final, r)
					result[len(result)-1], result[i] = result[i], result[len(result)-1]
					result = result[:len(result)-1]
					break
				}
			}
		}

		if r.FormValue("order[0][dir]") == "desc" {
			for i, j := 0, len(final)-1; i < j; i, j = i+1, j-1 {
				final[i], final[j] = final[j], final[i]
			}
		}

		if len(final) != 0 && len(final) > start+length-1 {
			final = final[start : start+length]
		}
	} else {
		final = result
	}

	output := make(map[string]interface{})

	output["draw"], _ = strconv.Atoi(r.FormValue("draw"))
	output["recordsTotal"] = total
	output["recordsFiltered"] = filtered
	output["data"] = final

	json.NewEncoder(w).Encode(output)
}
