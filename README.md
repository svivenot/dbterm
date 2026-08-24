# dbterm ⚡

[![Go Version](https://img.shields.io/github/go-mod/go-version/svivenot/dbterm)](https://golang.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Built with BubbleTea](https://img.shields.io/badge/Built%20with-BubbleTea-00ADD8.svg)](https://github.com/charmbracelet/bubbletea)

**dbterm** is a modern, fast, and ergonomic SQL client available both as a **Terminal User Interface (TUI)** (powered by BubbleTea) and a **Native Desktop GUI** (powered by Fyne) for querying and managing **Microsoft SQL Server**, **PostgreSQL**, and **Oracle Database**, designed with the productivity of **SQL Server Management Studio (SSMS)** and powered by an offline, schema-aware AI SQL generator.

```text
+---------------------------------------------------------------------------------------------------+
| dbterm  localhost:1433 | User: sa (SQL)   DB: SalesDB           [Ctrl+O: Connect] [Ctrl+H] [F1/?] |
+-----------------------+---------------------------------------------------------------------------+
| 🗄 DBs [1] 📁 Files [2]| Query-1.sql*  |  Query-2.sql                                              |
| ▼ 📁 Local / Docker   +---------------------------------------------------------------------------+
|   ▼ 🖥 [MSSQL] SalesDB 🟢 SELECT                                                                  |
|     ▼ 🗄 Databases     |     c.CustomerID, c.CompanyName, COUNT(o.OrderID) AS TotalOrders          |
|       ▼ 📂 SalesDB    | FROM sales.Customers c                                                    |
|         ▼ 📂 Tables   | LEFT JOIN sales.Orders o ON c.CustomerID = o.CustomerID                   |
|           📋 Customers| GROUP BY c.CustomerID, c.CompanyName;                                     |
|             🔑 CustID |                                                                           |
|   ▶ 🖥 [PG] Postgres ⚪+---------------------------------------------------------------------------+
|   ▶ 🖥 [ORA] Oracle ⚪| [ Results (8 rows) ]  Messages                                            |
| ▼ 📁 Production       +---------------------------------------------------------------------------+
|   ▶ 🖥 [MSSQL] Corp ⚪| #  | CustomerID | CompanyName              | TotalOrders                  |
|                       |----+------------+--------------------------+------------------------------|
|                       | 1  | CUST001    | Airbus Group SAS         | 1                            |
|                       | 2  | CUST002    | Siemens AG               | 1                            |
| +---------------------+---------------------------------------------------------------------------+
| Ready | Ln 1, Col 1   [F5: Run | Tab: Switch Pane | Ctrl+N: Tab | Ctrl+H: History | Ctrl+S: Export]   |
+---------------------------------------------------------------------------------------------------+
```

---

## ✨ Features

- **Dual Interface Modes**:
  - **TUI Mode (`dbterm`)**: Ultra-fast terminal client with single-line guarantee and mouse support.
  - **Native Desktop GUI Mode (`dbterm-gui`)**: Full-featured graphical window built with **Fyne v2** featuring resizable panels, grid inspector, and visual dialogs.
- **Multi-Database Support**: Native drivers for **MS SQL Server** (`go-mssqldb`), **PostgreSQL** (`pgx`), and **Oracle Database** (`sijms/go-ora`).
- **SSMS-Inspired Object Explorer (`F8`)**:
  - **Hierarchical Server Folders**: Classify servers into logical environments (`📁 Local / Docker`, `📁 Production / Enterprise`).
  - **Click to Connect**: Connect to any server dynamically and expand its database, table, view, stored procedure, and column catalogs.
  - **2-Tab Explorer**: Switch with **`1`** and **`2`** between Database catalogs and local **SQL script files** (`📁 Files [2]`).
  - **Right-Click Context Menu (`m`)**: One-click scripts for `SELECT TOP 100 *`, `CREATE DDL`, `INSERT Template`, and `Copy Name`.
- **Multi-Tab SQL Editor**:
  - Multiline query editing with syntax highlighting.
  - **Save to File (`Ctrl+S`)**: Save scripts directly to disk or prompt for path.
  - Selection execution (`F5` / `Ctrl+E` executes selected SQL text or full buffer).
  - Visual selection mode (`F2` / `Shift+Arrows`).
- **Single-Line Guarantee Results Grid**:
  - Data rows never wrap or disrupt layout.
  - **Horizontal Column Scrolling** (`Left`/`Right`, `h`/`l`) with sticky row numbering.
  - Cell value inspector (`Enter` / `v`) for multi-line text, JSON, and XML payloads.
  - Column sorting (`o`) and live in-memory row filtering (`/`).
- **Secure Credential Vault**:
  - **Native OS Keychain**: Store passwords securely in macOS Keychain, Windows Credential Manager, or Linux Secret Service via `go-keyring`.
  - **AES-256-GCM In-File Encryption**: Machine-keyed encryption for headless server environments.
  - **Unix `pass` & Environment Variables**: Native support for enterprise credential management.
  - Interactive Connection Manager (`Ctrl+O`) with full **Add (`a`)**, **Edit (`e`)**, and **Delete (`d`)** support.
- **Embedded Schema-Aware AI SQL Generator (`Ctrl+K` / `F4`)**:
  - Zero-setup offline text-to-SQL generation with **Defog SQLCoder-7B-2 (Q4)** & **Qwen 2.5 Coder 3B/7B**.
  - Dynamic schema tokenizer (no accents, snake_case/camelCase extraction) and auto foreign key join resolution.
- **Multi-Format Export Dialog (`e` on results / Ctrl+E)**:
  - Export query results directly to **Excel (.xlsx)**, **CSV**, **JSON**, **Markdown**, **HTML**, or **Plain Text**.
- **Full Mouse Navigation**: Focus panes by clicking, resize sidebars by dragging, double-click to connect servers, and right-click for context menus.

---

## 🚀 Quick Start

### Installation & Build

```bash
# Clone the repository
git clone https://github.com/svivenot/dbterm.git
cd dbterm

# 1. Build the Terminal UI (TUI)
go build -o bin/dbterm ./cmd/dbterm

# 2. Build the Graphical Desktop Application (Fyne GUI)
go build -o bin/dbterm-gui ./cmd/dbterm-gui

# Run TUI
./bin/dbterm

# Run Desktop GUI
./bin/dbterm-gui
```

### Starting the Local Test Database (MS SQL Server)

```bash
# Launch Docker container
docker-compose up -d

# Seed sample database (SalesDB with customers, orders, views, procs)
go run ./cmd/seed/main.go
```

---

## ⌨️ Keyboard Shortcuts

| Shortcut | Description |
| :--- | :--- |
| **`F5`** / **`Ctrl + E`** | Execute query or selected text (*SSMS standard*) |
| **`Ctrl + K`** / **`F4`** | Open Embedded AI SQL Assistant (*Offline Text-to-SQL*) |
| **`F8`** | Toggle Object Explorer sidebar visibility |
| **`Tab`** / **`Shift + Tab`** | Cycle pane focus (Explorer $\rightarrow$ Editor $\rightarrow$ Results) |
| **`1`** / **`2`** *(Explorer)* | Switch between **[1] Databases** and **[2] SQL Files** |
| **`Ctrl + S`** *(Editor)* | Save query to SQL file (or open Save As prompt) |
| **`Ctrl + N`** / **`Ctrl + W`** | New SQL Editor tab / Close active tab |
| **`Ctrl + O`** | Open Connection Manager (Add `a`, Edit `e`, Delete `d`, Test `t`) |
| **`Ctrl + H`** | Open Query Execution History |
| **`e`** *(Results)* | Open Export dialog (*Excel XLSX, CSV, JSON, Markdown, HTML*) |
| **`Enter`** / **`v`** *(Results)* | Inspect full cell value (*JSON / XML / Long text*) |
| **`o`** *(Results)* | Toggle column sort ascending / descending |
| **`/`** | Live search & filter (Explorer, Results, Connections) |
| **`m`** / *Right-Click* | Open Object Explorer context menu |
| **`?`** / **`F1`** | Open Help modal |
| **`Ctrl + Q`** | Exit dbterm |

---

## 🔒 Configuration & Security

dbterm stores connection profiles in `connections.json` (or `~/.config/dbterm/connections.json`) with strict `0600` permissions.

Copy the example template to get started:
```bash
cp connections.example.json connections.json
```

### Password Security Options:
1. **OS Keychain (`auth_type: "keyring"`)**: Passwords stored natively in OS Keychain; excluded from `connections.json`.
2. **AES-256-GCM (`auth_type: "sql"`)**: Passwords encrypted with AES-256-GCM (`enc:v1:...`).
3. **Unix `pass` (`auth_type: "pass"`)**: Passwords retrieved on-demand from Unix pass store.
4. **Environment Variable (`auth_type: "env"`)**: Passwords read from specified environment variables.

---

## 📄 License

MIT License - Copyright (c) 2026 Sylvain Vivenot.
