package drivers

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/jorgerojas26/lazysql/models"
	"github.com/xo/dburl"

	_ "github.com/lib/pq"
)

type Postgres struct {
	Connection       *sql.DB
	Provider         string
	CurrentDatabase  string
	PreviousDatabase string
	Urlstr           string
}

const (
	DEFAULT_PORT = "5432"
)

func (db *Postgres) Connect(urlstr string) (err error) {
	db.SetProvider("postgres")

	db.Connection, err = dburl.Open(urlstr)
	if err != nil {
		return err
	}

	err = db.Connection.Ping()
	if err != nil {
		return err
	}

	db.Urlstr = urlstr

	// get current database

	rows := db.Connection.QueryRow("SELECT current_database();")

	database := ""

	err = rows.Scan(&database)

	db.CurrentDatabase = database
	db.PreviousDatabase = database
	if err != nil {
		return err
	}

	return nil
}

func (db *Postgres) TestConnection(urlstr string) error {
	return db.Connect(urlstr)
}

func (db *Postgres) GetDatabases() (databases []string, err error) {
	rows, err := db.Connection.Query("SELECT datname FROM pg_database;")
	if err != nil {
		return databases, err
	}

	for rows.Next() {
		var database string
		err := rows.Scan(&database)
		if err != nil {
			return databases, err
		}
		databases = append(databases, database)
	}

	return databases, nil
}

func (db *Postgres) GetTables(database string) (tables map[string][]string, err error) {
	tables = make(map[string][]string)

	switchDatabase := false

	if database != db.CurrentDatabase {
		err = db.SwitchDatabase(database)
		if err != nil {
			return tables, err
		}
		switchDatabase = true
	}

	rows, err := db.Connection.Query(fmt.Sprintf("SELECT table_name, table_schema FROM information_schema.tables WHERE table_catalog = '%s'", database))
	if err != nil {
		if switchDatabase {
			err = db.SwitchDatabase(db.PreviousDatabase)
			if err != nil {
				return tables, err
			}
		}
		return tables, err
	}

	for rows.Next() {
		var tableName string
		var tableSchema string

		rows.Scan(&tableName, &tableSchema)

		tables[tableSchema] = append(tables[tableSchema], tableName)

	}

	return tables, nil
}

func (db *Postgres) GetTableColumns(database, table string) (results [][]string, error error) {
	tableSchema := strings.Split(table, ".")[0]
	tableName := strings.Split(table, ".")[1]
	rows, err := db.Connection.Query(fmt.Sprintf("SELECT column_name, data_type, is_nullable, column_default FROM information_schema.columns WHERE table_catalog = '%s' AND table_schema = '%s' AND table_name = '%s' ORDER by ordinal_position", database, tableSchema, tableName))
	if err != nil {
		return results, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return results, err
	}

	results = append(results, columns)

	for rows.Next() {
		rowValues := make([]interface{}, len(columns))
		for i := range columns {
			rowValues[i] = new(sql.RawBytes)
		}

		rows.Scan(rowValues...)

		var row []string
		for _, col := range rowValues {
			row = append(row, string(*col.(*sql.RawBytes)))
		}

		results = append(results, row)
	}

	return
}

func (db *Postgres) GetConstraints(table string) (constraints [][]string, error error) {
	splitTableString := strings.Split(table, ".")
	tableSchema := splitTableString[0]
	tableName := splitTableString[1]

	rows, err := db.Connection.Query(fmt.Sprintf(`
  SELECT
            tc.constraint_name,
            kcu.column_name,
            tc.constraint_type
        FROM
            information_schema.table_constraints AS tc
            JOIN information_schema.key_column_usage AS kcu ON tc.constraint_name = kcu.constraint_name
            AND tc.table_schema = kcu.table_schema
            JOIN information_schema.constraint_column_usage AS ccu ON ccu.constraint_name = tc.constraint_name
            AND ccu.table_schema = tc.table_schema
        WHERE
            NOT tc.constraint_type = 'FOREIGN KEY'
			AND tc.table_schema = '%s'
            AND tc.table_name = '%s'
            `, tableSchema, tableName))
	if err != nil {
		return constraints, err
	}

	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return constraints, err
	}

	constraints = append(constraints, columns)

	for rows.Next() {
		rowValues := make([]interface{}, len(columns))
		for i := range columns {
			rowValues[i] = new(sql.RawBytes)
		}

		rows.Scan(rowValues...)

		var row []string
		for _, col := range rowValues {
			row = append(row, string(*col.(*sql.RawBytes)))
		}

		constraints = append(constraints, row)
	}

	return
}

