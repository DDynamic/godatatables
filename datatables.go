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

	"github.com/jmoiron/sqlx"
)

// Column represents an SQL column. Search is the value that should be searched in the column. Name is the exact name of the column. Display is how the column should be mutated. Order is how the column should be ordered.
type Column struct {
	Name    string
	Search  string
	Display string
	Order   string
}

// DataTables is the primary rendering function. t is the table name, columns is a comma separated list of database columns.
// Columns are determined by a comma-space. If using functions, do not put a space between parameters.
func DataTables(w http.ResponseWriter, r *http.Request, mysqlDb *sql.DB, t string, additionalWhere string, columns ...Column) {
	db := sqlx.NewDb(mysqlDb, "mysql")

	// Count All Records
	query := "SELECT COUNT(*) FROM " + t

	if additionalWhere != "" {
		query += " WHERE " + additionalWhere
	}

	arows, err := db.Query(query)

	if err != nil {
		log.Fatal(err)
	}

	defer arows.Close()

	var total int

	for arows.Next() {
		if err := arows.Scan(&total); err != nil {
			log.Fatal(err)
		}
	}

	statement := "SELECT "

	// Select columns
	for i, column := range columns {
		statement += column.Display

		if i+1 != len(columns) {
			statement += ", "
		}
	}

	statement += " FROM " + t + " WHERE "

	// Append additional where clause
	if additionalWhere != "" {
		statement += additionalWhere + " AND ("
	}

	// Search columns
	for i, column := range columns {
		if column.Search == "" {
			statement += column.Name
		} else {
			statement += column.Search
		}

		statement += " LIKE CONCAT('%', :search, '%')"

		if i+1 != len(columns) {
			statement += " OR "
		}
	}

	// Close additional where clause
	if additionalWhere != "" {
		statement += ")"
	}

	search := r.FormValue("search[value]")

	// Count Filtered
	rows, err := db.NamedQuery("SELECT COUNT(*) "+"FROM "+strings.Split(statement, "FROM ")[1], map[string]interface{}{
		"search": search,
	})

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

	// Order
	orderColumnNumber, _ := strconv.Atoi(r.FormValue("order[0][column]"))
	orderColumn := columns[orderColumnNumber]
	name := ""

	if orderColumn.Order == "" {
		name = orderColumn.Name
	} else {
		name = orderColumn.Order
	}

	statement += " ORDER BY " + name + " " + r.FormValue("order[0][dir]")

	start := r.FormValue("start")
	length := r.FormValue("length")

	if length != "-1" {
		statement += " LIMIT :length OFFSET :start"
	}

	rows, err = db.NamedQuery(statement, map[string]interface{}{
		"search": search,
		"length": length,
		"start":  start,
	})

	if err != nil {
		fmt.Println(err)
	}

	cols, err := rows.Columns()

	if err != nil {
		fmt.Println(err)
	}

	var result [][]interface{}

	for rows.Next() {
		columns := make([]interface{}, len(cols))
		columnPointers := make([]interface{}, len(cols))

		for i := range columns {
			columnPointers[i] = &columns[i]
		}

		if err := rows.Scan(columnPointers...); err == nil {
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

	output := make(map[string]interface{})

	output["draw"], _ = strconv.Atoi(r.FormValue("draw"))
	output["recordsTotal"] = total
	output["recordsFiltered"] = filtered

	if len(result) == 0 {
		output["data"] = 0
	} else {
		output["data"] = result
	}

	json.NewEncoder(w).Encode(output)
}
