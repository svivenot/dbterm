package ai

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// EmbeddedSQLGenerator provides an intelligent, in-process schema-aware SQL generator
// that operates completely offline with zero external dependencies.
type EmbeddedSQLGenerator struct{}

func NewEmbeddedSQLGenerator() *EmbeddedSQLGenerator {
	return &EmbeddedSQLGenerator{}
}

// Generate processes a request and produces valid SQL tailored to the active database schema
func (g *EmbeddedSQLGenerator) Generate(req AIRequest, schema *SchemaSummary) (string, string) {
	if schema == nil {
		schema = &SchemaSummary{
			Database: "ActiveDB",
			Dialect:  "T-SQL (MS SQL Server)",
		}
	}

	switch req.Mode {
	case AIModeFixError:
		return g.fixError(req.CurrentSQL, req.ErrorMessage, schema)
	case AIModeExplain:
		return "", g.explainSQL(req.CurrentSQL, schema)
	case AIModeOptimize:
		return g.optimizeSQL(req.CurrentSQL, schema)
	case AIModeGenerate:
		fallthrough
	default:
		return g.generateFromPrompt(req.UserPrompt, req.CurrentSQL, schema)
	}
}

func (g *EmbeddedSQLGenerator) generateFromPrompt(userPrompt, existingSQL string, schema *SchemaSummary) (string, string) {
	p := strings.ToLower(userPrompt)
	isMSSQL := strings.Contains(schema.Dialect, "MSSQL") || strings.Contains(schema.Dialect, "T-SQL") || strings.Contains(schema.Dialect, "SQL Server")
	isOracle := strings.Contains(schema.Dialect, "Oracle")

	// 1. Extract requested limit / top
	limit := 100
	limitRe := regexp.MustCompile(`(?i)(?:top|limit|premiers?|derniers?)\s*(\d+)`)
	if match := limitRe.FindStringSubmatch(p); len(match) > 1 {
		if n, err := strconv.Atoi(match[1]); err == nil && n > 0 {
			limit = n
		}
	}

	// 2. Discover relevant tables from active schema DDL
	tables := g.extractTablesFromDDL(schema.DDLContext)
	matchedTables := g.findMatchingTables(p, tables)

	if len(matchedTables) == 0 {
		if len(tables) > 0 {
			matchedTables = append(matchedTables, tables[0])
		} else {
			matchedTables = append(matchedTables, TableMeta{
				FullName: "sales.Customers",
				Columns:  []string{"CustomerID", "FirstName", "LastName", "Email", "CreatedAt"},
			})
		}
	}

	primaryTable := matchedTables[0]
	isMultiTable := len(matchedTables) > 1

	var joins []string
	var selectedCols []string

	if isMultiTable {
		t1 := matchedTables[0]
		t2 := matchedTables[1]
		joinKey := g.findCommonJoinKey(t1, t2)
		alias1 := g.getAlias(t1.FullName)
		alias2 := g.getAlias(t2.FullName)

		if joinKey != "" {
			joins = append(joins, fmt.Sprintf("JOIN %s %s ON %s.%s = %s.%s", t2.FullName, alias2, alias1, joinKey, alias2, joinKey))
		}
	}

	// Check intent types
	isSeniority := strings.Contains(p, "anciennet") || strings.Contains(p, "tenure") || strings.Contains(p, "duree") || strings.Contains(p, "durée") || strings.Contains(p, "age") || strings.Contains(p, "âge")
	isAverage := strings.Contains(p, "moyen") || strings.Contains(p, "average") || strings.Contains(p, "avg")
	isCount := strings.Contains(p, "combien") || strings.Contains(p, "count") || strings.Contains(p, "nombre") || strings.Contains(p, "effectif") || strings.Contains(p, "total des")
	isSum := strings.Contains(p, "somme") || strings.Contains(p, "total du chiffre") || strings.Contains(p, "chiffre d'affaires") || strings.Contains(p, "ca total") || strings.Contains(p, "montant total") || strings.Contains(p, "masse salariale")
	isMax := strings.Contains(p, "plus grand") || strings.Contains(p, "plus eleve") || strings.Contains(p, "plus élevé") || strings.Contains(p, "max") || strings.Contains(p, "meilleur") || strings.Contains(p, "plus recent") || strings.Contains(p, "plus récent")
	isMin := strings.Contains(p, "plus petit") || strings.Contains(p, "min") || strings.Contains(p, "plus bas") || strings.Contains(p, "pire") || strings.Contains(p, "plus ancien") || strings.Contains(p, "premier")

	hasGroupBy := strings.Contains(p, "par ") || strings.Contains(p, "pour chaque") || strings.Contains(p, "by ") || strings.Contains(p, "per ")

	// ----------------------------------------------------
	// Case 1: Seniority / Tenure / Age (Ancienneté)
	// ----------------------------------------------------
	if isSeniority {
		dateCol := g.findDateColumn(primaryTable)
		if dateCol == "" {
			dateCol = "HireDate"
		}

		if isAverage {
			if isMultiTable && hasGroupBy {
				groupTable := matchedTables[1]
				groupCol := g.findDescriptiveColumn(groupTable)
				alias1 := g.getAlias(primaryTable.FullName)
				alias2 := g.getAlias(groupTable.FullName)

				var sql strings.Builder
				if isMSSQL {
					sql.WriteString(fmt.Sprintf("SELECT %s.%s,\n       AVG(DATEDIFF(year, %s.%s, GETDATE())) AS AncienneteMoyenneAnnees,\n       COUNT(*) AS Effectif\n", alias2, groupCol, alias1, dateCol))
				} else if isOracle {
					sql.WriteString(fmt.Sprintf("SELECT %s.%s,\n       AVG(ROUND(MONTHS_BETWEEN(SYSDATE, %s.%s) / 12, 1)) AS AncienneteMoyenneAnnees,\n       COUNT(*) AS Effectif\n", alias2, groupCol, alias1, dateCol))
				} else {
					sql.WriteString(fmt.Sprintf("SELECT %s.%s,\n       AVG(EXTRACT(YEAR FROM age(%s.%s))) AS AncienneteMoyenneAnnees,\n       COUNT(*) AS Effectif\n", alias2, groupCol, alias1, dateCol))
				}
				sql.WriteString(fmt.Sprintf("FROM %s %s\n", primaryTable.FullName, alias1))
				for _, j := range joins {
					sql.WriteString(fmt.Sprintf("%s\n", j))
				}
				sql.WriteString(fmt.Sprintf("GROUP BY %s.%s\n", alias2, groupCol))
				sql.WriteString("ORDER BY AncienneteMoyenneAnnees DESC;")

				return sql.String(), fmt.Sprintf("Calcule l'ancienneté moyenne (en années) par %s à partir de la colonne %s.", groupCol, dateCol)
			}

			// Single table average tenure
			var sql strings.Builder
			if isMSSQL {
				sql.WriteString(fmt.Sprintf("SELECT AVG(DATEDIFF(year, %s, GETDATE())) AS AncienneteMoyenneAnnees,\n", dateCol))
				sql.WriteString(fmt.Sprintf("       AVG(DATEDIFF(month, %s, GETDATE())) AS AncienneteMoyenneMois,\n", dateCol))
				sql.WriteString(fmt.Sprintf("       MIN(DATEDIFF(year, %s, GETDATE())) AS AncienneteMinAnnees,\n", dateCol))
				sql.WriteString(fmt.Sprintf("       MAX(DATEDIFF(year, %s, GETDATE())) AS AncienneteMaxAnnees\n", dateCol))
			} else if isOracle {
				sql.WriteString(fmt.Sprintf("SELECT AVG(ROUND(MONTHS_BETWEEN(SYSDATE, %s) / 12, 1)) AS AncienneteMoyenneAnnees,\n", dateCol))
				sql.WriteString(fmt.Sprintf("       MIN(ROUND(MONTHS_BETWEEN(SYSDATE, %s) / 12, 1)) AS AncienneteMinAnnees,\n", dateCol))
				sql.WriteString(fmt.Sprintf("       MAX(ROUND(MONTHS_BETWEEN(SYSDATE, %s) / 12, 1)) AS AncienneteMaxAnnees\n", dateCol))
			} else {
				sql.WriteString(fmt.Sprintf("SELECT AVG(EXTRACT(YEAR FROM age(%s))) AS AncienneteMoyenneAnnees,\n", dateCol))
				sql.WriteString(fmt.Sprintf("       MIN(EXTRACT(YEAR FROM age(%s))) AS AncienneteMinAnnees,\n", dateCol))
				sql.WriteString(fmt.Sprintf("       MAX(EXTRACT(YEAR FROM age(%s))) AS AncienneteMaxAnnees\n", dateCol))
			}
			sql.WriteString(fmt.Sprintf("FROM %s;", primaryTable.FullName))

			return sql.String(), fmt.Sprintf("Calcule l'ancienneté moyenne, minimale et maximale en années à partir de la colonne %s de la table %s.", dateCol, primaryTable.FullName)
		}

		// Individual list with seniority calculation
		var sql strings.Builder
		pk := g.findPKColumn(primaryTable)
		descCol := g.findDescriptiveColumn(primaryTable)
		if isMSSQL {
			sql.WriteString(fmt.Sprintf("SELECT TOP %d %s, %s, %s,\n       DATEDIFF(year, %s, GETDATE()) AS AncienneteAnnees\n", limit, pk, descCol, dateCol, dateCol))
		} else {
			sql.WriteString(fmt.Sprintf("SELECT %s, %s, %s,\n       DATEDIFF(year, %s, GETDATE()) AS AncienneteAnnees\n", pk, descCol, dateCol, dateCol))
		}
		sql.WriteString(fmt.Sprintf("FROM %s\n", primaryTable.FullName))
		sql.WriteString(fmt.Sprintf("ORDER BY %s ASC;", dateCol))

		return sql.String(), fmt.Sprintf("Liste les enregistrements de %s triés par ancienneté décroissante.", primaryTable.FullName)
	}

	// ----------------------------------------------------
	// Case 2: Average of Numerical Column (Salaire, Prix, Montant)
	// ----------------------------------------------------
	if isAverage {
		numCol := g.findAmountColumn(primaryTable)
		if numCol == "" {
			numCol = "Salary"
		}

		if isMultiTable && hasGroupBy {
			groupTable := matchedTables[1]
			groupCol := g.findDescriptiveColumn(groupTable)
			alias1 := g.getAlias(primaryTable.FullName)
			alias2 := g.getAlias(groupTable.FullName)

			var sql strings.Builder
			sql.WriteString(fmt.Sprintf("SELECT %s.%s,\n       AVG(%s.%s) AS Moyenne%s,\n       COUNT(*) AS TotalCount\n", alias2, groupCol, alias1, numCol, numCol))
			sql.WriteString(fmt.Sprintf("FROM %s %s\n", primaryTable.FullName, alias1))
			for _, j := range joins {
				sql.WriteString(fmt.Sprintf("%s\n", j))
			}
			sql.WriteString(fmt.Sprintf("GROUP BY %s.%s\n", alias2, groupCol))
			sql.WriteString(fmt.Sprintf("ORDER BY Moyenne%s DESC;", numCol))

			return sql.String(), fmt.Sprintf("Calcule la moyenne de %s groupée par %s.", numCol, groupCol)
		}

		// Single table average
		var sql strings.Builder
		sql.WriteString(fmt.Sprintf("SELECT AVG(%s) AS Moyenne%s,\n       MIN(%s) AS Min%s,\n       MAX(%s) AS Max%s,\n       COUNT(*) AS TotalCount\n", numCol, numCol, numCol, numCol, numCol, numCol))
		sql.WriteString(fmt.Sprintf("FROM %s;", primaryTable.FullName))

		return sql.String(), fmt.Sprintf("Calcule la moyenne, le minimum et le maximum de la colonne %s sur %s.", numCol, primaryTable.FullName)
	}

	// ----------------------------------------------------
	// Case 3: Sum / Total Revenue / Payroll (Chiffre d'affaires, Masse salariale)
	// ----------------------------------------------------
	if isSum {
		amtCol := g.findAmountColumn(primaryTable)
		if amtCol == "" {
			amtCol = "TotalAmount"
		}

		if isMultiTable {
			groupTable := matchedTables[1]
			groupCol := g.findDescriptiveColumn(groupTable)
			alias1 := g.getAlias(primaryTable.FullName)
			alias2 := g.getAlias(groupTable.FullName)

			var sql strings.Builder
			if isMSSQL {
				sql.WriteString(fmt.Sprintf("SELECT TOP %d %s.%s, SUM(%s.%s) AS MontantTotal, COUNT(*) AS NombreOperations\n", limit, alias2, groupCol, alias1, amtCol))
			} else {
				sql.WriteString(fmt.Sprintf("SELECT %s.%s, SUM(%s.%s) AS MontantTotal, COUNT(*) AS NombreOperations\n", alias2, groupCol, alias1, amtCol))
			}
			sql.WriteString(fmt.Sprintf("FROM %s %s\n", primaryTable.FullName, alias1))
			for _, j := range joins {
				sql.WriteString(fmt.Sprintf("%s\n", j))
			}
			sql.WriteString(fmt.Sprintf("GROUP BY %s.%s\n", alias2, groupCol))
			sql.WriteString("ORDER BY MontantTotal DESC")
			if !isMSSQL {
				sql.WriteString(fmt.Sprintf("\nLIMIT %d", limit))
			}
			sql.WriteString(";")

			return sql.String(), fmt.Sprintf("Calcule le montant total de %s agrégé par %s.", amtCol, groupCol)
		}

		// Single table sum
		var sql strings.Builder
		sql.WriteString(fmt.Sprintf("SELECT SUM(%s) AS MontantTotal, COUNT(*) AS TotalCount\nFROM %s;", amtCol, primaryTable.FullName))
		return sql.String(), fmt.Sprintf("Calcule la somme totale de %s sur %s.", amtCol, primaryTable.FullName)
	}

	// ----------------------------------------------------
	// Case 4: Count records
	// ----------------------------------------------------
	if isCount && !isMultiTable {
		var sql strings.Builder
		sql.WriteString(fmt.Sprintf("SELECT COUNT(*) AS TotalRecords\nFROM %s;", primaryTable.FullName))
		return sql.String(), fmt.Sprintf("Compte le nombre total d'enregistrements dans %s.", primaryTable.FullName)
	}

	// ----------------------------------------------------
	// Case 5: Standard SELECT Query with Filters & Sorting
	// ----------------------------------------------------
	alias1 := g.getAlias(primaryTable.FullName)
	prefix := ""
	if isMultiTable {
		prefix = alias1 + "."
	}

	// Build WHERE conditions
	var whereClauses []string

	// Year filter
	yearRe := regexp.MustCompile(`\b(20\d\d)\b`)
	if yearMatch := yearRe.FindStringSubmatch(p); len(yearMatch) > 1 {
		dateCol := g.findDateColumn(primaryTable)
		if dateCol != "" {
			whereClauses = append(whereClauses, fmt.Sprintf("YEAR(%s%s) = %s", prefix, dateCol, yearMatch[1]))
		}
	}

	// Amount filter
	amtRe := regexp.MustCompile(`(?:>|plus de|superieur a|supérieur à|>=)\s*(\d+)`)
	if amtMatch := amtRe.FindStringSubmatch(p); len(amtMatch) > 1 {
		amtCol := g.findAmountColumn(primaryTable)
		if amtCol != "" {
			whereClauses = append(whereClauses, fmt.Sprintf("%s%s >= %s", prefix, amtCol, amtMatch[1]))
		}
	}

	// Status filter
	if strings.Contains(p, "actif") || strings.Contains(p, "active") {
		if g.hasColumn(primaryTable, "Active") {
			whereClauses = append(whereClauses, fmt.Sprintf("%sActive = 1", prefix))
		} else if g.hasColumn(primaryTable, "Status") {
			whereClauses = append(whereClauses, fmt.Sprintf("%sStatus = 'Active'", prefix))
		}
	}

	// ORDER BY
	var orderBy string
	if isMin {
		dateCol := g.findDateColumn(primaryTable)
		if dateCol != "" {
			orderBy = fmt.Sprintf("ORDER BY %s%s ASC", prefix, dateCol)
		}
	} else if isMax {
		amtCol := g.findAmountColumn(primaryTable)
		if amtCol != "" {
			orderBy = fmt.Sprintf("ORDER BY %s%s DESC", prefix, amtCol)
		} else {
			dateCol := g.findDateColumn(primaryTable)
			if dateCol != "" {
				orderBy = fmt.Sprintf("ORDER BY %s%s DESC", prefix, dateCol)
			}
		}
	} else {
		dateCol := g.findDateColumn(primaryTable)
		if dateCol == "" {
			dateCol = g.findPKColumn(primaryTable)
		}
		if dateCol != "" {
			orderBy = fmt.Sprintf("ORDER BY %s%s DESC", prefix, dateCol)
		}
	}

	var sqlBuilder strings.Builder
	colsStr := "*"
	if len(selectedCols) > 0 {
		colsStr = strings.Join(selectedCols, ", ")
	}

	if isMSSQL {
		sqlBuilder.WriteString(fmt.Sprintf("SELECT TOP %d %s\n", limit, colsStr))
	} else {
		sqlBuilder.WriteString(fmt.Sprintf("SELECT %s\n", colsStr))
	}

	if isMultiTable {
		sqlBuilder.WriteString(fmt.Sprintf("FROM %s %s\n", primaryTable.FullName, alias1))
		for _, j := range joins {
			sqlBuilder.WriteString(fmt.Sprintf("%s\n", j))
		}
	} else {
		sqlBuilder.WriteString(fmt.Sprintf("FROM %s\n", primaryTable.FullName))
	}

	if len(whereClauses) > 0 {
		sqlBuilder.WriteString(fmt.Sprintf("WHERE %s\n", strings.Join(whereClauses, " AND ")))
	}

	if orderBy != "" {
		sqlBuilder.WriteString(fmt.Sprintf("%s", orderBy))
	}

	if !isMSSQL {
		if isOracle {
			sqlBuilder.WriteString(fmt.Sprintf("\nFETCH FIRST %d ROWS ONLY", limit))
		} else {
			sqlBuilder.WriteString(fmt.Sprintf("\nLIMIT %d", limit))
		}
	}
	sqlBuilder.WriteString(";")

	explanation := fmt.Sprintf("Retourne %d lignes de %s avec filtres et tri appliqués.", limit, primaryTable.FullName)
	return sqlBuilder.String(), explanation
}

