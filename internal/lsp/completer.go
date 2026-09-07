package lsp

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// SQLClause represents the current SQL context
type SQLClause string

const (
	ClauseSelect   SQLClause = "SELECT"
	ClauseFrom     SQLClause = "FROM"
	ClauseJoin     SQLClause = "JOIN"
	ClauseJoinOn   SQLClause = "JOIN_ON"
	ClauseWhere    SQLClause = "WHERE"
	ClauseGroupBy  SQLClause = "GROUP_BY"
	ClauseOrderBy  SQLClause = "ORDER_BY"
	ClauseHaving   SQLClause = "HAVING"
	ClauseInsert   SQLClause = "INSERT"
	ClauseUpdate   SQLClause = "UPDATE"
	ClauseSet      SQLClause = "SET"
	ClauseGeneral  SQLClause = "GENERAL"
)

// TableReference holds a table/view referenced in the query along with its alias
type TableReference struct {
	Schema string
	Table  string
	Alias  string
}

// Completer computes context-aware SQL suggestions
type Completer struct {
	catalog *Catalog
}

// NewCompleter creates a new Completer
func NewCompleter(catalog *Catalog) *Completer {
	if catalog == nil {
		catalog = NewCatalog()
	}
	return &Completer{catalog: catalog}
}

// Complete returns completion items for a position in the query
func (c *Completer) Complete(sql string, line int, char int) []CompletionItem {
	lines := strings.Split(sql, "\n")
	if line < 0 || line >= len(lines) {
		return c.getGeneralCompletions("")
	}

	lineText := lines[line]
	if char < 0 {
		char = 0
	}
	if char > len(lineText) {
		char = len(lineText)
	}

	textBeforeCursor := lineText[:char]
	allTextBeforeCursor := c.getAllTextBefore(lines, line, char)

	// Extract table aliases across entire query
	tableRefs := c.extractTableReferences(sql)

	// 1. Check for Dot Trigger (e.g. "sales." or "c." or "Customers.")
	dotQualifier, isDot, prefixAfterDot := c.extractDotPrefix(textBeforeCursor)
	if isDot {
		return c.getDotCompletions(dotQualifier, prefixAfterDot, tableRefs)
	}

	// Extract word prefix being typed
	wordPrefix := c.getWordBeforeCursor(textBeforeCursor)

	// 2. Identify active SQL clause
	clause := c.detectClause(allTextBeforeCursor)

	switch clause {
	case ClauseFrom, ClauseJoin, ClauseInsert, ClauseUpdate:
		return c.getFromJoinCompletions(wordPrefix)

	case ClauseJoinOn:
		return c.getJoinOnCompletions(wordPrefix, tableRefs)

	case ClauseSelect, ClauseWhere, ClauseGroupBy, ClauseOrderBy, ClauseHaving, ClauseSet:
		return c.getExpressionCompletions(wordPrefix, tableRefs)

	default:
		return c.getGeneralCompletions(wordPrefix)
	}
}