func (db *Postgres) GetForeignKeys(table string) (foreignKeys [][]string, error error) {
	splitTableString := strings.Split(table, ".")
	tableSchema := splitTableString[0]
	tableName := splitTableString[1]

	rows, err := db.Connection.Query(fmt.Sprintf(`
  SELECT
            tc.constraint_name,
            kcu.column_name,
            ccu.table_name AS foreign_table_name,
            ccu.column_name AS foreign_column_name
        FROM
            information_schema.table_constraints AS tc
            JOIN information_schema.key_column_usage AS kcu ON tc.constraint_name = kcu.constraint_name
            AND tc.table_schema = kcu.table_schema
            JOIN information_schema.constraint_column_usage AS ccu ON ccu.constraint_name = tc.constraint_name
            AND ccu.table_schema = tc.table_schema
        WHERE
            tc.constraint_type = 'FOREIGN KEY'
          	AND tc.table_schema = '%s'
            AND tc.table_name = '%s'
  `, tableSchema, tableName))
	if err != nil {
		return foreignKeys, err
	}

	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return foreignKeys, err
	}

	foreignKeys = append(foreignKeys, columns)

	for rows.Next() {
		rowValues := make([]interface{}, len(columns))
		for i := range columns {
			rowValues[i] = new(sql.RawBytes)
		}

		rows.Scan(rowValues...)

		var row []string
		for _, col := range rowValues {
			row = append(row, string(*col.(*sql.RawBytes)))
		}

		foreignKeys = append(foreignKeys, row)
	}

	return
}

func (db *Postgres) GetIndexes(table string) (indexes [][]string, error error) {
	splitTableString := strings.Split(table, ".")
	tableSchema := splitTableString[0]
	tableName := splitTableString[1]

	rows, err := db.Connection.Query(fmt.Sprintf(`
  SELECT
            i.relname AS index_name,
            a.attname AS column_name,
            am.amname AS type
        FROM
            pg_namespace n,
            pg_class t,
            pg_class i,
            pg_index ix,
            pg_attribute a,
            pg_am am
        WHERE
            t.oid = ix.indrelid
            and i.oid = ix.indexrelid
            and a.attrelid = t.oid
            and a.attnum = ANY(ix.indkey)
            and t.relkind = 'r'
            and am.oid = i.relam
          	and n.oid = t.relnamespace
            and n.nspname = '%s'
            and t.relname = '%s'
        ORDER BY
            t.relname,
            i.relname
  `, tableSchema, tableName))
	if err != nil {
		return indexes, err
	}
	defer rows.Close()

	columns, _ := rows.Columns()

	indexes = append(indexes, columns)

	for rows.Next() {
		rowValues := make([]interface{}, len(columns))
		for i := range columns {
			rowValues[i] = new(sql.RawBytes)
		}

		rows.Scan(rowValues...)

		var row []string
		for _, col := range rowValues {
			row = append(row, string(*col.(*sql.RawBytes)))
		}

		indexes = append(indexes, row)
	}

	return
}