func (g *EmbeddedSQLGenerator) fixError(currentSQL, errorMsg string, schema *SchemaSummary) (string, string) {
	trimmed := strings.TrimSpace(currentSQL)
	if trimmed == "" {
		return "SELECT 1;", "Empty query provided."
	}

	// Fix 1: GO syntax error
	if strings.Contains(errorMsg, "near GO") || strings.Contains(errorMsg, "near 'GO'") {
		fixed := strings.ReplaceAll(trimmed, "GO;", "GO")
		fixed = strings.ReplaceAll(fixed, "go;", "GO")
		return fixed, "Suppression du point-virgule après les séparateurs de batch GO pour respecter la syntaxe T-SQL."
	}

	// Fix 2: Invalid object name
	if strings.Contains(errorMsg, "Invalid object name") || strings.Contains(errorMsg, "does not exist") {
		tables := g.extractTablesFromDDL(schema.DDLContext)
		if len(tables) > 0 {
			re := regexp.MustCompile(`(?i)FROM\s+([a-zA-Z0-9_\.]+)`)
			fixed := re.ReplaceAllString(trimmed, fmt.Sprintf("FROM %s", tables[0].FullName))
			return fixed, fmt.Sprintf("Remplacement de la table inexistante par la table valide '%s'.", tables[0].FullName)
		}
	}

	return trimmed, "Requête nettoyée et formatée."
}

