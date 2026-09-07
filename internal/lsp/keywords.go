package lsp

import (
	"fmt"
	"strings"
)

// BuiltinFunctionDef defines a SQL function with its signature and documentation
type BuiltinFunctionDef struct {
	Name        string
	Signature   string
	Description string
	Snippet     string
	Category    string
	Dialect     string // "" for all, "mssql", "postgres", "oracle"
}

// BuiltinKeywordDef defines a SQL keyword
type BuiltinKeywordDef struct {
	Name        string
	Description string
	Dialect     string
}

// BuiltinSnippetDef defines a SQL template snippet
type BuiltinSnippetDef struct {
	Label       string
	InsertText  string
	Detail      string
	Description string
}

var standardKeywords = []BuiltinKeywordDef{
	{Name: "SELECT", Description: "Retrieves rows from one or more tables or views."},
	{Name: "FROM", Description: "Specifies the table, view, or subquery from which to retrieve data."},
	{Name: "WHERE", Description: "Filters rows returned by a query based on specified conditions."},
	{Name: "INSERT INTO", Description: "Adds new rows of data to a specified database table."},
	{Name: "UPDATE", Description: "Modifies existing data in a database table."},
	{Name: "DELETE FROM", Description: "Removes rows from a table based on a condition."},
	{Name: "JOIN", Description: "Combines columns from one or more tables based on related columns."},
	{Name: "INNER JOIN", Description: "Returns rows when there is a match in both joined tables."},
	{Name: "LEFT JOIN", Description: "Returns all rows from the left table and matched rows from the right table."},
	{Name: "RIGHT JOIN", Description: "Returns all rows from the right table and matched rows from the left table."},
	{Name: "FULL OUTER JOIN", Description: "Returns all rows when there is a match in either left or right table."},
	{Name: "CROSS JOIN", Description: "Produces a Cartesian product of rows from both tables."},
	{Name: "ON", Description: "Specifies the condition for joining two tables."},
	{Name: "GROUP BY", Description: "Groups rows sharing a property so aggregate functions can be applied."},
	{Name: "HAVING", Description: "Specifies a search condition for a group or an aggregate."},
	{Name: "ORDER BY", Description: "Sorts the query result set by one or more columns in ASC or DESC order."},
	{Name: "LIMIT", Description: "Constrains the number of rows returned by the query."},
	{Name: "TOP", Description: "Limits the rows returned in a query result set (MS SQL Server / Sybase)."},
	{Name: "OFFSET", Description: "Skips a specified number of rows before beginning to return rows."},
	{Name: "FETCH NEXT", Description: "Specifies the number of rows to return after the OFFSET clause."},
	{Name: "AS", Description: "Assigns an alias name to a table, column, or expression."},
	{Name: "AND", Description: "Combines two conditions; evaluates to TRUE if both conditions are TRUE."},
	{Name: "OR", Description: "Combines two conditions; evaluates to TRUE if either condition is TRUE."},
	{Name: "NOT", Description: "Reverses the value of any other Boolean operator."},
	{Name: "IN", Description: "Determines whether a specified value matches any value in a subquery or list."},
	{Name: "IS NULL", Description: "Tests whether a value is NULL."},
	{Name: "IS NOT NULL", Description: "Tests whether a value is not NULL."},
	{Name: "LIKE", Description: "Determines whether a specific character string matches a specified pattern."},
	{Name: "ILIKE", Description: "Case-insensitive pattern matching (PostgreSQL)."},
	{Name: "BETWEEN", Description: "Specifies an inclusive range to test."},
	{Name: "UNION", Description: "Combines the results of two or more queries into a single result set without duplicates."},
	{Name: "UNION ALL", Description: "Combines the results of two or more queries including all duplicates."},
	{Name: "EXISTS", Description: "Specifies a subquery to test for the existence of rows."},
	{Name: "DISTINCT", Description: "Removes duplicate rows from the query result set."},
	{Name: "CASE", Description: "Evaluates a list of conditions and returns one of multiple possible results."},
	{Name: "WHEN", Description: "Specifies the condition to evaluate in a CASE expression."},
	{Name: "THEN", Description: "Specifies the result to return when the WHEN condition is TRUE."},
	{Name: "ELSE", Description: "Specifies the result to return if no WHEN condition evaluates to TRUE."},
	{Name: "END", Description: "Terminates a CASE expression or block."},
	{Name: "CREATE TABLE", Description: "Creates a new table in the database."},
	{Name: "ALTER TABLE", Description: "Modifies an existing table definition."},
	{Name: "DROP TABLE", Description: "Removes an existing table and its schema from the database."},
	{Name: "TRUNCATE TABLE", Description: "Removes all rows from a table without logging individual row deletions."},
	{Name: "CREATE VIEW", Description: "Creates a virtual table based on the result-set of an SQL statement."},
	{Name: "CREATE INDEX", Description: "Creates a new index on a table to speed up query execution."},
	{Name: "WITH", Description: "Defines a Common Table Expression (CTE) for reuse in queries."},
	{Name: "OVER", Description: "Specifies the partitioning and ordering of a rowset before window functions are applied."},
	{Name: "PARTITION BY", Description: "Divides the query result set into partitions to which the window function is applied."},
	{Name: "VALUES", Description: "Specifies the values to be inserted into a table."},
	{Name: "SET", Description: "Specifies column names and new values in an UPDATE statement."},
}

