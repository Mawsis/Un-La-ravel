# Un(la)ravel - Laravel Project Analysis Tool

## Project Overview
Un(la)ravel is a comprehensive analysis tool designed to "unravel" Laravel projects by providing deep insights into their structure, architecture, performance, and quality. It helps developers understand complex Laravel codebases through automated analysis and visual representations.

## Core Philosophy
The tool aims to make Laravel projects more transparent and maintainable by providing automated discovery and documentation of:
- Project architecture and dependencies
- Performance bottlenecks and optimization opportunities
- Security patterns and potential vulnerabilities
- Code quality and technical debt

## Feature Categories

### 🗺️ Architecture & Structure Analysis
- **Route Analysis**: Complete route mapping with middleware chains, parameter validation, and auto-generated Swagger documentation
- **Database Schema Analysis**: ER diagram generation, migration history tracking, and relationship visualization
- **Model Relationships**: Interactive relationship mapping with cardinality and constraint analysis
- **Service Provider Mapping**: Container binding analysis and dependency injection visualization
- **Middleware Flow**: Request/response pipeline visualization with authentication/authorization flows

### 📊 Code Quality & Performance
- **Performance Analysis**: N+1 query detection, slow route identification, and database optimization suggestions
- **Code Quality Assessment**: Unused code detection, dead route identification, and technical debt analysis
- **Dependency Analysis**: Composer package analysis, version conflict detection, and security vulnerability scanning
- **Cache Usage Patterns**: Cache strategy analysis and optimization recommendations

### 🔐 Security Analysis
- **Authentication Flow Mapping**: Complete auth system visualization including guards, providers, and policies
- **Authorization Analysis**: Policy mapping, gate analysis, and permission gap detection
- **Security Vulnerability Scanning**: Known vulnerability detection in dependencies and code patterns
- **CORS & Rate Limiting**: Configuration analysis and security posture assessment

### 📋 Resource Mapping
- **Request/Resource Mapping**: API resource transformation analysis and validation rule documentation
- **Policy & Permission Mapping**: Authorization rule visualization and access control analysis  
- **Event/Listener Analysis**: Event system mapping and listener dependency analysis
- **Job Queue Analysis**: Background job mapping, worker configuration, and failure analysis

### 🛠️ Development Tools
- **Configuration Analysis**: Environment variable validation, config inconsistency detection
- **Artisan Command Discovery**: Custom command documentation and usage analysis
- **Migration Analysis**: Migration history, rollback safety analysis, and schema evolution tracking
- **Blade Template Analysis**: Template dependency mapping and component usage analysis

### 🚀 API & Integration
- **API Documentation**: Auto-generated comprehensive API docs with request/response examples
- **API Versioning**: Version detection and compatibility analysis
- **Third-party Integration**: External service dependency mapping and webhook analysis
- **Package Integration**: Laravel package usage analysis and compatibility assessment

### 🧪 Testing & Quality Assurance
- **Test Coverage Mapping**: Feature test coverage analysis and route testing gaps
- **Test Quality Analysis**: Test organization and effectiveness assessment
- **Environment Configuration**: Multi-environment setup validation and consistency checks

### 📈 Technology Overview
- **Stack Analysis**: Complete technology stack documentation and version analysis
- **Performance Metrics**: Application performance profiling and bottleneck identification
- **Deployment Analysis**: Server requirements, Docker configuration, and deployment pipeline analysis

## Technical Implementation

### Development Context
**Note**: This project serves as a Go learning experience. All code will follow Go best practices and idiomatic patterns, with detailed explanations of:
- Go project structure and organization patterns
- Concurrency patterns (goroutines, channels, sync primitives)
- Interface design and composition over inheritance
- Error handling patterns and conventions
- Testing strategies (unit tests, table-driven tests, benchmarks)
- Dependency injection and clean architecture principles

### Development Approach
**Learning-Focused Development Process**:
1. **Claude provides guidance**: Explains the next step, what to implement, and what to research
2. **Developer implements**: Writes code based on the guidance and research
3. **Claude reviews**: Provides feedback on code quality, Go idioms, and suggests improvements
4. **Claude writes code only when explicitly requested** - focus is on learning through doing

This approach ensures hands-on learning while maintaining code quality and Go best practices.

### Tech Stack: Go + SQLite/PostgreSQL

**Why Go:**
- **Fast Compilation**: Quick iteration during development
- **Excellent File I/O**: Built-in standard library optimized for file operations
- **Performance**: Fast execution for parsing and regex operations
- **Static Binaries**: Easy deployment across platforms
- **Goroutines**: Efficient concurrent file processing
- **Rich Ecosystem**: Mature libraries for parsing, databases, and CLI