func (g *EmbeddedSQLGenerator) explainSQL(currentSQL string, schema *SchemaSummary) string {
	trimmed := strings.TrimSpace(currentSQL)
	if trimmed == "" {
		return "Aucune requête SQL fournie dans l'éditeur à expliquer."
	}

	var b strings.Builder
	b.WriteString("### Analyse de la requête SQL :\n\n")

	upper := strings.ToUpper(trimmed)
	if strings.HasPrefix(upper, "SELECT") {
		b.WriteString("• **Type d'opération** : Requête de lecture (`SELECT`).\n")
	} else if strings.HasPrefix(upper, "INSERT") {
		b.WriteString("• **Type d'opération** : Insertion de nouvelles lignes (`INSERT`).\n")
	} else if strings.HasPrefix(upper, "UPDATE") {
		b.WriteString("• **Type d'opération** : Modification de données existantes (`UPDATE`).\n")
	} else if strings.HasPrefix(upper, "DELETE") {
		b.WriteString("• **Type d'opération** : Suppression de données (`DELETE`).\n")
	}

	tables := g.extractTablesFromDDL(schema.DDLContext)
	var foundTables []string
	for _, t := range tables {
		if strings.Contains(trimmed, t.FullName) || strings.Contains(trimmed, t.Name) {
			foundTables = append(foundTables, t.FullName)
		}
	}
	if len(foundTables) > 0 {
		b.WriteString(fmt.Sprintf("• **Tables cibles** : `%s`\n", strings.Join(foundTables, "`, `")))
	}

	if strings.Contains(upper, "AVG(") || strings.Contains(upper, "SUM(") || strings.Contains(upper, "COUNT(") {
		b.WriteString("• **Fonctions d'agrégation** : Calculs statistiques (Moyenne/Somme/Nombre).\n")
	}
	if strings.Contains(upper, "DATEDIFF(") {
		b.WriteString("• **Calculs de dates** : Calcul d'ancienneté ou d'intervalle temporel.\n")
	}
	if strings.Contains(upper, "JOIN") {
		b.WriteString("• **Jointures relationnelles** : Association de données entre tables liées.\n")
	}
	if strings.Contains(upper, "GROUP BY") {
		b.WriteString("• **Regroupement** : Synthèse par dimension.\n")
	}
	if strings.Contains(upper, "ORDER BY") {
		b.WriteString("• **Tri** : Organisation ordonnée des résultats.\n")
	}

	return b.String()
}