var standardFunctions = []BuiltinFunctionDef{
	// Aggregate
	{Name: "COUNT", Signature: "COUNT(expression)", Description: "Returns the number of items found in a group.", Snippet: "COUNT(${1:*})", Category: "Aggregate"},
	{Name: "SUM", Signature: "SUM(expression)", Description: "Returns the sum of all the values in an expression.", Snippet: "SUM(${1:column})", Category: "Aggregate"},
	{Name: "AVG", Signature: "AVG(expression)", Description: "Returns the average of the values in a group.", Snippet: "AVG(${1:column})", Category: "Aggregate"},
	{Name: "MIN", Signature: "MIN(expression)", Description: "Returns the minimum value in the expression.", Snippet: "MIN(${1:column})", Category: "Aggregate"},
	{Name: "MAX", Signature: "MAX(expression)", Description: "Returns the maximum value in the expression.", Snippet: "MAX(${1:column})", Category: "Aggregate"},
	{Name: "STRING_AGG", Signature: "STRING_AGG(expression, separator)", Description: "Concatenates the values of string expressions and places separator values between them.", Snippet: "STRING_AGG(${1:column}, ', ')", Category: "Aggregate"},

	// Conditional / Null
	{Name: "COALESCE", Signature: "COALESCE(val1, val2, ...)", Description: "Evaluates the arguments in order and returns the current value of the first expression that initially doesn't evaluate to NULL.", Snippet: "COALESCE(${1:val1}, ${2:val2})", Category: "Conditional"},
	{Name: "NULLIF", Signature: "NULLIF(val1, val2)", Description: "Returns a null value if the two specified expressions are equal.", Snippet: "NULLIF(${1:val1}, ${2:val2})", Category: "Conditional"},
	{Name: "ISNULL", Signature: "ISNULL(check_expression, replacement_value)", Description: "Replaces NULL with the specified replacement value (T-SQL).", Snippet: "ISNULL(${1:column}, ${2:default})", Category: "Conditional", Dialect: "mssql"},
	{Name: "NVL", Signature: "NVL(string1, replace_with)", Description: "Replaces null with a string in the results of a query (Oracle).", Snippet: "NVL(${1:column}, ${2:default})", Category: "Conditional", Dialect: "oracle"},

	// String
	{Name: "CONCAT", Signature: "CONCAT(str1, str2, ...)", Description: "Returns a string that is the result of concatenating two or more string values.", Snippet: "CONCAT(${1:str1}, ' ', ${2:str2})", Category: "String"},
	{Name: "SUBSTRING", Signature: "SUBSTRING(string, start, length)", Description: "Returns part of a character, binary, text, or image expression.", Snippet: "SUBSTRING(${1:str}, ${2:1}, ${3:10})", Category: "String"},
	{Name: "TRIM", Signature: "TRIM(string)", Description: "Removes the space character or other specified characters from the start and end of a string.", Snippet: "TRIM(${1:str})", Category: "String"},
	{Name: "UPPER", Signature: "UPPER(string)", Description: "Returns a character expression with lowercase character data converted to uppercase.", Snippet: "UPPER(${1:str})", Category: "String"},
	{Name: "LOWER", Signature: "LOWER(string)", Description: "Returns a character expression after converting uppercase character data to lowercase.", Snippet: "LOWER(${1:str})", Category: "String"},
	{Name: "REPLACE", Signature: "REPLACE(string, search, replace)", Description: "Replaces all occurrences of a specified string value with another string value.", Snippet: "REPLACE(${1:str}, ${2:search}, ${3:replace})", Category: "String"},
	{Name: "LEN", Signature: "LEN(string)", Description: "Returns the number of characters of the specified string expression.", Snippet: "LEN(${1:str})", Category: "String"},
	{Name: "LENGTH", Signature: "LENGTH(string)", Description: "Returns the number of characters of the string (Postgres/Oracle).", Snippet: "LENGTH(${1:str})", Category: "String"},

	// Date / Time
	{Name: "GETDATE", Signature: "GETDATE()", Description: "Returns the current database system timestamp as a datetime value (T-SQL).", Snippet: "GETDATE()", Category: "DateTime", Dialect: "mssql"},
	{Name: "SYSDATETIME", Signature: "SYSDATETIME()", Description: "Returns a datetime2(7) value that contains the date and time of the computer (T-SQL).", Snippet: "SYSDATETIME()", Category: "DateTime", Dialect: "mssql"},
	{Name: "NOW", Signature: "NOW()", Description: "Returns current date and time with time zone (PostgreSQL).", Snippet: "NOW()", Category: "DateTime", Dialect: "postgres"},
	{Name: "CURRENT_TIMESTAMP", Signature: "CURRENT_TIMESTAMP", Description: "Returns the current date and time.", Snippet: "CURRENT_TIMESTAMP", Category: "DateTime"},
	{Name: "DATEADD", Signature: "DATEADD(datepart, number, date)", Description: "Returns a specified date with the specified number interval added (T-SQL).", Snippet: "DATEADD(day, ${1:1}, ${2:GETDATE()})", Category: "DateTime", Dialect: "mssql"},
	{Name: "DATEDIFF", Signature: "DATEDIFF(datepart, startdate, enddate)", Description: "Returns the count of the specified datepart boundaries crossed between two dates (T-SQL).", Snippet: "DATEDIFF(day, ${1:startdate}, ${2:enddate})", Category: "DateTime", Dialect: "mssql"},

	// Window / Analytic
	{Name: "ROW_NUMBER", Signature: "ROW_NUMBER() OVER (PARTITION BY ... ORDER BY ...)", Description: "Numbers the output of a result set.", Snippet: "ROW_NUMBER() OVER (PARTITION BY ${1:col} ORDER BY ${2:col} ASC)", Category: "Window"},
	{Name: "RANK", Signature: "RANK() OVER (ORDER BY ...)", Description: "Returns the rank of each row within the partition of a result set.", Snippet: "RANK() OVER (ORDER BY ${1:col} DESC)", Category: "Window"},
	{Name: "DENSE_RANK", Signature: "DENSE_RANK() OVER (ORDER BY ...)", Description: "Returns the rank of rows within the partition without gaps in ranking values.", Snippet: "DENSE_RANK() OVER (ORDER BY ${1:col} DESC)", Category: "Window"},
	{Name: "LEAD", Signature: "LEAD(column, offset, default) OVER (ORDER BY ...)", Description: "Accesses data from a subsequent row in the same result set without using a self-join.", Snippet: "LEAD(${1:col}, 1, NULL) OVER (ORDER BY ${2:order_col})", Category: "Window"},
	{Name: "LAG", Signature: "LAG(column, offset, default) OVER (ORDER BY ...)", Description: "Accesses data from a previous row in the same result set without using a self-join.", Snippet: "LAG(${1:col}, 1, NULL) OVER (ORDER BY ${2:order_col})", Category: "Window"},
}

