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
	splitColumns := strings.Split(columns, ", ")

	for i, column := range splitColumns {
		search += "`" + string(column) + "`" + " LIKE CONCAT(:search, '%')"

		if i != len(splitColumns)-1 {
			search += " OR "
		}
	}

	// Order
	order := ""

	if i, err := strconv.Atoi(r.FormValue("order[0][column]")); err == nil {
		order += "ORDER BY " + splitColumns[i] + " " + r.FormValue("order[0][dir]")
	}

	filteredQuery += " WHERE "

	// Get all records with pagnation
	frows, err := db.NamedQuery("SELECT "+columns+" "+filteredQuery+" "+search+" "+order+" LIMIT :length OFFSET :start", map[string]interface{}{
		"search": r.FormValue("search[value]"),
		"length": r.FormValue("length"),
		"start":  r.FormValue("start"),
	})

	if err != nil {
		fmt.Println(err)
	}

	cols, err := frows.Columns()

	if err != nil {
		fmt.Println(err)
	}

	var result []interface{}

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
				}
			}

			result = append(result, m)
		}
	}

	output := make(map[string]interface{})

	output["draw"], _ = strconv.Atoi(r.FormValue("draw"))
	output["recordsTotal"] = total
	output["recordsFiltered"] = filtered
	output["data"] = result

	err = json.NewEncoder(w).Encode(output)
}