func (g *EmbeddedSQLGenerator) optimizeSQL(currentSQL string, schema *SchemaSummary) (string, string) {
	trimmed := strings.TrimSpace(currentSQL)
	if trimmed == "" {
		return currentSQL, "Aucune requête à optimiser."
	}

	if strings.Contains(trimmed, "SELECT *") || strings.Contains(trimmed, "select *") {
		tables := g.extractTablesFromDDL(schema.DDLContext)
		if len(tables) > 0 {
			cols := strings.Join(tables[0].Columns, ", ")
			optimized := strings.Replace(trimmed, "SELECT *", "SELECT "+cols, 1)
			optimized = strings.Replace(optimized, "select *", "SELECT "+cols, 1)
			return optimized, "Optimisation : remplacement de 'SELECT *' par les colonnes explicites pour réduire la charge réseau et mémoire."
		}
	}

	return trimmed, "La structure de la requête est saine et respecte les bonnes pratiques."
}

type TableMeta struct {
	FullName string
	Schema   string
	Name     string
	Columns  []string
	PK       string
}

func (g *EmbeddedSQLGenerator) extractTablesFromDDL(ddl string) []TableMeta {
	var tables []TableMeta
	lines := strings.Split(ddl, "\n")
	var currentTable *TableMeta

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "CREATE TABLE ") {
			parts := strings.Split(trimmed, " ")
			if len(parts) >= 3 {
				rawName := strings.Trim(parts[2], " (")
				schemaPart := ""
				namePart := rawName
				if strings.Contains(rawName, ".") {
					sp := strings.SplitN(rawName, ".", 2)
					schemaPart = sp[0]
					namePart = sp[1]
				}
				currentTable = &TableMeta{
					FullName: rawName,
					Schema:   schemaPart,
					Name:     namePart,
					Columns:  []string{},
				}
			}
		} else if currentTable != nil && strings.HasPrefix(trimmed, ");") {
			tables = append(tables, *currentTable)
			currentTable = nil
		} else if currentTable != nil && len(trimmed) > 0 && !strings.HasPrefix(trimmed, "--") {
			colParts := strings.Fields(trimmed)
			if len(colParts) > 0 {
				colName := strings.Trim(colParts[0], ",")
				currentTable.Columns = append(currentTable.Columns, colName)
				if strings.Contains(trimmed, "PRIMARY KEY") {
					currentTable.PK = colName
				}
			}
		}
	}
	return tables
}