var standardSnippets = []BuiltinSnippetDef{
	{
		Label:       "SELECT * FROM table",
		InsertText:  "SELECT * FROM ",
		Detail:      "SQL Query Template",
		Description: "Simple SELECT query from a table",
	},
	{
		Label:       "SELECT TOP 100 * FROM table",
		InsertText:  "SELECT TOP 100 * FROM ",
		Detail:      "MS SQL Query Template",
		Description: "SELECT query with TOP limit",
	},
	{
		Label:       "INSERT INTO table (cols) VALUES (vals)",
		InsertText:  "INSERT INTO ${1:table} (${2:columns})\nVALUES (${3:values});",
		Detail:      "SQL Insert Template",
		Description: "Insert rows into a database table",
	},
	{
		Label:       "UPDATE table SET col = val WHERE condition",
		InsertText:  "UPDATE ${1:table}\nSET ${2:col} = ${3:val}\nWHERE ${4:condition};",
		Detail:      "SQL Update Template",
		Description: "Update existing rows in a table",
	},
	{
		Label:       "CREATE TABLE ...",
		InsertText:  "CREATE TABLE ${1:table_name} (\n    ${2:id} INT IDENTITY(1,1) PRIMARY KEY,\n    ${3:name} NVARCHAR(100) NOT NULL,\n    created_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME()\n);",
		Detail:      "DDL Table Template",
		Description: "Create a new database table with primary key",
	},
	{
		Label:       "CTE (WITH cte AS (...))",
		InsertText:  "WITH ${1:cte_name} AS (\n    SELECT ${2:*}\n    FROM ${3:table}\n)\nSELECT *\nFROM ${1:cte_name};",
		Detail:      "Common Table Expression",
		Description: "Define and query a CTE",
	},
}

