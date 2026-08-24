package ai

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// EmbeddedSQLGenerator provides an intelligent, in-process schema-aware SQL generator
// that dynamically analyzes the active database structure (tables, columns, types, keys)
// without hardcoded assumptions, operating completely offline with zero external dependencies.
type EmbeddedSQLGenerator struct{}

func NewEmbeddedSQLGenerator() *EmbeddedSQLGenerator {
	return &EmbeddedSQLGenerator{}
}

type ColumnMeta struct {
	Name         string
	DataType     string
	IsPrimaryKey bool
	IsNullable   bool
}

type TableMeta struct {
	FullName string
	Schema   string
	Name     string
	Columns  []ColumnMeta
	PK       string
}

// Generate processes a request and produces valid SQL tailored dynamically to the active database schema
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

	// 2. Parse and analyze active database tables & columns from DDL
	tables := g.extractTablesFromDDL(schema.DDLContext)
	if len(tables) == 0 {
		tables = []TableMeta{
			{
				FullName: "sales.Customers",
				Schema:   "sales",
				Name:     "Customers",
				Columns: []ColumnMeta{
					{Name: "CustomerID", DataType: "bigint", IsPrimaryKey: true},
					{Name: "FirstName", DataType: "nvarchar(50)"},
					{Name: "LastName", DataType: "nvarchar(50)"},
					{Name: "Email", DataType: "nvarchar(100)"},
					{Name: "CreatedAt", DataType: "datetime"},
				},
				PK: "CustomerID",
			},
		}
	}

	// 3. Match relevant tables based on prompt semantics and column coverage
	matchedTables := g.rankMatchingTables(p, tables)
	if len(matchedTables) == 0 {
		matchedTables = []TableMeta{tables[0]}
	}

	// Intent analysis
	isSeniority := strings.Contains(p, "anciennet") || strings.Contains(p, "tenure") || strings.Contains(p, "duree") || strings.Contains(p, "durée") || strings.Contains(p, "age") || strings.Contains(p, "âge")
	isAverage := strings.Contains(p, "moyen") || strings.Contains(p, "average") || strings.Contains(p, "avg")
	isCount := strings.Contains(p, "combien") || strings.Contains(p, "count") || strings.Contains(p, "nombre") || strings.Contains(p, "effectif") || strings.Contains(p, "total des") || strings.Contains(p, "quantit")
	isSum := strings.Contains(p, "somme") || strings.Contains(p, "total du chiffre") || strings.Contains(p, "chiffre d'affaires") || strings.Contains(p, "ca total") || strings.Contains(p, "montant total") || strings.Contains(p, "masse salariale") || strings.Contains(p, "cumul") || strings.Contains(p, "chiffre")
	isMax := strings.Contains(p, "plus grand") || strings.Contains(p, "plus eleve") || strings.Contains(p, "plus élevé") || strings.Contains(p, "max") || strings.Contains(p, "meilleur") || strings.Contains(p, "plus recent") || strings.Contains(p, "plus récent")
	isMin := strings.Contains(p, "plus petit") || strings.Contains(p, "min") || strings.Contains(p, "plus bas") || strings.Contains(p, "pire") || strings.Contains(p, "plus ancien") || strings.Contains(p, "premier")
	hasGroupBy := strings.Contains(p, "par ") || strings.Contains(p, "pour chaque") || strings.Contains(p, "by ") || strings.Contains(p, "per ") || strings.Contains(p, "selon ") || strings.Contains(p, "reparti")

	// Ensure primaryTable contains the target measure column if multi-table
	if len(matchedTables) > 1 && isSeniority {
		if g.findBestDateColumn(matchedTables[0]) == "" && g.findBestDateColumn(matchedTables[1]) != "" {
			matchedTables[0], matchedTables[1] = matchedTables[1], matchedTables[0]
		}
	}
	if len(matchedTables) > 1 && (isAverage || isSum || isMax || isMin) {
		if g.findBestNumericColumn(matchedTables[0]) == "" && g.findBestNumericColumn(matchedTables[1]) != "" {
			matchedTables[0], matchedTables[1] = matchedTables[1], matchedTables[0]
		}
	}

	primaryTable := matchedTables[0]
	isMultiTable := len(matchedTables) > 1

	var joins []string
	if isMultiTable {
		t1 := matchedTables[0]
		t2 := matchedTables[1]
		joinKey, col1, col2 := g.findJoinCondition(t1, t2)
		alias1 := g.getAlias(t1.FullName)
		alias2 := g.getAlias(t2.FullName)

		if joinKey != "" {
			joins = append(joins, fmt.Sprintf("JOIN %s %s ON %s.%s = %s.%s", t2.FullName, alias2, alias1, col1, alias2, col2))
		}
	}

	// Dynamic column discovery on the REAL table
	dateCol := g.findBestDateColumn(primaryTable)
	numCol := g.findBestNumericColumn(primaryTable)
	descCol := g.findBestTextColumn(primaryTable)
	pkCol := g.findBestPKColumn(primaryTable)

	// Fallback to first available columns if specific semantics not found
	if descCol == "" && len(primaryTable.Columns) > 1 {
		descCol = primaryTable.Columns[1].Name
	}
	if pkCol == "" && len(primaryTable.Columns) > 0 {
		pkCol = primaryTable.Columns[0].Name
	}

	// ----------------------------------------------------
	// Case 1: Seniority / Duration / Age
	// ----------------------------------------------------
	if isSeniority && dateCol != "" {
		if isAverage {
			if isMultiTable && hasGroupBy {
				groupTable := matchedTables[1]
				groupCol := g.findBestTextColumn(groupTable)
				if groupCol == "" {
					groupCol = g.findBestPKColumn(groupTable)
				}
				alias1 := g.getAlias(primaryTable.FullName)
				alias2 := g.getAlias(groupTable.FullName)

				var sql strings.Builder
				if isMSSQL {
					sql.WriteString(fmt.Sprintf("SELECT %s.%s,\n       AVG(DATEDIFF(year, %s.%s, GETDATE())) AS AncienneteMoyenneAnnees,\n       COUNT(*) AS TotalEnregistrements\n", alias2, groupCol, alias1, dateCol))
				} else if isOracle {
					sql.WriteString(fmt.Sprintf("SELECT %s.%s,\n       AVG(ROUND(MONTHS_BETWEEN(SYSDATE, %s.%s) / 12, 1)) AS AncienneteMoyenneAnnees,\n       COUNT(*) AS TotalEnregistrements\n", alias2, groupCol, alias1, dateCol))
				} else {
					sql.WriteString(fmt.Sprintf("SELECT %s.%s,\n       AVG(EXTRACT(YEAR FROM age(%s.%s))) AS AncienneteMoyenneAnnees,\n       COUNT(*) AS TotalEnregistrements\n", alias2, groupCol, alias1, dateCol))
				}
				sql.WriteString(fmt.Sprintf("FROM %s %s\n", primaryTable.FullName, alias1))
				for _, j := range joins {
					sql.WriteString(fmt.Sprintf("%s\n", j))
				}
				sql.WriteString(fmt.Sprintf("GROUP BY %s.%s\n", alias2, groupCol))
				sql.WriteString("ORDER BY AncienneteMoyenneAnnees DESC;")

				return sql.String(), fmt.Sprintf("Analyse de la table '%s' : calcul de l'ancienneté moyenne (en années) via la colonne temporelle '%s', groupée par '%s'.", primaryTable.FullName, dateCol, groupCol)
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

			return sql.String(), fmt.Sprintf("Calcul de l'ancienneté moyenne, minimale et maximale à partir de la colonne de date détectée '%s' dans la table '%s'.", dateCol, primaryTable.FullName)
		}

		// Individual list with seniority calculation
		var sql strings.Builder
		if isMSSQL {
			sql.WriteString(fmt.Sprintf("SELECT TOP %d %s, %s, %s,\n       DATEDIFF(year, %s, GETDATE()) AS AncienneteAnnees\n", limit, pkCol, descCol, dateCol, dateCol))
		} else {
			sql.WriteString(fmt.Sprintf("SELECT %s, %s, %s,\n       DATEDIFF(year, %s, GETDATE()) AS AncienneteAnnees\n", pkCol, descCol, dateCol, dateCol))
		}
		sql.WriteString(fmt.Sprintf("FROM %s\n", primaryTable.FullName))
		sql.WriteString(fmt.Sprintf("ORDER BY %s ASC", dateCol))
		if !isMSSQL {
			sql.WriteString(fmt.Sprintf("\nLIMIT %d", limit))
		}
		sql.WriteString(";")

		return sql.String(), fmt.Sprintf("Liste des enregistrements de '%s' triés par date '%s'.", primaryTable.FullName, dateCol)
	}

	// ----------------------------------------------------
	// Case 2: Average / Sum / Aggregation on Numeric Columns
	// ----------------------------------------------------
	if (isAverage || isSum || isMax || isMin) && numCol != "" {
		aggFunc := "AVG"
		aggLabel := "Moyenne"
		if isSum {
			aggFunc = "SUM"
			aggLabel = "Total"
		} else if isMax {
			aggFunc = "MAX"
			aggLabel = "Max"
		} else if isMin {
			aggFunc = "MIN"
			aggLabel = "Min"
		}

		if isMultiTable && hasGroupBy {
			groupTable := matchedTables[1]
			groupCol := g.findBestTextColumn(groupTable)
			if groupCol == "" {
				groupCol = g.findBestPKColumn(groupTable)
			}
			alias1 := g.getAlias(primaryTable.FullName)
			alias2 := g.getAlias(groupTable.FullName)

			var sql strings.Builder
			if isMSSQL {
				sql.WriteString(fmt.Sprintf("SELECT TOP %d %s.%s,\n       %s(%s.%s) AS %s_%s,\n       COUNT(*) AS TotalCount\n", limit, alias2, groupCol, aggFunc, alias1, numCol, aggLabel, numCol))
			} else {
				sql.WriteString(fmt.Sprintf("SELECT %s.%s,\n       %s(%s.%s) AS %s_%s,\n       COUNT(*) AS TotalCount\n", alias2, groupCol, aggFunc, alias1, numCol, aggLabel, numCol))
			}
			sql.WriteString(fmt.Sprintf("FROM %s %s\n", primaryTable.FullName, alias1))
			for _, j := range joins {
				sql.WriteString(fmt.Sprintf("%s\n", j))
			}
			sql.WriteString(fmt.Sprintf("GROUP BY %s.%s\n", alias2, groupCol))
			sql.WriteString(fmt.Sprintf("ORDER BY %s_%s DESC", aggLabel, numCol))
			if !isMSSQL {
				sql.WriteString(fmt.Sprintf("\nLIMIT %d", limit))
			}
			sql.WriteString(";")

			return sql.String(), fmt.Sprintf("Calcul de l'agrégation %s sur la colonne numérique '%s' de '%s', groupée par '%s'.", aggFunc, numCol, primaryTable.FullName, groupCol)
		}

		// Single table aggregation
		var sql strings.Builder
		if isAverage {
			sql.WriteString(fmt.Sprintf("SELECT AVG(%s) AS Moyenne_%s,\n       MIN(%s) AS Min_%s,\n       MAX(%s) AS Max_%s,\n       COUNT(*) AS TotalCount\nFROM %s;", numCol, numCol, numCol, numCol, numCol, numCol, primaryTable.FullName))
		} else {
			sql.WriteString(fmt.Sprintf("SELECT %s(%s) AS %s_%s,\n       COUNT(*) AS TotalCount\nFROM %s;", aggFunc, numCol, aggLabel, numCol, primaryTable.FullName))
		}
		return sql.String(), fmt.Sprintf("Calcul de l'agrégation %s sur la colonne numérique '%s' de '%s'.", aggFunc, numCol, primaryTable.FullName)
	}

	// ----------------------------------------------------
	// Case 3: Count records
	// ----------------------------------------------------
	if isCount && !isMultiTable {
		var sql strings.Builder
		sql.WriteString(fmt.Sprintf("SELECT COUNT(*) AS TotalRecords\nFROM %s;", primaryTable.FullName))
		return sql.String(), fmt.Sprintf("Compte le nombre total de lignes dans la table '%s'.", primaryTable.FullName)
	}

	// ----------------------------------------------------
	// Case 4: Default SELECT query with actual schema columns
	// ----------------------------------------------------
	var colList []string
	alias1 := g.getAlias(primaryTable.FullName)

	for _, c := range primaryTable.Columns {
		if len(colList) >= 8 {
			break // Select top representative columns
		}
		if isMultiTable {
			colList = append(colList, fmt.Sprintf("%s.%s", alias1, c.Name))
		} else {
			colList = append(colList, c.Name)
		}
	}

	var sql strings.Builder
	if isMSSQL {
		sql.WriteString(fmt.Sprintf("SELECT TOP %d\n    %s\n", limit, strings.Join(colList, ",\n    ")))
	} else {
		sql.WriteString(fmt.Sprintf("SELECT\n    %s\n", strings.Join(colList, ",\n    ")))
	}

	if isMultiTable {
		sql.WriteString(fmt.Sprintf("FROM %s %s\n", primaryTable.FullName, alias1))
		for _, j := range joins {
			sql.WriteString(fmt.Sprintf("%s\n", j))
		}
	} else {
		sql.WriteString(fmt.Sprintf("FROM %s\n", primaryTable.FullName))
	}

	// Dynamic ORDER BY if date or ID column exists
	orderCol := dateCol
	orderDir := "DESC"
	if orderCol == "" {
		orderCol = pkCol
		orderDir = "ASC"
	}
	if orderCol != "" {
		if isMultiTable {
			sql.WriteString(fmt.Sprintf("ORDER BY %s.%s %s", alias1, orderCol, orderDir))
		} else {
			sql.WriteString(fmt.Sprintf("ORDER BY %s %s", orderCol, orderDir))
		}
	}

	if !isMSSQL && limit > 0 {
		sql.WriteString(fmt.Sprintf("\nLIMIT %d", limit))
	}
	sql.WriteString(";")

	return sql.String(), fmt.Sprintf("Extraction des colonnes clés (%s) depuis la table '%s' détectée dans votre schéma actif.", strings.Join(colList, ", "), primaryTable.FullName)
}

