// Copyright (c) 2019-2020 Dylan Seidt

// Package godatatables implements a function to handle a DataTables request.
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

// Column represents an SQL column.
// Name is the name of the column in the table.
// Search is how the column should be searched.
// Display is how the column should be rendered.
// Order is how the column should be ordered.
// If search, display, or order are blank, name will be used.
type Column struct {
	Name    string
	Search  string
	Display string
	Order   string
}

// DataTables handles a client DataTables request.
// w is the http.ResponseWriter.
// r is the *http.Request.
// mysqlDb is an instance of *sql.Db
// t is the name of the database table
// additionalWhere is any additional where clause to be applied to the query.
// groupBy is the group by clause.
// Specify columns that should be present in the response.
func DataTables(w http.ResponseWriter, r *http.Request, mysqlDb *sql.DB, t string, additionalWhere string, groupBy string, columns ...Column) {
	db := sqlx.NewDb(mysqlDb, "mysql")

	// Count All Records
	query := "SELECT COUNT(*) FROM " + t

	if groupBy != "" {
		query = "SELECT COUNT(*) FROM (SELECT COUNT(*) FROM " + t
		query += " GROUP BY " + groupBy
	}

	if additionalWhere != "" {
		query += " WHERE " + additionalWhere
	}

	if groupBy != "" {
		query += ") AS count_table"
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
	countFiltered := ""

	if groupBy != "" {
		countFiltered = "SELECT COUNT(*) FROM (SELECT COUNT(*) FROM " + t + " WHERE "
	} else {
		countFiltered = "SELECT COUNT(*) FROM " + t + " WHERE "
	}

	// Select columns
	for i, column := range columns {
		if groupBy == "" {
			if column.Display == "" {
				statement += "IF(ISNULL(" + column.Name + "),\"\"," + column.Name + ")"
			} else {
				statement += "IF(ISNULL(" + column.Display + "),\"\"," + column.Display + ")"
			}
		} else {
			if column.Display == "" {
				statement += column.Name
			} else {
				statement += column.Display
			}
		}

		if i+1 != len(columns) {
			statement += ", "
		}
	}

	statement += " FROM " + t + " WHERE "

	// Append additional where clause
	if additionalWhere != "" {
		statement += "(" + additionalWhere + ") AND ("
		countFiltered += "(" + additionalWhere + ") AND ("
	}

	var searchColumns []Column

	for _, column := range columns {
		if groupBy == "" {
			searchColumns = append(searchColumns, column)
		} else {
			if strings.Contains(groupBy, column.Name) || (strings.Contains(groupBy, column.Search) && column.Search != "") {
				searchColumns = append(searchColumns, column)
			}
		}
	}

	// Search columns
	for i, column := range searchColumns {
		if column.Search == "" {
			statement += column.Name
			countFiltered += column.Name
		} else {
			statement += column.Search
			countFiltered += column.Search
		}

		statement += " LIKE CONCAT('%', :search, '%')"
		countFiltered += " LIKE CONCAT('%', :search, '%')"

		if i+1 != len(searchColumns) {
			statement += " OR "
			countFiltered += " OR "
		}
	}

	// Close additional where clause
	if additionalWhere != "" {
		statement += ")"
		countFiltered += ")"
	}

	search := r.FormValue("search[value]")

	if groupBy != "" {
		countFiltered += " GROUP BY " + groupBy + ") AS count_table"
		statement += " GROUP BY " + groupBy
	}

	// Count Filtered
	rows, err := db.NamedQuery(countFiltered, map[string]interface{}{
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