// Hover returns hover documentation for the token at the cursor position
func (c *Completer) Hover(sql string, line int, char int) *Hover {
	lines := strings.Split(sql, "\n")
	if line < 0 || line >= len(lines) {
		return nil
	}
	lineText := lines[line]
	token := c.getWordAtCursor(lineText, char)
	if token == "" {
		return nil
	}

	tokenLower := strings.ToLower(token)
	tableRefs := c.extractTableReferences(sql)

	// 1. Check if token is an alias
	for _, ref := range tableRefs {
		if strings.EqualFold(ref.Alias, token) {
			fullName := ref.Table
			if ref.Schema != "" {
				fullName = ref.Schema + "." + ref.Table
			}
			tbl := c.catalog.FindTable(ref.Schema, ref.Table)
			content := fmt.Sprintf("### Table Alias: `%s` ➔ `%s`\n\n", ref.Alias, fullName)
			if tbl != nil {
				content += c.formatTableMarkdown(tbl)
			}
			return &Hover{
				Contents: MarkupContent{Kind: Markdown, Value: content},
			}
		}
	}

	// 2. Check if token is a Table or View
	tbl := c.catalog.FindTable("", token)
	if tbl != nil {
		return &Hover{
			Contents: MarkupContent{Kind: Markdown, Value: c.formatTableMarkdown(tbl)},
		}
	}

	// 3. Check if token is a Column
	locations := c.catalog.FindMatchingColumns(tokenLower)
	if len(locations) > 0 {
		var b strings.Builder
		for _, loc := range locations {
			if strings.EqualFold(loc.Column.Name, token) {
				pkBadge := ""
				if loc.Column.IsPrimaryKey {
					pkBadge = " `[PRIMARY KEY]`"
				}
				nullBadge := "NULL"
				if !loc.Column.IsNullable {
					nullBadge = "NOT NULL"
				}
				b.WriteString(fmt.Sprintf("### Column `%s`\n", loc.Column.Name))
				b.WriteString(fmt.Sprintf("- **Table**: `%s.%s`\n", loc.Schema, loc.Table))
				b.WriteString(fmt.Sprintf("- **Type**: `%s` (%s)%s\n\n", loc.Column.DataType, nullBadge, pkBadge))
			}
		}
		if b.Len() > 0 {
			return &Hover{
				Contents: MarkupContent{Kind: Markdown, Value: b.String()},
			}
		}
	}

	// 4. Check if token is a SQL Function
	for _, fn := range standardFunctions {
		if strings.EqualFold(fn.Name, token) {
			return &Hover{
				Contents: MarkupContent{
					Kind:  Markdown,
					Value: fmt.Sprintf("### SQL Function: `%s`\n```sql\n%s\n```\n*%s Function*\n\n%s", fn.Name, fn.Signature, fn.Category, fn.Description),
				},
			}
		}
	}

	// 5. Check if token is a SQL Keyword
	for _, kw := range standardKeywords {
		if strings.EqualFold(kw.Name, token) {
			return &Hover{
				Contents: MarkupContent{
					Kind:  Markdown,
					Value: fmt.Sprintf("### SQL Keyword: `%s`\n\n%s", kw.Name, kw.Description),
				},
			}
		}
	}

	return nil
}

func (c *Completer) formatTableMarkdown(tbl *TableMetadata) string {
	var b strings.Builder
	objType := "TABLE"
	if tbl.IsView {
		objType = "VIEW"
	}
	fullName := tbl.Name
	if tbl.Schema != "" {
		fullName = tbl.Schema + "." + tbl.Name
	}
	b.WriteString(fmt.Sprintf("### %s `%s`\n", objType, fullName))
	b.WriteString(fmt.Sprintf("- **Database**: `%s`\n", tbl.Database))
	if tbl.Schema != "" {
		b.WriteString(fmt.Sprintf("- **Schema**: `%s`\n", tbl.Schema))
	}
	b.WriteString(fmt.Sprintf("- **Columns**: %d\n\n", len(tbl.Columns)))

	if len(tbl.Columns) > 0 {
		b.WriteString("| Column | Type | Nullable | Key |\n")
		b.WriteString("| :--- | :--- | :--- | :--- |\n")
		for _, col := range tbl.Columns {
			pk := ""
			if col.IsPrimaryKey {
				pk = "🔑 PK"
			}
			nullStr := "YES"
			if !col.IsNullable {
				nullStr = "NO"
			}
			b.WriteString(fmt.Sprintf("| `%s` | `%s` | %s | %s |\n", col.Name, col.DataType, nullStr, pk))
		}
	}

	return b.String()
}