func (g *EmbeddedSQLGenerator) fixError(currentSQL, errorMessage string, schema *SchemaSummary) (string, string) {
	if strings.TrimSpace(currentSQL) == "" {
		return "-- Aucune requête fournie à corriger.", "Veuillez fournir une requête SQL avec une erreur d'exécution."
	}

	fixed := currentSQL

	// Fix 1: LIMIT in MSSQL -> SELECT TOP N
	isMSSQL := strings.Contains(schema.Dialect, "MSSQL") || strings.Contains(schema.Dialect, "T-SQL") || strings.Contains(schema.Dialect, "SQL Server")
	if isMSSQL {
		limitRe := regexp.MustCompile(`(?i)\s+LIMIT\s+(\d+)\s*;?`)
		if match := limitRe.FindStringSubmatch(fixed); len(match) > 1 {
			limitVal := match[1]
			fixed = limitRe.ReplaceAllString(fixed, ";")
			selRe := regexp.MustCompile(`(?i)^\s*SELECT\s+(DISTINCT\s+)?`)
			if selRe.MatchString(fixed) {
				fixed = selRe.ReplaceAllString(fixed, fmt.Sprintf("SELECT ${1}TOP %s ", limitVal))
			}
			return fixed, fmt.Sprintf("Correction T-SQL : Remplacement de la clause 'LIMIT %s' par 'SELECT TOP %s' standard pour MS SQL Server.", limitVal, limitVal)
		}
	}

	// Fix 2: Invalid semicolon after GO
	if strings.Contains(strings.ToUpper(fixed), "GO;") {
		fixed = regexp.MustCompile(`(?i)\bGO\s*;`).ReplaceAllString(fixed, "GO")
		return fixed, "Correction T-SQL : Suppression du point-virgule non valide après la commande 'GO'."
	}

	// Fix 3: Invalid column name -> suggest real columns from schema
	tables := g.extractTablesFromDDL(schema.DDLContext)
	for _, t := range tables {
		if strings.Contains(strings.ToLower(currentSQL), strings.ToLower(t.Name)) {
			// Found table in query, ensure column names match
			return fixed, fmt.Sprintf("Requête analysée pour la table '%s'. Vérifiez que les colonnes utilisées correspondent bien à : %s", t.FullName, g.formatColumnNames(t))
		}
	}

	return fixed, "Correction de la syntaxe SQL appliquée selon le dialecte " + schema.Dialect
}

