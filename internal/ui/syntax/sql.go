package syntax

import (
	"strings"
	"unicode"

	"github.com/charmbracelet/lipgloss"
)

// TokenType represents the semantic class of a SQL token
type TokenType int

const (
	TokenDefault TokenType = iota
	TokenKeyword
	TokenSpecialKeyword
	TokenTypeType
	TokenFunction
	TokenComment
	TokenString
	TokenNumber
	TokenIdentifier
	TokenVariable
	TokenOperator
)

// SQL Styles matching SSMS / VS Code Dark Theme
var (
	StyleKeyword = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#569CD6")).
			Bold(true)

	StyleSpecialKeyword = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#C586C0")).
				Bold(true)

	StyleType = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#4EC9B0"))

	StyleFunction = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#DCDCAA"))

	StyleComment = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6A9955")).
			Italic(true)

	StyleString = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#CE9178"))

	StyleNumber = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#B5CEA8"))

	StyleIdentifier = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9CDCFE"))

	StyleVariable = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9CDCFE")).
			Italic(true)

	StyleOperator = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#D4D4D4"))

	StyleDefault = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#D4D4D4"))
)

var keywords = map[string]bool{
	"SELECT": true, "FROM": true, "WHERE": true, "INSERT": true, "INTO": true,
	"UPDATE": true, "DELETE": true, "JOIN": true, "INNER": true, "LEFT": true,
	"RIGHT": true, "OUTER": true, "FULL": true, "CROSS": true, "ON": true,
	"GROUP": true, "BY": true, "ORDER": true, "HAVING": true, "LIMIT": true,
	"TOP": true, "OFFSET": true, "FETCH": true, "NEXT": true, "ROWS": true,
	"ONLY": true, "AS": true, "AND": true, "OR": true, "NOT": true,
	"IN": true, "IS": true, "NULL": true, "LIKE": true, "ILIKE": true,
	"BETWEEN": true, "UNION": true, "ALL": true, "EXISTS": true, "DISTINCT": true,
	"ASC": true, "DESC": true, "SET": true, "VALUES": true, "WITH": true,
	"NOLOCK": true, "OVER": true, "PARTITION": true, "OUTPUT": true, "RETURNING": true,
}

var specialKeywords = map[string]bool{
	"CREATE": true, "TABLE": true, "DROP": true, "ALTER": true, "VIEW": true,
	"INDEX": true, "PROCEDURE": true, "PROC": true, "FUNCTION": true, "TRIGGER": true,
	"SCHEMA": true, "DATABASE": true, "USE": true, "GO": true, "EXEC": true,
	"EXECUTE": true, "BEGIN": true, "COMMIT": true, "ROLLBACK": true, "TRANSACTION": true,
	"TRAN": true, "TRUNCATE": true, "DECLARE": true, "IF": true, "ELSE": true,
	"WHILE": true, "RETURN": true, "RETURNS": true, "CASE": true, "WHEN": true,
	"THEN": true, "END": true, "CONSTRAINT": true, "PRIMARY": true, "KEY": true,
	"FOREIGN": true, "REFERENCES": true, "DEFAULT": true, "CHECK": true, "UNIQUE": true,
}

var dataTypes = map[string]bool{
	"INT": true, "INTEGER": true, "BIGINT": true, "SMALLINT": true, "TINYINT": true,
	"VARCHAR": true, "NVARCHAR": true, "CHAR": true, "NCHAR": true, "TEXT": true,
	"NTEXT": true, "DATE": true, "TIME": true, "DATETIME": true, "DATETIME2": true,
	"SMALLDATETIME": true, "DATETIMEOFFSET": true, "TIMESTAMP": true, "DECIMAL": true,
	"NUMERIC": true, "FLOAT": true, "REAL": true, "MONEY": true, "SMALLMONEY": true,
	"BIT": true, "BOOLEAN": true, "BOOL": true, "UUID": true, "UNIQUEIDENTIFIER": true,
	"JSON": true, "JSONB": true, "XML": true, "VARBINARY": true, "BINARY": true,
	"IMAGE": true, "BLOB": true, "CLOB": true, "NCLOB": true, "SYSNAME": true,
	"ROWVERSION": true,
}