// extractDotPrefix checks if cursor follows a dot (e.g. "c." or "sales.Cus")
func (c *Completer) extractDotPrefix(text string) (qualifier string, isDot bool, prefixAfterDot string) {
	if text == "" {
		return "", false, ""
	}

	// Find last dot
	lastDot := strings.LastIndex(text, ".")
	if lastDot == -1 {
		return "", false, ""
	}

	// Check what follows the dot
	afterDot := text[lastDot+1:]
	for _, r := range afterDot {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != '#' && r != '$' {
			return "", false, ""
		}
	}

	// Extract qualifier before dot
	beforeDot := strings.TrimRight(text[:lastDot], " \t\r\n")
	if beforeDot == "" {
		return "", false, ""
	}

	// Get word before dot
	qualifier = c.getWordBeforeCursor(beforeDot)
	if qualifier == "" {
		// If wrapped in brackets e.g. [sales]
		if strings.HasSuffix(beforeDot, "]") {
			startBracket := strings.LastIndex(beforeDot, "[")
			if startBracket != -1 {
				qualifier = beforeDot[startBracket+1 : len(beforeDot)-1]
			}
		}
	}

	return qualifier, true, afterDot
}

// getDotCompletions resolves completions following a dot (e.g. "sales." or "c.")
func (c *Completer) getDotCompletions(qualifier, prefix string, tableRefs []TableReference) []CompletionItem {
	var items []CompletionItem
	qualifierLower := strings.ToLower(qualifier)
	prefixLower := strings.ToLower(prefix)

	// A. Check if qualifier is an alias in query (e.g. "c" -> sales.Customers)
	var resolvedTable *TableMetadata
	for _, ref := range tableRefs {
		if strings.EqualFold(ref.Alias, qualifier) {
			resolvedTable = c.catalog.FindTable(ref.Schema, ref.Table)
			break
		}
	}

	// B. Check if qualifier is a direct table name
	if resolvedTable == nil {
		resolvedTable = c.catalog.FindTable("", qualifier)
	}

	// If resolved to a table: return columns of this table
	if resolvedTable != nil {
		for _, col := range resolvedTable.Columns {
			if prefixLower != "" && !strings.HasPrefix(strings.ToLower(col.Name), prefixLower) && !strings.Contains(strings.ToLower(col.Name), prefixLower) {
				continue
			}

			pkBadge := ""
			if col.IsPrimaryKey {
				pkBadge = " 🔑"
			}
			nullStr := "NULL"
			if !col.IsNullable {
				nullStr = "NOT NULL"
			}

			items = append(items, CompletionItem{
				Label:      col.Name,
				Kind:       CompletionItemKindField,
				Detail:     fmt.Sprintf("%s (%s)%s", col.DataType, nullStr, pkBadge),
				InsertText: col.Name,
				SortText:   "0_" + col.Name,
				Documentation: &MarkupContent{
					Kind: Markdown,
					Value: fmt.Sprintf("**%s**\n- Table: `%s.%s`\n- Type: `%s`\n- Nullable: `%t`\n- Primary Key: `%t`",
						col.Name, resolvedTable.Schema, resolvedTable.Name, col.DataType, col.IsNullable, col.IsPrimaryKey),
				},
			})
		}
		return items
	}

	// C. Check if qualifier is a Schema name (e.g. "sales" -> return tables in sales)
	schemaTables := c.catalog.GetTablesForSchema(qualifierLower)
	if len(schemaTables) > 0 {
		for _, tbl := range schemaTables {
			if prefixLower != "" && !strings.HasPrefix(strings.ToLower(tbl.Name), prefixLower) && !strings.Contains(strings.ToLower(tbl.Name), prefixLower) {
				continue
			}

			kind := CompletionItemKindClass
			detail := "Table"
			if tbl.IsView {
				kind = CompletionItemKindInterface
				detail = "View"
			}

			items = append(items, CompletionItem{
				Label:      tbl.Name,
				Kind:       kind,
				Detail:     fmt.Sprintf("%s (%s)", detail, tbl.Schema),
				InsertText: tbl.Name,
				SortText:   "1_" + tbl.Name,
				Documentation: &MarkupContent{
					Kind:  Markdown,
					Value: c.formatTableMarkdown(&tbl),
				},
			})
		}
		return items
	}

	return items
}