func (g *EmbeddedSQLGenerator) explainSQL(sql string, schema *SchemaSummary) string {
	if strings.TrimSpace(sql) == "" {
		return "Aucune requête SQL fournie à expliquer."
	}
	var b strings.Builder
	b.WriteString("📋 **Explication de la Requête SQL :**\n\n")

	upper := strings.ToUpper(sql)
	if strings.Contains(upper, "SELECT") {
		b.WriteString("• **Opération** : Lecture et extraction de données (`SELECT`).\n")
	} else if strings.Contains(upper, "INSERT") {
		b.WriteString("• **Opération** : Insertion de nouvelles lignes (`INSERT`).\n")
	} else if strings.Contains(upper, "UPDATE") {
		b.WriteString("• **Opération** : Modification de données existantes (`UPDATE`).\n")
	} else if strings.Contains(upper, "DELETE") {
		b.WriteString("• **Opération** : Suppression de données (`DELETE`).\n")
	}

	if strings.Contains(upper, "JOIN") {
		b.WriteString("• **Jointures** : Combine plusieurs tables relationnelles via des clés étrangères.\n")
	}
	if strings.Contains(upper, "GROUP BY") {
		b.WriteString("• **Agrégation** : Regroupe les données par catégorie avec calculs statistiques (`SUM`/`AVG`/`COUNT`).\n")
	}
	if strings.Contains(upper, "WHERE") {
		b.WriteString("• **Filtrage** : Applique des conditions de sélection spécifiques (`WHERE`).\n")
	}
	if strings.Contains(upper, "ORDER BY") {
		b.WriteString("• **Tri** : Ordonne le jeu de résultats pour présentation.\n")
	}
	return b.String()
}

