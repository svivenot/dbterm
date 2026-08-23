package ai

import (
	"fmt"
	"strings"
)

// AIMode represents the user interaction mode
type AIMode int

const (
	AIModeGenerate AIMode = iota // Natural Language to SQL
	AIModeFixError               // Fix SQL Error
	AIModeExplain                // Explain Query
	AIModeOptimize               // Optimize Query
)

func (m AIMode) String() string {
	switch m {
	case AIModeGenerate:
		return "Text-to-SQL"
	case AIModeFixError:
		return "Fix Error"
	case AIModeExplain:
		return "Explain"
	case AIModeOptimize:
		return "Optimize"
	default:
		return "AI Assistant"
	}
}

// BuildSystemPrompt creates the system instruction containing the active database schema
func BuildSystemPrompt(dialect, schemaContext string) string {
	var b strings.Builder
	b.WriteString("You are dbterm AI, an expert Database Administrator and SQL engineer.\n")
	b.WriteString(fmt.Sprintf("Target SQL Dialect: %s.\n\n", dialect))
	b.WriteString("Rules:\n")
	b.WriteString("1. You MUST ONLY use the tables, columns, and relationships defined in the Database Schema below.\n")
	b.WriteString("2. Generate valid, efficient, modern SQL queries matching the target dialect syntax.\n")
	b.WriteString("3. Format your primary SQL query inside a single ```sql ... ``` markdown code block.\n")
	b.WriteString("4. Keep explanations concise (1-2 sentences) in the same language as the user's prompt (French or English).\n")
	b.WriteString("5. Never hallucinate non-existent table or column names.\n\n")

	b.WriteString("=== ACTIVE DATABASE SCHEMA ===\n")
	if strings.TrimSpace(schemaContext) != "" {
		b.WriteString(schemaContext)
	} else {
		b.WriteString("-- No schema definitions available.\n")
	}
	b.WriteString("==============================\n")

	return b.String()
}

// BuildUserPrompt prepares the prompt payload depending on the selected mode
func BuildUserPrompt(mode AIMode, userPrompt, existingSQL, errorMsg string) string {
	var b strings.Builder

	switch mode {
	case AIModeGenerate:
		b.WriteString(fmt.Sprintf("User Request: %s\n\n", userPrompt))
		if strings.TrimSpace(existingSQL) != "" {
			b.WriteString("Current editor query context:\n```sql\n" + existingSQL + "\n```\n\n")
		}
		b.WriteString("Please generate the SQL query to satisfy this request.")

	case AIModeFixError:
		b.WriteString("The following SQL query produced a database execution error.\n\n")
		b.WriteString("Failing Query:\n```sql\n" + existingSQL + "\n```\n\n")
		b.WriteString(fmt.Sprintf("Database Error Message:\n%s\n\n", errorMsg))
		if strings.TrimSpace(userPrompt) != "" {
			b.WriteString(fmt.Sprintf("Additional User Note: %s\n\n", userPrompt))
		}
		b.WriteString("Please fix the query to resolve the error while respecting the active database schema.")

	case AIModeExplain:
		b.WriteString("Please explain what the following SQL query does in clear, concise terms:\n\n")
		b.WriteString("```sql\n" + existingSQL + "\n```\n")

	case AIModeOptimize:
		b.WriteString("Please review the following SQL query and suggest an optimized, faster version:\n\n")
		b.WriteString("```sql\n" + existingSQL + "\n```\n")
		if strings.TrimSpace(userPrompt) != "" {
			b.WriteString(fmt.Sprintf("Optimization Goal: %s\n", userPrompt))
		}
	}

	return b.String()
}

// ExtractSQLAndExplanation separates the generated SQL code from the textual explanation
func ExtractSQLAndExplanation(response string) (string, string) {
	trimmed := strings.TrimSpace(response)
	if trimmed == "" {
		return "", ""
	}

	// 1. Look for ```sql ... ``` block
	sqlStart := strings.Index(trimmed, "```sql")
	if sqlStart == -1 {
		sqlStart = strings.Index(trimmed, "```SQL")
	}
	if sqlStart == -1 {
		sqlStart = strings.Index(trimmed, "```")
	}

	if sqlStart != -1 {
		codeStart := strings.Index(trimmed[sqlStart:], "\n")
		if codeStart != -1 {
			codeStart = sqlStart + codeStart + 1
			codeEnd := strings.Index(trimmed[codeStart:], "```")
			if codeEnd != -1 {
				sqlCode := strings.TrimSpace(trimmed[codeStart : codeStart+codeEnd])
				beforeCode := strings.TrimSpace(trimmed[:sqlStart])
				afterCode := strings.TrimSpace(trimmed[codeStart+codeEnd+3:])

				explanation := ""
				if beforeCode != "" {
					explanation = beforeCode
				}
				if afterCode != "" {
					if explanation != "" {
						explanation += "\n\n"
					}
					explanation += afterCode
				}
				return sqlCode, explanation
			}
		}
	}

	// Fallback: If output starts with standard SQL keywords, treat entire output as SQL
	upper := strings.ToUpper(trimmed)
	if strings.HasPrefix(upper, "SELECT ") || strings.HasPrefix(upper, "WITH ") ||
		strings.HasPrefix(upper, "INSERT ") || strings.HasPrefix(upper, "UPDATE ") ||
		strings.HasPrefix(upper, "DELETE ") || strings.HasPrefix(upper, "CREATE ") {
		return trimmed, ""
	}

	return "", trimmed
}