// getFromJoinCompletions suggests schemas, tables, and views for FROM / JOIN / INTO clauses
func (c *Completer) getFromJoinCompletions(prefix string) []CompletionItem {
	var items []CompletionItem
	prefixLower := strings.ToLower(prefix)

	// 1. Schemas
	for _, schema := range c.catalog.GetAllSchemas() {
		if prefixLower != "" && !strings.HasPrefix(strings.ToLower(schema), prefixLower) {
			continue
		}
		items = append(items, CompletionItem{
			Label:      schema,
			Kind:       CompletionItemKindModule,
			Detail:     "Schema",
			InsertText: schema,
			SortText:   "1_" + schema,
			Documentation: &MarkupContent{
				Kind:  Markdown,
				Value: fmt.Sprintf("**Schema**: `%s`", schema),
			},
		})
	}

	// 2. Tables and Views (with full schema.table and table)
	for _, tbl := range c.catalog.GetAllTables() {
		fullName := tbl.FullName()
		nameLower := strings.ToLower(tbl.Name)
		fullLower := strings.ToLower(fullName)

		if prefixLower != "" && !strings.HasPrefix(nameLower, prefixLower) && !strings.HasPrefix(fullLower, prefixLower) && !strings.Contains(fullLower, prefixLower) {
			continue
		}

		kind := CompletionItemKindClass
		detail := "Table"
		if tbl.IsView {
			kind = CompletionItemKindInterface
			detail = "View"
		}

		items = append(items, CompletionItem{
			Label:      fullName,
			Kind:       kind,
			Detail:     fmt.Sprintf("%s (%s)", detail, tbl.Database),
			InsertText: fullName,
			SortText:   "2_" + fullName,
			Documentation: &MarkupContent{
				Kind:  Markdown,
				Value: c.formatTableMarkdown(&tbl),
			},
		})
	}

	// 3. Relevant keywords for FROM / JOIN
	fromKeywords := []string{"JOIN", "LEFT JOIN", "RIGHT JOIN", "INNER JOIN", "CROSS JOIN", "AS", "WHERE", "ON"}
	for _, kw := range fromKeywords {
		if prefixLower != "" && !strings.HasPrefix(strings.ToLower(kw), prefixLower) {
			continue
		}
		items = append(items, CompletionItem{
			Label:      kw,
			Kind:       CompletionItemKindKeyword,
			Detail:     "Keyword",
			InsertText: kw,
			SortText:   "8_" + kw,
		})
	}

	return items
}

// getJoinOnCompletions generates smart join predicates for "JOIN ... ON"
func (c *Completer) getJoinOnCompletions(prefix string, tableRefs []TableReference) []CompletionItem {
	var items []CompletionItem

	// If we have at least 2 tables referenced in query, find common columns or keys
	if len(tableRefs) >= 2 {
		lastRef := tableRefs[len(tableRefs)-1] // Joined table
		otherRefs := tableRefs[:len(tableRefs)-1]

		joinedTable := c.catalog.FindTable(lastRef.Schema, lastRef.Table)
		if joinedTable != nil {
			for _, otherRef := range otherRefs {
				otherTable := c.catalog.FindTable(otherRef.Schema, otherRef.Table)
				if otherTable == nil {
					continue
				}

				// Find matching column names
				for _, jCol := range joinedTable.Columns {
					for _, oCol := range otherTable.Columns {
						if strings.EqualFold(jCol.Name, oCol.Name) {
							alias1 := otherRef.Alias
							if alias1 == "" {
								alias1 = otherRef.Table
							}
							alias2 := lastRef.Alias
							if alias2 == "" {
								alias2 = lastRef.Table
							}

							predicate := fmt.Sprintf("%s.%s = %s.%s", alias1, oCol.Name, alias2, jCol.Name)
							items = append(items, CompletionItem{
								Label:      predicate,
								Kind:       CompletionItemKindSnippet,
								Detail:     "Join Condition",
								InsertText: predicate,
								SortText:   "0_" + predicate,
								Documentation: &MarkupContent{
									Kind: Markdown,
									Value: fmt.Sprintf("Auto Join Predicate on common column `%s`:\n```sql\nON %s\n```",
										jCol.Name, predicate),
								},
							})
						}
					}
				}
			}
		}
	}

	// Also add columns from all referenced tables
	exprItems := c.getExpressionCompletions(prefix, tableRefs)
	items = append(items, exprItems...)

	return items
}