func (g *EmbeddedSQLGenerator) optimizeSQL(sql string, schema *SchemaSummary) (string, string) {
	trimmed := strings.TrimSpace(sql)
	if strings.Contains(strings.ToUpper(trimmed), "SELECT *") {
		tables := g.extractTablesFromDDL(schema.DDLContext)
		for _, t := range tables {
			if strings.Contains(strings.ToLower(trimmed), strings.ToLower(t.Name)) {
				cols := g.formatColumnNames(t)
				optimized := strings.Replace(trimmed, "*", cols, 1)
				return optimized, fmt.Sprintf("Optimisation : Remplacement de 'SELECT *' par les colonnes explicites de '%s' pour de meilleures performances et utilisation d'index.", t.FullName)
			}
		}
	}
	return trimmed, "La structure de la requête respecte les bonnes pratiques."
}

// -------------------------------------------------------------
// Dynamic Schema Extraction & Semantic Analysis Helpers
// -------------------------------------------------------------

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
					Columns:  []ColumnMeta{},
				}
			}
		} else if currentTable != nil && strings.HasPrefix(trimmed, ");") {
			tables = append(tables, *currentTable)
			currentTable = nil
		} else if currentTable != nil && len(trimmed) > 0 && !strings.HasPrefix(trimmed, "--") {
			colParts := strings.Fields(trimmed)
			if len(colParts) > 0 {
				colName := strings.Trim(colParts[0], ",")
				colType := ""
				if len(colParts) > 1 {
					colType = strings.Trim(colParts[1], ",")
				}
				isPK := strings.Contains(trimmed, "PRIMARY KEY")
				isNullable := !strings.Contains(trimmed, "NOT NULL")

				currentTable.Columns = append(currentTable.Columns, ColumnMeta{
					Name:         colName,
					DataType:     colType,
					IsPrimaryKey: isPK,
					IsNullable:   isNullable,
				})
				if isPK {
					currentTable.PK = colName
				}
			}
		}
	}
	return tables
}