func (db *Postgres) GetRecords(table, where, sort string, offset, limit int) (records [][]string, totalRecords int, err error) {
	table = db.formatTableName(table)
	defaultLimit := 300
	isPaginationEnabled := offset >= 0 && limit >= 0

	if limit != 0 {
		defaultLimit = limit
	}

	query := fmt.Sprintf("SELECT * FROM %s s LIMIT %d OFFSET %d", table, defaultLimit, offset)

	if where != "" {
		query = fmt.Sprintf("SELECT * FROM %s %s LIMIT %d OFFSET %d", table, where, defaultLimit, offset)
	}

	if sort != "" {
		query = fmt.Sprintf("SELECT * FROM %s %s ORDER BY %s LIMIT %d OFFSET %d", table, where, sort, defaultLimit, offset)
	}

	paginatedRows, err := db.Connection.Query(query)
	if err != nil {
		return records, totalRecords, err
	}

	if isPaginationEnabled {
		queryWithoutLimit := fmt.Sprintf("SELECT COUNT(*) FROM %s %s", table, where)

		rows := db.Connection.QueryRow(queryWithoutLimit)

		if err != nil {
			return records, totalRecords, err
		}

		err = rows.Scan(&totalRecords)
		if err != nil {
			return nil, 0, err
		}

		defer paginatedRows.Close()
	}

	columns, _ := paginatedRows.Columns()

	records = append(records, columns)

	for paginatedRows.Next() {
		rowValues := make([]interface{}, len(columns))
		for i := range columns {
			rowValues[i] = new(sql.RawBytes)
		}

		paginatedRows.Scan(rowValues...)

		var row []string
		for _, col := range rowValues {
			row = append(row, string(*col.(*sql.RawBytes)))
		}

		records = append(records, row)

	}

	return
}

func (db *Postgres) UpdateRecord(table, column, value, primaryKeyColumnName, primaryKeyValue string) (err error) {
	table = db.formatTableName(table)
	query := fmt.Sprintf("UPDATE %s SET %s = '%s' WHERE \"%s\" = '%s'", table, column, value, primaryKeyColumnName, primaryKeyValue)
	_, err = db.Connection.Exec(query)

	return err
}

func (db *Postgres) DeleteRecord(table, primaryKeyColumnName, primaryKeyValue string) (err error) {
	table = db.formatTableName(table)
	query := fmt.Sprintf("DELETE FROM %s WHERE \"%s\" = '%s'", table, primaryKeyColumnName, primaryKeyValue)
	_, err = db.Connection.Exec(query)

	return err
}

func (db *Postgres) ExecuteDMLStatement(query string) (result string, err error) {
	res, err := db.Connection.Exec(query)

	if err != nil {
		return result, err
	} else {
		rowsAffected, _ := res.RowsAffected()

		return fmt.Sprintf("%d rows affected", rowsAffected), err
	}
}

func (db *Postgres) ExecuteQuery(query string) (results [][]string, err error) {
	rows, err := db.Connection.Query(query)
	if err != nil {
		return results, err
	}

	defer rows.Close()

	columns, _ := rows.Columns()

	results = append(results, columns)

	for rows.Next() {
		rowValues := make([]interface{}, len(columns))
		for i := range columns {
			rowValues[i] = new(sql.RawBytes)
		}

		err = rows.Scan(rowValues...)
		if err != nil {
			return nil, err
		}

		var row []string
		for _, col := range rowValues {
			row = append(row, string(*col.(*sql.RawBytes)))
		}

		results = append(results, row)

	}

	return
}

func (db *Postgres) ExecutePendingChanges(changes []models.DbDmlChange) (err error) {
	return nil
}

func (db *Postgres) SetProvider(provider string) {
	db.Provider = provider
}

func (db *Postgres) GetProvider() string {
	return db.Provider
}

func (db *Postgres) SwitchDatabase(database string) error {
	parsedConn, err := dburl.Parse(db.Urlstr)
	if err != nil {
		return err
	}

	user := parsedConn.User.Username()
	password, _ := parsedConn.User.Password()
	host := parsedConn.Host
	port := parsedConn.Port()
	dbname := parsedConn.Path

	if port == "" {
		port = DEFAULT_PORT
	}

	if dbname == "" {
		dbname = database
	}

	connection, err := sql.Open("postgres", fmt.Sprintf("host=%s port=%s user=%s password=%s dbname='%s' sslmode=disable", host, port, user, password, dbname))
	if err != nil {
		return err
	}

	db.Connection.Close()
	db.Connection = connection
	db.PreviousDatabase = db.CurrentDatabase
	db.CurrentDatabase = database

	return nil
}

func (db *Postgres) formatTableName(table string) string {
	splittedTableName := strings.Split(table, ".")

	if len(splittedTableName) == 1 {
		return table
	}

	schema := splittedTableName[0]
	tableName := splittedTableName[1]

	formattedTableName := fmt.Sprintf("\"%s\".\"%s\"", schema, tableName)

	return formattedTableName
}