// getExpressionCompletions returns columns, functions, and keywords for SELECT/WHERE/ORDER BY
func (c *Completer) getExpressionCompletions(prefix string, tableRefs []TableReference) []CompletionItem {
	var items []CompletionItem
	prefixLower := strings.ToLower(prefix)
	seenColumns := make(map[string]bool)

	// 1. Columns from referenced tables in query (PRIORITY 0)
	for _, ref := range tableRefs {
		tbl := c.catalog.FindTable(ref.Schema, ref.Table)
		if tbl == nil {
			continue
		}

		aliasPrefix := ""
		if ref.Alias != "" {
			aliasPrefix = ref.Alias + "."
		}

		for _, col := range tbl.Columns {
			colLower := strings.ToLower(col.Name)
			if prefixLower != "" && !strings.HasPrefix(colLower, prefixLower) && !strings.Contains(colLower, prefixLower) {
				continue
			}

			pkBadge := ""
			if col.IsPrimaryKey {
				pkBadge = " 🔑"
			}
			nullStr := "NULL"
			if !col.IsNullable {
				nullStr = "NOT NULL"
			}

			// Plain column completion
			if !seenColumns[colLower] {
				seenColumns[colLower] = true
				items = append(items, CompletionItem{
					Label:      col.Name,
					Kind:       CompletionItemKindField,
					Detail:     fmt.Sprintf("%s (%s)%s - %s", col.DataType, nullStr, pkBadge, tbl.Name),
					InsertText: col.Name,
					SortText:   "0_" + col.Name,
					Documentation: &MarkupContent{
						Kind: Markdown,
						Value: fmt.Sprintf("**%s**\n- Table: `%s`\n- Type: `%s`\n- Nullable: `%t`\n- Primary Key: `%t`",
							col.Name, tbl.FullName(), col.DataType, col.IsNullable, col.IsPrimaryKey),
					},
				})
			}

			// Aliased column completion (e.g. "c.CustomerID") if alias exists
			if aliasPrefix != "" {
				aliasedName := aliasPrefix + col.Name
				if prefixLower != "" && !strings.HasPrefix(strings.ToLower(aliasedName), prefixLower) {
					continue
				}
				items = append(items, CompletionItem{
					Label:      aliasedName,
					Kind:       CompletionItemKindField,
					Detail:     fmt.Sprintf("%s - %s", col.DataType, tbl.Name),
					InsertText: aliasedName,
					SortText:   "1_" + aliasedName,
				})
			}
		}
	}

	// 2. All other columns from catalog
	for colName, locations := range c.catalog.AllColumnMap {
		if seenColumns[colName] {
			continue
		}
		if prefixLower != "" && !strings.HasPrefix(colName, prefixLower) && !strings.Contains(colName, prefixLower) {
			continue
		}
		if len(locations) > 0 {
			loc := locations[0]
			items = append(items, CompletionItem{
				Label:      loc.Column.Name,
				Kind:       CompletionItemKindField,
				Detail:     fmt.Sprintf("%s - %s", loc.Column.DataType, loc.Table),
				InsertText: loc.Column.Name,
				SortText:   "3_" + loc.Column.Name,
			})
			seenColumns[colName] = true
		}
	}

	// 3. SQL Functions
	for _, fn := range GetFunctionCompletions(c.catalog.Dialect) {
		if prefixLower != "" && !strings.HasPrefix(strings.ToLower(fn.Label), prefixLower) {
			continue
		}
		items = append(items, fn)
	}

	// 4. SQL Keywords & Expressions
	for _, kw := range GetKeywordCompletions(c.catalog.Dialect) {
		if prefixLower != "" && !strings.HasPrefix(strings.ToLower(kw.Label), prefixLower) {
			continue
		}
		items = append(items, kw)
	}

	return items
}