var functions = map[string]bool{
	"COUNT": true, "SUM": true, "AVG": true, "MIN": true, "MAX": true,
	"COALESCE": true, "GETDATE": true, "GETUTCDATE": true, "SYSDATETIME": true,
	"SYSUTCDATETIME": true, "NOW": true, "CURRENT_TIMESTAMP": true, "CONVERT": true,
	"CAST": true, "TRY_CONVERT": true, "TRY_CAST": true, "LEN": true,
	"LENGTH": true, "UPPER": true, "LOWER": true, "SUBSTRING": true, "SUBSTR": true,
	"CHARINDEX": true, "INSTR": true, "ROW_NUMBER": true, "RANK": true,
	"DENSE_RANK": true, "NTILE": true, "LEAD": true, "LAG": true,
	"FIRST_VALUE": true, "LAST_VALUE": true, "ISNULL": true, "IFNULL": true,
	"NVL": true, "NULLIF": true, "DATEADD": true, "DATEDIFF": true,
	"DATEPART": true, "DATENAME": true, "TRIM": true, "LTRIM": true,
	"RTRIM": true, "REPLACE": true, "STUFF": true, "CONCAT": true,
	"CONCAT_WS": true, "FORMAT": true, "ABS": true, "ROUND": true,
	"CEILING": true, "FLOOR": true, "POWER": true, "SQRT": true,
	"OBJECT_ID": true, "OBJECT_NAME": true, "SCHEMA_NAME": true, "DB_NAME": true,
	"USER_NAME": true, "SUSER_SNAME": true, "SCOPE_IDENTITY": true,
	"IDENT_CURRENT": true, "@@IDENTITY": true, "@@ROWCOUNT": true, "@@ERROR": true,
}

type Token struct {
	Type  TokenType
	Value string
}

// HighlightLine tokenizes and colors a single line of SQL code
func HighlightLine(line string) string {
	if len(line) == 0 {
		return ""
	}

	tokens := TokenizeLine(line)
	var b strings.Builder
	for _, tok := range tokens {
		switch tok.Type {
		case TokenKeyword:
			b.WriteString(StyleKeyword.Render(tok.Value))
		case TokenSpecialKeyword:
			b.WriteString(StyleSpecialKeyword.Render(tok.Value))
		case TokenTypeType:
			b.WriteString(StyleType.Render(tok.Value))
		case TokenFunction:
			b.WriteString(StyleFunction.Render(tok.Value))
		case TokenComment:
			b.WriteString(StyleComment.Render(tok.Value))
		case TokenString:
			b.WriteString(StyleString.Render(tok.Value))
		case TokenNumber:
			b.WriteString(StyleNumber.Render(tok.Value))
		case TokenIdentifier:
			b.WriteString(StyleIdentifier.Render(tok.Value))
		case TokenVariable:
			b.WriteString(StyleVariable.Render(tok.Value))
		case TokenOperator:
			b.WriteString(StyleOperator.Render(tok.Value))
		default:
			b.WriteString(StyleDefault.Render(tok.Value))
		}
	}
	return b.String()
}