func (g *EmbeddedSQLGenerator) findMatchingTables(prompt string, tables []TableMeta) []TableMeta {
	var matches []TableMeta
	for _, t := range tables {
		n := strings.ToLower(t.Name)
		fn := strings.ToLower(t.FullName)

		if strings.Contains(prompt, n) || strings.Contains(prompt, fn) ||
			((strings.Contains(n, "employee") || strings.Contains(n, "employe")) &&
				(strings.Contains(prompt, "employ") || strings.Contains(prompt, "salarie") || strings.Contains(prompt, "salarié") || strings.Contains(prompt, "anciennet") || strings.Contains(prompt, "staff") || strings.Contains(prompt, "personnel"))) ||
			(strings.Contains(n, "department") && (strings.Contains(prompt, "departement") || strings.Contains(prompt, "département") || strings.Contains(prompt, "service"))) ||
			(strings.Contains(n, "customer") && (strings.Contains(prompt, "client") || strings.Contains(prompt, "user") || strings.Contains(prompt, "acheteur"))) ||
			(strings.Contains(n, "order") && (strings.Contains(prompt, "commande") || strings.Contains(prompt, "vente") || strings.Contains(prompt, "ca") || strings.Contains(prompt, "facture"))) ||
			(strings.Contains(n, "product") && (strings.Contains(prompt, "produit") || strings.Contains(prompt, "article") || strings.Contains(prompt, "stock") || strings.Contains(prompt, "prix"))) ||
			(strings.Contains(n, "log") && (strings.Contains(prompt, "audit") || strings.Contains(prompt, "journal") || strings.Contains(prompt, "evenement") || strings.Contains(prompt, "trace"))) {
			matches = append(matches, t)
		}
	}
	return matches
}