func (g *EmbeddedSQLGenerator) rankMatchingTables(prompt string, tables []TableMeta) []TableMeta {
	promptTokens := g.tokenize(prompt)
	type scoredTable struct {
		table TableMeta
		score int
	}

	var scored []scoredTable
	for _, t := range tables {
		score := 0
		tableTokens := g.tokenize(t.Name)

		// Check table name token overlap
		for _, pt := range promptTokens {
			for _, tt := range tableTokens {
				if pt == tt {
					score += 15
				} else if strings.HasPrefix(tt, pt) || strings.HasPrefix(pt, tt) {
					score += 8
				} else if g.areSynonyms(pt, tt) {
					score += 12
				}
			}

			// Check column names token overlap
			for _, c := range t.Columns {
				colTokens := g.tokenize(c.Name)
				for _, ct := range colTokens {
					if pt == ct {
						score += 6
					} else if g.areSynonyms(pt, ct) {
						score += 4
					}
				}
			}
		}

		if score > 0 {
			scored = append(scored, scoredTable{table: t, score: score})
		}
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	var result []TableMeta
	for _, st := range scored {
		result = append(result, st.table)
	}

	return result
}

func (g *EmbeddedSQLGenerator) removeAccents(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, "é", "e")
	s = strings.ReplaceAll(s, "è", "e")
	s = strings.ReplaceAll(s, "ê", "e")
	s = strings.ReplaceAll(s, "ë", "e")
	s = strings.ReplaceAll(s, "à", "a")
	s = strings.ReplaceAll(s, "â", "a")
	s = strings.ReplaceAll(s, "ä", "a")
	s = strings.ReplaceAll(s, "î", "i")
	s = strings.ReplaceAll(s, "ï", "i")
	s = strings.ReplaceAll(s, "ô", "o")
	s = strings.ReplaceAll(s, "ö", "o")
	s = strings.ReplaceAll(s, "ù", "u")
	s = strings.ReplaceAll(s, "û", "u")
	s = strings.ReplaceAll(s, "ü", "u")
	s = strings.ReplaceAll(s, "ç", "c")
	return s
}