**Key Dependencies:**
```go
// Core parsing and analysis
"github.com/VKCOM/php-parser"          // PHP AST parsing (static analysis, never boots Laravel — ADR 0003)
"regexp"                               // Built-in regex for pattern matching
"path/filepath"                        // File system operations
"encoding/json"                        // JSON parsing for configs

// Database — DEFERRED. The MVP has NO database (ADR 0007): the Project Model
// lives in memory during a run and serializes to JSON (unlaravel.json). The
// GORM/SQLite/PostgreSQL stack below is deferred to a future hosted web
// service and is NOT a dependency of the CLI.
// "gorm.io/gorm"                     // (deferred) ORM for data models
// "gorm.io/driver/sqlite"           // (deferred) SQLite driver
// "gorm.io/driver/postgres"         // (deferred) PostgreSQL driver

// CLI and utilities
"github.com/spf13/cobra"              // CLI framework
"github.com/spf13/viper"              // Configuration management
"github.com/fatih/color"              // Colored terminal output
"golang.org/x/sync/errgroup"          // Concurrent error handling
```

**Database Strategy (DEFERRED — see ADR 0007):**
The MVP intentionally has **no database**. The Project Model is built in memory
and serialized to JSON (`unlaravel.json`, the versioned public contract — ADR
0004). The store below is deferred to a possible future hosted web service:
- **SQLite** _(deferred)_: Local analysis cache, relationship storage
- **PostgreSQL** _(deferred)_: Multi-project analysis, team collaboration
- **GORM Models** _(deferred)_: Type-safe database operations with migrations

### Core Architecture
- **Static Analysis Engine**: PHP AST parsing for deep code analysis (never boots Laravel — ADR 0003)
- **Schema Analysis**: Tables/columns parsed statically from `database/migrations/*.php` AST (no live DB connection — ADR 0007)
- **Configuration Parsing**: Laravel config and environment analysis
- **Route Discovery**: Static route analysis (deferred — not in this slice)
- **Concurrent Processing**: Goroutines for parallel file analysis
- **In-Memory Model + JSON**: The Project Model lives in memory and serializes to `unlaravel.json` (no database — ADR 0007)

### Output Formats
- **Interactive Web Dashboard**: Real-time analysis results with filtering and search
- **Exportable Reports**: PDF, HTML, and JSON format exports
- **Visual Diagrams**: SVG/PNG exports for architecture diagrams
- **API Documentation**: OpenAPI/Swagger format generation

### Integration Options
- **CLI Tool**: Standalone command-line interface
- **Laravel Package**: Installable Laravel package for existing projects
- **Web Service**: Hosted analysis service for remote projects
- **IDE Extensions**: Integration with popular IDEs (VSCode, PHPStorm)

## Development Commands
```bash
# Run analysis on current Laravel project
unlaravel analyze

# Generate specific analysis reports
unlaravel routes --swagger
unlaravel database --diagram
unlaravel performance --optimize

# Export analysis results
unlaravel export --format=pdf --output=analysis.pdf

# Development commands
go build -o unlaravel ./cmd/unlaravel    # Build binary
go test ./...                            # Run all tests
go mod tidy                              # Clean up dependencies
go run ./cmd/unlaravel analyze           # Run during development

# Go learning resources integrated into development
# - Code comments explaining Go idioms and patterns
# - Example tests demonstrating Go testing conventions
# - Documentation of design decisions and Go best practices
```

## Use Cases
- **Legacy Code Understanding**: Rapidly understand inherited Laravel projects
- **Code Review Assistance**: Identify potential issues before deployment
- **Performance Optimization**: Find and fix performance bottlenecks
- **Documentation Generation**: Auto-generate comprehensive project documentation
- **Security Auditing**: Identify security vulnerabilities and misconfigurations
- **Refactoring Planning**: Understand dependencies before making changes

## Future Enhancements
- **AI-Powered Insights**: Machine learning for code pattern analysis and recommendations
- **Real-time Monitoring**: Live performance monitoring integration
- **Team Collaboration**: Multi-developer analysis sharing and collaboration features
- **Custom Rule Engine**: User-defined analysis rules and quality gates
- **Integration Ecosystem**: Plugins for CI/CD, monitoring tools, and project management systems

## Memories
- Awlays explain what you want to do and do it step by step asking me about the next step