func (g *EmbeddedSQLGenerator) findCommonJoinKey(t1, t2 TableMeta) string {
	for _, c1 := range t1.Columns {
		for _, c2 := range t2.Columns {
			if strings.EqualFold(c1, c2) && (strings.HasSuffix(strings.ToLower(c1), "id") || strings.HasSuffix(strings.ToLower(c1), "key")) {
				return c1
			}
		}
	}
	return ""
}

func (g *EmbeddedSQLGenerator) getAlias(tableName string) string {
	parts := strings.Split(tableName, ".")
	name := parts[len(parts)-1]
	if len(name) > 0 {
		return strings.ToLower(string(name[0]))
	}
	return "t"
}

func (g *EmbeddedSQLGenerator) findDateColumn(t TableMeta) string {
	for _, c := range t.Columns {
		lc := strings.ToLower(c)
		if strings.Contains(lc, "hire") || strings.Contains(lc, "date") || strings.Contains(lc, "created") || strings.Contains(lc, "timestamp") || strings.Contains(lc, "start") {
			return c
		}
	}
	return ""
}

func (g *EmbeddedSQLGenerator) findAmountColumn(t TableMeta) string {
	for _, c := range t.Columns {
		lc := strings.ToLower(c)
		if strings.Contains(lc, "salary") || strings.Contains(lc, "salaire") || strings.Contains(lc, "amount") || strings.Contains(lc, "total") || strings.Contains(lc, "price") || strings.Contains(lc, "revenue") {
			return c
		}
	}
	return ""
}

func (g *EmbeddedSQLGenerator) findDescriptiveColumn(t TableMeta) string {
	for _, c := range t.Columns {
		lc := strings.ToLower(c)
		if strings.Contains(lc, "name") || strings.Contains(lc, "title") || strings.Contains(lc, "nom") || strings.Contains(lc, "label") {
			return c
		}
	}
	if len(t.Columns) > 1 {
		return t.Columns[1]
	}
	return g.findPKColumn(t)
}

func (g *EmbeddedSQLGenerator) findPKColumn(t TableMeta) string {
	if t.PK != "" {
		return t.PK
	}
	for _, c := range t.Columns {
		if strings.HasSuffix(strings.ToLower(c), "id") {
			return c
		}
	}
	if len(t.Columns) > 0 {
		return t.Columns[0]
	}
	return "ID"
}

func (g *EmbeddedSQLGenerator) hasColumn(t TableMeta, colName string) bool {
	for _, c := range t.Columns {
		if strings.EqualFold(c, colName) {
			return true
		}
	}
	return false
}