// getGeneralCompletions returns global keywords, snippets, and tables
func (c *Completer) getGeneralCompletions(prefix string) []CompletionItem {
	var items []CompletionItem
	prefixLower := strings.ToLower(prefix)

	// Snippets
	for _, snip := range GetSnippetCompletions(c.catalog.Dialect) {
		if prefixLower != "" && !strings.HasPrefix(strings.ToLower(snip.Label), prefixLower) {
			continue
		}
		items = append(items, snip)
	}

	// Tables
	for _, tbl := range c.catalog.GetAllTables() {
		name := tbl.FullName()
		if prefixLower != "" && !strings.HasPrefix(strings.ToLower(name), prefixLower) {
			continue
		}
		items = append(items, CompletionItem{
			Label:      name,
			Kind:       CompletionItemKindClass,
			Detail:     "Table",
			InsertText: name,
			SortText:   "4_" + name,
		})
	}

	// Keywords
	for _, kw := range GetKeywordCompletions(c.catalog.Dialect) {
		if prefixLower != "" && !strings.HasPrefix(strings.ToLower(kw.Label), prefixLower) {
			continue
		}
		items = append(items, kw)
	}

	// Functions
	for _, fn := range GetFunctionCompletions(c.catalog.Dialect) {
		if prefixLower != "" && !strings.HasPrefix(strings.ToLower(fn.Label), prefixLower) {
			continue
		}
		items = append(items, fn)
	}

	return items
}

// extractTableReferences parses the SQL query to discover all tables and their aliases
// Supports formats like:
// FROM sales.Customers c
// FROM [sales].[Customers] AS c
// JOIN inventory.Products p ON ...
// LEFT JOIN Orders AS o ON ...
func (c *Completer) extractTableReferences(sql string) []TableReference {
	var refs []TableReference
	cleanSQL := removeCommentsAndStrings(sql)

	// Regex for FROM / JOIN table patterns
	// Patterns:
	// FROM/JOIN [schema].[table] [AS] alias
	// FROM/JOIN schema.table [AS] alias
	// FROM/JOIN table [AS] alias
	pattern := `(?i)\b(FROM|JOIN|INTO|UPDATE)\s+([a-zA-Z0-9_\[\]]+(?:\.[a-zA-Z0-9_\[\]]+)?)(?:\s+(?:AS\s+)?([a-zA-Z0-9_]+))?`
	re := regexp.MustCompile(pattern)
	matches := re.FindAllStringSubmatch(cleanSQL, -1)

	for _, m := range matches {
		if len(m) < 3 {
			continue
		}

		rawTable := m[2]
		rawAlias := ""
		if len(m) >= 4 {
			rawAlias = m[3]
		}

		// Disallow SQL keywords as table alias
		if strings.EqualFold(rawAlias, "ON") || strings.EqualFold(rawAlias, "WHERE") ||
			strings.EqualFold(rawAlias, "JOIN") || strings.EqualFold(rawAlias, "INNER") ||
			strings.EqualFold(rawAlias, "LEFT") || strings.EqualFold(rawAlias, "RIGHT") ||
			strings.EqualFold(rawAlias, "GROUP") || strings.EqualFold(rawAlias, "ORDER") ||
			strings.EqualFold(rawAlias, "SET") || strings.EqualFold(rawAlias, "VALUES") {
			rawAlias = ""
		}

		// Clean brackets from table name
		cleanTable := strings.ReplaceAll(strings.ReplaceAll(rawTable, "[", ""), "]", "")
		parts := strings.Split(cleanTable, ".")

		ref := TableReference{
			Alias: rawAlias,
		}

		if len(parts) == 1 {
			ref.Table = parts[0]
		} else if len(parts) >= 2 {
			ref.Schema = parts[0]
			ref.Table = parts[1]
		}

		refs = append(refs, ref)
	}

	return refs
}