// GetKeywordCompletions returns completion items for SQL keywords
func GetKeywordCompletions(dialect string) []CompletionItem {
	var items []CompletionItem
	dialectLower := strings.ToLower(dialect)

	for _, kw := range standardKeywords {
		if kw.Dialect != "" && dialectLower != "" && !strings.Contains(dialectLower, kw.Dialect) {
			continue
		}
		items = append(items, CompletionItem{
			Label:      kw.Name,
			Kind:       CompletionItemKindKeyword,
			Detail:     "Keyword",
			InsertText: kw.Name,
			SortText:   "9_" + kw.Name,
			Documentation: &MarkupContent{
				Kind:  Markdown,
				Value: "**" + kw.Name + "** *(SQL Keyword)*\n\n" + kw.Description,
			},
		})
	}
	return items
}

// GetFunctionCompletions returns completion items for built-in SQL functions
func GetFunctionCompletions(dialect string) []CompletionItem {
	var items []CompletionItem
	dialectLower := strings.ToLower(dialect)

	for _, fn := range standardFunctions {
		if fn.Dialect != "" && dialectLower != "" && !strings.Contains(dialectLower, fn.Dialect) {
			continue
		}
		items = append(items, CompletionItem{
			Label:      fn.Name,
			Kind:       CompletionItemKindFunction,
			Detail:     fn.Signature,
			InsertText: fn.Name,
			SortText:   "5_" + fn.Name,
			Documentation: &MarkupContent{
				Kind:  Markdown,
				Value: fmt.Sprintf("### %s\n```sql\n%s\n```\n*%s Function*\n\n%s", fn.Name, fn.Signature, fn.Category, fn.Description),
			},
		})
	}
	return items
}

// GetSnippetCompletions returns completion items for code snippets
func GetSnippetCompletions(dialect string) []CompletionItem {
	var items []CompletionItem
	for _, snip := range standardSnippets {
		items = append(items, CompletionItem{
			Label:      snip.Label,
			Kind:       CompletionItemKindSnippet,
			Detail:     snip.Detail,
			InsertText: snip.InsertText,
			SortText:   "8_" + snip.Label,
			Documentation: &MarkupContent{
				Kind:  Markdown,
				Value: fmt.Sprintf("**%s**\n\n%s\n\n```sql\n%s\n```", snip.Label, snip.Description, snip.InsertText),
			},
		})
	}
	return items
}