// TokenizeLine parses a single line into semantic SQL tokens
func TokenizeLine(line string) []Token {
	var tokens []Token
	runes := []rune(line)
	n := len(runes)
	i := 0

	for i < n {
		r := runes[i]

		// 1. Whitespace
		if unicode.IsSpace(r) {
			start := i
			for i < n && unicode.IsSpace(runes[i]) {
				i++
			}
			tokens = append(tokens, Token{Type: TokenDefault, Value: string(runes[start:i])})
			continue
		}

		// 2. Single-line comment (-- ...)
		if r == '-' && i+1 < n && runes[i+1] == '-' {
			tokens = append(tokens, Token{Type: TokenComment, Value: string(runes[i:])})
			break
		}

		// 3. Multi-line comment on single line (/* ... */)
		if r == '/' && i+1 < n && runes[i+1] == '*' {
			start := i
			i += 2
			for i+1 < n && !(runes[i] == '*' && runes[i+1] == '/') {
				i++
			}
			if i+1 < n {
				i += 2
			} else {
				i = n
			}
			tokens = append(tokens, Token{Type: TokenComment, Value: string(runes[start:i])})
			continue
		}

		// 4. Bracketed Identifier ([schema].[table])
		if r == '[' {
			start := i
			i++
			for i < n && runes[i] != ']' {
				i++
			}
			if i < n && runes[i] == ']' {
				i++
			}
			tokens = append(tokens, Token{Type: TokenIdentifier, Value: string(runes[start:i])})
			continue
		}

		// 5. Quoted Identifier ("column" or `table`)
		if r == '"' || r == '`' {
			quote := r
			start := i
			i++
			for i < n && runes[i] != quote {
				i++
			}
			if i < n && runes[i] == quote {
				i++
			}
			tokens = append(tokens, Token{Type: TokenIdentifier, Value: string(runes[start:i])})
			continue
		}

		// 6. String Literal ('string' or N'unicode')
		if r == '\'' || ((r == 'N' || r == 'n') && i+1 < n && runes[i+1] == '\'') {
			start := i
			if r == 'N' || r == 'n' {
				i += 2
			} else {
				i++
			}
			for i < n {
				if runes[i] == '\'' {
					if i+1 < n && runes[i+1] == '\'' {
						i += 2 // Escaped quote
						continue
					}
					i++
					break
				}
				i++
			}
			tokens = append(tokens, Token{Type: TokenString, Value: string(runes[start:i])})
			continue
		}

		// 7. Local Variable (@param or @@variable)
		if r == '@' {
			start := i
			i++
			for i < n && (unicode.IsLetter(runes[i]) || unicode.IsDigit(runes[i]) || runes[i] == '_' || runes[i] == '@') {
				i++
			}
			val := string(runes[start:i])
			upper := strings.ToUpper(val)
			if functions[upper] {
				tokens = append(tokens, Token{Type: TokenFunction, Value: val})
			} else {
				tokens = append(tokens, Token{Type: TokenVariable, Value: val})
			}
			continue
		}

		// 8. Numbers (123, 45.67, 0x1A)
		if unicode.IsDigit(r) || (r == '.' && i+1 < n && unicode.IsDigit(runes[i+1])) {
			start := i
			isHex := false
			if r == '0' && i+1 < n && (runes[i+1] == 'x' || runes[i+1] == 'X') {
				isHex = true
				i += 2
			}
			for i < n {
				curr := runes[i]
				if isHex {
					if unicode.IsDigit(curr) || (curr >= 'a' && curr <= 'f') || (curr >= 'A' && curr <= 'F') {
						i++
						continue
					}
					break
				}
				if unicode.IsDigit(curr) || curr == '.' {
					i++
				} else {
					break
				}
			}
			tokens = append(tokens, Token{Type: TokenNumber, Value: string(runes[start:i])})
			continue
		}

		// 9. Words (Keywords, Data Types, Functions, Plain Identifiers)
		if unicode.IsLetter(r) || r == '_' || r == '#' {
			start := i
			for i < n && (unicode.IsLetter(runes[i]) || unicode.IsDigit(runes[i]) || runes[i] == '_' || runes[i] == '#' || runes[i] == '$') {
				i++
			}
			word := string(runes[start:i])
			upper := strings.ToUpper(word)

			if specialKeywords[upper] {
				tokens = append(tokens, Token{Type: TokenSpecialKeyword, Value: word})
			} else if keywords[upper] {
				tokens = append(tokens, Token{Type: TokenKeyword, Value: word})
			} else if dataTypes[upper] {
				tokens = append(tokens, Token{Type: TokenTypeType, Value: word})
			} else if functions[upper] {
				tokens = append(tokens, Token{Type: TokenFunction, Value: word})
			} else {
				tokens = append(tokens, Token{Type: TokenDefault, Value: word})
			}
			continue
		}

		// 10. Operators and Punctuations
		tokens = append(tokens, Token{Type: TokenOperator, Value: string(r)})
		i++
	}

	return tokens
}

// HighlightSQL formats and highlights an entire multi-line SQL query
func HighlightSQL(sql string) string {
	lines := strings.Split(sql, "\n")
	var highlighted []string
	for _, l := range lines {
		highlighted = append(highlighted, HighlightLine(l))
	}
	return strings.Join(highlighted, "\n")
}