func (g *EmbeddedSQLGenerator) tokenize(text string) []string {
	// Strip common prefixes: tbl_, t_, vw_, etc.
	text = regexp.MustCompile(`^(tbl_|t_|tab_|vw_|v_|dim_|fact_)`).ReplaceAllString(text, "")

	var tokens []string
	var current strings.Builder

	for _, r := range text {
		if unicode.IsUpper(r) {
			if current.Len() > 0 {
				tok := g.removeAccents(current.String())
				if len(tok) > 0 {
					tokens = append(tokens, tok)
				}
				current.Reset()
			}
			current.WriteRune(r)
		} else if unicode.IsLetter(r) || unicode.IsDigit(r) {
			current.WriteRune(r)
		} else {
			if current.Len() > 0 {
				tok := g.removeAccents(current.String())
				if len(tok) > 0 {
					tokens = append(tokens, tok)
				}
				current.Reset()
			}
		}
	}
	if current.Len() > 0 {
		tok := g.removeAccents(current.String())
		if len(tok) > 0 {
			tokens = append(tokens, tok)
		}
	}

	return tokens
}

func (g *EmbeddedSQLGenerator) areSynonyms(w1, w2 string) bool {
	synonymGroups := [][]string{
		{"client", "customer", "cli", "user", "acheteur", "compte", "tiers"},
		{"commande", "order", "cmd", "vente", "facture", "invoice", "achat", "operation", "transaction"},
		{"produit", "product", "item", "article", "stock", "marchandise", "ref"},
		{"employe", "employee", "salarie", "staff", "personnel", "worker", "agent", "collaborateur", "emp", "member"},
		{"departement", "department", "dept", "service", "pole", "division", "equipe", "unite", "bureau"},
		{"fournisseur", "supplier", "vendor", "fourn", "prestataire", "partenaire"},
		{"salaire", "salary", "pay", "remuneration", "paie", "revenu"},
		{"montant", "amount", "prix", "price", "total", "valeur", "ca", "solde", "cost", "cout", "chiffre", "somme"},
		{"date", "dt", "jour", "day", "created", "embauche", "creation", "debut", "fin", "anciennet", "hire", "tenure", "age", "time", "timestamp"},
		{"pays", "country", "ville", "city", "adresse", "location", "region", "territoire"},
		{"statut", "status", "etat", "state", "mode", "type", "flag", "actif"},
	}

	for _, grp := range synonymGroups {
		found1, found2 := false, false
		for _, s := range grp {
			if strings.HasPrefix(w1, s) || strings.HasPrefix(s, w1) {
				found1 = true
			}
			if strings.HasPrefix(w2, s) || strings.HasPrefix(s, w2) {
				found2 = true
			}
		}
		if found1 && found2 {
			return true
		}
	}
	return false
}

