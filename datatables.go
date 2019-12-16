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
func DataTables(mysqlDb *sql.DB, t string, columns string, w http.ResponseWriter, r *http.Request) {
	db := sqlx.NewDb(mysqlDb, "mysql")

	// Total records in database
	rows, err := db.Query("SELECT COUNT(*) FROM " + t)

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

	rows, err = db.Query("SELECT COUNT(*) " + filteredQuery)

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

	filteredQuery += " WHERE"

	// Get all records with pagnation
	frows, err := db.NamedQuery("SELECT "+columns+" "+filteredQuery+" "+search, map[string]interface{}{
		"search": r.FormValue("search[value]"),
	})

	if err != nil {
		fmt.Println(err)
	}

	cols, err := frows.Columns()

	if err != nil {
		fmt.Println(err)
	}

	orderColumn, _ := strconv.Atoi(r.FormValue("order[0][column]"))

	var keys []string
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

			keys = append(keys, fmt.Sprintf("%v", m[orderColumn]))
			result = append(result, m)
		}
	}

	natsort.Sort(keys)

	var sorted []interface{}

	for _, key := range keys {
		for _, r := range result {
			if key == fmt.Sprintf("%v", r[orderColumn]) {
				sorted = append(sorted, r)
				break
			}
		}
	}

	if r.FormValue("order[0][dir]") == "desc" {
		for i, j := 0, len(sorted)-1; i < j; i, j = i+1, j-1 {
			sorted[i], sorted[j] = sorted[j], sorted[i]
		}
	}

	length, _ := strconv.Atoi(r.FormValue("length"))
	start, _ := strconv.Atoi(r.FormValue("start"))

	output := make(map[string]interface{})

	output["draw"], _ = strconv.Atoi(r.FormValue("draw"))
	output["recordsTotal"] = total
	output["recordsFiltered"] = filtered
	output["data"] = sorted[start : start+length]

	err = json.NewEncoder(w).Encode(output)
}
