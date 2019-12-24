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
	rows, err := db.NamedQuery("SELECT COUNT(*) FROM "+t, map[string]interface{}{
		"search": r.FormValue("search[value]"),
	})

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

	// Count Filtered
	rows, err = db.NamedQuery("SELECT COUNT(*) "+"FROM "+strings.Split(statement, "FROM ")[1], map[string]interface{}{
		"search": r.FormValue("search[value]"),
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
		"search": r.FormValue("search[value]"),
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

/*
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

	// Search
	search := ""
	checkColumns := strings.Split(columns, ", ")

	for i, column := range checkColumns {
		search += string(column) + " LIKE CONCAT('%',:search,'%')"

		if i != len(checkColumns)-1 {
			search += " OR "
		}
	}

	// Total records filtered
	filteredQuery := "FROM " + t

	if additionalWhere != "" {
		if search == "" {
			filteredQuery += additionalWhere
		} else {
			filteredQuery += additionalWhere + " AND ("
			search += ")"
		}
	} else {
		if search != "" {
			filteredQuery += " WHERE"
		}
	}

	frows, err := db.NamedQuery("SELECT COUNT(*) "+filteredQuery+" "+search, map[string]interface{}{
		"search": r.FormValue("search[value]"),
	})

	if err != nil {
		log.Fatal(err)
	}

	defer frows.Close()

	var filtered int

	for frows.Next() {
		if err := frows.Scan(&filtered); err != nil {
			log.Fatal(err)
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

	start, _ := strconv.Atoi(r.FormValue("start"))
	length, _ := strconv.Atoi(r.FormValue("length"))

	if length == -1 {
		length = total
	}

	// Get all records with pagnation
	frows, err = db.NamedQuery("SELECT "+columns+" "+filteredQuery+" "+search+" "+order+" "+limit, map[string]interface{}{
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

	var final [][]interface{}

	if naturalSort {
		var keys []string

		for _, r := range result {
			key := strings.ReplaceAll(fmt.Sprintf("%v", r[orderColumn]), ",", "")
			keys = append(keys, key)
		}

		natsort.Sort(keys)

		for _, key := range keys {
			for i, r := range result {
				if key == strings.ReplaceAll(fmt.Sprintf("%v", r[orderColumn]), ",", "") {
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
		if len(result) > 0 {
			final = result
		}
	}

	output := make(map[string]interface{})

	output["draw"], _ = strconv.Atoi(r.FormValue("draw"))
	output["recordsTotal"] = total
	output["recordsFiltered"] = filtered

	if len(final) == 0 {
		output["data"] = 0
	} else {
		output["data"] = final
	}

	json.NewEncoder(w).Encode(output)
}
*/