// detectClause determines the active clause by looking backwards from cursor
func (c *Completer) detectClause(textBeforeCursor string) SQLClause {
	tokens := tokenizeSQLWords(textBeforeCursor)
	if len(tokens) == 0 {
		return ClauseGeneral
	}

	for i := len(tokens) - 1; i >= 0; i-- {
		upper := strings.ToUpper(tokens[i])
		switch upper {
		case "ON":
			return ClauseJoinOn
		case "JOIN", "INNER", "LEFT", "RIGHT", "FULL", "CROSS":
			return ClauseJoin
		case "FROM":
			return ClauseFrom
		case "SELECT":
			return ClauseSelect
		case "WHERE":
			return ClauseWhere
		case "GROUP":
			if i+1 < len(tokens) && strings.EqualFold(tokens[i+1], "BY") {
				return ClauseGroupBy
			}
			return ClauseGroupBy
		case "ORDER":
			if i+1 < len(tokens) && strings.EqualFold(tokens[i+1], "BY") {
				return ClauseOrderBy
			}
			return ClauseOrderBy
		case "HAVING":
			return ClauseHaving
		case "SET":
			return ClauseSet
		case "INTO":
			return ClauseInsert
		case "UPDATE":
			return ClauseUpdate
		}
	}

	return ClauseGeneral
}

func (c *Completer) getWordBeforeCursor(text string) string {
	i := len(text) - 1
	for i >= 0 {
		r := rune(text[i])
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != '#' && r != '$' {
			break
		}
		i--
	}
	return text[i+1:]
}

func (c *Completer) getWordAtCursor(text string, char int) string {
	if char > len(text) {
		char = len(text)
	}
	runes := []rune(text)
	if len(runes) == 0 {
		return ""
	}

	start := char
	for start > 0 && (unicode.IsLetter(runes[start-1]) || unicode.IsDigit(runes[start-1]) || runes[start-1] == '_' || runes[start-1] == '.') {
		start--
	}

	end := char
	for end < len(runes) && (unicode.IsLetter(runes[end]) || unicode.IsDigit(runes[end]) || runes[end] == '_' || runes[end] == '.') {
		end++
	}

	if start < end {
		return string(runes[start:end])
	}
	return ""
}

func (c *Completer) getAllTextBefore(lines []string, line, char int) string {
	var b strings.Builder
	for i := 0; i < line; i++ {
		b.WriteString(lines[i])
		b.WriteString(" \n")
	}
	if line < len(lines) {
		if char > len(lines[line]) {
			char = len(lines[line])
		}
		b.WriteString(lines[line][:char])
	}
	return b.String()
}

func tokenizeSQLWords(sql string) []string {
	var tokens []string
	clean := removeCommentsAndStrings(sql)
	words := strings.FieldsFunc(clean, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_'
	})
	for _, w := range words {
		if strings.TrimSpace(w) != "" {
			tokens = append(tokens, w)
		}
	}
	return tokens
}

func removeCommentsAndStrings(sql string) string {
	var b strings.Builder
	runes := []rune(sql)
	n := len(runes)
	i := 0

	for i < n {
		if runes[i] == '-' && i+1 < n && runes[i+1] == '-' {
			for i < n && runes[i] != '\n' {
				i++
			}
			continue
		}
		if runes[i] == '/' && i+1 < n && runes[i+1] == '*' {
			i += 2
			for i+1 < n && !(runes[i] == '*' && runes[i+1] == '/') {
				i++
			}
			if i+1 < n {
				i += 2
			}
			continue
		}
		if runes[i] == '\'' {
			i++
			for i < n && runes[i] != '\'' {
				i++
			}
			if i < n {
				i++
			}
			continue
		}
		b.WriteRune(runes[i])
		i++
	}
	return b.String()
}