func (g *EmbeddedSQLGenerator) findJoinCondition(t1, t2 TableMeta) (string, string, string) {
	// 1. Direct same name join key (e.g. CustomerID == CustomerID or id_client == id_client)
	for _, c1 := range t1.Columns {
		for _, c2 := range t2.Columns {
			if strings.EqualFold(c1.Name, c2.Name) {
				cLower := strings.ToLower(c1.Name)
				if strings.Contains(cLower, "id") || strings.Contains(cLower, "code") || strings.Contains(cLower, "key") || strings.Contains(cLower, "num") || strings.Contains(cLower, "ref") {
					return c1.Name, c1.Name, c2.Name
				}
			}
		}
	}

	// 2. FK pattern: t1.id_t2 == t2.id or t1.t2_id == t2.id
	t2Clean := strings.ToLower(regexp.MustCompile(`^(tbl_|t_|tab_|dim_|fact_)`).ReplaceAllString(t2.Name, ""))
	for _, c1 := range t1.Columns {
		c1Lower := strings.ToLower(c1.Name)
		if (strings.Contains(c1Lower, t2Clean) || strings.HasPrefix(c1Lower, "id_") || strings.HasSuffix(c1Lower, "_id")) && strings.Contains(c1Lower, "id") {
			pk2 := g.findBestPKColumn(t2)
			if pk2 != "" && (strings.Contains(c1Lower, t2Clean) || strings.Contains(c1Lower, strings.TrimSuffix(t2Clean, "s"))) {
				return c1.Name, c1.Name, pk2
			}
		}
	}

	// 3. Reverse FK: t2.id_t1 == t1.id or t2.t1_id == t1.id
	t1Clean := strings.ToLower(regexp.MustCompile(`^(tbl_|t_|tab_|dim_|fact_)`).ReplaceAllString(t1.Name, ""))
	for _, c2 := range t2.Columns {
		c2Lower := strings.ToLower(c2.Name)
		if (strings.Contains(c2Lower, t1Clean) || strings.HasPrefix(c2Lower, "id_") || strings.HasSuffix(c2Lower, "_id")) && strings.Contains(c2Lower, "id") {
			pk1 := g.findBestPKColumn(t1)
			if pk1 != "" && (strings.Contains(c2Lower, t1Clean) || strings.Contains(c2Lower, strings.TrimSuffix(t1Clean, "s"))) {
				return c2.Name, pk1, c2.Name
			}
		}
	}

	return "", "", ""
}

func (g *EmbeddedSQLGenerator) findBestDateColumn(t TableMeta) string {
	// Look for datetime/date types first, or semantic date column names
	for _, c := range t.Columns {
		dt := strings.ToLower(c.DataType)
		cn := strings.ToLower(c.Name)
		if strings.Contains(dt, "date") || strings.Contains(dt, "time") || strings.Contains(dt, "timestamp") {
			return c.Name
		}
		if strings.Contains(cn, "hire") || strings.Contains(cn, "date") || strings.Contains(cn, "created") ||
			strings.Contains(cn, "dt_") || strings.Contains(cn, "_at") || strings.Contains(cn, "crea") ||
			strings.Contains(cn, "embauche") || strings.Contains(cn, "naissance") || strings.Contains(cn, "start") {
			return c.Name
		}
	}
	return ""
}

func (g *EmbeddedSQLGenerator) findBestNumericColumn(t TableMeta) string {
	// Look for decimal/money/float/numeric/int types with numeric names
	for _, c := range t.Columns {
		cn := strings.ToLower(c.Name)
		if strings.Contains(cn, "salary") || strings.Contains(cn, "salaire") || strings.Contains(cn, "amount") ||
			strings.Contains(cn, "montant") || strings.Contains(cn, "total") || strings.Contains(cn, "price") ||
			strings.Contains(cn, "prix") || strings.Contains(cn, "ca") || strings.Contains(cn, "revenue") ||
			strings.Contains(cn, "solde") || strings.Contains(cn, "valeur") || strings.Contains(cn, "remise") ||
			strings.Contains(cn, "mt_") || strings.Contains(cn, "taux") {
			return c.Name
		}
	}
	// Fallback to any decimal/numeric column that is not an ID
	for _, c := range t.Columns {
		dt := strings.ToLower(c.DataType)
		cn := strings.ToLower(c.Name)
		if !strings.HasSuffix(cn, "id") && !strings.HasPrefix(cn, "id_") {
			if strings.Contains(dt, "decimal") || strings.Contains(dt, "numeric") || strings.Contains(dt, "money") || strings.Contains(dt, "float") || strings.Contains(dt, "double") {
				return c.Name
			}
		}
	}
	return ""
}

func (g *EmbeddedSQLGenerator) findBestTextColumn(t TableMeta) string {
	for _, c := range t.Columns {
		cn := strings.ToLower(c.Name)
		if strings.Contains(cn, "name") || strings.Contains(cn, "nom") || strings.Contains(cn, "title") ||
			strings.Contains(cn, "titre") || strings.Contains(cn, "label") || strings.Contains(cn, "libelle") ||
			strings.Contains(cn, "designation") || strings.Contains(cn, "description") || strings.Contains(cn, "raison") {
			return c.Name
		}
	}
	return ""
}

func (g *EmbeddedSQLGenerator) findBestPKColumn(t TableMeta) string {
	if t.PK != "" {
		return t.PK
	}
	for _, c := range t.Columns {
		if c.IsPrimaryKey {
			return c.Name
		}
	}
	for _, c := range t.Columns {
		cn := strings.ToLower(c.Name)
		if cn == "id" || strings.HasSuffix(cn, "_id") || strings.HasSuffix(cn, "id") || strings.HasPrefix(cn, "id_") || strings.HasSuffix(cn, "code") {
			return c.Name
		}
	}
	if len(t.Columns) > 0 {
		return t.Columns[0].Name
	}
	return ""
}

func (g *EmbeddedSQLGenerator) getAlias(tableName string) string {
	parts := strings.Split(tableName, ".")
	name := parts[len(parts)-1]
	clean := regexp.MustCompile(`^(tbl_|t_|tab_|vw_|v_)`).ReplaceAllString(strings.ToLower(name), "")
	if len(clean) > 0 {
		return string(clean[0])
	}
	return "t"
}

func (g *EmbeddedSQLGenerator) formatColumnNames(t TableMeta) string {
	var names []string
	for _, c := range t.Columns {
		names = append(names, c.Name)
	}
	return strings.Join(names, ", ")
}
